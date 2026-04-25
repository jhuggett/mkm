package main

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// fuzzyScore returns (score, matched). Higher score = better match.
// Scoring:
//   +1  per matched character
//   +3  for a match directly after the previous match (consecutive run)
//   +5  for a match at a word boundary (-, _, /, ., space)
//   +10 for a match at index 0 (prefix match)
//   −len(s)/10  tiebreaker that favors shorter strings
func fuzzyScore(s, pattern string) (int, bool) {
	if pattern == "" {
		return 0, true
	}
	ls := strings.ToLower(s)
	lp := strings.ToLower(pattern)

	score := 0
	pi := 0
	prevMatch := -2
	for i := 0; i < len(ls) && pi < len(lp); i++ {
		if ls[i] == lp[pi] {
			score += 1
			if i == prevMatch+1 {
				score += 3
			}
			if i == 0 && pi == 0 {
				score += 10
			} else if i > 0 && isWordBoundary(ls[i-1]) {
				score += 5
			}
			prevMatch = i
			pi++
		}
	}
	if pi < len(lp) {
		return 0, false
	}
	score -= len(s) / 10
	return score, true
}

func isWordBoundary(c byte) bool {
	return c == '-' || c == '_' || c == '/' || c == '.' || c == ' '
}

// fuzzyHighlight styles the matched characters of s with `highlight`, and
// the rest with `base`. Unmatched runs pass through the base style.
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

// updateFilter rebuilds m.filtered based on the current query. Empty query
// preserves flat/group order. Non-empty query sorts by fuzzyScore + a
// recency bonus from m.history so frequent targets float up on ties.
func (m *model) updateFilter() {
	m.filtered = m.filtered[:0]
	if m.filter == "" {
		for i := range m.flat {
			m.filtered = append(m.filtered, i)
		}
	} else {
		type scored struct {
			idx, score int
		}
		var list []scored
		now := time.Now().Unix()
		for i, t := range m.flat {
			label := t.Name
			if t.Dir != "." {
				label = t.Dir + "/" + t.Name
			}
			// Score against the target name first; fall back to the
			// description with a discount so name hits always outrank
			// description hits. Lets users find a target by what it
			// does ("docker") even when the target name is opaque.
			s, ok := fuzzyScore(label, m.filter)
			if !ok && t.Description != "" {
				if ds, dok := fuzzyScore(t.Description, m.filter); dok {
					s = ds/2 - 5
					ok = true
				}
			}
			if !ok {
				continue
			}
			s += recencyBonus(m.history[historyKey(t)], now)
			list = append(list, scored{i, s})
		}
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].score > list[j].score
		})
		for _, sc := range list {
			m.filtered = append(m.filtered, sc.idx)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}
