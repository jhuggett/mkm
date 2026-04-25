package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MakeParam struct {
	Name        string
	Kind        string   // "string", "int", "bool", "enum"
	Options     []string // only for enum
	Default     string
	Required    bool
	Description string
}

type MakeTarget struct {
	Name         string
	Dir          string
	Description  string
	Dependencies []string
	Recipe       []string
	Params       []MakeParam
	File         string // path to the Makefile (relative to cwd)
	Line         int    // 1-based line number of the target definition
	Phony        bool   // declared in a .PHONY: list in the same file
	// Annotation flags computed once after parsing (see annotateTargets).
	HasParams   bool // has its own @param or references a file-level @param
	HasScaffold bool // has $(VAR) refs not covered by @param or file assignments
}

type TargetGroup struct {
	Dir     string
	Targets []MakeTarget
	// Params are @param lines declared at file scope in Dir/Makefile (not
	// attached to any specific target). They apply to any target whose
	// recipe references them.
	Params []MakeParam
}

var excludedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	".venv":        true,
	"__pycache__":  true,
	"dist":         true,
	".next":        true,
	".cache":       true,
}

func findMakefiles() ([]string, error) {
	var makefiles []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && excludedDirs[info.Name()] {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.Name() == "Makefile" {
			makefiles = append(makefiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return makefiles, nil
}

// parseParam parses a `@param {type} NAME description` (or `[NAME=default]`) line.
// Returns ok=false on malformed input so the caller can skip it silently.
func parseParam(line string) (MakeParam, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "@param"))

	// {type}
	if !strings.HasPrefix(line, "{") {
		return MakeParam{}, false
	}
	end := strings.Index(line, "}")
	if end < 0 {
		return MakeParam{}, false
	}
	typeSpec := strings.TrimSpace(line[1:end])
	line = strings.TrimSpace(line[end+1:])

	var p MakeParam
	if strings.Contains(typeSpec, "|") {
		p.Kind = "enum"
		for _, o := range strings.Split(typeSpec, "|") {
			o = strings.TrimSpace(o)
			if o != "" {
				p.Options = append(p.Options, o)
			}
		}
		if len(p.Options) < 2 {
			return MakeParam{}, false
		}
	} else {
		switch typeSpec {
		case "string", "int", "bool":
			p.Kind = typeSpec
		default:
			return MakeParam{}, false
		}
	}

	// NAME or [NAME] or [NAME=default]
	p.Required = true
	if strings.HasPrefix(line, "[") {
		end := strings.Index(line, "]")
		if end < 0 {
			return MakeParam{}, false
		}
		inside := line[1:end]
		line = strings.TrimSpace(line[end+1:])
		p.Required = false
		if eq := strings.Index(inside, "="); eq >= 0 {
			p.Name = strings.TrimSpace(inside[:eq])
			p.Default = strings.TrimSpace(inside[eq+1:])
		} else {
			p.Name = strings.TrimSpace(inside)
		}
	} else {
		parts := strings.SplitN(line, " ", 2)
		p.Name = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			line = strings.TrimSpace(parts[1])
		} else {
			line = ""
		}
	}
	if p.Name == "" {
		return MakeParam{}, false
	}
	p.Description = line
	return p, true
}

// validParamForTarget returns an error describing why p can't attach to a
// target, or nil if it's fine. We surface these as warnings rather than
// silently dropping so obvious mistakes get noticed.
func validParamForTarget(p MakeParam) error {
	if p.Kind == "enum" && p.Default != "" {
		found := false
		for _, o := range p.Options {
			if o == p.Default {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("param %s: default %q not in options", p.Name, p.Default)
		}
	}
	return nil
}

func parseMakefileTargets(makefilePath string) ([]MakeTarget, []MakeParam, error) {
	file, err := os.Open(makefilePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	dir := filepath.Dir(makefilePath)

	var targets []MakeTarget
	var fileParams []MakeParam // @param lines not attached to any target
	var commentBuf []string
	var paramBuf []MakeParam
	var current *MakeTarget
	phony := map[string]bool{}

	// promoteParams flushes paramBuf into fileParams. Called whenever a
	// comment block ends without being claimed by a target definition —
	// those orphaned `@param` lines represent project-wide annotations.
	promoteParams := func() {
		if len(paramBuf) > 0 {
			fileParams = append(fileParams, paramBuf...)
			paramBuf = nil
		}
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Collect recipe lines for current target
		if current != nil {
			if strings.HasPrefix(line, "\t") {
				current.Recipe = append(current.Recipe, strings.TrimPrefix(line, "\t"))
				continue
			}
			current = nil
		}

		if trimmed == "" {
			commentBuf = nil
			promoteParams()
			continue
		}

		// .PHONY does NOT clear comment buffer (comment → .PHONY → target is common).
		// Collect declared names so we can flag the matching targets — having
		// `Phony` on the target lets the UI distinguish action recipes from
		// file-producing rules without re-scanning the source.
		if strings.HasPrefix(trimmed, ".PHONY") {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, ".PHONY"))
			rest = strings.TrimPrefix(rest, ":")
			for _, name := range strings.Fields(rest) {
				phony[name] = true
			}
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.HasPrefix(comment, "@param") {
				if p, ok := parseParam(comment); ok {
					if err := validParamForTarget(p); err == nil {
						paramBuf = append(paramBuf, p)
					}
				}
			} else {
				commentBuf = append(commentBuf, comment)
			}
			continue
		}

		colonIdx := strings.Index(line, ":")
		equalIdx := strings.Index(line, "=")
		isVarAssign := equalIdx >= 0 && (equalIdx < colonIdx || colonIdx+1 == equalIdx)
		if colonIdx >= 0 && !strings.HasPrefix(line, "\t") && !isVarAssign {
			parts := strings.SplitN(line, ":", 2)
			name := strings.TrimSpace(parts[0])
			if name == "" || strings.Contains(name, "%") {
				commentBuf = nil
				continue
			}

			// Make treats any `#` on a rule line as start-of-comment. Strip
			// everything from the first `#` onward; if it starts `##`, treat
			// the trailing text as an inline self-documenting description
			// (the `target: deps ## desc` convention tools like mkm surface).
			depsPart := ""
			var inlineDesc string
			if len(parts) > 1 {
				depsPart = parts[1]
				if hashIdx := strings.Index(depsPart, "#"); hashIdx >= 0 {
					if hashIdx+1 < len(depsPart) && depsPart[hashIdx+1] == '#' {
						inlineDesc = strings.TrimSpace(depsPart[hashIdx+2:])
					}
					depsPart = depsPart[:hashIdx]
				}
			}

			var deps []string
			for _, d := range strings.Fields(depsPart) {
				if d != "" && d != ";" {
					deps = append(deps, d)
				}
			}

			// Inline `##` description wins over preceding `#` comments; it's
			// attached directly to the target, so more intentional.
			desc := inlineDesc
			if desc == "" {
				desc = strings.Join(commentBuf, " ")
			}
			commentBuf = nil

			params := paramBuf
			paramBuf = nil

			targets = append(targets, MakeTarget{
				Name:         name,
				Dir:          dir,
				Description:  desc,
				Dependencies: deps,
				Params:       params,
				File:         makefilePath,
				Line:         lineNum,
			})
			current = &targets[len(targets)-1]
		} else {
			// A non-target, non-comment, non-blank line (variable assignment,
			// include, etc.) ends the current comment block. Any queued
			// @params become file-level.
			commentBuf = nil
			promoteParams()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	promoteParams() // flush any trailing unattached @params at EOF
	// Stamp Phony post-hoc: a .PHONY: line can appear before or after the
	// rule itself, so we resolve in a single pass after collecting both.
	for i := range targets {
		if phony[targets[i].Name] {
			targets[i].Phony = true
		}
	}
	return targets, fileParams, nil
}

func groupTargets(targets []MakeTarget) []TargetGroup {
	order := []string{}
	byDir := map[string][]MakeTarget{}
	for _, t := range targets {
		if _, exists := byDir[t.Dir]; !exists {
			order = append(order, t.Dir)
		}
		byDir[t.Dir] = append(byDir[t.Dir], t)
	}
	groups := make([]TargetGroup, 0, len(order))
	for _, dir := range order {
		groups = append(groups, TargetGroup{Dir: dir, Targets: byDir[dir]})
	}
	return groups
}
