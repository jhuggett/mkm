package main

import (
	"fmt"
	"strings"
	"time"

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
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Two-stage esc: clear an active filter first; quit only when
		// there's nothing left to dismiss. Matches the reflex of every
		// fuzzy picker.
		if m.filter != "" {
			m.filter = ""
			m.updateFilter()
			return m, nil
		}
		return m, tea.Quit
	case "ctrl+w":
		// Delete the previous word of the filter. Trims any trailing
		// whitespace first so successive ^w doesn't get stuck on a
		// space the user just typed.
		if m.filter != "" {
			f := strings.TrimRight(m.filter, " ")
			if i := strings.LastIndexByte(f, ' '); i >= 0 {
				m.filter = f[:i]
			} else {
				m.filter = ""
			}
			m.updateFilter()
		}
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
			CheckUpdates: m.checkUpdates,
		}
		m.settings = newSettingsState(cfg)
	case "ctrl+u":
		if m.updateInfo.Available && !m.bannerHidden {
			if err := copyToClipboard(installCommand()); err != nil {
				return m.toastUpdate("copy failed: " + err.Error())
			}
			return m.toastUpdate("copied: " + installCommand())
		}
	case "ctrl+x":
		if m.updateInfo.Available {
			m.bannerHidden = true
		}
	case "ctrl+y":
		// Copy the make command for the cursor target without running it.
		// Mirrors the cmdline preview verbatim so what you see is what
		// lands on the clipboard.
		if len(m.filtered) == 0 {
			break
		}
		target := m.flat[m.filtered[m.cursor]]
		cmd := strings.Join(makeCmd(target, nil, nil), " ")
		if err := copyToClipboard(cmd); err != nil {
			return m.toastSaved("copy failed: " + err.Error())
		}
		return m.toastSaved("copied: " + cmd)
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
// plus three single-cell indicators separated by spaces: " ◆ ◇ ⚙".
const listIndicatorWidth = 6

// descIndent is the left padding on a target's description row. Chosen so
// the description sits visually under the name (past the 4-char cursor
// prefix plus a small extra stagger) — makes it read as metadata.
const descIndent = 7

// renderDescLine returns the description row for a target, or "" if the
// target has no description. The `selected` flag elevates the line to
// the brighter textColor so the "selected unit" reads as a coherent
// two-row block (badge + name on top, description directly below in
// the same visual weight). Italic differentiates description text from
// target names regardless of selection state.
func renderDescLine(desc string, w int, selected bool) string {
	if desc == "" {
		return ""
	}
	avail := w - descIndent
	if avail < 1 {
		return ""
	}
	style := helpKeyStyle.Italic(true)
	if selected {
		style = lipgloss.NewStyle().Foreground(textColor).Italic(true)
	}
	return strings.Repeat(" ", descIndent) + style.Render(truncateStr(desc, avail))
}

// renderSelectedLabelRow assembles the badge + label + indicators for the
// highlighted target row. When there's room, a right-aligned dim ⏎ hint
// reinforces "press enter to run" without crowding the label area.
func renderSelectedLabelRow(label, indicators string, w int) string {
	badge := renderListCursor()
	left := badge + " " + label + indicators
	leftW := lipgloss.Width(left)

	hint := helpKeyStyle.Render("⏎ run")
	hintW := lipgloss.Width(hint)
	gap := w - leftW - hintW - 2
	if gap < 4 {
		// Not enough room for the right-side hint — drop it.
		return left
	}
	return left + strings.Repeat(" ", gap) + hint + "  "
}

// renderListCursor renders the eye-catching "this row is selected" badge
// shown left of the highlighted target name. A filled triangle on a
// solid accent-colored background reads as a play/run cue at a glance.
// Foreground uses subtleColor (the theme's darkest tone) so the badge
// holds high contrast against the bright accent fill in every theme,
// including light-fg ones like mono and tokyo-night.
func renderListCursor() string {
	return lipgloss.NewStyle().
		Background(accent).
		Foreground(subtleColor).
		Bold(true).
		Render(" ▶ ")
}

// rowStyleFor picks the unselected-row text style based on how recently
// the target was run. Recent (<24h) gets the brighter `recentItemStyle`;
// everything else stays at the default — making "what I just used"
// visually pop without dimming never-run rows so far that they blend
// into description text below them.
func (m model) rowStyleFor(t MakeTarget) lipgloss.Style {
	ts, ok := m.history[historyKey(t)]
	if ok && ts > 0 && time.Now().Unix()-ts < 86400 {
		return recentItemStyle
	}
	return normalItemStyle
}

// renderTargetIndicators returns a short styled string of glyphs that hint
// at a target's metadata state. Empty when the target has nothing notable.
//   ◆  target has applicable @param docs (its own or referenced file-level)
//   ◇  target has undocumented $(VAR) refs — scaffoldable with ctrl+a
//   ⚙  target is .PHONY (an action recipe, not a file-producing rule)
func renderTargetIndicators(t MakeTarget) string {
	var parts []string
	if t.HasParams {
		parts = append(parts, depsLabelStyle.Render("◆"))
	}
	if t.HasScaffold {
		parts = append(parts, descStyle.Render("◇"))
	}
	if t.Phony {
		parts = append(parts, helpKeyStyle.Render("⚙"))
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
	// headers maps a line index → group-header text. Used by finishListLayout
	// to pin the current group's header to the top of the viewport when the
	// user scrolls past it.
	headers := map[int]string{}

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
				// Header gets a faint trailing rule so the group has a
				// clear visual anchor, especially when several groups
				// stack on the same screen.
				dirText := groupHeaderStyle.Render(g.Dir + "/")
				dirW := lipgloss.Width(dirText)
				ruleW := w - dirW - 1
				headerText := dirText
				if ruleW > 4 {
					headerText += " " + ruleStyle.Render(strings.Repeat("─", ruleW))
				}
				headers[len(lines)] = headerText
				lines = append(lines, headerText)
			} else if gi == 0 {
				// Don't label the root group at all if it's first
			}

			labelW := w - 4 - listIndicatorWidth
			if labelW < 1 {
				labelW = 1
			}
			for _, t := range g.Targets {
				if visibleSet[flatIdx] {
					label := truncateStr(t.Name, labelW)
					indicators := renderTargetIndicators(t)
					mapping[len(lines)] = filteredIdx
					selected := filteredIdx == m.cursor
					if selected {
						cursorLine = len(lines)
						hl := fuzzyHighlight(label, m.filter, selectedItemStyle, selectedCursorStyle)
						lines = append(lines, renderSelectedLabelRow(hl, indicators, w))
					} else {
						hl := fuzzyHighlight(label, m.filter, m.rowStyleFor(t), matchStyle)
						lines = append(lines, "    "+hl+indicators)
					}
					if desc := renderDescLine(t.Description, w, selected); desc != "" {
						mapping[len(lines)] = filteredIdx
						lines = append(lines, desc)
					}
					filteredIdx++
				}
				flatIdx++
			}
		}
	}

	return m.finishListLayout(lines, mapping, headers, cursorLine, w, h)
}

func (m model) renderFlatList(w, h int) string {
	var lines []string
	cursorLine := 0
	mapping := map[int]int{}

	if len(m.filtered) == 0 {
		lines = append(lines, noMatchStyle.Render(truncateStr("no matches", w)))
		return m.finishListLayout(lines, mapping, nil, 0, w, h)
	}

	labelW := w - 4 - listIndicatorWidth
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
		selected := i == m.cursor
		if selected {
			cursorLine = len(lines)
			hl := fuzzyHighlight(label, m.filter, selectedItemStyle, selectedCursorStyle)
			lines = append(lines, renderSelectedLabelRow(hl, indicators, w))
		} else {
			hl := fuzzyHighlight(label, m.filter, m.rowStyleFor(t), matchStyle)
			lines = append(lines, "    "+hl+indicators)
		}
		if desc := renderDescLine(t.Description, w, selected); desc != "" {
			mapping[len(lines)] = i
			lines = append(lines, desc)
		}
	}

	return m.finishListLayout(lines, mapping, nil, cursorLine, w, h)
}

// finishListLayout applies scroll, height padding, and width padding, and
// writes the post-scroll row→filtered-idx mapping into m.layout so mouse
// clicks can resolve to a target. `headers` (nil for flat mode) maps a
// line index → group-header text; when the viewport scrolls past a
// header, finishListLayout pins it to the top row so the dir/ context
// stays visible. Truncation indicators (▲ N above / ▼ N more) replace
// the topmost / bottommost visible row when content is hidden.
func (m model) finishListLayout(lines []string, mapping map[int]int, headers map[int]string, cursorLine, w, h int) string {
	totalLines := len(lines)
	offset := 0
	if totalLines > h {
		offset = cursorLine - h/3
		if offset < 0 {
			offset = 0
		}
		if offset > totalLines-h {
			offset = totalLines - h
		}
	}

	// Take a copy so sticky/truncation overlays don't mutate the source.
	end := offset + h
	if end > totalLines {
		end = totalLines
	}
	visible := append([]string{}, lines[offset:end]...)
	hiddenTop := offset
	hiddenBot := totalLines - end

	// Sticky group header: when the most recent header sits above the
	// visible window, render it on row 0 (replacing whatever was there).
	// The same row also doubles as the "▲ N above" cue, so users get
	// both group context and scroll position from one line.
	stickyText := ""
	for i := offset - 1; i >= 0; i-- {
		if t, ok := headers[i]; ok {
			stickyText = t
			break
		}
	}
	suppressMappingTop := false
	if hiddenTop > 0 && len(visible) > 0 {
		visible[0] = renderStickyTop(stickyText, hiddenTop, w)
		suppressMappingTop = true
	}

	suppressMappingBot := false
	if hiddenBot > 0 && len(visible) > 0 {
		// Avoid clobbering the sticky row when the viewport is just one
		// or two lines tall — better to keep the header than the badge.
		idx := len(visible) - 1
		if !(suppressMappingTop && idx == 0) {
			visible[idx] = renderTruncationBottom(hiddenBot, w)
			suppressMappingBot = true
		}
	}

	if m.layout != nil {
		shifted := map[int]int{}
		topVis := offset
		botVis := offset + len(visible) - 1
		for k, v := range mapping {
			if k < topVis || k > botVis {
				continue
			}
			row := k - topVis
			if suppressMappingTop && row == 0 {
				continue
			}
			if suppressMappingBot && row == len(visible)-1 {
				continue
			}
			shifted[row] = v
		}
		m.layout.rowToIdx = shifted
	}

	for len(visible) < h {
		visible = append(visible, "")
	}
	for i, line := range visible {
		lw := lipgloss.Width(line)
		if lw < w {
			visible[i] = line + strings.Repeat(" ", w-lw)
		}
	}
	return strings.Join(visible, "\n")
}

// renderStickyTop renders the pinned-header / "▲ N above" row. When
// there's a known group header, the header text occupies the left and a
// dim "(N above)" annotation hugs the right; without a header, just the
// "▲ N above" badge centered-left.
func renderStickyTop(headerText string, hiddenAbove, w int) string {
	badge := helpKeyStyle.Render(fmt.Sprintf("▲ %d above", hiddenAbove))
	badgeW := lipgloss.Width(badge)
	if headerText == "" {
		return padLine("  "+badge, w)
	}
	headW := lipgloss.Width(headerText)
	pad := w - headW - badgeW - 2
	if pad < 1 {
		pad = 1
	}
	return headerText + strings.Repeat(" ", pad) + badge + " "
}

// renderTruncationBottom renders the "▼ N more" badge as a centered-left
// row, dim so it doesn't compete with target rows.
func renderTruncationBottom(hiddenBelow, w int) string {
	badge := helpKeyStyle.Render(fmt.Sprintf("▼ %d more", hiddenBelow))
	return padLine("  "+badge, w)
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

// renderTopLine renders the prompt line: `mkm v2.0.0 › <filter>` on the
// left, optional `N/M` match count on the right. The version is dim so
// it doesn't dominate. Pseudo-versions render as "(dev <hash>)" via
// displayVersion. Help keys live on the bottom legend row.
func (m model) renderTopLine(w int) string {
	version := helpKeyStyle.Render(" " + displayVersion())
	left := titleStyle.Render("mkm") + version + filterPromptStyle.Render(" › ")
	leftW := lipgloss.Width(left)

	// Right-side hint: match count when filtering, indicator legend
	// otherwise — gives the glyphs a discoverable home without an extra
	// row. Both forms use dim styling so they fade behind the prompt.
	var rightPart string
	if m.filter != "" {
		rightPart = helpKeyStyle.Render(fmt.Sprintf("%d/%d", len(m.filtered), len(m.flat)))
	} else {
		rightPart = depsLabelStyle.Render("◆") + helpKeyStyle.Render(" params  ") +
			descStyle.Render("◇") + helpKeyStyle.Render(" scaffoldable  ") +
			helpKeyStyle.Render("⚙ phony")
	}
	rightW := lipgloss.Width(rightPart)
	gap := 0
	if rightW > 0 {
		gap = 2
	}

	avail := w - leftW - rightW - gap
	if avail < 0 {
		// Drop the right-side hint entirely on tiny terminals.
		rightPart = ""
		rightW = 0
		gap = 0
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

	pad := w - leftW - filterW - rightW
	if pad < 0 {
		pad = 0
	}
	return left + filterPart + strings.Repeat(" ", pad) + rightPart
}

// renderUpdateBanner returns the one-line "update available" banner shown
// between the top line and the rule when a newer release is published.
// Returns "" when there's nothing to show or when the user has dismissed
// it for the session via ^x — the layout skips the row in that case.
// Styling is deliberately punchy (accent color + arrow glyph) so it
// stands out; the copy hint tells users how to dismiss it.
func (m model) renderUpdateBanner(w int) string {
	if !m.updateInfo.Available || m.bannerHidden {
		return ""
	}
	if m.updateMsg != "" {
		prefix := filterPromptStyle.Render(" ◆ ")
		return prefix + recipeStyle.Render(truncateStr(m.updateMsg, w-lipgloss.Width(prefix)))
	}
	prefix := filterPromptStyle.Render(" ▲ ")
	main := filterStyle.Render("update available: ") +
		titleStyle.Render(m.updateInfo.Latest) +
		normalItemStyle.Render("  — run  ") +
		recipeStyle.Render(installCommand()) +
		noMatchStyle.Render("  (") +
		helpKeyStyle.Render("^u") +
		noMatchStyle.Render(" copy, ") +
		helpKeyStyle.Render("^x") +
		noMatchStyle.Render(" hide)")
	line := prefix + main
	if lipgloss.Width(line) > w {
		// Compact form when the fat banner overflows the viewport.
		compact := prefix +
			filterStyle.Render("update: ") +
			titleStyle.Render(m.updateInfo.Latest) +
			noMatchStyle.Render("  (") +
			helpKeyStyle.Render("^u") +
			noMatchStyle.Render(" to copy install command)")
		if lipgloss.Width(compact) > w {
			return filterStyle.Render(truncateStr(" ▲ update: "+m.updateInfo.Latest+" (^u)", w))
		}
		return compact
	}
	return line
}

// renderListLegend is the bottom legend row for list mode.
func (m model) renderListLegend(w int) string {
	return renderLegend(w, []legendItem{
		{Key: "↑↓"},
		{Key: "type", Hint: "filter"},
		{Key: "enter", Hint: "run"},
		{Key: "^y", Hint: "copy"},
		{Key: "tab", Hint: "preview"},
		{Key: "^v", Hint: "view"},
		{Key: "^e", Hint: "edit"},
		{Key: "^a", Hint: "scaffold"},
		{Key: "^s", Hint: "settings"},
		{Key: "^g", Hint: "help"},
	})
}

// renderListCmdLine shows either a transient toast or the make command
// that pressing enter would run for the highlighted target. The command
// is rendered with a colored "run" pill on the left and tokenized args
// on the right (target in accent, VAR= in purple, values in green).
// When the target has @param docs, enter opens the form first — the
// line annotates that so the preview isn't misleading.
func (m model) renderListCmdLine(w int) string {
	if m.savedMsg != "" {
		pill := renderActionPill("✓", "done", greenColor)
		if strings.HasPrefix(m.savedMsg, "copy failed") {
			pill = renderActionPill("!", "fail", redColor)
		}
		return renderActionLine(w, pill, normalItemStyle.Render(m.savedMsg))
	}
	if len(m.filtered) == 0 {
		if m.filter != "" {
			pill := renderActionPill("∅", "none", dimColor)
			return renderActionLine(w, pill, helpKeyStyle.Render("no targets match `"+m.filter+"` — backspace or esc to reset"))
		}
		return strings.Repeat(" ", w)
	}
	target := m.flat[m.filtered[m.cursor]]
	args := makeCmd(target, nil, nil)
	body := renderMakeCmd(args)
	var hints []string
	if len(m.effectiveParams(target)) > 0 {
		hints = append(hints, "configure params")
	}
	if m.shellHistory {
		hints = append(hints, "+ shell history")
	}
	if len(hints) > 0 {
		body += helpKeyStyle.Render("   (" + strings.Join(hints, " · ") + ")")
	}
	pill := renderActionPill("⏎", "run", accent)
	return renderActionLine(w, pill, body)
}
