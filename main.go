package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termenv "github.com/muesli/termenv"
)

var (
	accent      = lipgloss.Color("#d08770")
	dimColor    = lipgloss.Color("#4c566a")
	subtleColor = lipgloss.Color("#3b4252")
	textColor   = lipgloss.Color("#7a8490")
	hiColor     = lipgloss.Color("#d8dee9")
	cyanColor   = lipgloss.Color("#5e81ac")
	greenColor  = lipgloss.Color("#a3be8c")
	purpleColor = lipgloss.Color("#b48ead")
	yellowColor = lipgloss.Color("#ebcb8b")

	titleStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	filterStyle = lipgloss.NewStyle().
			Foreground(hiColor).
			Bold(true)

	filterPromptStyle = lipgloss.NewStyle().
				Foreground(accent)

	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(cyanColor)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(hiColor).
				Bold(true)

	selectedCursorStyle = lipgloss.NewStyle().
				Foreground(accent).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(textColor)

	matchStyle = lipgloss.NewStyle().
			Foreground(accent)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	ruleStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	descStyle = lipgloss.NewStyle().
			Foreground(yellowColor).
			Italic(true)

	depsLabelStyle = lipgloss.NewStyle().
			Foreground(purpleColor).
			Bold(true)

	depsValueStyle = lipgloss.NewStyle().
			Foreground(textColor)

	recipeStyle = lipgloss.NewStyle().
			Foreground(greenColor)

	noMatchStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)

	previewNameStyle = lipgloss.NewStyle().
				Foreground(hiColor).
				Bold(true)
)

type model struct {
	groups      []TargetGroup
	flat        []MakeTarget
	filtered    []int
	cursor      int
	selected    bool
	width       int
	height      int
	filter      string
	showPreview bool
}

func newModel(groups []TargetGroup) model {
	var flat []MakeTarget
	for _, g := range groups {
		flat = append(flat, g.Targets...)
	}
	filtered := make([]int, len(flat))
	for i := range flat {
		filtered[i] = i
	}
	return model{
		groups:      groups,
		flat:        flat,
		filtered:    filtered,
		width:       80,
		height:      24,
		showPreview: true,
	}
}

func fuzzyMatch(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)
	pi := 0
	for i := 0; i < len(s) && pi < len(pattern); i++ {
		if s[i] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

func fuzzyHighlight(s, pattern string, base, highlight lipgloss.Style) string {
	if pattern == "" {
		return base.Render(s)
	}
	lower := strings.ToLower(s)
	pat := strings.ToLower(pattern)
	var b strings.Builder
	pi := 0
	run := []byte{}
	matched := false
	for i := 0; i < len(s); i++ {
		if pi < len(pat) && lower[i] == pat[pi] {
			if !matched && len(run) > 0 {
				b.WriteString(base.Render(string(run)))
				run = run[:0]
			}
			matched = true
			run = append(run, s[i])
			pi++
		} else {
			if matched && len(run) > 0 {
				b.WriteString(highlight.Render(string(run)))
				run = run[:0]
			}
			matched = false
			run = append(run, s[i])
		}
	}
	if len(run) > 0 {
		if matched {
			b.WriteString(highlight.Render(string(run)))
		} else {
			b.WriteString(base.Render(string(run)))
		}
	}
	return b.String()
}

func (m *model) updateFilter() {
	m.filtered = m.filtered[:0]
	for i, t := range m.flat {
		label := t.Name
		if t.Dir != "." {
			label = t.Dir + "/" + t.Name
		}
		if fuzzyMatch(label, m.filter) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
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
				m.selected = true
				return m, tea.Quit
			}
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
	}
	return m, nil
}

// truncateStr truncates s to fit within width visible characters, adding … if truncated.
func truncateStr(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return s[:width-1] + "…"
}

// renderTargetList renders the grouped target list filling w x h exactly.
func (m model) renderTargetList(w, h int) string {
	var lines []string
	cursorLine := 0

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

			labelW := w - 3
			if labelW < 1 {
				labelW = 1
			}
			for _, t := range g.Targets {
				if visibleSet[flatIdx] {
					label := truncateStr(t.Name, labelW)
					if filteredIdx == m.cursor {
						cursorLine = len(lines)
						hl := fuzzyHighlight(label, m.filter, selectedItemStyle, selectedCursorStyle)
						lines = append(lines, selectedCursorStyle.Render(" > ")+hl)
					} else {
						hl := fuzzyHighlight(label, m.filter, normalItemStyle, matchStyle)
						lines = append(lines, "   "+hl)
					}
					filteredIdx++
				}
				flatIdx++
			}
		}
	}

	if len(lines) > h {
		offset := cursorLine - h/3
		if offset < 0 {
			offset = 0
		}
		if offset > len(lines)-h {
			offset = len(lines) - h
		}
		lines = lines[offset : offset+h]
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

	// Gap between filter and help keys.
	gap := 2
	avail := w - leftW - helpW - gap
	if avail < 0 {
		// Not enough room for help — drop it.
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
	}
	if m.showPreview {
		segs = append(segs, helpKeyStyle.Render("tab:hide"))
	} else {
		segs = append(segs, helpKeyStyle.Render("tab:show"))
	}
	segs = append(segs, helpKeyStyle.Render("esc"))
	return strings.Join(segs, gap)
}

func (m model) View() string {
	if m.selected {
		return ""
	}

	w := m.width
	h := m.height
	if w < 10 || h < 3 {
		return ""
	}

	top := m.renderTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))

	// Top prompt + rule eat 2 rows.
	remain := h - 2
	if remain < 1 {
		return top + "\n" + rule
	}

	var parts []string
	parts = append(parts, top)
	parts = append(parts, rule)

	// Fixed preview height (≈ a third of the vertical space) keeps the list
	// above from shifting as the cursor moves across targets with different
	// amounts of metadata.
	if m.showPreview && len(m.filtered) > 0 && remain >= 8 {
		previewH := remain / 3
		if previewH < 4 {
			previewH = 4
		}
		target := m.flat[m.filtered[m.cursor]]
		listH := remain - 1 - previewH // 1 for the rule between list and preview
		parts = append(parts, m.renderTargetList(w, listH))
		parts = append(parts, rule)
		parts = append(parts, renderInlinePreview(target, w, previewH))
		return strings.Join(parts, "\n")
	}

	parts = append(parts, m.renderTargetList(w, remain))
	return strings.Join(parts, "\n")
}

func wordWrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func makeCmd(target MakeTarget) []string {
	if target.Dir == "." {
		return []string{"make", target.Name}
	}
	return []string{"make", "-C", target.Dir, target.Name}
}

func main() {
	makefiles, err := findMakefiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error finding Makefiles:", err)
		os.Exit(1)
	}

	if len(makefiles) == 0 {
		fmt.Fprintln(os.Stderr, "No Makefiles found.")
		os.Exit(1)
	}

	var allTargets []MakeTarget
	for _, makefile := range makefiles {
		targets, err := parseMakefileTargets(makefile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing Makefile %s: %v\n", makefile, err)
			os.Exit(1)
		}
		allTargets = append(allTargets, targets...)
	}

	if len(allTargets) == 0 {
		fmt.Fprintln(os.Stderr, "No targets found in Makefiles.")
		os.Exit(1)
	}

	groups := groupTargets(allTargets)

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening TTY:", err)
		os.Exit(1)
	}
	defer tty.Close()

	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		newModel(groups),
		tea.WithInput(tty),
		tea.WithOutput(tty),
		tea.WithAltScreen(),
	)

	m, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running program:", err)
		os.Exit(1)
	}

	finalModel := m.(model)
	if finalModel.selected {
		target := finalModel.flat[finalModel.filtered[finalModel.cursor]]
		fmt.Println(strings.Join(makeCmd(target), " "))
	}
}
