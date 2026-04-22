package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsState is the working copy of the user's config while the settings
// screen is open. Edits are kept separate from the live model config so esc
// can discard them. originalTheme is captured on open so we can revert the
// live theme preview when the user cancels.
//
// The shareHist* fields are snapshot state for the shell_history row — we
// detect on open, refresh after apply or after the user returns from the
// editor. This is intentionally not part of Config: it's not a user
// preference, it's an rc-file side effect we help the user set up as part
// of the same "make shell history work" feature.
type settingsState struct {
	focus               int
	cfg                 Config
	themes              []string
	originalTheme       string
	shareHistShell      string // "zsh" | "bash" | ""
	shareHistConfigured bool
	shareHistApplied    bool   // true after we wrote the snippet in this session
	shareHistErr        string
	clipMsg             string // transient feedback after a ctrl+y copy attempt
}

const (
	settingsFieldTheme = iota
	settingsFieldWriteHistory
	settingsFieldShellHistory
	settingsFieldCount
)

func newSettingsState(cfg Config) *settingsState {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	shell := detectedShell()
	return &settingsState{
		cfg:                 cfg,
		themes:              names,
		originalTheme:       cfg.Theme,
		shareHistShell:      shell,
		shareHistConfigured: shareHistConfigured(shell),
	}
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Transient copy-to-clipboard feedback clears on the next keystroke.
	if key != "ctrl+y" {
		m.settings.clipMsg = ""
	}
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		applyTheme(m.settings.originalTheme)
		m.settings = nil
		return m, nil
	case "enter":
		cfg := m.settings.cfg
		writeConfig(configPath(), cfg)
		applyTheme(cfg.Theme)
		// Toggling write_history mid-session takes effect now: reload or drop
		// the recency map so subsequent filtering scores match the new mode.
		if cfg.WriteHistory != m.writeHistory {
			if cfg.WriteHistory {
				m.history = loadHistory()
			} else {
				m.history = map[string]int64{}
			}
			m.writeHistory = cfg.WriteHistory
			m.updateFilter()
		}
		// Keep an installed managed wrapper in sync with the toggle so
		// turning shell_history off actually stops up-arrow from seeing
		// mkm commands. Legacy (unmanaged) wrappers aren't touched —
		// applyShareHistFromSettings surfaces those to the user.
		if m.settings.shareHistShell != "" && cfg.ShellHistory != m.shellHistory {
			_ = syncManagedWrapper(m.settings.shareHistShell, cfg.ShellHistory)
		}
		m.shellHistory = cfg.ShellHistory
		m.settings = nil
		return m, nil
	case "down", "tab":
		m.settings.focus = (m.settings.focus + 1) % settingsFieldCount
		return m, nil
	case "up", "shift+tab":
		m.settings.focus = (m.settings.focus - 1 + settingsFieldCount) % settingsFieldCount
		return m, nil
	}

	switch m.settings.focus {
	case settingsFieldTheme:
		switch key {
		case "left", "h":
			m.settings.cfg.Theme = cycleEnum(m.settings.themes, m.settings.cfg.Theme, -1)
			applyTheme(m.settings.cfg.Theme)
		case "right", "l", " ":
			m.settings.cfg.Theme = cycleEnum(m.settings.themes, m.settings.cfg.Theme, 1)
			applyTheme(m.settings.cfg.Theme)
		}
	case settingsFieldWriteHistory:
		if key == " " || key == "left" || key == "right" {
			m.settings.cfg.WriteHistory = !m.settings.cfg.WriteHistory
		}
	case settingsFieldShellHistory:
		switch key {
		case " ":
			m.settings.cfg.ShellHistory = !m.settings.cfg.ShellHistory
		case "ctrl+a":
			m.applyShareHistFromSettings()
		case "ctrl+e":
			if path := rcFilePath(m.settings.shareHistShell); path != "" {
				// Hand off to $EDITOR. When it returns, main's
				// editorFinishedMsg handler re-checks share-hist status so
				// the row reflects manual edits.
				return m, editCmd(path, 1)
			}
		case "ctrl+v":
			if path := rcFilePath(m.settings.shareHistShell); path != "" {
				if v := m.newViewerStateForPath(path); v != nil {
					m.viewer = v
				}
			}
		case "ctrl+y":
			snippet := strings.Trim(shareHistSnippet(m.settings.shareHistShell, m.settings.cfg.ShellHistory), "\n")
			if snippet == "" {
				m.settings.clipMsg = "no snippet available for this shell"
				break
			}
			if err := copyToClipboard(snippet); err != nil {
				m.settings.clipMsg = "copy failed: " + err.Error()
			} else {
				m.settings.clipMsg = "snippet copied to clipboard"
			}
		case "ctrl+r":
			rc := rcFilePath(m.settings.shareHistShell)
			if rc == "" {
				m.settings.clipMsg = "no rc file for this shell"
				break
			}
			cmd := "source " + abbreviateHome(rc)
			if err := copyToClipboard(cmd); err != nil {
				m.settings.clipMsg = "copy failed: " + err.Error()
			} else {
				m.settings.clipMsg = "copied: " + cmd
			}
		}
	}
	return m, nil
}

// applyShareHistFromSettings runs the rc-file fix inline from the
// shell_history row. Variant (with or without history push) comes from
// the current toggle. Works either as a fresh install or as an in-place
// rewrite of a managed block. Errors surface via shareHistErr.
func (m *model) applyShareHistFromSettings() {
	s := m.settings
	if s.shareHistShell == "" {
		return
	}
	// Legacy wrappers (no markers) — refuse to edit automatically; tell
	// the user to clean up manually.
	if s.shareHistConfigured && !hasManagedWrapper(s.shareHistShell) {
		s.shareHistErr = "existing mkm wrapper has no mkm markers — edit " + abbreviateHome(rcFilePath(s.shareHistShell)) + " manually (ctrl+e) or remove the old wrapper and retry"
		return
	}
	if err := applyShareHistFix(s.shareHistShell, s.cfg.ShellHistory); err != nil {
		s.shareHistErr = err.Error()
		return
	}
	s.shareHistConfigured = true
	s.shareHistApplied = true
	s.shareHistErr = ""
}

// refreshShareHistStatus re-runs the rc-file detection. Called from the
// editorFinishedMsg handler so manual edits to ~/.zshrc inside $EDITOR are
// reflected without having to close and reopen the settings screen.
func (s *settingsState) refreshShareHistStatus() {
	if s == nil || s.shareHistShell == "" {
		return
	}
	configured := shareHistConfigured(s.shareHistShell)
	if configured && !s.shareHistConfigured {
		// User set it up themselves in the editor.
		s.shareHistConfigured = true
	} else if !configured {
		// User removed it in the editor — surface the fix again.
		s.shareHistConfigured = false
		s.shareHistApplied = false
	}
	s.shareHistErr = ""
}

func (m model) renderSettingsView(w, h int) string {
	top := m.renderSettingsTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))

	path := configPath()
	if path == "" {
		path = "(config path unavailable)"
	}
	footer := padLine(helpKeyStyle.Render(truncateStr("config file: "+path, w)), w)

	bodyH := h - 4
	if bodyH < 1 {
		bodyH = 1
	}
	body := m.renderSettingsBody(w, bodyH)
	return strings.Join([]string{top, rule, body, rule, footer}, "\n")
}

func (m model) renderSettingsTopLine(w int) string {
	left := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ") + filterStyle.Render("settings")
	help := m.renderSettingsHelpKeys()
	pad := w - lipgloss.Width(left) - lipgloss.Width(help)
	if pad < 0 {
		pad = 0
	}
	return left + strings.Repeat(" ", pad) + help
}

func (m model) renderSettingsHelpKeys() string {
	gap := ruleStyle.Render("  ")
	segs := []string{
		helpKeyStyle.Render("↑↓"),
		helpKeyStyle.Render("←→"),
		helpKeyStyle.Render("enter:save"),
		helpKeyStyle.Render("esc:cancel"),
	}
	return strings.Join(segs, gap)
}

type settingsRow struct {
	name  string
	desc  string
	value string
}

func (m model) renderSettingsBody(w, h int) string {
	rows := []settingsRow{
		{
			name:  "theme",
			desc:  "color palette for the TUI",
			value: renderCycleValue(m.settings.cfg.Theme, m.settings.focus == settingsFieldTheme),
		},
		{
			name:  "write_history",
			desc:  "record selections to ~/.cache/mkm/history for recency ranking",
			value: renderBoolValue(m.settings.cfg.WriteHistory),
		},
		{
			name:  "shell_history",
			desc:  "push mkm commands into shell history — governs HISTFILE append AND the managed wrapper's print -s / history -s line",
			value: renderBoolValue(m.settings.cfg.ShellHistory),
		},
	}

	nameW := 0
	for _, r := range rows {
		if len(r.name) > nameW {
			nameW = len(r.name)
		}
	}

	var lines []string
	lines = append(lines, "")
	for i, r := range rows {
		cursor := "   "
		if i == m.settings.focus {
			cursor = selectedCursorStyle.Render(" › ")
		}
		nameStyled := depsLabelStyle.Render(padRight(r.name, nameW))
		lines = append(lines, cursor+nameStyled+"  "+r.value)

		descIndentW := 3 + nameW + 2
		avail := w - descIndentW
		if r.desc != "" && avail > 0 {
			lines = append(lines, strings.Repeat(" ", descIndentW)+noMatchStyle.Render(truncateStr(r.desc, avail)))
		}

		// shell_history row doubles as the rc-file integration control:
		// the rc-file status line is always shown so users can see whether
		// up-arrow will work. When the row is focused we also render the
		// snippet preview + apply/edit/view hints — the fool-proof mode.
		if i == settingsFieldShellHistory {
			for _, line := range m.renderShellHistStatusLines(w, descIndentW, m.settings.focus == settingsFieldShellHistory) {
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")
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

func renderBoolValue(on bool) string {
	if on {
		return recipeStyle.Render("[✓] on")
	}
	return noMatchStyle.Render("[ ] off")
}

// renderShellHistStatusLines is the bottom half of the shell_history row.
// Always shows a compact one-liner describing rc-file state; when focused,
// expands with a line-numbered preview of the exact insertion point, a
// post-apply "next step" block, and the action-key hints.
func (m model) renderShellHistStatusLines(w, indent int, focused bool) []string {
	s := m.settings
	if s == nil {
		return nil
	}
	pad := strings.Repeat(" ", indent)
	avail := w - indent
	if avail < 10 {
		return nil
	}

	var out []string

	// One-line status, always visible.
	out = append(out, pad+shareHistStatusLine(s, avail))

	if !focused {
		return out
	}

	// Post-apply next-step block — the thing that actually makes the fix
	// take effect in the current shell. Rendered prominently so it's hard
	// to miss.
	if s.shareHistApplied {
		out = append(out, "")
		for _, line := range m.renderReloadHint(s, avail) {
			out = append(out, pad+line)
		}
	}

	// Snippet preview with line numbers + file context. Only shown while a
	// fix is pending; once configured, there's nothing useful to preview.
	if s.shareHistShell != "" && !s.shareHistConfigured {
		out = append(out, "")
		out = append(out, m.renderSnippetPreview(s, w, indent)...)
	}

	if s.shareHistErr != "" {
		out = append(out, "")
		out = append(out, pad+diffRemoveStyle.Render(truncateStr("error: "+s.shareHistErr, avail)))
	}

	if s.clipMsg != "" {
		out = append(out, "")
		style := recipeStyle
		if strings.HasPrefix(s.clipMsg, "copy failed") || strings.HasPrefix(s.clipMsg, "no snippet") {
			style = diffRemoveStyle
		}
		out = append(out, pad+style.Render(truncateStr(s.clipMsg, avail)))
	}

	if hints := shellHistActionHints(s); hints != "" {
		out = append(out, "")
		out = append(out, pad+hints)
	}
	return out
}

// renderReloadHint is the "here's how to make this take effect" block
// shown right after a successful apply. Returns one or more full lines
// (without the leading pad applied by the caller). Includes an inline
// copy-key hint so the source command can be yanked with one keystroke.
func (m model) renderReloadHint(s *settingsState, w int) []string {
	rc := abbreviateHome(rcFilePath(s.shareHistShell))
	cmd := "source " + rc
	return []string{
		previewNameStyle.Render("Next: reload your shell so the wrapper takes effect."),
		helpKeyStyle.Render("  run ") + recipeStyle.Render(cmd) + helpKeyStyle.Render(" in your current shell — or just open a new one.  ") + helpKeyStyle.Render("(") + helpKeyStyle.Render("^r") + helpKeyStyle.Render(" to copy)"),
	}
}

// renderSnippetPreview builds a diff-style preview block: a clear header
// showing the target file path and exact line range we'd touch, a few
// lines of existing file content as context (with line numbers), and the
// snippet rendered as `+` additions (also numbered). An [EOF] marker makes
// the end-of-file append visually unambiguous.
func (m model) renderSnippetPreview(s *settingsState, w, indent int) []string {
	pad := strings.Repeat(" ", indent)
	avail := w - indent
	rcPath := rcFilePath(s.shareHistShell)
	rcShort := abbreviateHome(rcPath)

	// Snippet body, trimmed of leading/trailing blank lines for display —
	// the file write preserves those as a separator, but they don't add
	// anything in the preview.
	snippet := strings.Trim(shareHistSnippet(s.shareHistShell, s.cfg.ShellHistory), "\n")
	snippetLines := strings.Split(snippet, "\n")

	// Context = last contextBefore lines of the existing file (fewer if
	// the file is shorter or missing).
	const contextBefore = 3
	var ctxLines []string
	fileLineCount := 0
	fileMissing := true
	if data, err := os.ReadFile(rcPath); err == nil {
		fileMissing = false
		content := strings.TrimRight(string(data), "\n")
		if content == "" {
			fileLineCount = 0
		} else {
			existing := strings.Split(content, "\n")
			fileLineCount = len(existing)
			start := fileLineCount - contextBefore
			if start < 0 {
				start = 0
			}
			ctxLines = existing[start:]
		}
	}

	insertStart := fileLineCount + 1
	insertEnd := fileLineCount + len(snippetLines)

	// Header: file + exact lines being inserted.
	var header string
	switch {
	case fileMissing:
		header = fmt.Sprintf("will create %s and write lines 1–%d:", rcShort, len(snippetLines))
	case fileLineCount == 0:
		header = fmt.Sprintf("will append to empty file %s (lines 1–%d):", rcShort, len(snippetLines))
	default:
		header = fmt.Sprintf("will append to %s at lines %d–%d:", rcShort, insertStart, insertEnd)
	}

	// Gutter width driven by the highest line number we print.
	maxLN := insertEnd
	gutter := len(strconv.Itoa(maxLN))
	if gutter < 3 {
		gutter = 3
	}
	sep := diffGutterStyle.Render(" │ ")
	// `pad + N + sep + "+ " + text` — reserve space so snippet lines fit.
	textW := avail - gutter - lipgloss.Width(sep) - 2
	if textW < 10 {
		textW = 10
	}

	out := []string{pad + previewNameStyle.Render(truncateStr(header, avail))}

	if fileMissing {
		out = append(out, pad+diffGutterStyle.Render(padLeft("—", gutter))+sep+noMatchStyle.Render("(file doesn't exist yet)"))
	} else if len(ctxLines) == 0 {
		out = append(out, pad+diffGutterStyle.Render(padLeft("—", gutter))+sep+noMatchStyle.Render("(file is empty)"))
	} else {
		// Context lines: numbered, dimmed, leading two spaces to align
		// with the "+ " prefix on additions.
		startLN := fileLineCount - len(ctxLines) + 1
		for i, line := range ctxLines {
			ln := startLN + i
			numStr := padLeft(strconv.Itoa(ln), gutter)
			out = append(out, pad+diffContextStyle.Render(numStr)+sep+diffContextStyle.Render("  "+truncateStr(line, textW)))
		}
	}

	// The additions.
	for i, line := range snippetLines {
		ln := insertStart + i
		numStr := padLeft(strconv.Itoa(ln), gutter)
		out = append(out, pad+diffAddStyle.Render(numStr)+sep+diffAddStyle.Render("+ "+truncateStr(line, textW)))
	}

	// EOF marker so the end-of-file append is visually unambiguous.
	out = append(out, pad+diffGutterStyle.Render(padLeft("—", gutter))+sep+diffGutterStyle.Render("[EOF]"))

	return out
}

// shareHistStatusLine condenses rc-file state into one dim line under the
// shell_history row. Intentionally verbose enough that the user doesn't
// have to focus the row to see the situation.
func shareHistStatusLine(s *settingsState, w int) string {
	rc := abbreviateHome(rcFilePath(s.shareHistShell))
	switch {
	case s.shareHistShell == "":
		return noMatchStyle.Render(truncateStr("rc-file: (n/a — $SHELL isn't zsh or bash)", w))
	case s.shareHistApplied:
		return recipeStyle.Render(truncateStr("rc-file: installed mkm wrapper in "+rc+" ✓", w))
	case s.shareHistConfigured && hasLegacyWrapper(s.shareHistShell):
		return descStyle.Render(truncateStr("rc-file: "+rc+" has a legacy mkm wrapper (no mkm markers) — shell_history toggle won't sync; edit with ^e or re-apply", w))
	case s.shareHistConfigured:
		return recipeStyle.Render(truncateStr("rc-file: mkm wrapper in "+rc+" ✓ (tracked with mkm markers — shell_history toggle kept in sync)", w))
	default:
		return descStyle.Render(truncateStr("rc-file: no mkm wrapper in "+rc+" — up-arrow in this shell won't recall mkm-launched commands", w))
	}
}

// shellHistActionHints lists the ctrl-key actions available on the focused
// shell_history row. Adapts to state so we don't advertise keys that do
// nothing in the current situation.
func shellHistActionHints(s *settingsState) string {
	if s.shareHistShell == "" {
		return ""
	}
	parts := []string{
		helpKeyStyle.Render("space") + normalItemStyle.Render(": toggle"),
	}
	if !s.shareHistConfigured {
		parts = append(parts,
			helpKeyStyle.Render("^a")+normalItemStyle.Render(": apply fix"),
			helpKeyStyle.Render("^y")+normalItemStyle.Render(": copy snippet"),
		)
	} else {
		// Once the wrapper's in place, the most useful thing to copy is
		// the reload command — surface it as a top-level action.
		parts = append(parts,
			helpKeyStyle.Render("^r")+normalItemStyle.Render(": copy `source` cmd"),
		)
	}
	rc := abbreviateHome(rcFilePath(s.shareHistShell))
	parts = append(parts,
		helpKeyStyle.Render("^e")+normalItemStyle.Render(": edit "+rc),
		helpKeyStyle.Render("^v")+normalItemStyle.Render(": view "+rc),
	)
	return strings.Join(parts, ruleStyle.Render("   "))
}

// renderCycleValue is a compact alternative to renderEnumValue for fields
// with many options — shows just the current value bracketed by arrows.
func renderCycleValue(current string, focused bool) string {
	if focused {
		return selectedCursorStyle.Render("‹ ") +
			selectedItemStyle.Render(current) +
			selectedCursorStyle.Render(" ›")
	}
	return noMatchStyle.Render("‹ ") +
		normalItemStyle.Render(current) +
		noMatchStyle.Render(" ›")
}

func padLine(line string, w int) string {
	lw := lipgloss.Width(line)
	if lw < w {
		return line + strings.Repeat(" ", w-lw)
	}
	return line
}
