package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type themePalette struct {
	bg           lipgloss.TerminalColor
	surface      lipgloss.TerminalColor
	border       lipgloss.TerminalColor
	muted        lipgloss.TerminalColor
	text         lipgloss.TerminalColor
	textSoft     lipgloss.TerminalColor
	accent       lipgloss.TerminalColor
	accentStrong lipgloss.TerminalColor
	accentSoft   lipgloss.TerminalColor
	accentText   lipgloss.TerminalColor
	link         lipgloss.TerminalColor
	success      lipgloss.TerminalColor
	warning      lipgloss.TerminalColor
}

var (
	colorBg           lipgloss.TerminalColor
	colorSurface      lipgloss.TerminalColor
	colorBorder       lipgloss.TerminalColor
	colorMuted        lipgloss.TerminalColor
	colorText         lipgloss.TerminalColor
	colorTextSoft     lipgloss.TerminalColor
	colorAccent       lipgloss.TerminalColor
	colorAccentStrong lipgloss.TerminalColor
	colorAccentSoft   lipgloss.TerminalColor
	colorAccentText   lipgloss.TerminalColor
	colorLink         lipgloss.TerminalColor
	colorSuccess      lipgloss.TerminalColor
	colorWarning      lipgloss.TerminalColor

	baseStyle lipgloss.Style

	tabBarStyle lipgloss.Style

	tabItemStyle lipgloss.Style

	tabItemActiveStyle lipgloss.Style

	contentStyle lipgloss.Style

	footerStyle lipgloss.Style

	statusModeStyle lipgloss.Style

	statusPageStyle lipgloss.Style

	statusInfoStyle lipgloss.Style

	statusSessionOnlineStyle lipgloss.Style

	statusSessionOfflineStyle lipgloss.Style

	statusClockStyle lipgloss.Style

	vTitleStyle lipgloss.Style

	vSubtitleStyle lipgloss.Style

	vSectionTitleStyle lipgloss.Style

	vSectionTitleFocused lipgloss.Style

	vDetailSectionFocused lipgloss.Style

	vDetailSection lipgloss.Style

	vStatusCard lipgloss.Style

	vStatsCard lipgloss.Style

	vStatLabelStyle lipgloss.Style

	vStatValueStyle lipgloss.Style

	vStatusRunningStyle lipgloss.Style

	vStatusStoppedStyle lipgloss.Style

	vButtonDefault lipgloss.Style

	vButtonActive lipgloss.Style

	vButtonActiveDanger lipgloss.Style

	vButtonDisabled lipgloss.Style

	vErrorStyle lipgloss.Style

	vHelpStyle lipgloss.Style

	vListItemStyle lipgloss.Style

	vListItemSelectedStyle lipgloss.Style

	vSearchInput lipgloss.Style

	vSearchInputFocused lipgloss.Style

	vLoadingStyle lipgloss.Style

	vEmptyStyle lipgloss.Style

	vPaginationStyle lipgloss.Style

	vPostPidStyle lipgloss.Style

	vPostTextStyle lipgloss.Style

	vPostMetaStyle lipgloss.Style

	vPostTimeStyle lipgloss.Style

	vPostReplyStyle lipgloss.Style

	vPostLikeStyle lipgloss.Style

	vCommentQuoteStyle lipgloss.Style

	vCommentMetaTimeStyle lipgloss.Style

	vCommentAuthorStyle lipgloss.Style

	vCommentSelectedStyle lipgloss.Style

	vDividerStyle lipgloss.Style

	vFormLabelStyle lipgloss.Style

	vFormInput lipgloss.Style

	vFormInputFocused lipgloss.Style

	vFormSaveBtn lipgloss.Style

	vFormSaveActive lipgloss.Style

	vLogLineStyle lipgloss.Style

	vDialogTitleStyle lipgloss.Style

	vDialogHelpStyle lipgloss.Style

	dialogCard lipgloss.Style

	helpCard lipgloss.Style
)

func init() {
	applyTheme("")
}

func applyTheme(mode string) {
	palette := paletteForTheme(resolveThemeMode(mode))

	colorBg = palette.bg
	colorSurface = palette.surface
	colorBorder = palette.border
	colorMuted = palette.muted
	colorText = palette.text
	colorTextSoft = palette.textSoft
	colorAccent = palette.accent
	colorAccentStrong = palette.accentStrong
	colorAccentSoft = palette.accentSoft
	colorAccentText = palette.accentText
	colorLink = palette.link
	colorSuccess = palette.success
	colorWarning = palette.warning

	baseStyle = lipgloss.NewStyle().
		Foreground(colorText)

	tabBarStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Background(colorSurface)

	tabItemStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(colorTextSoft).
		Background(colorSurface)

	tabItemActiveStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(colorAccentText).
		Background(colorAccentStrong).
		Bold(true)

	contentStyle = lipgloss.NewStyle().
		Padding(1, 2)

	footerStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Background(colorSurface)

	statusModeStyle = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorAccentStrong).
		Bold(true).
		Padding(0, 1)

	statusPageStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorAccentSoft).
		Bold(true).
		Padding(0, 1)

	statusInfoStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorSurface).
		Padding(0, 1)

	statusSessionOnlineStyle = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorSuccess).
		Bold(true).
		Padding(0, 1)

	statusSessionOfflineStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorBorder).
		Bold(true).
		Padding(0, 1)

	statusClockStyle = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorAccent).
		Bold(true).
		Padding(0, 1)

	vTitleStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true).
		Padding(0, 1)

	vSubtitleStyle = lipgloss.NewStyle().
		Foreground(colorTextSoft).
		Bold(true).
		Padding(0, 1)

	vSectionTitleStyle = lipgloss.NewStyle().
		Foreground(colorTextSoft).
		Bold(true).
		Padding(0, 1)

	vSectionTitleFocused = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorAccent).
		Bold(true).
		Padding(0, 1)

	vDetailSectionFocused = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorAccent).
		Padding(0, 0, 0, 1)

	vDetailSection = lipgloss.NewStyle().
		Padding(0, 0, 0, 2)

	vStatusCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)

	vStatsCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)

	vStatLabelStyle = lipgloss.NewStyle().
		Foreground(colorMuted)

	vStatValueStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true)

	vStatusRunningStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true)

	vStatusStoppedStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Bold(true)

	vButtonDefault = lipgloss.NewStyle().
		Foreground(colorTextSoft).
		Background(colorBorder).
		Padding(0, 2).
		Margin(0, 1).
		Bold(true)

	vButtonActive = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorAccent).
		Padding(0, 2).
		Margin(0, 1).
		Bold(true)

	vButtonActiveDanger = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorAccentStrong).
		Padding(0, 2).
		Margin(0, 1).
		Bold(true)

	vButtonDisabled = lipgloss.NewStyle().
		Foreground(colorBorder).
		Padding(0, 2).
		Margin(0, 1)

	vErrorStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 1)

	vHelpStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(1, 0)

	vListItemStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(colorTextSoft)

	vListItemSelectedStyle = lipgloss.NewStyle().
		Foreground(colorAccent).
		Padding(0, 1).
		Bold(true)

	vSearchInput = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(60).
		Foreground(colorMuted)

	vSearchInputFocused = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorAccent).
		Padding(0, 1).
		Width(60).
		Foreground(colorText)

	vLoadingStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(1, 0)

	vEmptyStyle = lipgloss.NewStyle().
		Foreground(colorBorder).
		Padding(1, 0)

	vPaginationStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(1, 0)

	vPostPidStyle = lipgloss.NewStyle().
		Foreground(colorLink).
		Bold(true)

	vPostTextStyle = lipgloss.NewStyle().
		Foreground(colorTextSoft).
		Padding(0, 0, 0, 2)

	vPostMetaStyle = lipgloss.NewStyle().
		Foreground(colorMuted)

	vPostTimeStyle = lipgloss.NewStyle().
		Foreground(colorLink)

	vPostReplyStyle = lipgloss.NewStyle().
		Foreground(colorLink)

	vPostLikeStyle = lipgloss.NewStyle().
		Foreground(colorLink)

	vCommentQuoteStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorBorder).
		Padding(0, 0, 0, 1)

	vCommentMetaTimeStyle = lipgloss.NewStyle().
		Foreground(colorLink)

	vCommentAuthorStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true)

	vCommentSelectedStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorAccent).
		Padding(0, 0, 0, 1)

	vDividerStyle = lipgloss.NewStyle().
		Foreground(colorBorder)

	vFormLabelStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Width(12).
		Align(lipgloss.Right).
		Padding(0, 1)

	vFormInput = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(40).
		Foreground(colorTextSoft)

	vFormInputFocused = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorAccent).
		Padding(0, 1).
		Width(40).
		Foreground(colorText)

	vFormSaveBtn = lipgloss.NewStyle().
		Foreground(colorTextSoft).
		Background(colorBorder).
		Padding(0, 2).
		Margin(1, 0, 0, 13).
		Bold(true)

	vFormSaveActive = lipgloss.NewStyle().
		Foreground(colorAccentText).
		Background(colorAccent).
		Padding(0, 2).
		Margin(1, 0, 0, 13).
		Bold(true)

	vLogLineStyle = lipgloss.NewStyle().
		Foreground(colorTextSoft)

	vDialogTitleStyle = lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		Padding(0, 1)

	vDialogHelpStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(1, 0)

	dialogCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 3).
		Width(70)

	helpCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 1)
}

func resolveThemeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(os.Getenv("TREEHOLE_THEME")))
	}

	switch mode {
	case "light", "dark":
		return mode
	default:
		return "auto"
	}
}

func paletteForTheme(mode string) themePalette {
	if shouldUseDarkTheme(mode) {
		return themePalette{
			bg:           lipgloss.Color("#000000"),
			surface:      lipgloss.Color("#333333"),
			border:       lipgloss.Color("#4A4A4A"),
			muted:        lipgloss.Color("#888888"),
			textSoft:     lipgloss.Color("#B0B0B0"),
			text:         lipgloss.Color("#FFFFFF"),
			accent:       lipgloss.Color("#ff78bb"),
			accentStrong: lipgloss.Color("#c8618d"),
			accentSoft:   lipgloss.Color("#2A1520"),
			accentText:   lipgloss.Color("#000000"),
			link:         lipgloss.Color("#fe9fcf"),
			success:      lipgloss.Color("#d0ee90"),
			warning:      lipgloss.Color("#FFA500"),
		}
	}

	return themePalette{
		bg:           lipgloss.Color("#FFF9F2"),
		surface:      lipgloss.Color("#e7e7e7"),
		border:       lipgloss.Color("#C8B7AE"),
		muted:        lipgloss.Color("#c79f92"),
		textSoft:     lipgloss.Color("#5E4741"),
		text:         lipgloss.Color("#241916"),
		accent:       lipgloss.Color("#b8467d"),
		accentStrong: lipgloss.Color("#8b325d"),
		accentSoft:   lipgloss.Color("#F4DCE7"),
		accentText:   lipgloss.Color("#FFF9F2"),
		link:         lipgloss.Color("#92657b"),
		success:      lipgloss.Color("#adde58"),
		warning:      lipgloss.Color("#A65A00"),
	}
}

func shouldUseDarkTheme(mode string) bool {
	switch mode {
	case "dark":
		return true
	case "light":
		return false
	default:
		return lipgloss.HasDarkBackground()
	}
}
