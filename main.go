package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termenv "github.com/muesli/termenv"
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
	showHelp    bool
	form        *formState
	viewer      *viewerState
	scaffold    *scaffoldState
	history     map[string]int64
	layout      *listLayoutCache
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
		history:     loadHistory(),
		layout:      &listLayoutCache{rowToIdx: map[int]int{}},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// Update dispatches to mode-specific handlers in list.go / form.go /
// viewer.go. This file just owns the top-level routing.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case editorFinishedMsg:
		// Editor may have modified the Makefile; re-parse to pick up changes.
		if flat, groups, err := collectTargets(); err == nil {
			m.flat = flat
			m.groups = groups
			m.updateFilter()
			m.form = nil // form target pointer is now stale
			if m.viewer != nil {
				if err := m.viewer.reload(m.flat); err == nil {
					m.clampViewer()
				}
			}
		}
		return m, nil
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		if m.showHelp {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.showHelp = false
			return m, nil
		}
		if m.scaffold != nil {
			return m.updateScaffold(msg)
		}
		if m.viewer != nil {
			return m.updateViewer(msg)
		}
		if m.form != nil {
			return m.updateForm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if m.viewer != nil {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewer.cursor -= 3
			m.clampViewer()
		case tea.MouseButtonWheelDown:
			m.viewer.cursor += 3
			m.clampViewer()
		}
		return m, nil
	}
	if m.form != nil {
		return m, nil
	}
	// List mode: wheel moves cursor, left click sets cursor on clicked row.
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if len(m.filtered) > 0 && m.cursor > 0 {
			m.cursor--
		}
	case tea.MouseButtonWheelDown:
		if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case tea.MouseButtonLeft:
		if m.layout != nil {
			row := msg.Y - m.layout.firstRow
			if idx, ok := m.layout.rowToIdx[row]; ok {
				m.cursor = idx
			}
		}
	}
	return m, nil
}

// View dispatches to the right render fn based on current mode.
func (m model) View() string {
	if m.selected {
		return ""
	}

	w := m.width
	h := m.height
	if w < 10 || h < 3 {
		return ""
	}

	if m.showHelp {
		return m.renderHelpView(w, h)
	}
	if m.scaffold != nil {
		return m.renderScaffoldView(w, h)
	}
	if m.viewer != nil {
		return m.renderViewerView(w, h)
	}
	if m.form != nil {
		return m.renderFormView(w, h)
	}

	top := m.renderTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))

	remain := h - 2
	if remain < 1 {
		return top + "\n" + rule
	}

	var parts []string
	parts = append(parts, top)
	parts = append(parts, rule)

	// List body always begins on row 2 (row 0 = prompt, row 1 = rule).
	if m.layout != nil {
		m.layout.firstRow = 2
	}

	// Fixed preview height (≈ a third of the vertical space) keeps the list
	// above from shifting as the cursor moves across targets with different
	// amounts of metadata.
	if m.showPreview && len(m.filtered) > 0 && remain >= 8 {
		previewH := remain / 3
		if previewH < 4 {
			previewH = 4
		}
		target := m.flat[m.filtered[m.cursor]]
		listH := remain - 1 - previewH
		parts = append(parts, m.renderTargetList(w, listH))
		parts = append(parts, rule)
		parts = append(parts, renderInlinePreview(target, w, previewH))
		return strings.Join(parts, "\n")
	}

	parts = append(parts, m.renderTargetList(w, remain))
	return strings.Join(parts, "\n")
}

// collectTargets walks the cwd for Makefiles, parses each, and returns the
// flat list + grouped-by-dir view. Groups also carry the file-level
// @param declarations from their Makefile. Used at startup and after the
// editor exits so we can reflect on-disk changes without restarting mkm.
func collectTargets() ([]MakeTarget, []TargetGroup, error) {
	makefiles, err := findMakefiles()
	if err != nil {
		return nil, nil, err
	}
	type perFile struct {
		targets []MakeTarget
		params  []MakeParam
	}
	byDir := map[string]*perFile{}
	var all []MakeTarget
	for _, mf := range makefiles {
		ts, fileParams, err := parseMakefileTargets(mf)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", mf, err)
		}
		all = append(all, ts...)
		dir := filepathDir(mf)
		if len(ts) > 0 {
			dir = ts[0].Dir
		}
		if _, ok := byDir[dir]; !ok {
			byDir[dir] = &perFile{}
		}
		byDir[dir].targets = append(byDir[dir].targets, ts...)
		byDir[dir].params = append(byDir[dir].params, fileParams...)
	}

	groups := groupTargets(all)
	for i := range groups {
		if pf, ok := byDir[groups[i].Dir]; ok {
			groups[i].Params = pf.params
		}
	}
	annotateTargets(all, groups)
	return all, groups, nil
}

// filepathDir is a tiny wrapper so we don't need to import path/filepath
// just for one call here (main.go already imports os+strings etc.).
func filepathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

func main() {
	var runFlag bool
	flag.BoolVar(&runFlag, "r", false, "run the selected command directly instead of printing it to stdout")
	flag.BoolVar(&runFlag, "run", false, "run the selected command directly instead of printing it to stdout")
	flag.Parse()

	allTargets, groups, err := collectTargets()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(allTargets) == 0 {
		fmt.Fprintln(os.Stderr, "No Makefile targets found.")
		os.Exit(1)
	}

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
		tea.WithMouseCellMotion(),
	)

	m, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running program:", err)
		os.Exit(1)
	}

	finalModel := m.(model)
	if !finalModel.selected {
		return
	}

	var args []string
	if finalModel.form != nil {
		appendHistory(*finalModel.form.target)
		args = makeCmd(*finalModel.form.target, finalModel.form.params, finalModel.form.values)
	} else {
		target := finalModel.flat[finalModel.filtered[finalModel.cursor]]
		appendHistory(target)
		args = makeCmd(target, nil, nil)
	}

	if runFlag {
		// Exec the make command directly, wiring std streams so the user sees
		// output like a normal `make` invocation.
		c := exec.Command(args[0], args[1:]...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Default: print the command so the shell wrapper can eval it.
	fmt.Println(strings.Join(args, " "))
}
