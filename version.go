package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// currentVersion returns the raw version string from the build info. This
// can be a clean tag ("v2.0.0"), a Go pseudo-version built off a commit
// between tags ("v1.2.1-0.20260422012742-bb04ad50c2fb"), either of those
// with a "+dirty" suffix when the worktree had uncommitted changes, or
// "(devel)" / "" when built without module info. displayVersion formats
// this for the UI.
func currentVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(dev)"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "(dev)"
	}
	return v
}

// pseudoVersionRe matches the timestamp+hash suffix of a Go pseudo-version,
// e.g. "-20260422012742-bb04ad50c2fb". Its presence means the commit being
// built isn't exactly on a published tag.
var pseudoVersionRe = regexp.MustCompile(`-\d{14}-[0-9a-f]{12}`)

// isPseudoVersion reports whether v is a Go pseudo-version (between-tags
// build) rather than a clean tag like "v2.0.0".
func isPseudoVersion(v string) bool {
	return pseudoVersionRe.MatchString(v)
}

// displayVersion returns a short, human-friendly version string suitable
// for the TUI top line. Clean tags pass through; pseudo-versions collapse
// to "(dev <hash>)" so we don't dump 50 characters of timestamp+hash in
// front of the user. The "+dirty" indicator is preserved because it's a
// useful "you're running something uncommitted" signal.
func displayVersion() string {
	v := currentVersion()
	if v == "(dev)" {
		return v
	}
	dirty := ""
	if strings.HasSuffix(v, "+dirty") {
		dirty = "+dirty"
		v = strings.TrimSuffix(v, "+dirty")
	}
	if isPseudoVersion(v) {
		hash := ""
		if m := pseudoVersionRe.FindString(v); m != "" {
			parts := strings.Split(m, "-")
			if len(parts) >= 3 {
				hash = parts[len(parts)-1]
				if len(hash) > 7 {
					hash = hash[:7]
				}
			}
		}
		if hash != "" {
			return "(dev " + hash + dirty + ")"
		}
		return "(dev" + dirty + ")"
	}
	return v + dirty
}

// isDevBuild reports whether we're running anything other than a clean
// tagged release. Pseudo-versions and +dirty builds are treated as dev so
// the update-check doesn't nag when we can't meaningfully compare versions.
func isDevBuild() bool {
	v := currentVersion()
	if v == "(dev)" {
		return true
	}
	if strings.HasSuffix(v, "+dirty") {
		return true
	}
	return isPseudoVersion(v)
}

// updateInfo holds the result of an update check. Checked is true once the
// Cmd has run (regardless of outcome) so the UI can tell "haven't checked
// yet" apart from "checked, nothing to update".
type updateInfo struct {
	Latest    string
	Available bool
	Err       error
	Checked   bool
}

const (
	updateCheckInterval = 24 * time.Hour
	githubReleasesURL   = "https://api.github.com/repos/jhuggett/mkm/releases/latest"
)

type cachedCheck struct {
	Latest    string `json:"latest"`
	CheckedAt int64  `json:"checked_at"`
}

func updateCheckCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "mkm", "update-check.json")
}

// checkForUpdateCmd returns a bubbletea Cmd that asynchronously checks for
// a newer mkm release. Results under 24h old are served from
// ~/.cache/mkm/update-check.json to avoid hammering the GitHub API. The
// caller is expected to gate invocation on cfg.CheckUpdates.
func checkForUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		current := currentVersion()
		if isDevBuild() {
			return updateCheckMsg{updateInfo{Checked: true}}
		}
		if c := loadCachedCheck(); c.Latest != "" && time.Since(time.Unix(c.CheckedAt, 0)) < updateCheckInterval {
			return updateCheckMsg{updateInfo{
				Latest:    c.Latest,
				Available: c.Latest != current,
				Checked:   true,
			}}
		}
		latest, err := fetchLatestRelease()
		if err != nil {
			return updateCheckMsg{updateInfo{Err: err, Checked: true}}
		}
		saveCachedCheck(cachedCheck{Latest: latest, CheckedAt: time.Now().Unix()})
		return updateCheckMsg{updateInfo{
			Latest:    latest,
			Available: latest != "" && latest != current,
			Checked:   true,
		}}
	}
}

type updateCheckMsg struct {
	info updateInfo
}

func loadCachedCheck() cachedCheck {
	path := updateCheckCachePath()
	if path == "" {
		return cachedCheck{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cachedCheck{}
	}
	var c cachedCheck
	_ = json.Unmarshal(data, &c)
	return c
}

func saveCachedCheck(c cachedCheck) {
	path := updateCheckCachePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// fetchLatestRelease queries the GitHub releases API for the tag of the
// latest release. 5s timeout keeps the check from hanging on flaky
// networks. Returns "" + nil when the repo has no releases yet.
func fetchLatestRelease() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", githubReleasesURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api: %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.TagName, nil
}

// installCommand is the shell line users should run to pick up the latest
// release. Shown on the banner; copied to clipboard on ctrl+u.
func installCommand() string {
	return "go install github.com/jhuggett/mkm@latest"
}
