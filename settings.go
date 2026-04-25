package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsState is the working copy of the user's config while the settings
// screen is open. Edits are kept separate from the live model config so esc
// can discard them. originalTheme is captured on open so we can revert the
// live theme preview when the user cancels.
//
// The shareHist* fields are snapshot state for the shell_history row — we
// detect on open, refresh after apply or after the user returns from the
// editor. This is intentionally not part of Config: it's not a user
// preference, it's an rc-file side effect we help the user set up as part
// of the same "make shell history work" feature.
type settingsState struct {
	focus               int
	cfg                 Config
	themes              []string
	originalTheme       string
	shareHistShell      string // "zsh" | "bash" | ""
	shareHistConfigured bool
	shareHistApplied    bool   // true after we wrote the snippet in this session
	shareHistErr        string
	clipMsg             string // transient feedback after a ctrl+y copy attempt
	clipMsgSeq          int
}

const (
	settingsFieldTheme = iota
	settingsFieldWriteHistory
	settingsFieldShellHistory
	settingsFieldCheckUpdates
	settingsFieldCount
)

func newSettingsState(cfg Config) *settingsState {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	shell := detectedShell()
	return &settingsState{
		cfg:                 cfg,
		themes:              names,
		originalTheme:       cfg.Theme,
		shareHistShell:      shell,
		shareHistConfigured: shareHistConfigured(shell),
	}
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		applyTheme(m.settings.originalTheme)
		m.settings = nil
		return m, nil
	case "enter":
		cfg := m.settings.cfg
		writeConfig(configPath(), cfg)
		applyTheme(cfg.Theme)
		// Toggling write_history mid-session takes effect now: reload or drop
		// the recency map so subsequent filtering scores match the new mode.
		if cfg.WriteHistory != m.writeHistory {
			if cfg.WriteHistory {
				m.history = loadHistory()
			} else {
				m.history = map[string]int64{}
			}
			m.writeHistory = cfg.WriteHistory
			m.updateFilter()
		}
		// Keep an installed managed wrapper in sync with the toggle so
		// turning shell_history off actually stops up-arrow from seeing
		// mkm commands. Legacy (unmanaged) wrappers aren't touched —
		// applyShareHistFromSettings surfaces those to the user.
		if m.settings.shareHistShell != "" && cfg.ShellHistory != m.shellHistory {
			_ = syncManagedWrapper(m.settings.shareHistShell, cfg.ShellHistory)
		}
		m.shellHistory = cfg.ShellHistory
		m.checkUpdates = cfg.CheckUpdates
		m.settings = nil
		return m.toastSaved("config saved → " + configPath())
	case "down", "tab":
		m.settings.focus = (m.settings.focus + 1) % settingsFieldCount
		return m, nil
	case "up", "shift+tab":
		m.settings.focus = (m.settings.focus - 1 + settingsFieldCount) % settingsFieldCount
		return m, nil
	}

	switch m.settings.focus {
	case settingsFieldTheme:
		switch {
		case key == "left" || key == "h":
			m.settings.cfg.Theme = cycleEnum(m.settings.themes, m.settings.cfg.Theme, -1)
			applyTheme(m.settings.cfg.Theme)
		case key == "right" || key == "l" || key == " ":
			m.settings.cfg.Theme = cycleEnum(m.settings.themes, m.settings.cfg.Theme, 1)
			applyTheme(m.settings.cfg.Theme)
		case len(key) == 1 && key[0] >= 'a' && key[0] <= 'z':
			// Type-ahead: jump to the next theme starting with this
			// letter. Wraps around so repeated presses cycle through
			// matches (e.g. 'g' → "github-dark" → "gruvbox-dark" → ...).
			if name := nextByPrefix(m.settings.themes, m.settings.cfg.Theme, key); name != "" {
				m.settings.cfg.Theme = name
				applyTheme(name)
			}
		}
	case settingsFieldWriteHistory:
		if key == " " || key == "left" || key == "right" {
			m.settings.cfg.WriteHistory = !m.settings.cfg.WriteHistory
		}
	case settingsFieldCheckUpdates:
		if key == " " || key == "left" || key == "right" {
			m.settings.cfg.CheckUpdates = !m.settings.cfg.CheckUpdates
		}
	case settingsFieldShellHistory:
		switch key {
		case " ":
			m.settings.cfg.ShellHistory = !m.settings.cfg.ShellHistory
		case "ctrl+a":
			m.applyShareHistFromSettings()
		case "ctrl+e":
			if path := rcFilePath(m.settings.shareHistShell); path != "" {
				// Hand off to $EDITOR. When it returns, main's
				// editorFinishedMsg handler re-checks share-hist status so
				// the row reflects manual edits.
				return m, editCmd(path, 1)
			}
		case "ctrl+v":
			if path := rcFilePath(m.settings.shareHistShell); path != "" {
				if v := m.newViewerStateForPath(path); v != nil {
					m.viewer = v
				}
			}
		case "ctrl+y":
			snippet := strings.Trim(shareHistSnippet(m.settings.shareHistShell, m.settings.cfg.ShellHistory), "\n")
			if snippet == "" {
				return m.toastClip("no snippet available for this shell")
			}
			if err := copyToClipboard(snippet); err != nil {
				return m.toastClip("copy failed: " + err.Error())
			}
			return m.toastClip("snippet copied to clipboard")
		case "ctrl+r":
			rc := rcFilePath(m.settings.shareHistShell)
			if rc == "" {
				return m.toastClip("no rc file for this shell")
			}
			cmd := "source " + abbreviateHome(rc)
			if err := copyToClipboard(cmd); err != nil {
				return m.toastClip("copy failed: " + err.Error())
			}
			return m.toastClip("copied: " + cmd)
		}
	}
	return m, nil
}

// toastClip sets a transient clipboard-feedback message on the settings
// page and returns a tea.Cmd that clears it after toastDuration.
func (m model) toastClip(text string) (tea.Model, tea.Cmd) {
	m.settings.clipMsg = text
	m.settings.clipMsgSeq++
	return m, clearToastCmd(toastClip, m.settings.clipMsgSeq)
}

// applyShareHistFromSettings runs the rc-file fix inline from the
// shell_history row. Variant (with or without history push) comes from
// the current toggle. Works either as a fresh install or as an in-place
// rewrite of a managed block. Errors surface via shareHistErr.
func (m *model) applyShareHistFromSettings() {
	s := m.settings
	if s.shareHistShell == "" {
		return
	}
	// Legacy wrappers (no markers) — refuse to edit automatically; tell
	// the user to clean up manually.
	if s.shareHistConfigured && !hasManagedWrapper(s.shareHistShell) {
		s.shareHistErr = "existing mkm wrapper has no mkm markers — edit " + abbreviateHome(rcFilePath(s.shareHistShell)) + " manually (ctrl+e) or remove the old wrapper and retry"
		return
	}
	if err := applyShareHistFix(s.shareHistShell, s.cfg.ShellHistory); err != nil {
		s.shareHistErr = err.Error()
		return
	}
	s.shareHistConfigured = true
	s.shareHistApplied = true
	s.shareHistErr = ""
}

// refreshShareHistStatus re-runs the rc-file detection. Called from the
// editorFinishedMsg handler so manual edits to ~/.zshrc inside $EDITOR are
// reflected without having to close and reopen the settings screen.
func (s *settingsState) refreshShareHistStatus() {
	if s == nil || s.shareHistShell == "" {
		return
	}
	configured := shareHistConfigured(s.shareHistShell)
	if configured && !s.shareHistConfigured {
		// User set it up themselves in the editor.
		s.shareHistConfigured = true
	} else if !configured {
		// User removed it in the editor — surface the fix again.
		s.shareHistConfigured = false
		s.shareHistApplied = false
	}
	s.shareHistErr = ""
}

func (m model) renderSettingsView(w, h int) string {
	top := m.renderSettingsTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))
	cmd := m.renderSettingsCmdLine(w)
	legend := m.renderSettingsLegend(w)

	bodyH := h - 5
	if bodyH < 1 {
		bodyH = 1
	}
	body := m.renderSettingsBody(w, bodyH)
	return strings.Join([]string{top, rule, body, rule, cmd, legend}, "\n")
}

func (m model) renderSettingsTopLine(w int) string {
	left := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ") + filterStyle.Render("settings")
	return padLine(left, w)
}

func (m model) renderSettingsLegend(w int) string {
	return renderLegend(w, []legendItem{
		{Key: "↑↓"},
		{Key: "←→/space", Hint: "cycle/toggle"},
		{Key: "enter", Hint: "save"},
		{Key: "esc", Hint: "cancel"},
	})
}

// renderSettingsCmdLine describes the action enter would take — writing the
// pending config back to disk. Surfaces the path so the user knows exactly
// which file is being touched. Green pill cues "this commits a change."
func (m model) renderSettingsCmdLine(w int) string {
	path := configPath()
	if path == "" {
		path = "(config path unavailable)"
	}
	body := normalItemStyle.Render("config →") + " " + selectedItemStyle.Render(path)
	pill := renderActionPill("⏎", "save", greenColor)
	return renderActionLine(w, pill, body)
}

type settingsRow struct {
	name    string
	desc    string
	value   string
	section string // visual section label rendered above the first row in each group
	extra   func() []string
}

func (m model) renderSettingsBody(w, h int) string {
	rows := []settingsRow{
		{
			section: "Display",
			name:    "theme",
			desc:    "color palette for the TUI",
			value:   renderCycleValue(m.settings.cfg.Theme, themeIndex(m.settings.themes, m.settings.cfg.Theme)+1, len(m.settings.themes), m.settings.focus == settingsFieldTheme),
			extra: func() []string {
				// Color swatch under the theme row gives users a preview
				// of what they're picking without cycling through all
				// ten options. Uses the same accent colors the rest of
				// the UI is built from, so the swatch is honest.
				return []string{"      " + renderThemeSwatch()}
			},
		},
		{
			section: "History",
			name:    "write_history",
			desc:    "record selections to ~/.cache/mkm/history for recency ranking",
			value:   renderBoolValue(m.settings.cfg.WriteHistory),
		},
		{
			name:  "shell_history",
			desc:  "push mkm commands into shell history — governs HISTFILE append AND the managed wrapper's print -s / history -s line",
			value: renderBoolValue(m.settings.cfg.ShellHistory),
		},
		{
			section: "Updates",
			name:    "check_updates",
			desc:    "check GitHub daily for a newer mkm release (result cached in ~/.cache/mkm)",
			value:   renderBoolValue(m.settings.cfg.CheckUpdates),
		},
	}

	nameW := 0
	for _, r := range rows {
		if len(r.name) > nameW {
			nameW = len(r.name)
		}
	}

	var lines []string
	for i, r := range rows {
		// Section header: shown above the first row of each group, with
		// a separating blank line before it (unless it's the very top).
		if r.section != "" {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "  "+groupHeaderStyle.Render(r.section))
			lines = append(lines, "")
		}
		cursor := "   "
		if i == m.settings.focus {
			cursor = selectedCursorStyle.Render(" › ")
		}
		nameStyled := depsLabelStyle.Render(padRight(r.name, nameW))
		lines = append(lines, cursor+nameStyled+"  "+r.value)

		descIndentW := 3 + nameW + 2
		avail := w - descIndentW
		if r.desc != "" && avail > 0 {
			lines = append(lines, strings.Repeat(" ", descIndentW)+noMatchStyle.Render(truncateStr(r.desc, avail)))
		}
		if r.extra != nil {
			lines = append(lines, r.extra()...)
		}

		// shell_history row doubles as the rc-file integration control:
		// the rc-file status line is always shown so users can see whether
		// up-arrow will work. When the row is focused we also render the
		// snippet preview + apply/edit/view hints — the fool-proof mode.
		if i == settingsFieldShellHistory {
			for _, line := range m.renderShellHistStatusLines(w, descIndentW, m.settings.focus == settingsFieldShellHistory) {
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")
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

// renderThemeSwatch renders a row of small colored blocks using the
// active theme's palette. Helps users see what a theme actually looks
// like without cycling through every option to compare.
func renderThemeSwatch() string {
	block := "███ "
	swatches := []lipgloss.Color{accent, cyanColor, greenColor, yellowColor, purpleColor, redColor}
	var b strings.Builder
	for _, c := range swatches {
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render(block))
	}
	return strings.TrimRight(b.String(), " ")
}

func renderBoolValue(on bool) string {
	if on {
		return recipeStyle.Render("[✓] on")
	}
	return noMatchStyle.Render("[ ] off")
}

// renderShellHistStatusLines is the bottom half of the shell_history row.
// Always shows a compact one-liner describing rc-file state; when focused,
// expands with a line-numbered preview of the exact insertion point, a
// post-apply "next step" block, and the action-key hints.
func (m model) renderShellHistStatusLines(w, indent int, focused bool) []string {
	s := m.settings
	if s == nil {
		return nil
	}
	pad := strings.Repeat(" ", indent)
	avail := w - indent
	if avail < 10 {
		return nil
	}

	var out []string

	// One-line status, always visible.
	out = append(out, pad+shareHistStatusLine(s, avail))

	if !focused {
		// Even when unfocused, surface the most actionable key so the
		// row doesn't feel inert. Only shown when there's something to
		// do (not configured yet) — avoids advertising a no-op key on
		// rows that are already set up.
		if s.shareHistShell != "" && !s.shareHistConfigured {
			hint := helpKeyStyle.Render("focus row → ") + helpKeyStyle.Render("^a") + helpKeyStyle.Render(" applies the wrapper")
			out = append(out, pad+hint)
		}
		return out
	}

	// Post-apply next-step block — the thing that actually makes the fix
	// take effect in the current shell. Rendered prominently so it's hard
	// to miss.
	if s.shareHistApplied {
		out = append(out, "")
		for _, line := range m.renderReloadHint(s, avail) {
			out = append(out, pad+line)
		}
	}

	// Snippet preview with line numbers + file context. Only shown while a
	// fix is pending; once configured, there's nothing useful to preview.
	if s.shareHistShell != "" && !s.shareHistConfigured {
		out = append(out, "")
		out = append(out, m.renderSnippetPreview(s, w, indent)...)
	}

	if s.shareHistErr != "" {
		out = append(out, "")
		out = append(out, pad+diffRemoveStyle.Render(truncateStr("error: "+s.shareHistErr, avail)))
	}

	if s.clipMsg != "" {
		out = append(out, "")
		style := recipeStyle
		if strings.HasPrefix(s.clipMsg, "copy failed") || strings.HasPrefix(s.clipMsg, "no snippet") {
			style = diffRemoveStyle
		}
		out = append(out, pad+style.Render(truncateStr(s.clipMsg, avail)))
	}

	if hints := shellHistActionHints(s); hints != "" {
		out = append(out, "")
		out = append(out, pad+hints)
	}
	return out
}

// renderReloadHint is the "here's how to make this take effect" block
// shown right after a successful apply. Returns one or more full lines
// (without the leading pad applied by the caller). Includes an inline
// copy-key hint so the source command can be yanked with one keystroke.
func (m model) renderReloadHint(s *settingsState, w int) []string {
	rc := abbreviateHome(rcFilePath(s.shareHistShell))
	cmd := "source " + rc
	return []string{
		previewNameStyle.Render("Next: reload your shell so the wrapper takes effect."),
		helpKeyStyle.Render("  run ") + recipeStyle.Render(cmd) + helpKeyStyle.Render(" in your current shell — or just open a new one.  ") + helpKeyStyle.Render("(") + helpKeyStyle.Render("^r") + helpKeyStyle.Render(" to copy)"),
	}
}

// renderSnippetPreview builds a diff-style preview block: a clear header
// showing the target file path and exact line range we'd touch, a few
// lines of existing file content as context (with line numbers), and the
// snippet rendered as `+` additions (also numbered). An [EOF] marker makes
// the end-of-file append visually unambiguous.
func (m model) renderSnippetPreview(s *settingsState, w, indent int) []string {
	pad := strings.Repeat(" ", indent)
	avail := w - indent
	rcPath := rcFilePath(s.shareHistShell)
	rcShort := abbreviateHome(rcPath)

	// Snippet body, trimmed of leading/trailing blank lines for display —
	// the file write preserves those as a separator, but they don't add
	// anything in the preview.
	snippet := strings.Trim(shareHistSnippet(s.shareHistShell, s.cfg.ShellHistory), "\n")
	snippetLines := strings.Split(snippet, "\n")

	// Context = last contextBefore lines of the existing file (fewer if
	// the file is shorter or missing).
	const contextBefore = 3
	var ctxLines []string
	fileLineCount := 0
	fileMissing := true
	if data, err := os.ReadFile(rcPath); err == nil {
		fileMissing = false
		content := strings.TrimRight(string(data), "\n")
		if content == "" {
			fileLineCount = 0
		} else {
			existing := strings.Split(content, "\n")
			fileLineCount = len(existing)
			start := fileLineCount - contextBefore
			if start < 0 {
				start = 0
			}
			ctxLines = existing[start:]
		}
	}

	insertStart := fileLineCount + 1
	insertEnd := fileLineCount + len(snippetLines)

	// Header: file + exact lines being inserted.
	var header string
	switch {
	case fileMissing:
		header = fmt.Sprintf("will create %s and write lines 1–%d:", rcShort, len(snippetLines))
	case fileLineCount == 0:
		header = fmt.Sprintf("will append to empty file %s (lines 1–%d):", rcShort, len(snippetLines))
	default:
		header = fmt.Sprintf("will append to %s at lines %d–%d:", rcShort, insertStart, insertEnd)
	}

	// Gutter width driven by the highest line number we print.
	maxLN := insertEnd
	gutter := len(strconv.Itoa(maxLN))
	if gutter < 3 {
		gutter = 3
	}
	sep := diffGutterStyle.Render(" │ ")
	// `pad + N + sep + "+ " + text` — reserve space so snippet lines fit.
	textW := avail - gutter - lipgloss.Width(sep) - 2
	if textW < 10 {
		textW = 10
	}

	out := []string{pad + previewNameStyle.Render(truncateStr(header, avail))}

	if fileMissing {
		out = append(out, pad+diffGutterStyle.Render(padLeft("—", gutter))+sep+noMatchStyle.Render("(file doesn't exist yet)"))
	} else if len(ctxLines) == 0 {
		out = append(out, pad+diffGutterStyle.Render(padLeft("—", gutter))+sep+noMatchStyle.Render("(file is empty)"))
	} else {
		// Context lines: numbered, dimmed, leading two spaces to align
		// with the "+ " prefix on additions.
		startLN := fileLineCount - len(ctxLines) + 1
		for i, line := range ctxLines {
			ln := startLN + i
			numStr := padLeft(strconv.Itoa(ln), gutter)
			out = append(out, pad+diffContextStyle.Render(numStr)+sep+diffContextStyle.Render("  "+truncateStr(line, textW)))
		}
	}

	// The additions.
	for i, line := range snippetLines {
		ln := insertStart + i
		numStr := padLeft(strconv.Itoa(ln), gutter)
		out = append(out, pad+diffAddStyle.Render(numStr)+sep+diffAddStyle.Render("+ "+truncateStr(line, textW)))
	}

	// EOF marker so the end-of-file append is visually unambiguous.
	out = append(out, pad+diffGutterStyle.Render(padLeft("—", gutter))+sep+diffGutterStyle.Render("[EOF]"))

	return out
}

// shareHistStatusLine condenses rc-file state into one dim line under the
// shell_history row. Intentionally verbose enough that the user doesn't
// have to focus the row to see the situation.
func shareHistStatusLine(s *settingsState, w int) string {
	rc := abbreviateHome(rcFilePath(s.shareHistShell))
	switch {
	case s.shareHistShell == "":
		return noMatchStyle.Render(truncateStr("rc-file: (n/a — $SHELL isn't zsh or bash)", w))
	case s.shareHistApplied:
		return recipeStyle.Render(truncateStr("rc-file: installed mkm wrapper in "+rc+" ✓", w))
	case s.shareHistConfigured && hasLegacyWrapper(s.shareHistShell):
		return descStyle.Render(truncateStr("rc-file: "+rc+" has a legacy mkm wrapper (no mkm markers) — shell_history toggle won't sync; edit with ^e or re-apply", w))
	case s.shareHistConfigured:
		return recipeStyle.Render(truncateStr("rc-file: mkm wrapper in "+rc+" ✓ (tracked with mkm markers — shell_history toggle kept in sync)", w))
	default:
		return descStyle.Render(truncateStr("rc-file: no mkm wrapper in "+rc+" — up-arrow in this shell won't recall mkm-launched commands", w))
	}
}

// shellHistActionHints lists the ctrl-key actions available on the focused
// shell_history row. Adapts to state so we don't advertise keys that do
// nothing in the current situation.
func shellHistActionHints(s *settingsState) string {
	if s.shareHistShell == "" {
		return ""
	}
	parts := []string{
		helpKeyStyle.Render("space") + normalItemStyle.Render(": toggle"),
	}
	if !s.shareHistConfigured {
		parts = append(parts,
			helpKeyStyle.Render("^a")+normalItemStyle.Render(": apply fix"),
			helpKeyStyle.Render("^y")+normalItemStyle.Render(": copy snippet"),
		)
	} else {
		// Once the wrapper's in place, the most useful thing to copy is
		// the reload command — surface it as a top-level action.
		parts = append(parts,
			helpKeyStyle.Render("^r")+normalItemStyle.Render(": copy `source` cmd"),
		)
	}
	rc := abbreviateHome(rcFilePath(s.shareHistShell))
	parts = append(parts,
		helpKeyStyle.Render("^e")+normalItemStyle.Render(": edit "+rc),
		helpKeyStyle.Render("^v")+normalItemStyle.Render(": view "+rc),
	)
	return strings.Join(parts, ruleStyle.Render("   "))
}

// renderCycleValue is a compact alternative to renderEnumValue for fields
// with many options — shows just the current value bracketed by arrows
// plus a `[N/M]` position so users can see how many options exist
// without exhaustively cycling.
func renderCycleValue(current string, idx, total int, focused bool) string {
	pos := fmt.Sprintf("  [%d/%d]", idx, total)
	if focused {
		return selectedCursorStyle.Render("‹ ") +
			selectedItemStyle.Render(current) +
			selectedCursorStyle.Render(" ›") +
			helpKeyStyle.Render(pos)
	}
	return noMatchStyle.Render("‹ ") +
		normalItemStyle.Render(current) +
		noMatchStyle.Render(" ›") +
		helpKeyStyle.Render(pos)
}

// themeIndex returns the 0-based index of `name` in `names` or 0 if not
// found. Used purely for the position indicator.
func themeIndex(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return 0
}

// nextByPrefix returns the next entry in `options` whose name starts with
// `prefix`, beginning the search just past `current`. Wraps to the start
// when needed. Returns "" if no entry matches.
func nextByPrefix(options []string, current, prefix string) string {
	start := 0
	for i, o := range options {
		if o == current {
			start = i + 1
			break
		}
	}
	for i := 0; i < len(options); i++ {
		o := options[(start+i)%len(options)]
		if strings.HasPrefix(o, prefix) {
			return o
		}
	}
	return ""
}

