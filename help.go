package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpBodyHeight is the height of the scrollable body inside the help
// view: total minus the 5 fixed chrome rows.
func (m model) helpBodyHeight() int {
	h := m.height - 5
	if h < 1 {
		h = 1
	}
	return h
}

// updateHelp handles the help view's own keymap. Only esc/q/^g close —
// arrow / j / k / pgup / pgdown / g / G scroll the body so users
// instinctively reaching for navigation aren't kicked back to list mode.
func (m model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	body := buildHelpBody()
	bodyH := m.helpBodyHeight()
	maxScroll := len(body) - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q", "ctrl+g":
		m.showHelp = false
		m.helpScroll = 0
		return m, nil
	case "down", "j":
		m.helpScroll++
	case "up", "k":
		m.helpScroll--
	case "pgdown", "ctrl+d":
		m.helpScroll += bodyH / 2
	case "pgup", "ctrl+u":
		m.helpScroll -= bodyH / 2
	case "g", "home":
		m.helpScroll = 0
	case "G", "end":
		m.helpScroll = maxScroll
	}

	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
	if m.helpScroll > maxScroll {
		m.helpScroll = maxScroll
	}
	return m, nil
}

// renderHelpView renders a full-screen cheat sheet: the key bindings plus a
// compact reminder of the `@param` annotation syntax, so users never have to
// leave mkm to remember how to annotate a target.
func (m model) renderHelpView(w, h int) string {
	rule := ruleStyle.Render(strings.Repeat("─", w))
	top := padLine(titleStyle.Render("mkm")+filterPromptStyle.Render(" › ")+filterStyle.Render("help"), w)

	body := buildHelpBody()
	bodyH := h - 5
	if bodyH < 1 {
		bodyH = 1
	}

	maxScroll := len(body) - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.helpScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + bodyH
	if end > len(body) {
		end = len(body)
	}
	visible := append([]string{}, body[scroll:end]...)
	for len(visible) < bodyH {
		visible = append(visible, "")
	}
	for i, r := range visible {
		lw := lipgloss.Width(r)
		if lw < w {
			visible[i] = r + strings.Repeat(" ", w-lw)
		}
	}

	// Cmdline shows scroll progress so users know there's more below.
	pos := "top"
	if maxScroll > 0 {
		pct := (scroll * 100) / maxScroll
		switch {
		case scroll == 0:
			pos = "top"
		case scroll >= maxScroll:
			pos = "bottom"
		default:
			pos = formatPercent(pct)
		}
	}
	pill := renderActionPill("?", "help", purpleColor)
	cmdBody := normalItemStyle.Render("scrolled to ") +
		selectedItemStyle.Render(pos) +
		helpKeyStyle.Render("   keys · indicators · @param syntax")
	cmd := renderActionLine(w, pill, cmdBody)
	legend := renderLegend(w, []legendItem{
		{Key: "↑↓/jk"},
		{Key: "g/G", Hint: "top/bot"},
		{Key: "esc/q", Hint: "back"},
		{Key: "^c", Hint: "quit"},
	})

	rows := []string{top, rule}
	rows = append(rows, visible...)
	rows = append(rows, rule, cmd, legend)
	return strings.Join(rows, "\n")
}

func formatPercent(p int) string {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return percentStr(p)
}

func percentStr(p int) string {
	// hand-rolled to avoid pulling fmt for one Sprintf
	if p == 100 {
		return "100%"
	}
	if p < 10 {
		return string(rune('0'+p)) + "%"
	}
	return string(rune('0'+p/10)) + string(rune('0'+p%10)) + "%"
}

// buildHelpBody returns the full cheatsheet as a slice of styled lines.
// Pure render — depends on no model state, so it can be measured for
// scroll bounds independently.
func buildHelpBody() []string {
	keys := [][2]string{
		{"list mode:", ""},
		{"↑↓ / ctrl+p/n / wheel", "move cursor"},
		{"type any letter", "fuzzy filter targets (matches name + description)"},
		{"backspace / ctrl+w", "delete one char / one word"},
		{"esc", "clear active filter (or quit when filter is empty)"},
		{"enter", "run target (or open param form)"},
		{"tab", "toggle preview pane"},
		{"ctrl+v", "view the Makefile in-TUI"},
		{"ctrl+e", "open $EDITOR at target line"},
		{"ctrl+y", "copy the make command for the cursor target (without running)"},
		{"ctrl+a", "scaffold @param block from $(VAR) refs, then edit"},
		{"ctrl+s", "open settings (theme, history flags, rc-file fix, updates)"},
		{"ctrl+u", "copy `go install …@latest` when an update is available"},
		{"ctrl+x", "dismiss the update banner for this session"},
		{"ctrl+g", "show this help"},
		{"ctrl+c", "quit"},
		{"", ""},
		{"viewer mode:", ""},
		{"j/k ↑↓ / wheel", "line down/up"},
		{"n / N", "next / previous target"},
		{"g / G", "top / bottom of file"},
		{"ctrl+d / ctrl+u", "half-page down/up"},
		{"e (or ctrl+e)", "open $EDITOR at current line"},
		{"esc / q", "back to list"},
		{"", ""},
		{"settings (ctrl+s):", ""},
		{"↑↓ / tab / wheel", "move between fields"},
		{"←→ / space", "cycle theme, toggle flag"},
		{"a–z (theme row)", "type-ahead jump to next theme starting with that letter"},
		{"shell_history row:", ""},
		{"ctrl+a", "install mkm shell wrapper into rc file"},
		{"ctrl+y", "copy wrapper snippet to clipboard (pbcopy/wl-copy/xclip)"},
		{"ctrl+r", "copy `source <rc-file>` reload command to clipboard"},
		{"ctrl+e", "open rc file in $EDITOR"},
		{"ctrl+v", "view rc file in-TUI"},
		{"enter", "save config and close"},
		{"esc", "cancel, revert theme preview"},
		{"", ""},
		{"param form:", ""},
		{"↑↓ / tab / wheel", "move between fields"},
		{"←→ / space", "cycle enum, toggle bool"},
		{"typing / ctrl+u", "edit text field / clear"},
		{"enter", "emit make command (blocked when required fields are empty)"},
		{"esc", "back to list"},
		{"", ""},
		{"scaffold form (ctrl+a):", ""},
		{"↑↓ / tab / wheel", "move between fields (any param)"},
		{"←→ / space", "cycle type, toggle required"},
		{"typing / ctrl+u", "edit name/default/options/desc text"},
		{"ctrl+n", "add a new param row (manual)"},
		{"ctrl+d", "delete the focused param row"},
		{"enter", "write formatted @param lines to Makefile"},
		{"ctrl+e", "write, then open $EDITOR for tweaks"},
		{"esc", "cancel, no file changes"},
	}

	var rows []string
	rows = append(rows, "")
	rows = append(rows, "  "+previewNameStyle.Render("Keys"))
	rows = append(rows, "")
	for _, k := range keys {
		if k[0] == "" && k[1] == "" {
			rows = append(rows, "")
			continue
		}
		if k[1] == "" {
			rows = append(rows, "  "+depsLabelStyle.Render(k[0]))
			continue
		}
		rows = append(rows, "  "+padRight(helpKeyStyle.Render(k[0]), 24)+"  "+normalItemStyle.Render(k[1]))
	}

	rows = append(rows, "")
	rows = append(rows, "  "+previewNameStyle.Render("List indicators"))
	rows = append(rows, "")
	rows = append(rows, "  "+depsLabelStyle.Render("◆")+"   "+normalItemStyle.Render("target has applicable @param docs (its own or file-level)"))
	rows = append(rows, "  "+descStyle.Render("◇")+"   "+normalItemStyle.Render("recipe has $(VAR) refs that could be scaffolded (ctrl+a)"))
	rows = append(rows, "  "+helpKeyStyle.Render("⚙")+"   "+normalItemStyle.Render("target is .PHONY — an action recipe, not a file-producing rule"))

	rows = append(rows, "")
	rows = append(rows, "  "+previewNameStyle.Render("@param syntax"))
	rows = append(rows, "")
	rows = append(rows, "  "+depsLabelStyle.Render("# @param {<type>} <name-spec>  description"))
	rows = append(rows, "")
	rows = append(rows, "  "+helpKeyStyle.Render("<type>     ")+normalItemStyle.Render("string | int | bool | a|b|c"))
	rows = append(rows, "  "+helpKeyStyle.Render("<name>     ")+normalItemStyle.Render("NAME (required) | [NAME] | [NAME=default]"))
	rows = append(rows, "")
	rows = append(rows, "  "+noMatchStyle.Render("example:"))
	rows = append(rows, "  "+depsLabelStyle.Render("# @param {dev|staging|prod} ENV          target environment"))
	rows = append(rows, "  "+depsLabelStyle.Render("# @param {string} [VERSION=latest]       release tag"))
	rows = append(rows, "  "+depsLabelStyle.Render("# @param {bool} [DRY_RUN=false]          preview without executing"))

	return rows
}
