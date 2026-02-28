package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	accent     = lipgloss.Color("#d08770")
	dimColor   = lipgloss.Color("#4c566a")
	faintColor = lipgloss.Color("#2e3440")
	textColor  = lipgloss.Color("#7a8490")
	hiColor    = lipgloss.Color("#d8dee9")
	cyanColor  = lipgloss.Color("#5e81ac")

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

	barStyle = lipgloss.NewStyle().
			Foreground(faintColor)

	makingStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	noMatchStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)
)

type model struct {
	groups   []TargetGroup
	flat     []MakeTarget
	filtered []int // indices into flat
	cursor   int   // index into filtered
	selected bool
	width    int
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
	return model{groups: groups, flat: flat, filtered: filtered, width: 80}
}

// fuzzyMatch returns true if all characters in pattern appear in s in order (case-insensitive).
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

// fuzzyHighlight renders s with matched characters highlighted.
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
			// Single printable character -> append to filter
			r := msg.String()
			if len(r) == 1 && r[0] >= 32 && r[0] < 127 {
				m.filter += r
				m.updateFilter()
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.selected {
		target := m.flat[m.filtered[m.cursor]]
		label := target.Name
		if target.Dir != "." {
			label = target.Dir + "/" + target.Name
		}
		return makingStyle.Render("▸ "+label) + "\n"
	}

	w := m.width

	// Top bar: ── mkm ────────────── or ── mkm / filter ──────
	var topTag string
	var topTagLen int
	if m.filter == "" {
		topTag = titleStyle.Render(" mkm ")
		topTagLen = 5
	} else {
		topTag = titleStyle.Render(" mkm ") +
			filterPromptStyle.Render("/") +
			filterStyle.Render(m.filter) + " "
		topTagLen = 5 + 1 + len(m.filter) + 1
	}
	leftBar := barStyle.Render("──")
	rightBar := barStyle.Render(strings.Repeat("─", max(0, w-topTagLen-4)))
	topBar := leftBar + topTag + rightBar

	var lines []string
	lines = append(lines, topBar)

	if len(m.filtered) == 0 {
		lines = append(lines, noMatchStyle.Render("    no matches"))
	} else {
		// Build a set of which flat indices are visible
		visibleSet := map[int]bool{}
		for _, fi := range m.filtered {
			visibleSet[fi] = true
		}

		// Render grouped, but only show visible items
		filteredIdx := 0
		flatIdx := 0
		for _, g := range m.groups {
			// Check if this group has any visible items
			hasVisible := false
			for _, t := range g.Targets {
				_ = t
				if visibleSet[flatIdx] {
					hasVisible = true
				}
				flatIdx++
			}
			flatIdx -= len(g.Targets) // reset for actual render

			if !hasVisible {
				flatIdx += len(g.Targets)
				continue
			}

			if g.Dir != "." {
				lines = append(lines, groupHeaderStyle.Render("  "+g.Dir+"/"))
			}
			for _, t := range g.Targets {
				if visibleSet[flatIdx] {
					label := t.Name
					if filteredIdx == m.cursor {
						hl := fuzzyHighlight(label, m.filter, selectedItemStyle, makingStyle)
						lines = append(lines, selectedCursorStyle.Render("  > ")+hl)
					} else {
						hl := fuzzyHighlight(label, m.filter, normalItemStyle, matchStyle)
						lines = append(lines, "    "+hl)
					}
					filteredIdx++
				}
				flatIdx++
			}
		}
	}

	// Bottom bar
	var help string
	if m.filter == "" {
		help = helpKeyStyle.Render(" ↑↓ enter type to filter ")
	} else {
		help = helpKeyStyle.Render(" ↑↓ enter bksp clear ")
	}
	helpLen := lipgloss.Width(help)
	leftHelp := barStyle.Render("──")
	rightHelp := barStyle.Render(strings.Repeat("─", max(0, w-helpLen-4)))
	bottomBar := leftHelp + help + rightHelp

	lines = append(lines, bottomBar)

	return strings.Join(lines, "\n") + "\n"
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
	p := tea.NewProgram(newModel(groups), tea.WithOutput(os.Stderr))

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
