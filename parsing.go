package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type MakeTarget struct {
	Name string
	Dir  string
}

type TargetGroup struct {
	Dir     string
	Targets []MakeTarget
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

func parseMakefileTargets(makefilePath string) ([]MakeTarget, error) {
	file, err := os.Open(makefilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir := filepath.Dir(makefilePath)

	var targets []MakeTarget
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ".PHONY") {
			continue
		}
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "\t") {
			parts := strings.SplitN(line, ":", 2)
			name := strings.TrimSpace(parts[0])
			if name == "" {
				continue
			}
			targets = append(targets, MakeTarget{
				Name: name,
				Dir:  dir,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return targets, nil
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
