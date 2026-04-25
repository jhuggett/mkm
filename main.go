package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termenv "github.com/muesli/termenv"
)

// toastKind identifies which transient string a toastClearMsg targets.
// Kept narrow — toasts that share a kind clobber each other (intended).
type toastKind int

const (
	toastUpdate toastKind = iota // model.updateMsg ("copied", "copy failed")
	toastClip                    // settingsState.clipMsg
	toastSaved                   // model.savedMsg ("config saved")
	toastFormErr                 // formState.errMsg ("missing required: ...")
	toastScafErr                 // scaffoldState.errMsg
)

// toastClearMsg fires after toastDuration and clears the named toast iff
// its sequence number still matches the latest set — so fast successive
// toasts don't get pre-cleared by an older tick.
type toastClearMsg struct {
	kind toastKind
	seq  int
}

const toastDuration = 2 * time.Second

func clearToastCmd(kind toastKind, seq int) tea.Cmd {
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastClearMsg{kind: kind, seq: seq}
	})
}

// toastUpdate sets the update-banner toast (e.g. "copied: ...") and
// schedules its clearing.
func (m model) toastUpdate(text string) (tea.Model, tea.Cmd) {
	m.updateMsg = text
	m.updateMsgSeq++
	return m, clearToastCmd(toastUpdate, m.updateMsgSeq)
}

// toastSaved sets the cmdline "config saved" toast on the list page.
func (m model) toastSaved(text string) (tea.Model, tea.Cmd) {
	m.savedMsg = text
	m.savedMsgSeq++
	return m, clearToastCmd(toastSaved, m.savedMsgSeq)
}

type model struct {
	groups       []TargetGroup
	flat         []MakeTarget
	filtered     []int
	cursor       int
	selected     bool
	width        int
	height       int
	filter       string
	showPreview  bool
	showHelp     bool
	form         *formState
	viewer       *viewerState
	scaffold     *scaffoldState
	settings     *settingsState
	history      map[string]int64
	writeHistory bool
	shellHistory bool
	checkUpdates bool
	updateInfo    updateInfo
	updateMsg     string // transient feedback after ctrl+u
	updateMsgSeq  int
	bannerHidden  bool   // true after ^x dismisses the update banner this session
	savedMsg      string // transient "config saved" toast on the list cmdline
	savedMsgSeq   int
	helpScroll    int // top visible row of the help cheatsheet
	layout        *listLayoutCache
}

func newModel(groups []TargetGroup, cfg Config) model {
	var flat []MakeTarget
	for _, g := range groups {
		flat = append(flat, g.Targets...)
	}
	filtered := make([]int, len(flat))
	for i := range flat {
		filtered[i] = i
	}
	// When history is disabled, skip the load entirely — recency ranking just
	// uses an empty map so every target scores equally on freshness.
	var hist map[string]int64
	if cfg.WriteHistory {
		hist = loadHistory()
	} else {
		hist = map[string]int64{}
	}
	return model{
		groups:       groups,
		flat:         flat,
		filtered:     filtered,
		width:        80,
		height:       24,
		showPreview:  true,
		history:      hist,
		writeHistory: cfg.WriteHistory,
		shellHistory: cfg.ShellHistory,
		checkUpdates: cfg.CheckUpdates,
		layout:       &listLayoutCache{rowToIdx: map[int]int{}},
	}
}

func (m model) Init() tea.Cmd {
	// Kick the update check in the background at startup; result arrives
	// via updateCheckMsg. Skipped in dev builds (handled inside the cmd)
	// and when the user opted out.
	if m.checkUpdates {
		return checkForUpdateCmd()
	}
	return nil
}

// Update dispatches to mode-specific handlers in list.go / form.go /
// viewer.go. This file just owns the top-level routing.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case updateCheckMsg:
		m.updateInfo = msg.info
		return m, nil
	case toastClearMsg:
		switch msg.kind {
		case toastUpdate:
			if msg.seq == m.updateMsgSeq {
				m.updateMsg = ""
			}
		case toastSaved:
			if msg.seq == m.savedMsgSeq {
				m.savedMsg = ""
			}
		case toastClip:
			if m.settings != nil && msg.seq == m.settings.clipMsgSeq {
				m.settings.clipMsg = ""
			}
		case toastFormErr:
			if m.form != nil && msg.seq == m.form.errMsgSeq {
				m.form.errMsg = ""
			}
		case toastScafErr:
			if m.scaffold != nil && msg.seq == m.scaffold.errMsgSeq {
				m.scaffold.errMsg = ""
			}
		}
		return m, nil
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
		// Editor may also have been pointed at the user's shell rc file
		// from the shell_history row — refresh that status so the settings
		// screen doesn't show stale info after a manual edit.
		m.settings.refreshShareHistStatus()
		return m, nil
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		if m.showHelp {
			return m.updateHelp(msg)
		}
		if m.scaffold != nil {
			return m.updateScaffold(msg)
		}
		if m.viewer != nil {
			return m.updateViewer(msg)
		}
		if m.settings != nil {
			return m.updateSettings(msg)
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
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.form.focus = (m.form.focus - 1 + len(m.form.params)) % len(m.form.params)
		case tea.MouseButtonWheelDown:
			m.form.focus = (m.form.focus + 1) % len(m.form.params)
		}
		return m, nil
	}
	if m.settings != nil {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.settings.focus = (m.settings.focus - 1 + settingsFieldCount) % settingsFieldCount
		case tea.MouseButtonWheelDown:
			m.settings.focus = (m.settings.focus + 1) % settingsFieldCount
		}
		return m, nil
	}
	if m.scaffold != nil {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scaffold.advance(-1)
		case tea.MouseButtonWheelDown:
			m.scaffold.advance(1)
		}
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
	if m.settings != nil {
		return m.renderSettingsView(w, h)
	}
	if m.form != nil {
		return m.renderFormView(w, h)
	}

	top := m.renderTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))
	banner := m.renderUpdateBanner(w)
	cmdLine := m.renderListCmdLine(w)
	legend := m.renderListLegend(w)

	// Banner (when present) sits between the top line and the rule; its
	// row shifts the list body down by one and eats into the available
	// list height.
	bannerRows := 0
	if banner != "" {
		bannerRows = 1
	}

	// Fixed bottom strip: rule + cmdline + legend.
	fixed := 2 + bannerRows + 3
	remain := h - fixed
	if remain < 1 {
		// Degraded: just show the top + rule + legend so we still have key hints.
		if banner != "" {
			return strings.Join([]string{top, banner, rule, legend}, "\n")
		}
		return strings.Join([]string{top, rule, legend}, "\n")
	}

	var parts []string
	parts = append(parts, top)
	if banner != "" {
		parts = append(parts, banner)
	}
	parts = append(parts, rule)

	// Mouse click-to-row math: list body begins right after the rule.
	if m.layout != nil {
		m.layout.firstRow = 2 + bannerRows
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
	} else {
		parts = append(parts, m.renderTargetList(w, remain))
	}

	parts = append(parts, rule, cmdLine, legend)
	return strings.Join(parts, "\n")
}

// runLastTarget resolves the most-recent history entry against the
// freshly-parsed target list and runs it without showing the TUI.
// Designed for `mkm --last`: muscle memory for "do the same thing
// again". Exits non-zero if there's no history or the recorded target
// no longer exists in the cwd.
func runLastTarget(allTargets []MakeTarget, cfg Config, printFlag bool) {
	dir, name, ok := lastHistoryEntry()
	if !ok {
		fmt.Fprintln(os.Stderr, "mkm --last: no history recorded yet — run a target normally first.")
		os.Exit(1)
	}
	var target *MakeTarget
	for i := range allTargets {
		if allTargets[i].Dir == dir && allTargets[i].Name == name {
			target = &allTargets[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintf(os.Stderr, "mkm --last: previous target %q in %q is no longer present.\n", name, dir)
		os.Exit(1)
	}
	if cfg.WriteHistory {
		appendHistory(*target)
	}
	args := makeCmd(*target, nil, nil)
	if printFlag {
		fmt.Println(strings.Join(args, " "))
		return
	}
	if cfg.ShellHistory {
		appendShellHistory(strings.Join(args, " "))
	}
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
	// Default: mkm executes the selected command itself. --print restores
	// the old stdout-only mode so the legacy zsh/bash wrapper in README
	// keeps working. -r/--run are kept as no-op aliases — executing is now
	// the default, but scripts that still pass the flag continue to work.
	var printFlag, runFlag, lastFlag bool
	flag.BoolVar(&printFlag, "p", false, "print the selected command to stdout instead of executing it (for the legacy shell wrapper)")
	flag.BoolVar(&printFlag, "print", false, "print the selected command to stdout instead of executing it (for the legacy shell wrapper)")
	flag.BoolVar(&runFlag, "r", false, "run the selected command directly (now the default — kept for backward compat)")
	flag.BoolVar(&runFlag, "run", false, "run the selected command directly (now the default — kept for backward compat)")
	flag.BoolVar(&lastFlag, "last", false, "skip the TUI and re-run the most recently recorded target")
	flag.Parse()
	_ = runFlag // no-op: exec is already the default

	cfg := loadConfig()
	applyTheme(cfg.Theme)

	allTargets, groups, err := collectTargets()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(allTargets) == 0 {
		fmt.Fprintln(os.Stderr, "No Makefile targets found.")
		os.Exit(1)
	}

	if lastFlag {
		runLastTarget(allTargets, cfg, printFlag)
		return
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening TTY:", err)
		os.Exit(1)
	}
	defer tty.Close()

	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		newModel(groups, cfg),
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
		if finalModel.writeHistory {
			appendHistory(*finalModel.form.target)
		}
		args = makeCmd(*finalModel.form.target, finalModel.form.params, finalModel.form.values)
	} else {
		target := finalModel.flat[finalModel.filtered[finalModel.cursor]]
		if finalModel.writeHistory {
			appendHistory(target)
		}
		args = makeCmd(target, nil, nil)
	}

	if printFlag {
		// Print mode: emit the command for the legacy wrapper to eval.
		// Shell-history recording is the wrapper's responsibility here.
		fmt.Println(strings.Join(args, " "))
		return
	}

	// Default: exec the make command directly, wiring std streams so the
	// user sees output like a normal `make` invocation. Optionally append
	// the command to $HISTFILE so up-arrow in future shells recalls it.
	if finalModel.shellHistory {
		appendShellHistory(strings.Join(args, " "))
	}
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
}
