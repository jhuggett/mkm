package main

import (
	"fmt"
	"os"
	"os/exec"

	"bufio"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

type model struct {
	choices  []string
	cursor   int
	selected bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.selected {
		return fmt.Sprintf("You selected: %s\n", m.choices[m.cursor])
	}

	s := "Choose a Makefile target:\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}
	s += "\nPress q to quit.\n"
	return s
}

func main() {
	makefiles, err := findMakefiles()
	if err != nil {
		fmt.Println("Error finding Makefiles:", err)
		os.Exit(1)
	}

	if len(makefiles) == 0 {
		fmt.Println("No Makefiles found.")
		os.Exit(1)
	}

	var allTargets []string
	for _, makefile := range makefiles {
		targets, err := parseMakefileTargets(makefile)
		if err != nil {
			fmt.Printf("Error parsing Makefile %s: %v\n", makefile, err)
			os.Exit(1)
		}
		allTargets = append(allTargets, targets...)
	}

	if len(allTargets) == 0 {
		fmt.Println("No targets found in Makefiles.")
		os.Exit(1)
	}

	p := tea.NewProgram(model{choices: allTargets})

	m, err := p.StartReturningModel()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	finalModel := m.(model)
	if finalModel.selected {
		cmd := exec.Command("make", finalModel.choices[finalModel.cursor])
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("Error executing command:", err)
			os.Exit(1)
		}
	}
}
