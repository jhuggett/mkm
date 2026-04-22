package main

import "github.com/charmbracelet/lipgloss"

// Theme is the palette every style is built from. A theme just picks colors;
// the style shapes (bold, italic, etc.) stay constant across themes.
type Theme struct {
	Accent lipgloss.Color
	Dim    lipgloss.Color
	Subtle lipgloss.Color
	Text   lipgloss.Color
	Hi     lipgloss.Color
	Cyan   lipgloss.Color
	Green  lipgloss.Color
	Purple lipgloss.Color
	Yellow lipgloss.Color
	Red    lipgloss.Color
}

var themes = map[string]Theme{
	"nord": {
		Accent: "#d08770",
		Dim:    "#4c566a",
		Subtle: "#3b4252",
		Text:   "#7a8490",
		Hi:     "#d8dee9",
		Cyan:   "#5e81ac",
		Green:  "#a3be8c",
		Purple: "#b48ead",
		Yellow: "#ebcb8b",
		Red:    "#bf616a",
	},
	"dracula": {
		Accent: "#ff79c6",
		Dim:    "#6272a4",
		Subtle: "#44475a",
		Text:   "#bfbfbf",
		Hi:     "#f8f8f2",
		Cyan:   "#8be9fd",
		Green:  "#50fa7b",
		Purple: "#bd93f9",
		Yellow: "#f1fa8c",
		Red:    "#ff5555",
	},
	"solarized-dark": {
		Accent: "#cb4b16",
		Dim:    "#586e75",
		Subtle: "#073642",
		Text:   "#839496",
		Hi:     "#fdf6e3",
		Cyan:   "#268bd2",
		Green:  "#859900",
		Purple: "#6c71c4",
		Yellow: "#b58900",
		Red:    "#dc322f",
	},
	"mono": {
		Accent: "#ffffff",
		Dim:    "#5a5a5a",
		Subtle: "#3a3a3a",
		Text:   "#a0a0a0",
		Hi:     "#ffffff",
		Cyan:   "#c8c8c8",
		Green:  "#d0d0d0",
		Purple: "#b0b0b0",
		Yellow: "#e0e0e0",
		Red:    "#ffffff",
	},
	"gruvbox-dark": {
		Accent: "#fe8019",
		Dim:    "#665c54",
		Subtle: "#3c3836",
		Text:   "#a89984",
		Hi:     "#ebdbb2",
		Cyan:   "#83a598",
		Green:  "#b8bb26",
		Purple: "#d3869b",
		Yellow: "#fabd2f",
		Red:    "#fb4934",
	},
	"tokyo-night": {
		Accent: "#7aa2f7",
		Dim:    "#565f89",
		Subtle: "#2a2f4a",
		Text:   "#a9b1d6",
		Hi:     "#c0caf5",
		Cyan:   "#7dcfff",
		Green:  "#9ece6a",
		Purple: "#bb9af7",
		Yellow: "#e0af68",
		Red:    "#f7768e",
	},
	"catppuccin-mocha": {
		Accent: "#f5c2e7",
		Dim:    "#585b70",
		Subtle: "#313244",
		Text:   "#a6adc8",
		Hi:     "#cdd6f4",
		Cyan:   "#89dceb",
		Green:  "#a6e3a1",
		Purple: "#cba6f7",
		Yellow: "#f9e2af",
		Red:    "#f38ba8",
	},
	"rose-pine": {
		Accent: "#eb6f92",
		Dim:    "#524f67",
		Subtle: "#26233a",
		Text:   "#908caa",
		Hi:     "#e0def4",
		Cyan:   "#9ccfd8",
		Green:  "#31748f",
		Purple: "#c4a7e7",
		Yellow: "#f6c177",
		Red:    "#eb6f92",
	},
	"one-dark": {
		Accent: "#61afef",
		Dim:    "#5c6370",
		Subtle: "#3e4451",
		Text:   "#abb2bf",
		Hi:     "#dcdfe4",
		Cyan:   "#56b6c2",
		Green:  "#98c379",
		Purple: "#c678dd",
		Yellow: "#e5c07b",
		Red:    "#e06c75",
	},
	"github-dark": {
		Accent: "#58a6ff",
		Dim:    "#6e7681",
		Subtle: "#30363d",
		Text:   "#c9d1d9",
		Hi:     "#f0f6fc",
		Cyan:   "#79c0ff",
		Green:  "#7ee787",
		Purple: "#d2a8ff",
		Yellow: "#e3b341",
		Red:    "#ff7b72",
	},
}

// activeTheme is the name of the most recently applied theme — exposed via
// currentThemeName so the settings screen can seed its edit state from the
// live palette rather than re-reading the config file.
var activeTheme = "nord"

func currentThemeName() string { return activeTheme }

// Style vars — populated by applyTheme so every call site throughout the
// codebase can keep referring to them as package globals without threading a
// theme handle through.
var (
	accent      lipgloss.Color
	dimColor    lipgloss.Color
	subtleColor lipgloss.Color
	textColor   lipgloss.Color
	hiColor     lipgloss.Color
	cyanColor   lipgloss.Color
	greenColor  lipgloss.Color
	purpleColor lipgloss.Color
	yellowColor lipgloss.Color
	redColor    lipgloss.Color

	titleStyle          lipgloss.Style
	filterStyle         lipgloss.Style
	filterPromptStyle   lipgloss.Style
	groupHeaderStyle    lipgloss.Style
	selectedItemStyle   lipgloss.Style
	selectedCursorStyle lipgloss.Style
	normalItemStyle     lipgloss.Style
	matchStyle          lipgloss.Style
	helpKeyStyle        lipgloss.Style
	ruleStyle           lipgloss.Style
	descStyle           lipgloss.Style
	depsLabelStyle      lipgloss.Style
	depsValueStyle      lipgloss.Style
	recipeStyle         lipgloss.Style
	noMatchStyle        lipgloss.Style
	previewNameStyle    lipgloss.Style
	diffAddStyle        lipgloss.Style
	diffRemoveStyle     lipgloss.Style
	diffContextStyle    lipgloss.Style
	diffGutterStyle     lipgloss.Style
)

func init() {
	applyTheme("nord")
}

func applyTheme(name string) {
	t, ok := themes[name]
	if !ok {
		t = themes["nord"]
		name = "nord"
	}
	activeTheme = name
	accent = t.Accent
	dimColor = t.Dim
	subtleColor = t.Subtle
	textColor = t.Text
	hiColor = t.Hi
	cyanColor = t.Cyan
	greenColor = t.Green
	purpleColor = t.Purple
	yellowColor = t.Yellow
	redColor = t.Red

	titleStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	filterStyle = lipgloss.NewStyle().Foreground(hiColor).Bold(true)
	filterPromptStyle = lipgloss.NewStyle().Foreground(accent)
	groupHeaderStyle = lipgloss.NewStyle().Foreground(cyanColor)
	selectedItemStyle = lipgloss.NewStyle().Foreground(hiColor).Bold(true)
	selectedCursorStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	normalItemStyle = lipgloss.NewStyle().Foreground(textColor)
	matchStyle = lipgloss.NewStyle().Foreground(accent)
	helpKeyStyle = lipgloss.NewStyle().Foreground(dimColor)
	ruleStyle = lipgloss.NewStyle().Foreground(subtleColor)
	descStyle = lipgloss.NewStyle().Foreground(yellowColor).Italic(true)
	depsLabelStyle = lipgloss.NewStyle().Foreground(purpleColor).Bold(true)
	depsValueStyle = lipgloss.NewStyle().Foreground(textColor)
	recipeStyle = lipgloss.NewStyle().Foreground(greenColor)
	noMatchStyle = lipgloss.NewStyle().Foreground(dimColor).Italic(true)
	previewNameStyle = lipgloss.NewStyle().Foreground(hiColor).Bold(true)
	diffAddStyle = lipgloss.NewStyle().Foreground(greenColor)
	diffRemoveStyle = lipgloss.NewStyle().Foreground(redColor)
	diffContextStyle = lipgloss.NewStyle().Foreground(dimColor)
	diffGutterStyle = lipgloss.NewStyle().Foreground(subtleColor)
}
