package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// scrollbarState describes a vertical scrollbar for one scroll container.
// It is a pure function of the container metrics so the same values drive
// both rendering and mouse hit-testing.
type scrollbarState struct {
	Visible   bool
	TrackH    int
	ThumbSize int
	ThumbPos  int
	MaxOffset int
}

func scrollbarFor(trackH, contentSize, viewportSize, offset int) scrollbarState {
	if trackH <= 0 || viewportSize <= 0 || contentSize <= viewportSize {
		return scrollbarState{}
	}
	thumbSize := clampInt(trackH*viewportSize/contentSize, 1, trackH)
	maxOffset := contentSize - viewportSize
	thumbPos := 0
	if space := trackH - thumbSize; space > 0 {
		thumbPos = clampInt(offset*space/maxOffset, 0, space)
	}
	return scrollbarState{Visible: true, TrackH: trackH, ThumbSize: thumbSize, ThumbPos: thumbPos, MaxOffset: maxOffset}
}

// offsetAtRow maps a row inside the track to a scroll offset while keeping
// the grabbed point of the thumb under the mouse.
func (s scrollbarState) offsetAtRow(row, grabDelta int) int {
	if !s.Visible {
		return 0
	}
	space := s.TrackH - s.ThumbSize
	if space <= 0 {
		return 0
	}
	thumbTop := clampInt(row-grabDelta, 0, space)
	return clampInt(thumbTop*s.MaxOffset/space, 0, s.MaxOffset)
}

// grabAtRow resolves a press on the track: pressing the thumb starts a drag
// at the grabbed point, pressing the track centers the thumb on that row.
// Both return the resulting thumb top row and the grab delta.
func (s scrollbarState) grabAtRow(row int) (thumbTop, grabDelta int) {
	space := s.TrackH - s.ThumbSize
	if space <= 0 {
		return 0, 0
	}
	if row >= s.ThumbPos && row < s.ThumbPos+s.ThumbSize {
		return s.ThumbPos, row - s.ThumbPos
	}
	thumbTop = clampInt(row-s.ThumbSize/2, 0, space)
	return thumbTop, row - thumbTop
}

func (s scrollbarState) thumbTopForOffset(offset int) int {
	space := s.TrackH - s.ThumbSize
	if space <= 0 {
		return 0
	}
	return clampInt(offset*space/maxInt(1, s.MaxOffset), 0, space)
}

// render returns one styled cell per track row.
func (s scrollbarState) render() []string {
	if !s.Visible {
		return nil
	}
	trackStyle := lipgloss.NewStyle().Foreground(colorBorder)
	thumbStyle := lipgloss.NewStyle().Foreground(colorMuted)
	lines := make([]string, s.TrackH)
	for i := range lines {
		if i >= s.ThumbPos && i < s.ThumbPos+s.ThumbSize {
			lines[i] = thumbStyle.Render("█")
		} else {
			lines[i] = trackStyle.Render("│")
		}
	}
	return lines
}

// appendScrollbar joins a scrollbar column to the right edge of a rendered
// block of bodyWidth cells (the column adds one cell to each row).
func appendScrollbar(body string, bar scrollbarState, bodyWidth int) string {
	barLines := bar.render()
	if len(barLines) == 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	if len(lines) < len(barLines) {
		padded := make([]string, len(barLines))
		copy(padded, lines)
		lines = padded
	}
	for i, barLine := range barLines {
		pad := bodyWidth - lipgloss.Width(lines[i])
		if pad > 0 {
			lines[i] += strings.Repeat(" ", pad)
		}
		lines[i] += barLine
	}
	return strings.Join(lines, "\n")
}
