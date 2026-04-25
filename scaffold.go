package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// scaffoldParamEdit is the editable state for one `@param` being authored
// in the scaffold form. Users pick a type, mark required, optionally set a
// default, write a description — the form turns this into a well-formed
// annotation line, so nobody has to learn the `@param` syntax to use it.
type scaffoldParamEdit struct {
	Name        string
	Kind        string // "string" | "int" | "bool" | "enum"
	Options     string // raw input for enum, comma or pipe separated
	Required    bool
	Default     string
	Description string
}

var scaffoldKinds = []string{"string", "int", "bool", "enum"}

// visibleFields returns the subfield names exposed in the form for this
// edit, in tab order. The set varies: no `default` when required is set,
// no `options` unless kind is enum. "name" is always first so auto-
// detected names can be edited and manually-added rows can be named.
func (p scaffoldParamEdit) visibleFields() []string {
	fields := []string{"name", "type", "required"}
	if !p.Required {
		fields = append(fields, "default")
	}
	if p.Kind == "enum" {
		fields = append(fields, "options")
	}
	fields = append(fields, "desc")
	return fields
}

// formatParamLine turns an edit into its canonical `# @param ...` string.
// Enum with fewer than two options falls back to `{string}` so the line
// always parses cleanly.
func formatParamLine(p scaffoldParamEdit) string {
	typeSpec := p.Kind
	if p.Kind == "enum" {
		opts := splitOptions(p.Options)
		if len(opts) >= 2 {
			typeSpec = strings.Join(opts, "|")
		} else {
			typeSpec = "string"
		}
	}
	var nameSpec string
	if p.Required {
		nameSpec = p.Name
	} else {
		nameSpec = "[" + p.Name + "=" + p.Default + "]"
	}
	line := fmt.Sprintf("# @param {%s} %s", typeSpec, nameSpec)
	if p.Description != "" {
		line += "  " + p.Description
	}
	return line
}

// splitOptions parses an enum options string (commas or pipes accepted)
// into trimmed non-empty entries.
func splitOptions(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '|'
	}) {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// scaffoldState drives the target-metadata authoring form. It lets the
// user edit the target's description, edit existing @param definitions,
// and add new ones — all without learning the `@param` syntax. focus=-1
// addresses the description row; focus>=0 indexes into edits.
type scaffoldState struct {
	target      MakeTarget
	description string   // editable; starts from target.Description
	fileLevel   []string // read-only display of file-level @param names
	edits       []scaffoldParamEdit
	focus       int    // -1 = description row, 0..N-1 = edits[focus]
	field       string // for description "text"; for edits, one of visibleFields()
	errMsg      string // transient feedback above the legend (write errors / no-op)
	errMsgSeq   int
}

func (m model) newScaffoldState(target MakeTarget) *scaffoldState {
	// Pre-load rows for every existing target-level @param so editing works.
	edits := make([]scaffoldParamEdit, 0, len(target.Params))
	for _, p := range target.Params {
		edits = append(edits, scaffoldParamEdit{
			Name:        p.Name,
			Kind:        p.Kind,
			Options:     strings.Join(p.Options, ", "),
			Required:    p.Required,
			Default:     p.Default,
			Description: p.Description,
		})
	}
	// Add rows for new candidates the recipe references but nothing documents.
	for _, name := range m.scaffoldCandidates(target) {
		edits = append(edits, scaffoldParamEdit{Name: name, Kind: "string"})
	}

	var fileLevel []string
	for _, p := range m.fileParamsFor(target.Dir) {
		fileLevel = append(fileLevel, p.Name)
	}

	s := &scaffoldState{
		target:      target,
		description: target.Description,
		fileLevel:   fileLevel,
		edits:       edits,
		focus:       -1,
		field:       "text",
	}
	return s
}

// advance moves the active (focus, field) pair forward (delta=+1) or
// backward (delta=-1) through the flat tab-order of visible fields, with
// wraparound at both ends. The description row (-1, "text") is always
// first in the order.
func (s *scaffoldState) advance(delta int) {
	type fp struct {
		idx   int
		field string
	}
	pairs := []fp{{-1, "text"}}
	for i, e := range s.edits {
		for _, f := range e.visibleFields() {
			pairs = append(pairs, fp{i, f})
		}
	}
	cur := 0
	for i, p := range pairs {
		if p.idx == s.focus && p.field == s.field {
			cur = i
			break
		}
	}
	cur = (cur + delta + len(pairs)) % len(pairs)
	s.focus = pairs[cur].idx
	s.field = pairs[cur].field
}

// snapField ensures s.field still names a visible field on s.edits[s.focus];
// if a toggle hid the current field, fall back to the first visible one.
// No-op when focused on the description row.
func (s *scaffoldState) snapField() {
	if s.focus < 0 || s.focus >= len(s.edits) {
		return
	}
	visible := s.edits[s.focus].visibleFields()
	for _, f := range visible {
		if f == s.field {
			return
		}
	}
	s.field = visible[0]
}

func (m model) updateScaffold(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.scaffold
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.scaffold = nil
		return m, nil
	case "ctrl+g":
		m.showHelp = true
		return m, nil
	case "ctrl+n":
		// Add a new empty param row, focus its name field.
		s.edits = append(s.edits, scaffoldParamEdit{Kind: "string"})
		s.focus = len(s.edits) - 1
		s.field = "name"
		return m, nil
	case "ctrl+d":
		// Delete the focused param row (not the description).
		if s.focus >= 0 && s.focus < len(s.edits) {
			s.edits = append(s.edits[:s.focus], s.edits[s.focus+1:]...)
			if s.focus >= len(s.edits) {
				s.focus = len(s.edits) - 1
			}
			if s.focus < 0 {
				s.focus = -1
				s.field = "text"
			} else {
				s.field = s.edits[s.focus].visibleFields()[0]
			}
		}
		return m, nil
	case "enter":
		return m.commitScaffold(false)
	case "ctrl+e":
		return m.commitScaffold(true)
	case "tab", "down":
		s.advance(1)
		return m, nil
	case "shift+tab", "up":
		s.advance(-1)
		return m, nil
	}

	// Description row is its own single text field.
	if s.focus == -1 {
		switch key {
		case "backspace":
			if len(s.description) > 0 {
				s.description = s.description[:len(s.description)-1]
			}
		case "ctrl+u":
			s.description = ""
		default:
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				s.description += key
			}
		}
		return m, nil
	}

	if s.focus < 0 || s.focus >= len(s.edits) {
		return m, nil
	}

	e := &s.edits[s.focus]
	switch s.field {
	case "type":
		switch key {
		case "left", "h":
			e.Kind = cycleStr(scaffoldKinds, e.Kind, -1)
			s.snapField()
		case "right", "l", " ":
			e.Kind = cycleStr(scaffoldKinds, e.Kind, 1)
			s.snapField()
		}
	case "required":
		switch key {
		case "left", "right", " ":
			e.Required = !e.Required
			s.snapField()
		}
	default: // "name" | "default" | "options" | "desc" are text inputs
		text := fieldText(e, s.field)
		switch key {
		case "backspace":
			if len(text) > 0 {
				setFieldText(e, s.field, text[:len(text)-1])
			}
		case "ctrl+u":
			setFieldText(e, s.field, "")
		default:
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				setFieldText(e, s.field, text+key)
			}
		}
	}
	return m, nil
}

func cycleStr(options []string, current string, delta int) string {
	idx := 0
	for i, o := range options {
		if o == current {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(options)) % len(options)
	return options[idx]
}

func fieldText(e *scaffoldParamEdit, field string) string {
	switch field {
	case "name":
		return e.Name
	case "default":
		return e.Default
	case "options":
		return e.Options
	case "desc":
		return e.Description
	}
	return ""
}

func setFieldText(e *scaffoldParamEdit, field, value string) {
	switch field {
	case "name":
		e.Name = value
	case "default":
		e.Default = value
	case "options":
		e.Options = value
	case "desc":
		e.Description = value
	}
}

// commitScaffold rewrites the target's comment block: description line(s)
// plus @param lines, preserving any adjacent .PHONY marker. Unnamed rows
// are skipped. Refuses to write a no-op (no description + no named
// params) when there's an existing block too — that would be a delete,
// which the user should opt into explicitly via ^d on rows.
func (m model) commitScaffold(openEditor bool) (tea.Model, tea.Cmd) {
	s := m.scaffold
	committable := committableEdits(s.edits)
	paramLines := make([]string, 0, len(committable))
	for _, e := range committable {
		paramLines = append(paramLines, formatParamLine(e))
	}
	var descLines []string
	if strings.TrimSpace(s.description) != "" {
		descLines = []string{"# " + strings.TrimSpace(s.description)}
	}
	if len(descLines) == 0 && len(paramLines) == 0 {
		// Nothing committable. If there's no existing block either, this
		// is a true no-op — surface that instead of silently closing.
		if !hasOldBlock(s.target) {
			return m.toastScaffoldErr("nothing to write — name at least one param row or set the description")
		}
		// Falls through: existing block + empty new = explicit delete.
	}
	newLine, err := rewriteTargetBlock(s.target, descLines, paramLines)
	if err != nil {
		return m.toastScaffoldErr("write failed: " + err.Error())
	}
	path := s.target.File
	m.scaffold = nil
	if openEditor {
		return m, editCmd(path, newLine)
	}
	return m, nil
}

// toastScaffoldErr sets a transient error message on the scaffold page
// and schedules its clearing.
func (m model) toastScaffoldErr(text string) (tea.Model, tea.Cmd) {
	m.scaffold.errMsg = text
	m.scaffold.errMsgSeq++
	return m, clearToastCmd(toastScafErr, m.scaffold.errMsgSeq)
}

// rewriteTargetBlock replaces the comment block immediately above `target`
// with the concatenation of descLines + paramLines. An adjacent .PHONY
// line is preserved in place below the rewritten block. Returns the
// target's new 1-based line number.
//
// "Comment block" = the run of lines directly above the target (or above
// its .PHONY, if present) that are either blank-free # comments or empty.
// We stop walking up at the first non-comment line.
func rewriteTargetBlock(target MakeTarget, descLines, paramLines []string) (int, error) {
	data, err := os.ReadFile(target.File)
	if err != nil {
		return target.Line, err
	}
	content := string(data)
	trailingNewline := strings.HasSuffix(content, "\n")
	if trailingNewline {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")

	// 0-based index of the target line.
	targetIdx := target.Line - 1
	if targetIdx < 0 {
		targetIdx = 0
	}

	// Pick off an adjacent .PHONY line to preserve.
	blockEnd := targetIdx
	phonyLine := ""
	if blockEnd > 0 && strings.HasPrefix(strings.TrimSpace(lines[blockEnd-1]), ".PHONY") {
		phonyLine = lines[blockEnd-1]
		blockEnd--
	}

	// Walk up over contiguous # comment lines — the existing block.
	blockStart := blockEnd
	for blockStart > 0 {
		trimmed := strings.TrimSpace(lines[blockStart-1])
		if !strings.HasPrefix(trimmed, "#") {
			break
		}
		blockStart--
	}

	// Build the replacement block.
	var newBlock []string
	newBlock = append(newBlock, descLines...)
	newBlock = append(newBlock, paramLines...)
	if phonyLine != "" {
		newBlock = append(newBlock, phonyLine)
	}

	out := make([]string, 0, len(lines)-(targetIdx-blockStart)+len(newBlock))
	out = append(out, lines[:blockStart]...)
	out = append(out, newBlock...)
	out = append(out, lines[targetIdx:]...)

	joined := strings.Join(out, "\n")
	if trailingNewline {
		joined += "\n"
	}
	if err := os.WriteFile(target.File, []byte(joined), 0o644); err != nil {
		return target.Line, err
	}
	return blockStart + len(newBlock) + 1, nil // 1-based line of the target
}

// --- Rendering ---------------------------------------------------------

func (m model) renderScaffoldView(w, h int) string {
	top := m.renderScaffoldTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))
	cmd := m.renderScaffoldCmdLine(w)
	legend := m.renderScaffoldLegend(w)
	body := m.renderScaffoldBody(w, h-5)
	return strings.Join([]string{top, rule, body, rule, cmd, legend}, "\n")
}

func (m model) renderScaffoldTopLine(w int) string {
	left := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ") + filterStyle.Render("scaffold @param")
	right := helpKeyStyle.Render("target: ") + previewNameStyle.Render(m.scaffold.target.Name)
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := w - leftW - rightW - 2
	if pad < 0 {
		pad = 0
	}
	return left + strings.Repeat(" ", pad) + right + "  "
}

func (m model) renderScaffoldLegend(w int) string {
	return renderLegend(w, []legendItem{
		{Key: "↑↓/tab", Hint: "field"},
		{Key: "←→/space", Hint: "cycle/toggle"},
		{Key: "^n", Hint: "add"},
		{Key: "^d", Hint: "delete"},
		{Key: "enter", Hint: "write"},
		{Key: "^e", Hint: "write+edit"},
		{Key: "esc", Hint: "cancel"},
	})
}

// renderScaffoldCmdLine describes the file mutation enter would commit:
// the target file and a summary of the change (N @param lines + optional
// description). When a transient error is set, that takes precedence so
// the user can see why their last enter didn't apply.
func (m model) renderScaffoldCmdLine(w int) string {
	s := m.scaffold
	if s.errMsg != "" {
		pill := renderActionPill("!", "fail", redColor)
		return renderActionLine(w, pill, diffRemoveStyle.Render(s.errMsg))
	}
	committable := committableEdits(s.edits)
	descKept := strings.TrimSpace(s.description) != ""
	parts := []string{}
	if descKept {
		parts = append(parts, "description")
	}
	if n := len(committable); n > 0 {
		if n == 1 {
			parts = append(parts, "1 @param")
		} else {
			parts = append(parts, fmt.Sprintf("%d @params", n))
		}
	}
	summary := normalItemStyle.Render("no changes pending")
	if len(parts) > 0 {
		summary = depsLabelStyle.Render(strings.Join(parts, " + "))
	}
	body := summary + normalItemStyle.Render(" → ") + selectedItemStyle.Render(s.target.File)
	pill := renderActionPill("⏎", "write", greenColor)
	return renderActionLine(w, pill, body)
}

func (m model) renderScaffoldBody(w, h int) string {
	s := m.scaffold
	var lines []string

	lines = append(lines, "")
	lines = append(lines, "  "+previewNameStyle.Render("Editing target metadata — description, existing @params, and new ones."))
	if len(s.fileLevel) > 0 {
		lines = append(lines, "  "+helpKeyStyle.Render("File-level @params available to this recipe: "+strings.Join(s.fileLevel, ", ")))
	}
	lines = append(lines, "")

	// Description row (focus == -1).
	lines = append(lines, m.renderScaffoldDescriptionRow(s.focus == -1, w)...)
	lines = append(lines, "")

	if len(s.edits) == 0 {
		lines = append(lines, "  "+noMatchStyle.Render("No @params yet. Press ")+helpKeyStyle.Render("ctrl+n")+noMatchStyle.Render(" to add one."))
		lines = append(lines, "")
	} else {
		for i, e := range s.edits {
			focused := i == s.focus
			lines = append(lines, m.renderScaffoldParamBlock(e, focused, s.field, w)...)
			lines = append(lines, "")
		}
	}

	// Preview of the commit as a GitHub-style diff against the live file.
	committable := committableEdits(s.edits)
	var descPreview []string
	if strings.TrimSpace(s.description) != "" {
		descPreview = []string{"# " + strings.TrimSpace(s.description)}
	}
	var paramPreview []string
	for _, e := range committable {
		paramPreview = append(paramPreview, formatParamLine(e))
	}
	if len(paramPreview) > 0 || len(descPreview) > 0 || hasOldBlock(s.target) {
		lines = append(lines, "  "+previewNameStyle.Render(fmt.Sprintf("Diff against %s:", s.target.File)))
		lines = append(lines, "")
		diff := buildBlockDiff(s.target, descPreview, paramPreview, 2, 2)
		lines = append(lines, renderDiffLines(diff, w)...)
	} else if len(s.edits) > 0 {
		lines = append(lines, "  "+noMatchStyle.Render("Diff: name a row or set a description to see what will change."))
	}

	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lw := lipgloss.Width(line)
		if lw < w {
			lines[i] = line + strings.Repeat(" ", w-lw)
		}
	}
	return strings.Join(lines, "\n")
}

// committableEdits filters out rows with an empty name — those are
// in-progress entries, not yet writable.
func committableEdits(edits []scaffoldParamEdit) []scaffoldParamEdit {
	out := make([]scaffoldParamEdit, 0, len(edits))
	for _, e := range edits {
		if strings.TrimSpace(e.Name) != "" {
			out = append(out, e)
		}
	}
	return out
}

// hasOldBlock reports whether the target currently has a comment block
// above it. Used to show the diff even when there are no pending edits,
// so the user sees the delete-only case explicitly.
func hasOldBlock(target MakeTarget) bool {
	data, err := os.ReadFile(target.File)
	if err != nil {
		return false
	}
	content := strings.TrimRight(string(data), "\n")
	lines := strings.Split(content, "\n")
	idx := target.Line - 1
	if idx <= 0 || idx >= len(lines) {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(lines[idx-1]), ".PHONY") {
		idx--
	}
	if idx <= 0 {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(lines[idx-1]), "#")
}

// --- Diff preview -------------------------------------------------------

type diffLine struct {
	kind    rune // ' ' context, '-' removed, '+' added
	lineNum int  // which line number to show in the gutter
	text    string
}

// buildBlockDiff renders the scaffold commit as a GitHub-style diff:
// context lines before the edit, then an LCS-aligned delta over the old
// comment block vs the new descLines+paramLines, then context lines after
// (including any .PHONY and the target itself). Returns [] on read failure.
func buildBlockDiff(target MakeTarget, descLines, paramLines []string, contextBefore, contextAfter int) []diffLine {
	data, err := os.ReadFile(target.File)
	if err != nil {
		return nil
	}
	content := string(data)
	if strings.HasSuffix(content, "\n") {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")
	if target.Line-1 < 0 || target.Line-1 >= len(lines) {
		return nil
	}
	targetIdx := target.Line - 1

	// Find the comment block boundaries (same rules as rewriteTargetBlock).
	blockEnd := targetIdx
	if blockEnd > 0 && strings.HasPrefix(strings.TrimSpace(lines[blockEnd-1]), ".PHONY") {
		blockEnd--
	}
	blockStart := blockEnd
	for blockStart > 0 {
		trimmed := strings.TrimSpace(lines[blockStart-1])
		if !strings.HasPrefix(trimmed, "#") {
			break
		}
		blockStart--
	}

	oldBlock := lines[blockStart:blockEnd]
	newBlock := append(append([]string{}, descLines...), paramLines...)

	var result []diffLine

	// Context before the edit.
	ctxStart := blockStart - contextBefore
	if ctxStart < 0 {
		ctxStart = 0
	}
	for i := ctxStart; i < blockStart; i++ {
		result = append(result, diffLine{' ', i + 1, lines[i]})
	}

	// LCS-aligned diff of old block vs new block.
	result = append(result, lcsDiff(oldBlock, newBlock, blockStart+1, blockStart+1)...)

	// Context after: from blockEnd (preserved .PHONY if any, then target),
	// plus contextAfter additional lines. Line numbers reflect the
	// post-commit state so the target's new position reads cleanly.
	postOldStart := blockEnd
	postNewStart := blockStart + len(newBlock)
	postEnd := targetIdx + 1 + contextAfter
	if postEnd > len(lines) {
		postEnd = len(lines)
	}
	for i := postOldStart; i < postEnd; i++ {
		result = append(result, diffLine{' ', postNewStart + (i - postOldStart) + 1, lines[i]})
	}
	return result
}

// lcsDiff produces a line-level diff using the longest-common-subsequence
// algorithm, so unchanged lines show as context instead of churn. oldStart
// and newStart are the 1-based line numbers of the first line of each
// block in the source/new file respectively.
func lcsDiff(oldLines, newLines []string, oldStart, newStart int) []diffLine {
	m, n := len(oldLines), len(newLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var rev []diffLine
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			rev = append(rev, diffLine{' ', newStart + j - 1, newLines[j-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			rev = append(rev, diffLine{'+', newStart + j - 1, newLines[j-1]})
			j--
		default:
			rev = append(rev, diffLine{'-', oldStart + i - 1, oldLines[i-1]})
			i--
		}
	}
	for l, r := 0, len(rev)-1; l < r; l, r = l+1, r-1 {
		rev[l], rev[r] = rev[r], rev[l]
	}
	return rev
}

// renderDiffLines turns diff rows into styled, width-capped strings. The
// gutter shows the relevant line number (old for removed, new for added
// or context). `w` is the total cell width the rows should fit within.
func renderDiffLines(diff []diffLine, w int) []string {
	if len(diff) == 0 {
		return nil
	}
	maxLN := 0
	for _, d := range diff {
		if d.lineNum > maxLN {
			maxLN = d.lineNum
		}
	}
	gutter := len(strconv.Itoa(maxLN))
	if gutter < 3 {
		gutter = 3
	}
	// "  NN │ + text" — prefix(1) + space(1) + num(gutter) + " │ "(3) + sign(1) + " "(1) + text
	textW := w - 2 - gutter - 3 - 1 - 1
	if textW < 4 {
		textW = 4
	}

	out := make([]string, 0, len(diff))
	for _, d := range diff {
		numStr := padLeft(strconv.Itoa(d.lineNum), gutter)
		text := truncateStr(d.text, textW)
		sep := diffGutterStyle.Render(" │ ")

		var sign, lineText, gutterText string
		switch d.kind {
		case '+':
			sign = diffAddStyle.Render("+")
			lineText = diffAddStyle.Render(text)
			gutterText = diffAddStyle.Render(numStr)
		case '-':
			sign = diffRemoveStyle.Render("-")
			lineText = diffRemoveStyle.Render(text)
			gutterText = diffRemoveStyle.Render(numStr)
		default:
			sign = diffContextStyle.Render(" ")
			lineText = diffContextStyle.Render(text)
			gutterText = diffContextStyle.Render(numStr)
		}
		out = append(out, "  "+gutterText+sep+sign+" "+lineText)
	}
	return out
}

// renderScaffoldDescriptionRow returns the header + input row for the
// target's description (focus=-1 in the scaffold state).
func (m model) renderScaffoldDescriptionRow(focused bool, w int) []string {
	var cursor string
	if focused {
		cursor = selectedCursorStyle.Render(" › ")
	} else {
		cursor = "   "
	}
	header := cursor + depsLabelStyle.Render("Description") + "   " + noMatchStyle.Render("(shown in the target picker)")
	indent := "      "
	label := depsValueStyle.Render(padRight("text:", 10))
	textW := w - len(indent) - 12
	if textW < 10 {
		textW = 10
	}
	row := indent + label + renderScaffoldText(m.scaffold.description, focused, textW)
	return []string{header, row}
}

// renderScaffoldParamBlock returns the 3-6 rows for one param edit row.
// `focused` is true when this is the active param; `activeField` is the
// subfield currently being edited (only meaningful when focused).
func (m model) renderScaffoldParamBlock(e scaffoldParamEdit, focused bool, activeField string, w int) []string {
	var out []string

	var cursor string
	if focused {
		cursor = selectedCursorStyle.Render(" › ")
	} else {
		cursor = "   "
	}
	displayName := e.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = "(new param)"
	}
	nameRow := cursor + depsLabelStyle.Render(displayName)
	if e.Name != "" {
		if e.Required {
			nameRow += "   " + selectedCursorStyle.Render("(required)")
		} else {
			nameRow += "   " + noMatchStyle.Render("(optional)")
		}
	}
	out = append(out, nameRow)

	indent := "      "
	// Subfield rows, honoring visibility.
	for _, field := range e.visibleFields() {
		fieldFocused := focused && field == activeField
		out = append(out, indent+m.renderScaffoldField(e, field, fieldFocused, w-len(indent)))
	}
	return out
}

// renderScaffoldField renders one subfield line for a param.
func (m model) renderScaffoldField(e scaffoldParamEdit, field string, focused bool, w int) string {
	label := depsValueStyle.Render(padRight(field+":", 10))
	switch field {
	case "name":
		return label + renderScaffoldText(e.Name, focused, w-12)
	case "type":
		return label + renderScaffoldEnum(scaffoldKinds, e.Kind, focused)
	case "required":
		return label + renderScaffoldBool(e.Required, focused)
	case "default":
		return label + renderScaffoldText(e.Default, focused, w-12)
	case "options":
		hint := noMatchStyle.Render("  (comma or pipe separated, e.g. dev,staging,prod)")
		return label + renderScaffoldText(e.Options, focused, w-lipgloss.Width(hint)-12) + hint
	case "desc":
		return label + renderScaffoldText(e.Description, focused, w-12)
	}
	return label
}

func renderScaffoldEnum(options []string, current string, focused bool) string {
	var b strings.Builder
	for i, o := range options {
		if i > 0 {
			b.WriteString(noMatchStyle.Render("  "))
		}
		if o == current {
			if focused {
				b.WriteString(selectedCursorStyle.Render("‹ ") + selectedItemStyle.Render(o) + selectedCursorStyle.Render(" ›"))
			} else {
				b.WriteString(selectedItemStyle.Render(o))
			}
		} else {
			b.WriteString(normalItemStyle.Render(o))
		}
	}
	return b.String()
}

func renderScaffoldBool(on, focused bool) string {
	var mark string
	if on {
		mark = recipeStyle.Render("[✓] yes")
	} else {
		mark = noMatchStyle.Render("[ ] no")
	}
	if focused {
		return selectedCursorStyle.Render("‹ ") + mark + selectedCursorStyle.Render(" ›")
	}
	return mark
}

func renderScaffoldText(value string, focused bool, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	shown := value
	if len(shown) > maxLen {
		shown = "…" + shown[len(shown)-(maxLen-1):]
	}
	if focused {
		return selectedCursorStyle.Render("[") +
			filterStyle.Render(shown) +
			selectedCursorStyle.Render("▋") +
			selectedCursorStyle.Render("]")
	}
	if shown == "" {
		return noMatchStyle.Render("[ ]")
	}
	return noMatchStyle.Render("[") + normalItemStyle.Render(shown) + noMatchStyle.Render("]")
}

// --- Candidate scanning + indicator annotation (unchanged below) -------

// makeVarRefRe matches `$(NAME)` or `${NAME}` where NAME starts with a
// letter/underscore. We deliberately don't match single-char automatic
// variables ($@, $<, $^, $?, $*, $+, $|) because those are never user
// inputs — they're set by make itself per-rule.
var makeVarRefRe = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)

// assignmentRe matches a file-level variable assignment: `NAME := ...`,
// `NAME = ...`, `NAME ?= ...`, `NAME += ...`. Indented lines (recipe
// commands) are filtered out by the caller.
var assignmentRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*(:=|\?=|\+=|=)`)

// makeBuiltins are variables set or managed by make itself; they shouldn't
// be surfaced as @param candidates.
var makeBuiltins = map[string]bool{
	"MAKE": true, "MAKECMDGOALS": true, "MAKEFLAGS": true,
	"MAKEFILE_LIST": true, "MAKELEVEL": true, "MAKEOVERRIDES": true,
	"MFLAGS": true, "CURDIR": true, "SHELL": true, "VPATH": true,
	".DEFAULT_GOAL": true, ".VARIABLES": true, ".RECIPEPREFIX": true,
}

// fileAssignedVars returns the set of variable names assigned at file scope
// in `path`. These are project constants or computed values from the
// Makefile author — not things the user needs to supply, so scaffolding
// skips them. Returns an empty map on any read error.
func fileAssignedVars(path string) map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "\t") {
			continue
		}
		if m := assignmentRe.FindStringSubmatch(line); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

// annotateTargets fills in t.HasParams / t.HasScaffold on every target in
// `flat` and in `groups[*].Targets` (which hold separate copies). Called
// once after collectTargets so render code can show indicators without
// re-scanning files per frame.
func annotateTargets(flat []MakeTarget, groups []TargetGroup) {
	filePsByDir := map[string][]MakeParam{}
	for _, g := range groups {
		filePsByDir[g.Dir] = g.Params
	}
	fileAssignedByPath := map[string]map[string]bool{}
	getFileAssigned := func(path string) map[string]bool {
		if cached, ok := fileAssignedByPath[path]; ok {
			return cached
		}
		cached := fileAssignedVars(path)
		fileAssignedByPath[path] = cached
		return cached
	}
	for i := range flat {
		t := &flat[i]
		annotateTarget(t, filePsByDir[t.Dir], getFileAssigned(t.File))
	}
	for gi := range groups {
		for ti := range groups[gi].Targets {
			t := &groups[gi].Targets[ti]
			annotateTarget(t, filePsByDir[t.Dir], getFileAssigned(t.File))
		}
	}
}

func annotateTarget(t *MakeTarget, fileParams []MakeParam, fileAssigned map[string]bool) {
	refs := scanRecipeVars(*t)
	refSet := map[string]bool{}
	for _, r := range refs {
		refSet[r] = true
	}
	t.HasParams = len(t.Params) > 0
	if !t.HasParams {
		for _, fp := range fileParams {
			if refSet[fp.Name] {
				t.HasParams = true
				break
			}
		}
	}
	documented := map[string]bool{}
	for _, p := range t.Params {
		documented[p.Name] = true
	}
	for _, fp := range fileParams {
		documented[fp.Name] = true
	}
	t.HasScaffold = false
	for _, r := range refs {
		if !makeBuiltins[r] && !documented[r] && !fileAssigned[r] {
			t.HasScaffold = true
			break
		}
	}
}

// scanRecipeVars extracts the deduplicated list of $(VAR) / ${VAR} names
// referenced in target.Recipe, preserving discovery order.
func scanRecipeVars(target MakeTarget) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range target.Recipe {
		for _, m := range makeVarRefRe.FindAllStringSubmatch(line, -1) {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

// scaffoldCandidates scans target.Recipe for $(VAR) / ${VAR} references
// and returns names that are: not make builtins, not already documented
// by a target-level or file-level @param, and not assigned at file scope
// in the target's Makefile.
func (m model) scaffoldCandidates(target MakeTarget) []string {
	existing := map[string]bool{}
	for _, p := range target.Params {
		existing[p.Name] = true
	}
	for _, p := range m.fileParamsFor(target.Dir) {
		existing[p.Name] = true
	}
	fileAssigned := fileAssignedVars(target.File)
	var out []string
	for _, name := range scanRecipeVars(target) {
		if makeBuiltins[name] || existing[name] || fileAssigned[name] {
			continue
		}
		out = append(out, name)
	}
	return out
}

// fileParamsFor returns file-level @params for the Makefile in `dir`, or
// nil if the group isn't found.
func (m model) fileParamsFor(dir string) []MakeParam {
	for _, g := range m.groups {
		if g.Dir == dir {
			return g.Params
		}
	}
	return nil
}

// effectiveParams returns the combined list of params that apply to
// `target`: its own @params plus any file-level @params from the same
// Makefile that the recipe references. Preserves target-level order first,
// then file-level order.
func (m model) effectiveParams(target MakeTarget) []MakeParam {
	params := append([]MakeParam{}, target.Params...)
	filePs := m.fileParamsFor(target.Dir)
	if len(filePs) == 0 {
		return params
	}
	refs := map[string]bool{}
	for _, name := range scanRecipeVars(target) {
		refs[name] = true
	}
	existing := map[string]bool{}
	for _, p := range params {
		existing[p.Name] = true
	}
	for _, fp := range filePs {
		if refs[fp.Name] && !existing[fp.Name] {
			params = append(params, fp)
		}
	}
	return params
}
