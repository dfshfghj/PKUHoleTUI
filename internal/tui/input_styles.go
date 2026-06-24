package tui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

func styleTextInput(input *textinput.Model, bg, textColor, placeholderColor color.Color) {
	base := lipgloss.NewStyle().Background(bg)
	styles := input.Styles()
	styles.Focused.Prompt = base
	styles.Focused.Text = base.Foreground(textColor)
	styles.Focused.Placeholder = base.Foreground(placeholderColor)
	styles.Focused.Suggestion = base.Foreground(placeholderColor)
	styles.Blurred.Prompt = base
	styles.Blurred.Text = base.Foreground(textColor)
	styles.Blurred.Placeholder = base.Foreground(placeholderColor)
	styles.Blurred.Suggestion = base.Foreground(placeholderColor)
	input.SetStyles(styles)
}

func styleTextarea(input *textarea.Model, bg, textColor, placeholderColor color.Color) {
	base := lipgloss.NewStyle().Background(bg)
	styles := input.Styles()
	focused := styles.Focused
	focused.Base = base
	focused.CursorLine = lipgloss.NewStyle().Background(bg)
	focused.EndOfBuffer = base.Foreground(textColor)
	focused.Placeholder = base.Foreground(placeholderColor)
	focused.Prompt = base.Foreground(textColor)
	focused.Text = base.Foreground(textColor)

	blurred := styles.Blurred
	blurred.Base = base
	blurred.CursorLine = lipgloss.NewStyle().Background(bg)
	blurred.EndOfBuffer = base.Foreground(textColor)
	blurred.Placeholder = base.Foreground(placeholderColor)
	blurred.Prompt = base.Foreground(textColor)
	blurred.Text = base.Foreground(textColor)

	styles.Focused = focused
	styles.Blurred = blurred
	input.SetStyles(styles)
	if input.Focused() {
		_ = input.Focus()
	} else {
		input.Blur()
	}
}

func fillRenderedBackground(rendered string, width int, fill lipgloss.Style) string {
	if width < 1 || rendered == "" {
		return rendered
	}

	hasTrailingNewline := strings.HasSuffix(rendered, "\n")
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		missing := width - lipgloss.Width(trimmed)
		if missing < 0 {
			missing = 0
		}
		lines[i] = trimmed + fill.Render(strings.Repeat(" ", missing))
	}

	result := strings.Join(lines, "\n")
	if hasTrailingNewline {
		result += "\n"
	}
	return result
}
