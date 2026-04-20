package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelpView renders a full-screen cheat sheet: the key bindings plus a
// compact reminder of the `@param` annotation syntax, so users never have to
// leave mkm to remember how to annotate a target.
func (m model) renderHelpView(w, h int) string {
	rule := ruleStyle.Render(strings.Repeat("─", w))
	titlePart := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ") + filterStyle.Render("help")
	hint := helpKeyStyle.Render("any key: back")
	pad := w - lipgloss.Width(titlePart) - lipgloss.Width(hint) - 2
	if pad < 0 {
		pad = 0
	}
	top := titlePart + strings.Repeat(" ", pad) + hint

	keys := [][2]string{
		{"list mode:", ""},
		{"↑↓ / ctrl+p/n / wheel", "move cursor"},
		{"type any letter", "fuzzy filter targets"},
		{"enter", "run target (or open param form)"},
		{"tab", "toggle preview pane"},
		{"ctrl+v", "view the Makefile in-TUI"},
		{"ctrl+e", "open $EDITOR at target line"},
		{"ctrl+a", "scaffold @param block from $(VAR) refs, then edit"},
		{"ctrl+g", "show this help"},
		{"esc / ctrl+c", "quit"},
		{"", ""},
		{"viewer mode:", ""},
		{"j/k ↑↓ / wheel", "line down/up"},
		{"n / N", "next / previous target"},
		{"g / G", "top / bottom of file"},
		{"ctrl+d / ctrl+u", "half-page down/up"},
		{"e (or ctrl+e)", "open $EDITOR at current line"},
		{"esc / q", "back to list"},
		{"", ""},
		{"param form:", ""},
		{"↑↓ / tab", "move between fields"},
		{"←→ / space", "cycle enum, toggle bool"},
		{"typing / ctrl+u", "edit text field / clear"},
		{"enter", "emit make command"},
		{"esc", "back to list"},
		{"", ""},
		{"scaffold form (ctrl+a):", ""},
		{"↑↓ / tab", "move between fields (any param)"},
		{"←→ / space", "cycle type, toggle required"},
		{"typing / ctrl+u", "edit name/default/options/desc text"},
		{"ctrl+n", "add a new param row (manual)"},
		{"ctrl+d", "delete the focused param row"},
		{"enter", "write formatted @param lines to Makefile"},
		{"ctrl+e", "write, then open $EDITOR for tweaks"},
		{"esc", "cancel, no file changes"},
	}

	var rows []string
	rows = append(rows, top)
	rows = append(rows, rule)
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

	if len(rows) > h {
		rows = rows[:h]
	}
	for len(rows) < h {
		rows = append(rows, "")
	}
	for i, r := range rows {
		lw := lipgloss.Width(r)
		if lw < w {
			rows[i] = r + strings.Repeat(" ", w-lw)
		}
	}
	return strings.Join(rows, "\n")
}
