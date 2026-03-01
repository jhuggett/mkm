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

	borderStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	previewBorderStyle = lipgloss.NewStyle().
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

	previewLabelStyle = lipgloss.NewStyle().
				Foreground(subtleColor)

	previewNameStyle = lipgloss.NewStyle().
				Foreground(hiColor).
				Bold(true)
)

type model struct {
	groups   []TargetGroup
	flat     []MakeTarget
	filtered []int
	cursor   int
	selected bool
	width    int
	height   int
	filter   string
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
	return model{groups: groups, flat: flat, filtered: filtered, width: 80, height: 24}
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

// renderTargetList renders the left pane: grouped targets with fuzzy highlighting and scrolling.
func (m model) renderTargetList(w, h int) string {
	var lines []string
	cursorLine := 0

	if len(m.filtered) == 0 {
		lines = append(lines, noMatchStyle.Render("no matches"))
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

			// Add spacing between groups (not before the first)
			if len(lines) > 0 {
				lines = append(lines, "")
			}

			if g.Dir != "." {
				lines = append(lines, groupHeaderStyle.Render(g.Dir+"/"))
			} else if gi == 0 {
				// Don't label the root group at all if it's first
			}

			for _, t := range g.Targets {
				if visibleSet[flatIdx] {
					label := t.Name
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

	// Scroll: pick a window of h lines around the cursor
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

	// Pad to fill height
	for len(lines) < h {
		lines = append(lines, "")
	}

	// Truncate each line to width
	for i, line := range lines {
		lw := lipgloss.Width(line)
		if lw < w {
			lines[i] = line + strings.Repeat(" ", w-lw)
		}
	}

	return strings.Join(lines, "\n")
}

// renderPreview renders the right pane: info about the highlighted target.
func (m model) renderPreview(w, h int) string {
	border := lipgloss.RoundedBorder()
	bs := previewBorderStyle

	// Inner dimensions (subtract border + 1 char padding each side)
	innerW := w - 4
	innerH := h - 2
	if innerW < 5 || innerH < 3 {
		return ""
	}

	var contentLines []string

	if len(m.filtered) > 0 {
		target := m.flat[m.filtered[m.cursor]]

		// Target name
		fullName := target.Name
		if target.Dir != "." {
			fullName = target.Dir + "/" + target.Name
		}
		contentLines = append(contentLines, previewNameStyle.Render(fullName))
		contentLines = append(contentLines, "")

		// Description
		if target.Description != "" {
			wrapped := wordWrap(target.Description, innerW)
			for _, wl := range strings.Split(wrapped, "\n") {
				contentLines = append(contentLines, descStyle.Render(wl))
			}
			contentLines = append(contentLines, "")
		}

		// Dependencies
		if len(target.Dependencies) > 0 {
			depsStr := strings.Join(target.Dependencies, ", ")
			contentLines = append(contentLines, depsLabelStyle.Render("deps ")+depsValueStyle.Render(depsStr))
			contentLines = append(contentLines, "")
		}

		// Recipe
		if len(target.Recipe) > 0 {
			for _, rl := range target.Recipe {
				if lipgloss.Width(rl) > innerW {
					rl = rl[:innerW-1] + "…"
				}
				contentLines = append(contentLines, recipeStyle.Render(rl))
			}
		} else {
			contentLines = append(contentLines, noMatchStyle.Render("(no recipe)"))
		}
	}

	// Trim to available height
	if len(contentLines) > innerH {
		contentLines = contentLines[:innerH]
	}

	// Pad to fill inner height
	for len(contentLines) < innerH {
		contentLines = append(contentLines, "")
	}

	// Pad each line to inner width
	for i, line := range contentLines {
		lw := lipgloss.Width(line)
		if lw < innerW {
			contentLines[i] = line + strings.Repeat(" ", innerW-lw)
		}
	}

	// Build the bordered preview box manually
	label := " preview "
	topFill := innerW + 2 - lipgloss.Width(label) - 1 // +2 for padding, -1 for left dash
	if topFill < 0 {
		topFill = 0
	}
	top := bs.Render(border.TopLeft+"─") +
		previewLabelStyle.Render(label) +
		bs.Render(strings.Repeat("─", topFill)+border.TopRight)

	bottom := bs.Render(border.BottomLeft + strings.Repeat("─", innerW+2) + border.BottomRight)

	var rows []string
	rows = append(rows, top)
	for _, cl := range contentLines {
		rows = append(rows, bs.Render(border.Left)+" "+cl+" "+bs.Render(border.Right))
	}
	rows = append(rows, bottom)

	return strings.Join(rows, "\n")
}

// renderPanel constructs the outer bordered panel with title in top border and help in bottom.
func (m model) renderPanel(body string, panelW int) string {
	border := lipgloss.RoundedBorder()
	bs := borderStyle

	innerW := panelW - 2 // subtract left + right border chars

	// Top border with title
	var titleTag string
	var titleLen int
	if m.filter == "" {
		titleTag = bs.Render("[ ") + titleStyle.Render("mkm") + bs.Render(" ]")
		titleLen = 7 // "[ mkm ]"
	} else {
		titleTag = bs.Render("[ ") + titleStyle.Render("mkm") + " " +
			filterPromptStyle.Render("/") +
			filterStyle.Render(m.filter) + bs.Render(" ]")
		titleLen = 7 + 1 + len(m.filter) // "[ mkm /filter ]"
	}
	topFill := innerW - titleLen - 2 // 2 for leading "──"
	if topFill < 0 {
		topFill = 0
	}
	top := bs.Render(border.TopLeft+"──") + titleTag + bs.Render(strings.Repeat("─", topFill)+border.TopRight)

	// Bottom border with help
	var helpText string
	if m.filter == "" {
		helpText = helpKeyStyle.Render("type to filter") + bs.Render("  ") +
			helpKeyStyle.Render("↑↓") + bs.Render("  ") +
			helpKeyStyle.Render("enter") + bs.Render("  ") +
			helpKeyStyle.Render("esc")
	} else {
		helpText = helpKeyStyle.Render("/"+m.filter) + bs.Render("  ") +
			helpKeyStyle.Render("↑↓") + bs.Render("  ") +
			helpKeyStyle.Render("enter") + bs.Render("  ") +
			helpKeyStyle.Render("bksp") + bs.Render("  ") +
			helpKeyStyle.Render("esc")
	}
	helpLen := lipgloss.Width(helpText)
	botFill := innerW - helpLen - 4 // 4 for "[ " and " ]"
	if botFill < 0 {
		botFill = 0
	}
	bottom := bs.Render(border.BottomLeft+strings.Repeat("─", botFill)) +
		bs.Render("[ ") + helpText + bs.Render(" ]") +
		bs.Render(border.BottomRight)

	// Body rows with side borders and padding
	bodyLines := strings.Split(body, "\n")
	var rows []string
	rows = append(rows, top)
	// Top padding row
	rows = append(rows, bs.Render(border.Left)+strings.Repeat(" ", innerW)+bs.Render(border.Right))
	for _, bl := range bodyLines {
		lw := lipgloss.Width(bl)
		pad := innerW - 2 - lw // 2 for left padding
		if pad < 0 {
			pad = 0
		}
		rows = append(rows, bs.Render(border.Left)+"  "+bl+strings.Repeat(" ", pad)+bs.Render(border.Right))
	}
	// Bottom padding row
	rows = append(rows, bs.Render(border.Left)+strings.Repeat(" ", innerW)+bs.Render(border.Right))
	rows = append(rows, bottom)

	return strings.Join(rows, "\n")
}

func (m model) View() string {
	if m.selected {
		return ""
	}

	// Calculate panel dimensions
	panelW := int(float64(m.width) * 0.65)
	panelH := int(float64(m.height) * 0.70)
	panelW = clamp(panelW, 50, m.width-4)
	panelH = clamp(panelH, 15, m.height-2)

	// Inner area (subtract border, padding rows)
	innerW := panelW - 2 - 4 // border(2) + left padding indent(4)
	innerH := panelH - 4     // border(2) + padding rows(2)
	if innerW < 20 {
		innerW = 20
	}
	if innerH < 5 {
		innerH = 5
	}

	// Split into left list and right preview
	leftW := int(float64(innerW) * 0.50)
	rightW := innerW - leftW - 2 // 2 char gap

	leftContent := m.renderTargetList(leftW, innerH)
	rightContent := m.renderPreview(rightW, innerH)

	// Join horizontally with gap
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftContent, "  ", rightContent)

	panel := m.renderPanel(body, panelW)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
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

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
