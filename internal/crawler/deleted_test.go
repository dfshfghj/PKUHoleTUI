package crawler

import (
	"os"
	"path/filepath"
	"testing"

	"treehole/internal/models"
)

func TestDeletedTrackerRecordsGapsAndInterpolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deleted.json")
	tr := NewDeletedTracker(path)
	tr.RecordPage([]models.Post{
		{Pid: 10, Timestamp: 100},
		{Pid: 12, Timestamp: 300},
		{Pid: 13, Timestamp: 400},
	})
	if err := tr.Finalize(); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	rep, err := loadDeletedReport(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rep.Deleted != 1 || len(rep.DeletedPosts) != 1 {
		t.Fatalf("expected 1 deleted post, got %d (%d entries)", rep.Deleted, len(rep.DeletedPosts))
	}
	got := rep.DeletedPosts[0]
	if got.PID != 11 {
		t.Fatalf("expected pid 11, got %d", got.PID)
	}
	// 插值: tsLo=100 (pid 10), tsHi=300 (pid 12), span=2 → pid 11 → 200
	if got.Timestamp != 200 {
		t.Fatalf("expected interpolated timestamp 200, got %d", got.Timestamp)
	}
	if rep.Span != [2]int64{10, 13} {
		t.Fatalf("unexpected span %v", rep.Span)
	}
}

func TestDeletedTrackerMergeSelfCorrectsAlivePids(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deleted.json")
	tr := NewDeletedTracker(path)
	tr.RecordPage([]models.Post{{Pid: 10, Timestamp: 100}, {Pid: 12, Timestamp: 300}})
	if err := tr.Finalize(); err != nil {
		t.Fatalf("Finalize #1: %v", err)
	}

	// 第二次扫描观测到 11 存活 → 合并时应自我纠错移除
	tr2 := NewDeletedTracker(path)
	tr2.RecordPage([]models.Post{
		{Pid: 10, Timestamp: 100},
		{Pid: 11, Timestamp: 200},
		{Pid: 12, Timestamp: 300},
	})
	if err := tr2.Finalize(); err != nil {
		t.Fatalf("Finalize #2: %v", err)
	}
	rep, err := loadDeletedReport(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rep.Deleted != 0 {
		t.Fatalf("expected 0 deleted after self-correction, got %d", rep.Deleted)
	}
}

func TestDeletedTrackerKeepsUnparsableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deleted.json")
	// 写坏文件，Finalize 必须报错且保留原内容
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	tr := NewDeletedTracker(path)
	tr.RecordPage([]models.Post{{Pid: 1, Timestamp: 1}, {Pid: 3, Timestamp: 3}})
	if err := tr.Finalize(); err == nil {
		t.Fatalf("expected error for broken existing file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "{broken" {
		t.Fatalf("broken file must be preserved, got %q", string(data))
	}
}

func TestDeletedTrackerResetClearsState(t *testing.T) {
	tr := NewDeletedTracker(filepath.Join(t.TempDir(), "x.json"))
	tr.RecordPage([]models.Post{{Pid: 1, Timestamp: 1}})
	tr.Reset()
	if err := tr.Finalize(); err == nil {
		t.Fatalf("expected error after reset (no collected data)")
	}
}
