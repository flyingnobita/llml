package tui

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// EnvLLMLTheme is the environment variable that selects the color theme.
// Values: "dark", "light", or "auto" (default). When "auto" or unset, the
// terminal background is queried via lipgloss.HasDarkBackground().
const EnvLLMLTheme = "LLML_THEME"

// themePick selects which palette mode is active (including auto). The user
// cycles these with the t key: dark → light → auto → dark …
const (
	themePickDark = iota
	themePickLight
	themePickAuto
)

const themePickCount = 3

// themeToastText is the transient message after cycling themes (pick + resolved palette for auto).
func themeToastText(pick int, resolved Theme) string {
	switch pick {
	case themePickDark:
		return "Theme: dark"
	case themePickLight:
		return "Theme: light"
	default:
		if resolved == DarkTheme() {
			return "Theme: auto (dark)"
		}
		return "Theme: auto (light)"
	}
}

// Theme holds semantic colors for the TUI. All values are image/color.Color
// values, typically created via lipgloss.Color("ANSI-or-hex-string").
type Theme struct {
	Title    color.Color
	Subtitle color.Color
	Body     color.Color
	Footer   color.Color
	Error    color.Color
	// Border is the default pane and modal border color. In split-server mode it
	// is also the inactive split-pane chrome (dim model table, launch preview,
	// server log borders) when keyboard focus is on another pane.
	Border       color.Color
	RuntimePanel color.Color
	ModalTitle   color.Color
	// ParamSectionHeading labels nested blocks inside the parameters modal (env / argv).
	ParamSectionHeading color.Color
	ModalBody           color.Color
	TableHeader         color.Color
	TableCell           color.Color
	TableSelected       color.Color
	TableSelectedBg     color.Color
	// ParamProfileName highlights the active parameter profile name in the params modal list.
	ParamProfileName color.Color
	// ParamProfileInactive is used for non-active profile names in the params modal list.
	ParamProfileInactive color.Color
	// SplitPaneBorderFocused accents the keyboard-focused pane in split-server mode.
	SplitPaneBorderFocused color.Color
	// SplitPaneBorderDim matches Border (same lipgloss.Color in theme constructors)
	// so dim split panes and inactive launch preview use one grey; kept separate so
	// splitPaneChromeDim in styles stays semantically named.
	SplitPaneBorderDim color.Color
}

// DarkTheme returns the default dark-terminal palette (original llml colors).
func DarkTheme() Theme {
	border := lipgloss.Color("240")
	return Theme{
		Title:        lipgloss.Color("99"),
		Subtitle:     lipgloss.Color("241"),
		Body:         lipgloss.Color("252"),
		Footer:       lipgloss.Color("240"),
		Error:        lipgloss.Color("203"),
		Border:       border,
		RuntimePanel: lipgloss.Color("246"),
		// Brighter orchid than main Title (99); modal chrome reads as its own layer.
		ModalTitle: lipgloss.Color("183"),
		// Muted slate below ModalTitle (183); env/argv section captions.
		ParamSectionHeading: lipgloss.Color("109"),
		ModalBody:           lipgloss.Color("252"),
		TableHeader:         lipgloss.Color("252"),
		TableCell:           lipgloss.Color("252"),
		TableSelected:       lipgloss.Color("17"),
		TableSelectedBg:     lipgloss.Color("51"),
		// Distinct from ModalTitle / ParamSectionHeading; warm vs purple chrome.
		ParamProfileName:       lipgloss.Color("178"),
		ParamProfileInactive:   lipgloss.Color("246"),
		SplitPaneBorderFocused: lipgloss.Color("51"),
		SplitPaneBorderDim:     border,
	}
}

// LightTheme returns a palette tuned for light terminal backgrounds.
func LightTheme() Theme {
	border := lipgloss.Color("249")
	return Theme{
		Title:        lipgloss.Color("55"),
		Subtitle:     lipgloss.Color("243"),
		Body:         lipgloss.Color("235"),
		Footer:       lipgloss.Color("249"),
		Error:        lipgloss.Color("160"),
		Border:       border,
		RuntimePanel: lipgloss.Color("238"),
		// Richer purple than main Title (55); dialogs stand out from the header.
		ModalTitle: lipgloss.Color("99"),
		// Steel blue; secondary to ModalTitle (99) for env/argv section captions.
		ParamSectionHeading: lipgloss.Color("61"),
		ModalBody:           lipgloss.Color("235"),
		TableHeader:         lipgloss.Color("235"),
		TableCell:           lipgloss.Color("235"),
		TableSelected:       lipgloss.Color("255"),
		TableSelectedBg:     lipgloss.Color("27"),
		// Distinct from ModalTitle / ParamSectionHeading; green accent on light bg.
		ParamProfileName:       lipgloss.Color("30"),
		ParamProfileInactive:   lipgloss.Color("238"),
		SplitPaneBorderFocused: lipgloss.Color("27"),
		SplitPaneBorderDim:     border,
	}
}

// initialThemePick maps LLML_THEME to the starting cycle index.
func initialThemePick() int {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(EnvLLMLTheme)))
	switch v {
	case "dark":
		return themePickDark
	case "light":
		return themePickLight
	default:
		return themePickAuto
	}
}

// themeFromPick returns the palette for a pick value. themePickAuto uses
// isDark to choose dark vs light (same rules as LLML_THEME=auto).
func themeFromPick(pick int, isDark bool) Theme {
	switch pick {
	case themePickDark:
		return DarkTheme()
	case themePickLight:
		return LightTheme()
	default:
		if isDark {
			return DarkTheme()
		}
		return LightTheme()
	}
}

// resolveTheme picks a theme from LLML_THEME and terminal background detection.
func resolveTheme() Theme {
	return resolveThemeWithDetector(func() bool { return compat.HasDarkBackground })
}

// resolveThemeWithDetector is like resolveTheme but uses detectDark for the auto path
// (including unknown env values), so tests do not depend on the real terminal.
func resolveThemeWithDetector(detectDark func() bool) Theme {
	return themeFromPick(initialThemePick(), detectDark())
}
