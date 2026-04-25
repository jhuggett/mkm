package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// legendItem is one key:hint pair on the bottom legend row. An empty Hint
// renders as the key alone (e.g. "↑↓").
type legendItem struct {
	Key  string
	Hint string
}

// renderLegend renders the bottom-row legend: dim key:hint pairs separated
// by a soft gap, padded out to w. Every page uses this so the cursor row
// doesn't drift between modes.
func renderLegend(w int, items []legendItem) string {
	gap := ruleStyle.Render("   ")
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if it.Hint == "" {
			parts = append(parts, helpKeyStyle.Render(it.Key))
		} else {
			parts = append(parts, helpKeyStyle.Render(it.Key)+normalItemStyle.Render(":"+it.Hint))
		}
	}
	return padLine(strings.Join(parts, gap), w)
}

// renderActionPill renders a small colored badge labelled with an action
// verb (e.g. " ⏎ run ", " ✎ edit "). Used as the leading visual on each
// page's cmdline so the user can see at a glance what enter would do.
// The icon + label split keeps it scannable: icon for shape recognition,
// label for unambiguous meaning. Foreground uses subtleColor (the
// theme's darkest tone) so the text holds high contrast against the
// bright accent fill in every theme — light-fg themes like mono and
// tokyo-night otherwise wash white-on-color into illegibility.
func renderActionPill(icon, label string, bg lipgloss.Color) string {
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(subtleColor).
		Bold(true).
		Padding(0, 1).
		Render(icon + " " + label)
}

// renderActionLine assembles the full action row: pill + body. body is
// already styled by the caller (see renderMakeCmd for the tokenized make
// form). Pads to w; truncates body if the row would overflow.
func renderActionLine(w int, pill, body string) string {
	pillW := lipgloss.Width(pill)
	avail := w - pillW - 1 // gap after the pill
	if avail < 1 {
		return padLine(pill, w)
	}
	bodyW := lipgloss.Width(body)
	if bodyW > avail {
		// body is already styled — re-truncating ANSI safely is non-trivial,
		// so we just clip raw width and accept some color truncation on
		// extreme overflow. In practice most cmdlines fit comfortably.
		body = truncateStyled(body, avail)
	}
	return padLine(pill+" "+body, w)
}

// truncateStyled clips a styled string to a target visible width by
// walking byte-by-byte and counting only printable cells (ANSI escape
// sequences are passed through). Crude but enough for cmdline overflow.
func truncateStyled(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	visible := 0
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inEsc {
			b.WriteByte(c)
			if c == 'm' {
				inEsc = false
			}
			continue
		}
		if c == 0x1b {
			inEsc = true
			b.WriteByte(c)
			continue
		}
		if visible >= w-1 {
			b.WriteRune('…')
			break
		}
		b.WriteByte(c)
		visible++
	}
	return b.String()
}

// renderMakeCmd tokenizes a `make ...` argv into colored components so
// the cmdline visually parses at a glance — `make` and option flags
// stay dim, the target name pops in the accent color, `-C dir` highlights
// the working directory, and each `VAR=value` pair splits the key (purple)
// from the value (green). Returns a single styled string ready to be
// dropped into renderActionLine.
func renderMakeCmd(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(helpKeyStyle.Render(args[0])) // "make"
	i := 1
	if i+1 < len(args) && args[i] == "-C" {
		b.WriteString(" " + helpKeyStyle.Render("-C "))
		b.WriteString(lipgloss.NewStyle().Foreground(cyanColor).Render(args[i+1]))
		i += 2
	}
	if i < len(args) {
		b.WriteString(" " + selectedItemStyle.Render(args[i]))
		i++
	}
	for ; i < len(args); i++ {
		a := args[i]
		if eq := strings.Index(a, "="); eq > 0 {
			b.WriteString(" " + depsLabelStyle.Render(a[:eq+1]))
			b.WriteString(recipeStyle.Render(a[eq+1:]))
		} else {
			b.WriteString(" " + normalItemStyle.Render(a))
		}
	}
	return b.String()
}

// padLine right-pads `line` with spaces so it reaches width w. Lipgloss
// width measurement handles ANSI styling.
func padLine(line string, w int) string {
	lw := lipgloss.Width(line)
	if lw < w {
		return line + strings.Repeat(" ", w-lw)
	}
	return line
}

// truncateStr truncates s to fit within width visible characters, adding …
// if truncated. Byte-length based — fine for ASCII-heavy Makefile content.
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

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// wordWrap wraps s at whitespace to fit within width cells. Words longer
// than width are kept intact on their own line (caller truncates).
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
