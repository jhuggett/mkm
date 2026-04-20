package main

import "github.com/charmbracelet/lipgloss"

// Shared color palette and lipgloss styles. Any new style goes here so the
// visual language stays in one place.
var (
	accent      = lipgloss.Color("#d08770")
	dimColor    = lipgloss.Color("#4c566a")
	subtleColor = lipgloss.Color("#3b4252")
	textColor   = lipgloss.Color("#7a8490")
	hiColor     = lipgloss.Color("#d8dee9")
	cyanColor   = lipgloss.Color("#5e81ac")
	greenColor  = lipgloss.Color("#a3be8c")
	purpleColor = lipgloss.Color("#b48ead")
	yellowColor = lipgloss.Color("#ebcb8b")
	redColor    = lipgloss.Color("#bf616a")

	titleStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	filterStyle = lipgloss.NewStyle().
			Foreground(hiColor).
			Bold(true)

	filterPromptStyle = lipgloss.NewStyle().
				Foreground(accent)

	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(cyanColor)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(hiColor).
				Bold(true)

	selectedCursorStyle = lipgloss.NewStyle().
				Foreground(accent).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(textColor)

	matchStyle = lipgloss.NewStyle().
			Foreground(accent)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	ruleStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	descStyle = lipgloss.NewStyle().
			Foreground(yellowColor).
			Italic(true)

	depsLabelStyle = lipgloss.NewStyle().
			Foreground(purpleColor).
			Bold(true)

	depsValueStyle = lipgloss.NewStyle().
			Foreground(textColor)

	recipeStyle = lipgloss.NewStyle().
			Foreground(greenColor)

	noMatchStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)

	previewNameStyle = lipgloss.NewStyle().
				Foreground(hiColor).
				Bold(true)

	diffAddStyle = lipgloss.NewStyle().
			Foreground(greenColor)

	diffRemoveStyle = lipgloss.NewStyle().
			Foreground(redColor)

	diffContextStyle = lipgloss.NewStyle().
				Foreground(dimColor)

	diffGutterStyle = lipgloss.NewStyle().
			Foreground(subtleColor)
)
