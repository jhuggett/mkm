package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewerState holds the in-TUI Makefile viewer. nil on model means we're
// not viewing.
type viewerState struct {
	path        string
	lines       []string
	scroll      int   // topmost visible line (0-based)
	cursor      int   // highlighted line (0-based)
	targetLines []int // sorted 0-based line numbers of targets in this file
}

// newViewerState loads `target`'s file and returns a viewer cursored on the
// target's definition line. Returns nil on read failure (caller stays in list).
func (m model) newViewerState(target MakeTarget) *viewerState {
	v := &viewerState{path: target.File, cursor: target.Line - 1}
	if err := v.reload(m.flat); err != nil {
		return nil
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.lines) {
		v.cursor = len(v.lines) - 1
	}
	return v
}

// reload re-reads the file from disk and recomputes target-line positions
// from the given flat target list. Used after $EDITOR modifies the file.
func (v *viewerState) reload(flat []MakeTarget) error {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return err
	}
	v.lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	seen := map[int]bool{}
	var tls []int
	for _, t := range flat {
		if t.File == v.path && t.Line > 0 && !seen[t.Line-1] {
			seen[t.Line-1] = true
			tls = append(tls, t.Line-1)
		}
	}
	sort.Ints(tls)
	v.targetLines = tls
	return nil
}

// editCmd returns a tea.Cmd that suspends the TUI and launches $EDITOR at
// path:line. Most TUI editors accept `+<line>` before the path.
func editCmd(path string, line int) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	bin := parts[0]
	args := append([]string{}, parts[1:]...)
	args = append(args, "+"+strconv.Itoa(line), path)
	c := exec.Command(bin, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

type editorFinishedMsg struct{ err error }

func (m model) updateViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	v := m.viewer
	viewportH := m.viewerBodyHeight()
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.viewer = nil
		return m, nil
	case "down", "j":
		v.cursor++
	case "up", "k":
		v.cursor--
	case "ctrl+d":
		v.cursor += viewportH / 2
	case "ctrl+u":
		v.cursor -= viewportH / 2
	case "pgdown":
		v.cursor += viewportH
	case "pgup":
		v.cursor -= viewportH
	case "g":
		v.cursor = 0
	case "G":
		v.cursor = len(v.lines) - 1
	case "n":
		v.cursor = nextTargetLine(v.targetLines, v.cursor, 1)
	case "N":
		v.cursor = nextTargetLine(v.targetLines, v.cursor, -1)
	case "e", "ctrl+e":
		return m, editCmd(v.path, v.cursor+1)
	case "ctrl+g":
		m.showHelp = true
	}
	m.clampViewer()
	return m, nil
}

// nextTargetLine picks the next/previous target-def line relative to cur.
// delta=+1 for next, -1 for prev. Wraps at ends.
func nextTargetLine(targetLines []int, cur, delta int) int {
	if len(targetLines) == 0 {
		return cur
	}
	if delta > 0 {
		for _, l := range targetLines {
			if l > cur {
				return l
			}
		}
		return targetLines[0]
	}
	for i := len(targetLines) - 1; i >= 0; i-- {
		if targetLines[i] < cur {
			return targetLines[i]
		}
	}
	return targetLines[len(targetLines)-1]
}

// clampViewer keeps cursor/scroll within bounds and the cursor visible.
func (m *model) clampViewer() {
	v := m.viewer
	if v == nil {
		return
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor > len(v.lines)-1 {
		v.cursor = len(v.lines) - 1
	}
	h := m.viewerBodyHeight()
	if h < 1 {
		h = 1
	}
	if v.cursor < v.scroll {
		v.scroll = v.cursor
	}
	if v.cursor >= v.scroll+h {
		v.scroll = v.cursor - h + 1
	}
	if v.scroll < 0 {
		v.scroll = 0
	}
	maxScroll := len(v.lines) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.scroll > maxScroll {
		v.scroll = maxScroll
	}
}

func (m model) viewerBodyHeight() int {
	// top(1) + rule(1) + rule(1) + status(1) = 4 fixed rows
	h := m.height - 4
	if h < 1 {
		h = 1
	}
	return h
}

// renderViewerView renders the in-TUI Makefile viewer. Top line shows the
// path + viewer-specific help keys, the body shows source with line numbers
// and light syntax styling, and the footer shows position + key hints.
func (m model) renderViewerView(w, h int) string {
	v := m.viewer
	top := m.renderViewerTopLine(w)
	rule := ruleStyle.Render(strings.Repeat("─", w))

	bodyH := m.viewerBodyHeight()
	body := renderViewerBody(v, w, bodyH)
	footer := renderViewerFooter(v, w)

	return strings.Join([]string{top, rule, body, rule, footer}, "\n")
}

func (m model) renderViewerTopLine(w int) string {
	left := titleStyle.Render("mkm") + filterPromptStyle.Render(" › ")
	path := m.viewer.path
	help := strings.Join([]string{
		helpKeyStyle.Render("↑↓ jk"),
		helpKeyStyle.Render("n/N:target"),
		helpKeyStyle.Render("e:edit"),
		helpKeyStyle.Render("esc:back"),
	}, ruleStyle.Render("  "))
	leftW := lipgloss.Width(left)
	helpW := lipgloss.Width(help)

	avail := w - leftW - helpW - 2
	if avail < 1 {
		help = ""
		helpW = 0
		avail = w - leftW
		if avail < 1 {
			avail = 1
		}
	}
	if len(path) > avail {
		path = truncateStr(path, avail)
	}
	pathPart := filterStyle.Render(path)
	pathW := lipgloss.Width(pathPart)
	pad := w - leftW - pathW - helpW
	if pad < 0 {
		pad = 0
	}
	return left + pathPart + strings.Repeat(" ", pad) + help
}

// renderViewerBody returns exactly h rows of the viewer body: line numbers
// in a narrow gutter, then styled source content, padded/truncated to w.
func renderViewerBody(v *viewerState, w, h int) string {
	gutterW := len(strconv.Itoa(len(v.lines)))
	if gutterW < 2 {
		gutterW = 2
	}
	sepW := 3 // " │ "
	contentW := w - gutterW - sepW
	if contentW < 1 {
		contentW = 1
	}

	out := make([]string, 0, h)
	end := v.scroll + h
	if end > len(v.lines) {
		end = len(v.lines)
	}
	for i := v.scroll; i < end; i++ {
		lineNum := i + 1
		numStr := padLeft(strconv.Itoa(lineNum), gutterW)
		sep := ruleStyle.Render(" │ ")
		styledContent := styleMakefileLine(v.lines[i], contentW)
		row := helpKeyStyle.Render(numStr) + sep + styledContent

		if i == v.cursor {
			row = selectedCursorStyle.Render(padLeft(strconv.Itoa(lineNum), gutterW)) +
				sep + styledContent
		}

		lw := lipgloss.Width(row)
		if lw < w {
			row += strings.Repeat(" ", w-lw)
		}
		out = append(out, row)
	}
	for len(out) < h {
		out = append(out, strings.Repeat(" ", w))
	}
	return strings.Join(out, "\n")
}

// styleMakefileLine applies light syntax styling to a single Makefile line.
// Recognized: `#` comments (dim italic), `@param` lines (purple), `.PHONY`
// and rule definitions (target name bold), recipe lines starting with tab
// (green). Result is truncated to maxW cells.
func styleMakefileLine(line string, maxW int) string {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(line, "\t") {
		content := strings.TrimPrefix(line, "\t")
		return recipeStyle.Render(truncateStr("  "+content, maxW))
	}

	if strings.HasPrefix(trimmed, "#") {
		c := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		if strings.HasPrefix(c, "@param") {
			// Highlight `@param` distinctly so the annotation pattern stands out.
			return depsLabelStyle.Render(truncateStr(line, maxW))
		}
		return noMatchStyle.Render(truncateStr(line, maxW))
	}

	if strings.HasPrefix(trimmed, ".PHONY") {
		return depsValueStyle.Render(truncateStr(line, maxW))
	}

	// Target rule: `name: deps ## desc`. Style the name specially.
	if idx := strings.Index(line, ":"); idx > 0 && !strings.Contains(line[:idx], "=") {
		name := line[:idx]
		rest := line[idx:]
		styledName := previewNameStyle.Render(name)
		styledRest := normalItemStyle.Render(rest)
		combined := styledName + styledRest
		if lipgloss.Width(combined) > maxW {
			return normalItemStyle.Render(truncateStr(line, maxW))
		}
		return combined
	}

	return normalItemStyle.Render(truncateStr(line, maxW))
}

func renderViewerFooter(v *viewerState, w int) string {
	pos := fmt.Sprintf("line %d/%d", v.cursor+1, len(v.lines))
	pct := "--"
	if len(v.lines) > 1 {
		pct = fmt.Sprintf("%d%%", (v.cursor*100)/(len(v.lines)-1))
	}
	left := helpKeyStyle.Render(pos) +
		ruleStyle.Render("  ") +
		helpKeyStyle.Render(pct)
	right := helpKeyStyle.Render("g/G  n/N  ctrl+d/u  e  esc")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := w - leftW - rightW
	if pad < 0 {
		pad = 0
		right = ""
		rightW = 0
		pad = w - leftW
		if pad < 0 {
			pad = 0
		}
	}
	return left + strings.Repeat(" ", pad) + right
}
