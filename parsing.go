package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func findMakefiles() ([]string, error) {
	var makefiles []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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

func parseMakefileTargets(makefilePath string) ([]string, error) {
	file, err := os.Open(makefilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Ignore comments, empty lines, and .PHONY declarations
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") || strings.HasPrefix(strings.TrimSpace(line), ".PHONY") {
			continue
		}
		// Check if the line contains a target
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "\t") {
			parts := strings.SplitN(line, ":", 2)
			target := strings.TrimSpace(parts[0])
			if target != "" {
				targets = append(targets, target)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}
