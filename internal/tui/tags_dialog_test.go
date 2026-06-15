package tui

import (
	"testing"

	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTagsDialogEnterBackAndSelectedTag(t *testing.T) {
	dialog := NewTagsDialog()
	dialog.SetTags([]models.Tag{
		{ID: 1, Name: "课程", ParentID: 0},
		{ID: 11, Label: "课程心得", ParentID: 1},
		{ID: 12, Label: "课程吐槽", ParentID: 1},
		{ID: 2, Name: "生活", ParentID: 0},
	})

	if tag := dialog.SelectedTag(); tag != nil {
		t.Fatalf("selected tag in group phase = %+v, want nil while children exist", tag)
	}
	if applied := dialog.Enter(); applied {
		t.Fatal("enter on parent with children should drill into child phase")
	}

	dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	tag := dialog.SelectedTag()
	if tag == nil || tag.ID != 12 {
		t.Fatalf("selected child = %+v, want child #12", tag)
	}

	if !dialog.Back() {
		t.Fatal("back should return to group phase")
	}
	if tag := dialog.SelectedTag(); tag != nil {
		t.Fatalf("selected tag after backing to groups = %+v, want nil", tag)
	}
}
