package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// appendShellHistory writes cmd to the user's shell history file so pressing
// up-arrow in a *new* shell recalls the selected make invocation. Best-
// effort: unknown shells, missing HOME, and write failures are silent —
// mkm's core job already succeeded by the time we get here.
//
// In-memory history of the current shell is NOT updated — no reliable way
// to do that from a subprocess. For current-shell up-arrow recall, install
// the shell wrapper via the settings screen (ctrl+s → shell_history row →
// ctrl+a). The wrapper runs mkm in --print mode and uses `print -s` /
// `history -s` to push the entry into the live shell.
func appendShellHistory(cmd string) {
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		writeHistoryEntry(zshHistFile(), fmt.Sprintf(": %d:0;%s\n", time.Now().Unix(), cmd))
	case strings.Contains(shell, "bash"):
		writeHistoryEntry(bashHistFile(), cmd+"\n")
	}
	// fish and other shells use their own formats/databases — skip.
}

func zshHistFile() string {
	if p := os.Getenv("HISTFILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zsh_history")
}

func bashHistFile() string {
	if p := os.Getenv("HISTFILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".bash_history")
}

func writeHistoryEntry(path, entry string) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(entry)
}

// --- Shell wrapper detection + remediation ---------------------------------

// detectedShell returns "zsh", "bash", or "" based on $SHELL. Used by the
// settings screen's shell_history row to decide which rc file to inspect
// and what wrapper snippet to offer.
func detectedShell() string {
	s := os.Getenv("SHELL")
	switch {
	case strings.Contains(s, "zsh"):
		return "zsh"
	case strings.Contains(s, "bash"):
		return "bash"
	}
	return ""
}

func rcFilePath(shell string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	}
	return ""
}

// mkmWrapperRe matches any top-level `mkm() { ... }` or `function mkm { ... }`
// declaration. Used to detect existing wrappers (ours or the user's own)
// so we don't double-install.
var mkmWrapperRe = regexp.MustCompile(`(?m)^\s*(mkm\s*\(\s*\)\s*\{|function\s+mkm\s*(\{|\(\s*\)\s*\{))`)

// Markers bracketing wrappers we manage. Their presence means mkm can
// safely rewrite the block when the shell_history toggle changes. Legacy
// wrappers (from earlier mkm versions, or hand-written) are left alone.
const (
	wrapperBeginMarker = "# mkm:wrapper:begin — managed by mkm, do not edit between markers"
	wrapperEndMarker   = "# mkm:wrapper:end"
)

// Wrapper body pieces. We build the full block at runtime so the snippet
// reflects the *current* shell_history toggle — the `print -s` /
// `history -s` line is present only when the user wants mkm commands in
// shell history.
func zshWrapperBlock(addHistory bool) string {
	pushLine := ""
	if addHistory {
		pushLine = "    print -s \"$cmd\"\n"
	}
	commentSuffix := " (history push disabled via shell_history setting)"
	if addHistory {
		commentSuffix = ""
	}
	return wrapperBeginMarker + "\n" +
		"# mkm: shell wrapper — evals the selected command in your current shell" + commentSuffix + "\n" +
		"mkm() {\n" +
		"  local cmd\n" +
		"  cmd=$(command mkm --print)\n" +
		"  if [ -n \"$cmd\" ]; then\n" +
		pushLine +
		"    eval \"$cmd\"\n" +
		"  fi\n" +
		"}\n" +
		wrapperEndMarker + "\n"
}

func bashWrapperBlock(addHistory bool) string {
	pushLine := ""
	if addHistory {
		pushLine = "    history -s \"$cmd\"\n"
	}
	commentSuffix := " (history push disabled via shell_history setting)"
	if addHistory {
		commentSuffix = ""
	}
	return wrapperBeginMarker + "\n" +
		"# mkm: shell wrapper — evals the selected command in your current shell" + commentSuffix + "\n" +
		"mkm() {\n" +
		"  local cmd\n" +
		"  cmd=$(command mkm --print)\n" +
		"  if [ -n \"$cmd\" ]; then\n" +
		pushLine +
		"    eval \"$cmd\"\n" +
		"  fi\n" +
		"}\n" +
		wrapperEndMarker + "\n"
}

// shareHistSnippet returns the rc-file snippet mkm would install for
// `shell`, including the begin/end markers. addHistory controls whether
// the wrapper pushes commands into the shell's live history via
// `print -s` (zsh) or `history -s` (bash). Empty string for unknown
// shells.
func shareHistSnippet(shell string, addHistory bool) string {
	switch shell {
	case "zsh":
		return "\n" + zshWrapperBlock(addHistory)
	case "bash":
		return "\n" + bashWrapperBlock(addHistory)
	}
	return ""
}

// shareHistConfigured reports whether the rc file for `shell` already has a
// mkm wrapper function defined (ours or the user's own).
func shareHistConfigured(shell string) bool {
	path := rcFilePath(shell)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return mkmWrapperRe.Match(data)
}

// hasManagedWrapper reports whether the rc file contains a marker-bracketed
// mkm wrapper block — the kind mkm can safely rewrite. Legacy wrappers
// (matched by mkmWrapperRe but lacking markers) are not "managed".
func hasManagedWrapper(shell string) bool {
	path := rcFilePath(shell)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(wrapperBeginMarker)) && bytes.Contains(data, []byte(wrapperEndMarker))
}

// hasLegacyWrapper reports that the rc file has a mkm() function but no
// managed markers — meaning mkm shouldn't touch it automatically. The
// settings UI surfaces this so the user knows to edit manually.
func hasLegacyWrapper(shell string) bool {
	return shareHistConfigured(shell) && !hasManagedWrapper(shell)
}

// applyShareHistFix installs the wrapper or rewrites a managed block in
// place so the installed wrapper reflects the current shell_history
// toggle. addHistory controls the `print -s` / `history -s` line.
//
// Returns an error for legacy wrappers — we don't want to silently edit a
// block we didn't write, so the caller surfaces guidance to the user.
func applyShareHistFix(shell string, addHistory bool) error {
	path := rcFilePath(shell)
	if path == "" {
		return fmt.Errorf("no rc file path for shell %q", shell)
	}
	snippet := shareHistSnippet(shell, addHistory)
	if snippet == "" {
		return fmt.Errorf("no snippet for shell %q", shell)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var content string
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}

	newContent, replaced := replaceManagedBlock(content, snippet)
	if !replaced {
		if shareHistConfiguredInContent(content) {
			return fmt.Errorf("existing mkm wrapper has no mkm markers — edit %s manually (ctrl+e) or remove the old wrapper and re-apply", abbreviateHome(path))
		}
		newContent = content
		if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += snippet
	}

	return os.WriteFile(path, []byte(newContent), 0o644)
}

// syncManagedWrapper rewrites the managed wrapper block to match
// `addHistory` — but only if such a block actually exists. Used from the
// settings save path so toggling shell_history keeps the installed wrapper
// in sync. No-op (and no error) when there's no managed wrapper to sync.
func syncManagedWrapper(shell string, addHistory bool) error {
	if !hasManagedWrapper(shell) {
		return nil
	}
	return applyShareHistFix(shell, addHistory)
}

// replaceManagedBlock finds the first wrapper-begin/wrapper-end pair in
// content and replaces everything between and including them with
// newSnippet. Returns (new content, replaced?).
func replaceManagedBlock(content, newSnippet string) (string, bool) {
	beginIdx := strings.Index(content, wrapperBeginMarker)
	if beginIdx < 0 {
		return content, false
	}
	// Walk back to include any leading blank line that preceded the block,
	// so repeated rewrites don't accumulate blanks above the markers.
	blockStart := beginIdx
	for blockStart > 0 && content[blockStart-1] == '\n' {
		blockStart--
		if blockStart > 0 && content[blockStart-1] == '\n' {
			// keep exactly one separating newline
			break
		}
	}

	rest := content[beginIdx:]
	endRel := strings.Index(rest, wrapperEndMarker)
	if endRel < 0 {
		return content, false
	}
	// Consume through the rest of the end-marker line (and its newline, if any).
	endAbs := beginIdx + endRel + len(wrapperEndMarker)
	if endAbs < len(content) && content[endAbs] == '\n' {
		endAbs++
	}

	return content[:blockStart] + newSnippet + content[endAbs:], true
}

// shareHistConfiguredInContent mirrors shareHistConfigured but operates on
// an already-read buffer — used mid-apply to avoid a second read.
func shareHistConfiguredInContent(content string) bool {
	return mkmWrapperRe.MatchString(content)
}

// copyToClipboard pipes text through the platform's native clipboard tool.
// On macOS that's pbcopy; on Linux we try wl-copy (Wayland) then xclip
// (X11). Returns a descriptive error when no tool is available — callers
// surface that to the user so they know to install one or copy manually.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		switch {
		case hasExecutable("wl-copy"):
			cmd = exec.Command("wl-copy")
		case hasExecutable("xclip"):
			cmd = exec.Command("xclip", "-selection", "clipboard")
		case hasExecutable("xsel"):
			cmd = exec.Command("xsel", "--clipboard", "--input")
		default:
			return fmt.Errorf("no clipboard tool found — install wl-copy, xclip, or xsel")
		}
	default:
		return fmt.Errorf("clipboard unsupported on %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func hasExecutable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func abbreviateHome(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
