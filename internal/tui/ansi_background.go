package tui

import (
	"fmt"
	"image/color"
	"strings"
)

func preserveBackgroundAfterReset(rendered string, bg color.Color) string {
	if rendered == "" || bg == nil {
		return rendered
	}
	bgReset := resetWithBackground(bg)
	return strings.NewReplacer(
		"\x1b[m", bgReset,
		"\x1b[0m", bgReset,
	).Replace(rendered)
}

func resetWithBackground(bg color.Color) string {
	r, g, b, _ := bg.RGBA()
	return fmt.Sprintf("\x1b[0;48;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}
