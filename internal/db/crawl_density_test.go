package db

import (
	"testing"

	"treehole/internal/models"
)

func TestPostPIDSampleIncludesDeletedEndpoints(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	posts := []models.Post{{Pid: 10, DeletedAt: 1000}, {Pid: 12}, {Pid: 16}, {Pid: 20, DeletedAt: 2000}, {Pid: 30}}
	if err := d.db.Create(&posts).Error; err != nil {
		t.Fatal(err)
	}
	sample, err := d.GetPostPIDSample(10, 20)
	if err != nil || sample != (PIDSample{MinPID: 10, MaxPID: 20, LivePosts: 2}) {
		t.Fatalf("sample=%+v err=%v", sample, err)
	}
	// The window around 21 still includes the nearby posts.
	sample, err = d.GetPostPIDSample(21, 29)
	if err != nil || sample != (PIDSample{MinPID: 10, MaxPID: 20, LivePosts: 2}) {
		t.Fatalf("nearby sample=%+v err=%v", sample, err)
	}
	sample, err = d.GetPostPIDSample(100000, 200000)
	if err != nil || sample != (PIDSample{}) {
		t.Fatalf("empty sample=%+v err=%v", sample, err)
	}
	if _, err := d.GetPostPIDSample(0, 20); err == nil {
		t.Fatal("invalid PID accepted")
	}
}
