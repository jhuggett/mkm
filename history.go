package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// History is an append-only log of target selections used to bump
// recently-used targets in the fuzzy ranking. One line per selection:
//   <unix-ts>\t<dir>\t<name>

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "mkm", "history")
}

// Compaction thresholds. If the raw log has more than historyMaxLines
// entries, we rewrite it keeping only the most recent historyKeepLines.
const (
	historyMaxLines  = 2000
	historyKeepLines = 1000
)

// loadHistory returns a map of target-key → most recent unix timestamp.
// Missing file or read errors yield an empty map (non-fatal). The raw log
// grows unbounded on append; when it crosses historyMaxLines we compact it
// down to the most-recent historyKeepLines entries so it doesn't swell
// forever.
func loadHistory() map[string]int64 {
	path := historyPath()
	if path == "" {
		return map[string]int64{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]int64{}
	}
	rawLines := strings.Split(string(data), "\n")
	out := map[string]int64{}
	for _, line := range rawLines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		ts, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		key := parts[1] + "\x00" + parts[2]
		if ts > out[key] {
			out[key] = ts
		}
	}
	if len(rawLines) > historyMaxLines {
		compactHistory(out)
	}
	return out
}

// compactHistory rewrites the log file in place, keeping only the
// historyKeepLines most-recent unique entries. Failures are non-fatal.
func compactHistory(entries map[string]int64) {
	type kv struct {
		key string
		ts  int64
	}
	list := make([]kv, 0, len(entries))
	for k, v := range entries {
		list = append(list, kv{k, v})
	}
	// newest first
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].ts > list[j-1].ts; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
	if len(list) > historyKeepLines {
		list = list[:historyKeepLines]
	}
	path := historyPath()
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	for i := len(list) - 1; i >= 0; i-- { // write oldest first so order matches append-log
		e := list[i]
		parts := strings.SplitN(e.key, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		fmt.Fprintf(f, "%d\t%s\t%s\n", e.ts, parts[0], parts[1])
	}
}

// appendHistory records a target selection. Failures are ignored — history
// is a nicety, not a correctness requirement.
func appendHistory(target MakeTarget) {
	path := historyPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%d\t%s\t%s\n", time.Now().Unix(), target.Dir, target.Name)
}

func historyKey(t MakeTarget) string {
	return t.Dir + "\x00" + t.Name
}

// recencyBonus returns a score bump for targets used recently. Decays in
// coarse steps so the "freshness" signal is noticeable but doesn't drown
// out a better fuzzy match on an older target.
func recencyBonus(lastUsed, now int64) int {
	if lastUsed == 0 {
		return 0
	}
	age := now - lastUsed
	switch {
	case age < 3600: // <1h
		return 30
	case age < 86400: // <1d
		return 20
	case age < 604800: // <1w
		return 10
	case age < 2592000: // <30d
		return 5
	default:
		return 0
	}
}
