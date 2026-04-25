package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// formState holds user-editable values and focus when a target has
// applicable @param annotations (its own and/or referenced file-level).
// nil on model means we're in list/filter mode.
type formState struct {
	target    *MakeTarget
	params    []MakeParam // effective params: target-level + referenced file-level
	values    map[string]string
	focus     int
	errMsg    string // transient validation feedback shown above the legend
	errMsgSeq int
}

func newFormState(target *MakeTarget, params []MakeParam) *formState {
	values := map[string]string{}
	for _, p := range params {
		switch {
		case p.Default != "":
			values[p.Name] = p.Default
		case p.Kind == "enum" && len(p.Options) > 0:
			values[p.Name] = p.Options[0]
		case p.Kind == "bool":
			values[p.Name] = "false"
		default:
			values[p.Name] = ""
		}
	}
	return &formState{target: target, params: params, values: values}
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.form = nil
		return m, nil
	case "enter":
		// Block running until every required param has a value. Without
		// this the * indicator was a lie — make would be invoked with
		// blank VAR= args and the recipe would silently misbehave.
		var missing []string
		for _, p := range m.form.params {
			if p.Required && strings.TrimSpace(m.form.values[p.Name]) == "" {
				missing = append(missing, p.Name)
			}
		}
		if len(missing) > 0 {
			// Park focus on the first missing param so the user can fix it.
			for i, p := range m.form.params {
				if p.Name == missing[0] {
					m.form.focus = i
					break
				}
			}
			m.form.errMsg = "missing required: " + strings.Join(missing, ", ")
			m.form.errMsgSeq++
			return m, clearToastCmd(toastFormErr, m.form.errMsgSeq)
		}
		m.selected = true
		return m, tea.Quit
	case "down", "tab":
		m.form.focus = (m.form.focus + 1) % len(m.form.params)
		return m, nil
	case "up", "shift+tab":
		m.form.focus = (m.form.focus - 1 + len(m.form.params)) % len(m.form.params)
		return m, nil
	}

	p := m.form.params[m.form.focus]
	v := m.form.values[p.Name]

	switch p.Kind {
	case "enum":
		switch {
		case key == "left" || key == "h":
			m.form.values[p.Name] = cycleEnum(p.Options, v, -1)
		case key == "right" || key == "l" || key == " ":
			m.form.values[p.Name] = cycleEnum(p.Options, v, 1)
		case len(key) == 1 && key[0] >= 'a' && key[0] <= 'z':
			// Type-ahead: same behavior as the settings theme row.
			// Wraps so a repeated press cycles through matches.
			if name := nextByPrefix(p.Options, v, key); name != "" {
				m.form.values[p.Name] = name
			}
		}
	case "bool":
		if key == " " || key == "left" || key == "right" {
			if v == "true" {
				m.form.values[p.Name] = "false"
			} else {
				m.form.values[p.Name] = "true"
			}
		}
	default: // string, int, or unknown
		switch key {
		case "backspace":
			if len(v) > 0 {
				m.form.values[p.Name] = v[:len(v)-1]
			}
		case "ctrl+u":
			m.form.values[p.Name] = ""
		default:
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if p.Kind == "int" && (key[0] < '0' || key[0] > '9') && !(v == "" && key == "-") {
					return m, nil
				}
				m.form.values[p.Name] = v + key
			}
		}
	}
	return m, nil
}

func cycleEnum(options []string, current string, delta int) string {
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

func (m model) renderFormLegend(w int) string {
	return renderLegend(w, []legendItem{
		{Key: "↑↓"},
		{Key: "←→/space", Hint: "cycle/toggle"},
		{Key: "type", Hint: "edit"},
		{Key: "^u", Hint: "clear"},
		{Key: "enter", Hint: "run"},
		{Key: "esc", Hint: "back"},
	})
}

// renderFormTopLine is like renderTopLine but the "filter" slot shows the
// target name. Help moved to the bottom legend row.
func (m model) renderFormTopLine(w int) string {
	left := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ")
	leftW := lipgloss.Width(left)

	avail := w - leftW
	if avail < 0 {
		avail = 0
	}
	name := m.form.target.Name
	if len(name) > avail {
		name = truncateStr(name, avail)
	}
	return padLine(left+filterStyle.Render(name), w)
}

// renderFormView renders the param-input form: top + rule + body + rule +
// command preview + legend.
func (m model) renderFormView(w, h int) string {
	top := m.renderFormTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))
	cmdPreview := m.renderFormCmd(w)
	legend := m.renderFormLegend(w)

	// top(1) + rule(1) + rule(1) + cmd(1) + legend(1) = 5 fixed rows.
	bodyH := h - 5
	if bodyH < 1 {
		bodyH = 1
	}
	body := m.renderFormBody(w, bodyH)

	return strings.Join([]string{top, rule, body, rule, cmdPreview, legend}, "\n")
}

func (m model) renderFormBody(w, h int) string {
	params := m.form.params
	var lines []string

	if m.form.target.Description != "" {
		wrapped := wordWrap(m.form.target.Description, w-2)
		for _, wl := range strings.Split(wrapped, "\n") {
			lines = append(lines, "  "+descStyle.Render(truncateStr(wl, w-2)))
		}
		lines = append(lines, "")
	}

	nameW := 0
	for _, p := range params {
		if len(p.Name) > nameW {
			nameW = len(p.Name)
		}
	}

	for i, p := range params {
		focused := i == m.form.focus
		lines = append(lines, m.renderParamRow(p, m.form.values[p.Name], nameW, w, focused))
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

func (m model) renderParamRow(p MakeParam, value string, nameW, w int, focused bool) string {
	var cursor string
	if focused {
		cursor = selectedCursorStyle.Render(" › ")
	} else {
		cursor = "   "
	}

	nameStyled := depsLabelStyle.Render(padRight(p.Name, nameW))
	gap := "  "

	var valueStr string
	switch p.Kind {
	case "enum":
		valueStr = renderEnumValue(p.Options, value, focused)
	case "bool":
		if value == "true" {
			valueStr = recipeStyle.Render("[✓] on")
		} else {
			valueStr = noMatchStyle.Render("[ ] off")
		}
	default:
		valueStr = renderTextValue(value, focused, w-nameW-10)
	}

	trailing := ""
	if p.Description != "" {
		// Same dim style used for target descriptions in the list — keeps
		// "metadata about this thing" visually consistent across pages.
		trailing = "  " + helpKeyStyle.Render(p.Description)
	}
	requiredMark := ""
	if p.Required {
		requiredMark = " " + selectedCursorStyle.Render("*")
	}

	return cursor + nameStyled + gap + valueStr + requiredMark + trailing
}

func renderEnumValue(options []string, current string, focused bool) string {
	idx := 0
	for i, o := range options {
		if o == current {
			idx = i
			break
		}
	}
	var out strings.Builder
	if focused {
		out.WriteString(selectedCursorStyle.Render("‹ "))
	} else {
		out.WriteString(noMatchStyle.Render("‹ "))
	}
	for i, o := range options {
		if i > 0 {
			out.WriteString(noMatchStyle.Render(" | "))
		}
		if i == idx {
			out.WriteString(selectedItemStyle.Render(o))
		} else {
			out.WriteString(normalItemStyle.Render(o))
		}
	}
	if focused {
		out.WriteString(selectedCursorStyle.Render(" ›"))
	} else {
		out.WriteString(noMatchStyle.Render(" ›"))
	}
	return out.String()
}

func renderTextValue(value string, focused bool, maxLen int) string {
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

func (m model) renderFormCmd(w int) string {
	if m.form.errMsg != "" {
		// Red pill makes the failed enter unmistakable.
		pill := renderActionPill("!", "fix", redColor)
		return renderActionLine(w, pill, diffRemoveStyle.Render(m.form.errMsg))
	}
	args := makeCmd(*m.form.target, m.form.params, m.form.values)
	pill := renderActionPill("⏎", "run", accent)
	return renderActionLine(w, pill, renderMakeCmd(args))
}

// makeCmd builds the argv to run for `target` given param `values`. The
// param list controls which names are emitted as VAR=value args — callers
// pass the effective set (target-level + referenced file-level). Pass nil
// for a bare `make target` with no var overrides.
func makeCmd(target MakeTarget, params []MakeParam, values map[string]string) []string {
	args := []string{"make"}
	if target.Dir != "." {
		args = append(args, "-C", target.Dir)
	}
	args = append(args, target.Name)
	// Preserve declared param order — map iteration would shuffle.
	for _, p := range params {
		v, ok := values[p.Name]
		if !ok || v == "" {
			continue
		}
		args = append(args, p.Name+"="+shellQuote(v))
	}
	return args
}

// shellQuote wraps s in single quotes if it contains shell-special chars.
// Values that are all [a-zA-Z0-9_.-/:] are left bare.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '.' || c == '/' || c == ':' || c == ',') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
