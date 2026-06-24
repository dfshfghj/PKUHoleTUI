package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func paintedDialogSpaces(width int) string {
	spaces := strings.Repeat(" ", width)
	return strings.TrimSuffix(dialogBackgroundFillStyle().Render(spaces), "\x1b[m")
}

func TestLogsDialogPaintsEmptyTextAreaBlanks(t *testing.T) {
	dialog := NewLogsDialog()
	dialog.SetLines(nil)

	output := dialog.View(50, 8)
	firstLine := strings.Split(output, "\n")[0]
	if got := lipgloss.Width(firstLine); got != 50 {
		t.Fatalf("logs empty line width = %d, want 50:\n%q", got, firstLine)
	}
	if !strings.Contains(firstLine, paintedDialogSpaces(4)) {
		t.Fatalf("logs empty line trailing blanks should carry dialog background:\n%q", firstLine)
	}
}
