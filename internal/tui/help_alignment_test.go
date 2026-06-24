package tui

import (
	"fmt"
	"strings"
	"testing"
	"treehole/internal/models"

	"charm.land/lipgloss/v2"
)

func TestHelpPanelAlignmentAcrossWidths(t *testing.T) {
	widths := []int{80, 100, 120, 138, 140, 160}
	for _, w := range widths {
		t.Run(fmt.Sprintf("w=%d", w), func(t *testing.T) {
			m := newTestModel()
			m.Page = PagePosts
			m.Width = w
			m.Height = 30
			m.Dialog = DialogHelp
			m.Posts.CanWrite = true
			m.Posts.PostList = []models.Post{{Pid: 1, Text: "hello", Timestamp: 1000}}

			items := m.helpItems()
			panelWidth := m.helpPanelWidth(w)
			cardWidth := panelWidth - helpCard.GetHorizontalFrameSize()
			if cardWidth < 18 {
				cardWidth = 18
			}
			innerWidth := cardWidth - helpCard.GetHorizontalPadding()
			keyWidth := maxInt(6, minInt(10, innerWidth/3))
			panel := stripANSI(m.renderHelpPanel(panelWidth))
			lines := strings.Split(strings.TrimSuffix(panel, "\n"), "\n")

			// Verify all help item keys (clipped to keyWidth) appear on their own line with desc.
			for _, item := range items {
				visibleKey := item.key
				if lipgloss.Width(visibleKey) > keyWidth {
					visibleKey = clipToVisibleWidth(visibleKey, keyWidth)
				}
				found := 0
				for _, line := range lines {
					if strings.Contains(line, visibleKey) && strings.Contains(line, item.desc) {
						found++
					}
				}
				if found != 1 {
					t.Errorf("item key=%q desc=%q (visibleKey=%q): found on %d combined lines, want 1",
						item.key, item.desc, visibleKey, found)
				}
			}

			// Verify key and description columns are stable across all help-item rows.
			var keyCol, descCol int
			detected := 0
			for _, line := range lines {
				if !strings.HasPrefix(strings.TrimSpace(line), "│") {
					continue
				}
				// Skip title / context / footer lines
				if strings.Contains(line, "快捷键") || strings.Contains(line, m.helpContextTitle()) ||
					strings.Contains(line, "Esc: 关闭") || strings.Contains(line, "╭") || strings.Contains(line, "╰") {
					continue
				}
				// Extract the interior of the card row (between │ ... │)
				inner := strings.TrimPrefix(strings.TrimSpace(line), "│")
				inner = strings.TrimSuffix(inner, "│")
				if strings.TrimSpace(inner) == "" {
					continue
				}

				groups := []int{}
				inSpace := true
				offset := 0
				for _, r := range inner {
					if r == ' ' {
						inSpace = true
					} else if inSpace {
						groups = append(groups, offset)
						inSpace = false
					}
					offset += lipgloss.Width(string(r))
					if len(groups) == 2 {
						break
					}
				}
				if len(groups) < 2 {
					continue
				}

				// Column relative to the panel: interior offset + left border (1) + helpCard padding (1)
				kc := groups[0] + 2
				dc := groups[1] + 2
				if detected == 0 {
					keyCol = kc
					descCol = dc
				} else {
					if kc != keyCol {
						t.Errorf("key column drift: got %d, want %d in line %q", kc, keyCol, line)
					}
					if dc != descCol {
						t.Errorf("desc column drift: got %d, want %d in line %q", dc, descCol, line)
					}
				}
				detected++
			}
			if detected < 3 {
				t.Fatalf("detected only %d help-item rows in panel", detected)
			}
			if descCol <= keyCol {
				t.Fatalf("descCol (%d) should be > keyCol (%d)", descCol, keyCol)
			}
			t.Logf("panelWidth=%d items=%d keyCol=%d descCol=%d", panelWidth, len(items), keyCol, descCol)
		})
	}
}
