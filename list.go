package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// listLayoutCache lets View() share its row→target mapping with Update(),
// so a mouse click on a list row can resolve to a filtered index without
// duplicating the render logic. Stored behind a pointer so writes survive
// the by-value model copies that bubbletea makes.
type listLayoutCache struct {
	firstRow int         // absolute screen row where list body begins
	rowToIdx map[int]int // body row offset → filtered index
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		return m, tea.Quit
	case "ctrl+g":
		m.showHelp = true
	case "tab":
		m.showPreview = !m.showPreview
	case "up", "ctrl+p":
		if len(m.filtered) > 0 {
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.filtered) - 1
			}
		}
	case "down", "ctrl+n":
		if len(m.filtered) > 0 {
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		}
	case "enter":
		if len(m.filtered) > 0 {
			target := &m.flat[m.filtered[m.cursor]]
			effective := m.effectiveParams(*target)
			if len(effective) > 0 {
				m.form = newFormState(target, effective)
				return m, nil
			}
			m.selected = true
			return m, tea.Quit
		}
	case "ctrl+v":
		if len(m.filtered) > 0 {
			target := m.flat[m.filtered[m.cursor]]
			if v := m.newViewerState(target); v != nil {
				m.viewer = v
			}
		}
	case "ctrl+e":
		if len(m.filtered) > 0 {
			target := m.flat[m.filtered[m.cursor]]
			return m, editCmd(target.File, target.Line)
		}
	case "ctrl+a":
		if len(m.filtered) > 0 {
			target := m.flat[m.filtered[m.cursor]]
			m.scaffold = m.newScaffoldState(target)
		}
	case "ctrl+s":
		cfg := Config{
			Theme:        currentThemeName(),
			WriteHistory: m.writeHistory,
			ShellHistory: m.shellHistory,
		}
		m.settings = newSettingsState(cfg)
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.updateFilter()
		}
	default:
		r := msg.String()
		if len(r) == 1 && r[0] >= 32 && r[0] < 127 {
			m.filter += r
			m.updateFilter()
		}
	}
	return m, nil
}

// renderTargetList renders the target list into w × h. Filter empty →
// grouped view; filter non-empty → flat ranked view.
func (m model) renderTargetList(w, h int) string {
	if m.filter == "" {
		return m.renderGroupedList(w, h)
	}
	return m.renderFlatList(w, h)
}

// listIndicatorWidth is the number of cells reserved on the right of each
// target name row for the indicator glyphs. Enough for a leading space
// plus two single-cell indicators separated by a space: " ◆ ◇".
const listIndicatorWidth = 4

// descIndent is the left padding on a target's description row. Chosen so
// the description sits visually under the name (past the "   " / " > "
// prefix plus a small extra stagger) — makes it read as metadata.
const descIndent = 6

// renderDescLine returns the dim description row for a target, or "" if
// the target has no description. Truncated to fit within `w` cells.
func renderDescLine(desc string, w int) string {
	if desc == "" {
		return ""
	}
	avail := w - descIndent
	if avail < 1 {
		return ""
	}
	return strings.Repeat(" ", descIndent) + helpKeyStyle.Render(truncateStr(desc, avail))
}

// renderTargetIndicators returns a short styled string of glyphs that hint
// at a target's metadata state. Empty when the target has nothing notable.
//   ◆  target has applicable @param docs (its own or referenced file-level)
//   ◇  target has undocumented $(VAR) refs — scaffoldable with ctrl+a
func renderTargetIndicators(t MakeTarget) string {
	var parts []string
	if t.HasParams {
		parts = append(parts, depsLabelStyle.Render("◆"))
	}
	if t.HasScaffold {
		parts = append(parts, descStyle.Render("◇"))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func (m model) renderGroupedList(w, h int) string {
	var lines []string
	cursorLine := 0
	mapping := map[int]int{}

	if len(m.filtered) == 0 {
		lines = append(lines, noMatchStyle.Render(truncateStr("no matches", w)))
	} else {
		visibleSet := map[int]bool{}
		for _, fi := range m.filtered {
			visibleSet[fi] = true
		}

		filteredIdx := 0
		flatIdx := 0
		for gi, g := range m.groups {
			hasVisible := false
			for range g.Targets {
				if visibleSet[flatIdx] {
					hasVisible = true
				}
				flatIdx++
			}
			flatIdx -= len(g.Targets)

			if !hasVisible {
				flatIdx += len(g.Targets)
				continue
			}

			if len(lines) > 0 {
				lines = append(lines, "")
			}

			if g.Dir != "." {
				lines = append(lines, groupHeaderStyle.Render(truncateStr(g.Dir+"/", w)))
			} else if gi == 0 {
				// Don't label the root group at all if it's first
			}

			labelW := w - 3 - listIndicatorWidth
			if labelW < 1 {
				labelW = 1
			}
			for _, t := range g.Targets {
				if visibleSet[flatIdx] {
					label := truncateStr(t.Name, labelW)
					indicators := renderTargetIndicators(t)
					mapping[len(lines)] = filteredIdx
					if filteredIdx == m.cursor {
						cursorLine = len(lines)
						hl := fuzzyHighlight(label, m.filter, selectedItemStyle, selectedCursorStyle)
						lines = append(lines, selectedCursorStyle.Render(" > ")+hl+indicators)
					} else {
						hl := fuzzyHighlight(label, m.filter, normalItemStyle, matchStyle)
						lines = append(lines, "   "+hl+indicators)
					}
					if desc := renderDescLine(t.Description, w); desc != "" {
						mapping[len(lines)] = filteredIdx
						lines = append(lines, desc)
					}
					filteredIdx++
				}
				flatIdx++
			}
		}
	}

	return m.finishListLayout(lines, mapping, cursorLine, w, h)
}

func (m model) renderFlatList(w, h int) string {
	var lines []string
	cursorLine := 0
	mapping := map[int]int{}

	if len(m.filtered) == 0 {
		lines = append(lines, noMatchStyle.Render(truncateStr("no matches", w)))
		return m.finishListLayout(lines, mapping, 0, w, h)
	}

	labelW := w - 3 - listIndicatorWidth
	if labelW < 1 {
		labelW = 1
	}
	for i, idx := range m.filtered {
		t := m.flat[idx]
		label := t.Name
		if t.Dir != "." {
			label = t.Dir + "/" + t.Name
		}
		label = truncateStr(label, labelW)
		indicators := renderTargetIndicators(t)
		mapping[len(lines)] = i
		if i == m.cursor {
			cursorLine = len(lines)
			hl := fuzzyHighlight(label, m.filter, selectedItemStyle, selectedCursorStyle)
			lines = append(lines, selectedCursorStyle.Render(" > ")+hl+indicators)
		} else {
			hl := fuzzyHighlight(label, m.filter, normalItemStyle, matchStyle)
			lines = append(lines, "   "+hl+indicators)
		}
		if desc := renderDescLine(t.Description, w); desc != "" {
			mapping[len(lines)] = i
			lines = append(lines, desc)
		}
	}

	return m.finishListLayout(lines, mapping, cursorLine, w, h)
}

// finishListLayout applies scroll, height padding, and width padding, and
// writes the post-scroll row→filtered-idx mapping into m.layout so mouse
// clicks can resolve to a target.
func (m model) finishListLayout(lines []string, mapping map[int]int, cursorLine, w, h int) string {
	offset := 0
	if len(lines) > h {
		offset = cursorLine - h/3
		if offset < 0 {
			offset = 0
		}
		if offset > len(lines)-h {
			offset = len(lines) - h
		}
		lines = lines[offset : offset+h]
	}

	if m.layout != nil {
		shifted := map[int]int{}
		for k, v := range mapping {
			if k >= offset && k < offset+h {
				shifted[k-offset] = v
			}
		}
		m.layout.rowToIdx = shifted
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

// renderInlinePreview renders preview content for `target` into exactly w x h
// cells. Short content is padded with blank rows so the preview always takes
// the same height — this keeps the list above from resizing as the cursor
// moves across targets with different-sized metadata.
func renderInlinePreview(target MakeTarget, w, h int) string {
	if w < 4 || h < 1 {
		return ""
	}

	var lines []string

	fullName := target.Name
	if target.Dir != "." {
		fullName = target.Dir + "/" + target.Name
	}
	lines = append(lines, previewNameStyle.Render(truncateStr(fullName, w)))

	if target.Description != "" {
		wrapped := wordWrap(target.Description, w)
		for _, wl := range strings.Split(wrapped, "\n") {
			lines = append(lines, descStyle.Render(truncateStr(wl, w)))
		}
	}

	if len(target.Dependencies) > 0 {
		depsStr := strings.Join(target.Dependencies, ", ")
		prefix := "deps "
		avail := w - len(prefix)
		if avail > 0 {
			lines = append(lines, depsLabelStyle.Render(prefix)+depsValueStyle.Render(truncateStr(depsStr, avail)))
		}
	}

	if len(target.Recipe) > 0 {
		if len(lines) > 1 {
			lines = append(lines, "")
		}
		for _, rl := range target.Recipe {
			lines = append(lines, recipeStyle.Render(truncateStr(rl, w)))
		}
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

// renderTopLine renders the prompt line: `mkm › <filter>` on the left, help keys on the right.
func (m model) renderTopLine(w int) string {
	left := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ")
	leftW := lipgloss.Width(left)

	help := m.renderHelpKeys()
	helpW := lipgloss.Width(help)

	gap := 2
	avail := w - leftW - helpW - gap
	if avail < 0 {
		help = ""
		helpW = 0
		avail = w - leftW
		if avail < 0 {
			avail = 0
		}
	}

	var filterPart string
	if m.filter == "" {
		hint := "type to filter"
		if len(hint) <= avail {
			filterPart = noMatchStyle.Render(hint)
		}
	} else {
		display := m.filter
		if len(display) > avail {
			display = truncateStr(display, avail)
		}
		filterPart = filterStyle.Render(display)
	}
	filterW := lipgloss.Width(filterPart)

	pad := w - leftW - filterW - helpW
	if pad < 0 {
		pad = 0
	}

	return left + filterPart + strings.Repeat(" ", pad) + help
}

func (m model) renderHelpKeys() string {
	gap := ruleStyle.Render("  ")
	segs := []string{
		helpKeyStyle.Render("↑↓"),
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("^v:view"),
		helpKeyStyle.Render("^e:edit"),
		helpKeyStyle.Render("^a:scaf"),
		helpKeyStyle.Render("^s:set"),
		helpKeyStyle.Render("^g:help"),
	}
	return strings.Join(segs, gap)
}
