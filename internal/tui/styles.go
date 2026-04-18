package tui

import (
	"charm.land/lipgloss/v2"

	btable "charm.land/bubbles/v2/table"
)

// styles holds all lipgloss styles for one resolved theme.
type styles struct {
	app               lipgloss.Style
	title             lipgloss.Style
	titleBoldLeft     lipgloss.Style
	titleToastRowWrap lipgloss.Style
	subtitle          lipgloss.Style
	body              lipgloss.Style
	bodyBold          lipgloss.Style
	bodyDim           lipgloss.Style
	footer            lipgloss.Style
	// scrollBarColumn colors the █/░ track beside viewports; no margin (unlike footer).
	scrollBarColumn        lipgloss.Style
	errLine                lipgloss.Style
	runtimePanel           lipgloss.Style
	portConfigTitle        lipgloss.Style
	portConfigBox          lipgloss.Style
	paramSectionBox        lipgloss.Style
	paramConfirmDialog     lipgloss.Style
	paramSectionHeading    lipgloss.Style
	themeToastInline       lipgloss.Style
	paramProfileName       lipgloss.Style
	serverLogViewport      lipgloss.Style
	splitPaneChromeFocused lipgloss.Style
	splitPaneChromeDim     lipgloss.Style
	// launchPreview wraps the bordered launch command viewport (margin only).
	launchPreview lipgloss.Style
	// launchPreviewViewport frames the scrollable command text (border + horizontal padding).
	launchPreviewViewport lipgloss.Style
	// launchPreviewContent styles the command string inside the viewport.
	launchPreviewContent lipgloss.Style
	table                btable.Styles
}

// newStyles builds lipgloss styles from a Theme. Header and Cell use
// PaddingRight(1) for column spacing. Selected has no padding — it wraps
// the fully-rendered row (cells already padded) so adding padding here
// would double-pad and shift the content.
func newStyles(theme Theme) styles {
	return styles{
		app: lipgloss.NewStyle().Padding(1, appPaddingH),
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Title).
			MarginBottom(1),
		titleBoldLeft: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Title),
		titleToastRowWrap: lipgloss.NewStyle().
			MarginBottom(1),
		subtitle: lipgloss.NewStyle().
			Foreground(theme.Subtitle).
			MarginBottom(1),
		body: lipgloss.NewStyle().
			Foreground(theme.Body),
		bodyBold: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Body),
		bodyDim: lipgloss.NewStyle().
			Foreground(theme.Subtitle),
		footer: lipgloss.NewStyle().
			Foreground(theme.Footer).
			MarginTop(1),
		scrollBarColumn: lipgloss.NewStyle().
			Foreground(theme.Footer),
		errLine: lipgloss.NewStyle().Foreground(theme.Error),
		runtimePanel: lipgloss.NewStyle().
			BorderTop(true).
			BorderForeground(theme.Border).
			Foreground(theme.RuntimePanel).
			Padding(1, 0).
			MarginTop(1),
		portConfigTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.ModalTitle),
		portConfigBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2).
			Foreground(theme.ModalBody),
		// Nested sections inside the parameters modal (env + argv).
		paramSectionBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		paramConfirmDialog: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Error).
			Padding(0, 1),
		paramSectionHeading: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.ParamSectionHeading),
		// Compact reversed chip on the title row (no extra viewport row).
		themeToastInline: lipgloss.NewStyle().
			Bold(true).
			Reverse(true).
			Padding(0, 1),
		paramProfileName: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.ParamProfileName),
		serverLogViewport: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		splitPaneChromeFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SplitPaneBorderFocused).
			Padding(0, 1),
		splitPaneChromeDim: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SplitPaneBorderDim).
			Padding(0, 1),
		launchPreview: lipgloss.NewStyle().MarginTop(1),
		launchPreviewViewport: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		launchPreviewContent: lipgloss.NewStyle().
			Foreground(theme.Body).
			Bold(true),
		table: btable.Styles{
			Header: lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.TableHeader).
				PaddingRight(1),
			Cell: lipgloss.NewStyle().
				PaddingRight(1),
			Selected: lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.TableSelected).
				Background(theme.TableSelectedBg),
		},
	}
}
