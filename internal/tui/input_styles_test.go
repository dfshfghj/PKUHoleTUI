package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestNewSearchInputUsesSurfaceBackgroundForPlaceholderAndText(t *testing.T) {
	input := newSearchInput()
	styles := input.Styles()

	if got := styles.Focused.Placeholder.GetBackground(); got != colorSurface {
		t.Fatalf("search placeholder background = %v, want %v", got, colorSurface)
	}
	if got := styles.Focused.Text.GetBackground(); got != colorSurface {
		t.Fatalf("search text background = %v, want %v", got, colorSurface)
	}
	if got := styles.Focused.Prompt.GetBackground(); got != colorSurface {
		t.Fatalf("search prompt background = %v, want %v", got, colorSurface)
	}
}

func TestNewComposerDialogUsesPanelBackgroundForPlaceholderAndText(t *testing.T) {
	dialog := NewComposerDialog()
	styles := dialog.input.Styles()

	if got := styles.Focused.Placeholder.GetBackground(); got != colorBg {
		t.Fatalf("composer placeholder background = %v, want %v", got, colorBg)
	}
	if got := styles.Focused.Text.GetBackground(); got != colorBg {
		t.Fatalf("composer text background = %v, want %v", got, colorBg)
	}
	if got := styles.Focused.Base.GetBackground(); got != colorBg {
		t.Fatalf("composer base background = %v, want %v", got, colorBg)
	}
}

func TestFillRenderedBackgroundReplacesPlainTrailingSpaces(t *testing.T) {
	fill := lipgloss.NewStyle().Background(colorSurface)
	rendered := "abc   "

	got := fillRenderedBackground(rendered, 4, fill)

	if lipgloss.Width(got) != 4 {
		t.Fatalf("filled output width = %d, want 4", lipgloss.Width(got))
	}
	if got == rendered {
		t.Fatalf("filled output should be normalized when trailing spaces exceed width: %q", got)
	}
}

func TestFillRenderedBackgroundPreservesLineWidthsAcrossNewlines(t *testing.T) {
	fill := lipgloss.NewStyle().Background(colorBg)
	rendered := "a  \nxy"

	got := fillRenderedBackground(rendered, 4, fill)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 4 {
			t.Fatalf("line %d width = %d, want 4", i, lipgloss.Width(line))
		}
	}
}
