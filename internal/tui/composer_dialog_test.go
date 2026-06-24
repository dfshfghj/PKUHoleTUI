package tui

import (
	"strings"
	"testing"

	"treehole/internal/models"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestComposerDialogViewUsesPanelSpaceAndShowsQuotePreview(t *testing.T) {
	dialog := NewComposerDialog()
	dialog.Configure(ComposerModeComment)
	dialog.SetQuoteTarget(&models.Comment{
		Cid:     23,
		NameTag: "tester",
		Text:    "quoted comment",
	})

	small := stripANSI(dialog.View(60, 18))
	large := stripANSI(dialog.View(90, 30))

	if !strings.Contains(large, "发布评论") {
		t.Fatalf("composer title missing from large panel:\n%s", large)
	}
	if !strings.Contains(large, "引用 #23 tester: quoted comment") {
		t.Fatalf("quote preview missing from large panel:\n%s", large)
	}
	if !strings.Contains(large, "Tab: 切换 | Ctrl+S: 提交") {
		t.Fatalf("composer help text missing from large panel:\n%s", large)
	}
	if len(frameLines(large)) <= len(frameLines(small)) {
		t.Fatalf("expected larger panel to render taller composer, small=%d large=%d", len(frameLines(small)), len(frameLines(large)))
	}
}

func TestParseComposerImagePaths(t *testing.T) {
	got := parseComposerImagePaths(" /tmp/a.jpg, /tmp/b.png\n/tmp/c.webp；/tmp/a.jpg ")
	want := []string{"/tmp/a.jpg", "/tmp/b.png", "/tmp/c.webp"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestComposerDialogHighlightsFocusedInput(t *testing.T) {
	dialog := NewComposerDialog()
	dialog.Configure(ComposerModePost)

	bodyFocused := dialog.renderInputBlock(dialog.input, 32, 2, dialog.composerInputFocused(false))
	if got := lipgloss.NewStyle().Background(colorSurface).Foreground(colorMuted).Render(composerPlaceholder); !strings.Contains(bodyFocused, got) {
		t.Fatalf("body input should include focused background escape")
	}

	_ = dialog.Update(keyCode(tea.KeyTab))
	imageFocused := dialog.renderInputBlock(dialog.imageInput, 32, 2, dialog.composerInputFocused(true), composerImagePlaceholder)
	if got := lipgloss.NewStyle().Background(colorSurface).Foreground(colorMuted).Render(composerImagePlaceholder); !strings.Contains(imageFocused, got) {
		t.Fatalf("image input should include focused background escape")
	}
}

func TestComposerDialogImageLabelAlignsWithInputAndHasPadding(t *testing.T) {
	dialog := NewComposerDialog()
	dialog.Configure(ComposerModePost)

	lines := frameLines(stripANSI(dialog.View(90, 30)))
	labelIdx := -1
	inputIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "图片") && !strings.Contains(line, "图片路径") {
			labelIdx = i
		}
		if strings.Contains(line, composerImagePlaceholder) {
			inputIdx = i
			break
		}
	}
	if labelIdx < 0 || inputIdx < 0 {
		t.Fatalf("missing image label/input in view:\n%s", strings.Join(lines, "\n"))
	}
	if inputIdx-labelIdx != 2 {
		t.Fatalf("image label/input spacing = %d, want one blank padding line", inputIdx-labelIdx)
	}
	if strings.TrimSpace(lines[labelIdx+1]) != "" {
		t.Fatalf("expected blank padding line after image label, got %q", lines[labelIdx+1])
	}
	if leadingSpaces(lines[labelIdx]) != leadingSpaces(lines[inputIdx]) {
		t.Fatalf("image label should align with input: label=%q input=%q", lines[labelIdx], lines[inputIdx])
	}
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
