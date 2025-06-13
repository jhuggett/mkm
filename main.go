package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

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
			} else {
				m.cursor = len(m.choices) - 1
			}
		case "down":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			} else {
				m.cursor = 0
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
		return fmt.Sprintf("Making: \033[32m%s\033[0m\n", m.choices[m.cursor])
	}

	s := "\033[33mMake:\033[0m\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		choiceText := choice
		if m.cursor == i {
			s += fmt.Sprintf("%s \033[32m%s\033[0m\n", cursor, choiceText)
		} else {
			s += fmt.Sprintf("%s %s\n", cursor, choiceText)
		}
	}
	s += "\nPress \033[31mq\033[0m to quit.\n"
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
