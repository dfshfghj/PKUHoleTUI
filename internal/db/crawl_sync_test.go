package db

import (
	"reflect"
	"testing"

	"treehole/internal/models"
)

func TestSaveCrawlSnapshotReconcilesAndRestores(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1, Text: "preserved"}, {Pid: 2}, {Pid: 3}}, []models.Comment{{Cid: 10, Pid: 1, Text: "old"}, {Cid: 11, Pid: 1}, {Cid: 20, Pid: 2}, {Cid: 30, Pid: 3}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 1, Text: "updated"}}, []models.Comment{{Cid: 11, Pid: 1}, {Cid: 12, Pid: 1}}, []int32{2, 99}); err != nil {
		t.Fatal(err)
	}
	var comments []models.Comment
	if err := d.db.Order("cid").Find(&comments).Error; err != nil {
		t.Fatal(err)
	}
	if got := []bool{comments[0].DeletedAt != 0, comments[1].DeletedAt != 0, comments[2].DeletedAt != 0, comments[3].DeletedAt != 0, comments[4].DeletedAt != 0}; !reflect.DeepEqual(got, []bool{true, false, false, true, false}) {
		t.Fatalf("deleted flags = %v", got)
	}
	if comments[0].Text != "old" {
		t.Fatalf("deleted text lost: %#v", comments[0])
	}
	var missing models.Post
	if err := d.db.First(&missing, "pid = ?", 2).Error; err != nil || missing.DeletedAt == 0 {
		t.Fatalf("missing post: %#v, %v", missing, err)
	}
	var count int64
	d.db.Model(&models.Post{}).Where("pid = ?", 99).Count(&count)
	if count != 0 {
		t.Fatal("created synthetic deleted post")
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 2}}, []models.Comment{{Cid: 20, Pid: 2}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.db.First(&missing, "pid = ?", 2).Error; err != nil || missing.DeletedAt != 0 {
		t.Fatalf("restored post: %#v, %v", missing, err)
	}
	var restored models.Comment
	d.db.First(&restored, "cid = ?", 20)
	if restored.DeletedAt != 0 {
		t.Fatal("comment not restored")
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 1}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	d.db.Model(&models.Comment{}).Where("pid = ? AND deleted_at = 0", 1).Count(&count)
	if count != 0 {
		t.Fatalf("empty snapshot left %d comments active", count)
	}
}

func TestSaveCrawlSnapshotKeepsFirstDeletionTime(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1}}, []models.Comment{{Cid: 10, Pid: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 1}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	var comment models.Comment
	if err := d.db.First(&comment, "cid = ?", 10).Error; err != nil {
		t.Fatal(err)
	}
	if comment.DeletedAt == 0 {
		t.Fatal("deletion time not recorded")
	}
	// Simulate an earlier detection and confirm repeated snapshots keep it.
	if err := d.db.Model(&models.Comment{}).Where("cid = ?", 10).Update("deleted_at", 12345).Error; err != nil {
		t.Fatal(err)
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 1}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.db.First(&comment, "cid = ?", 10).Error; err != nil || comment.DeletedAt != 12345 {
		t.Fatalf("first deletion time overwritten: %#v, %v", comment, err)
	}
}

func TestSaveCrawlSnapshotRollsBack(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1, Text: "before"}, {Pid: 2}}, []models.Comment{{Cid: 10, Pid: 1}, {Cid: 20, Pid: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := d.db.Exec("CREATE TRIGGER fail_snapshot_comment BEFORE INSERT ON comments BEGIN SELECT RAISE(ABORT, 'forced failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 1, Text: "after"}}, []models.Comment{{Cid: 11, Pid: 1}}, []int32{2}); err == nil {
		t.Fatal("expected failure")
	}
	var post models.Post
	d.db.First(&post, "pid = ?", 1)
	if post.Text != "before" || post.DeletedAt != 0 {
		t.Fatalf("post changed: %#v", post)
	}
	var count int64
	d.db.Model(&models.Comment{}).Where("deleted_at > 0 OR cid = ?", 11).Count(&count)
	if count != 0 {
		t.Fatalf("partial comment changes: %d", count)
	}
	post = models.Post{}
	d.db.First(&post, "pid = ?", 2)
	if post.DeletedAt != 0 {
		t.Fatal("missing post deletion was not rolled back")
	}
}

func TestFindMissingPostPIDs(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1}, {Pid: 2}, {Pid: 3, DeletedAt: 1000}, {Pid: 4}, {Pid: 5}}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := d.FindMissingPostPIDs(2, 4, []int32{4, 99})
	if err != nil || !reflect.DeepEqual(got, []int32{2}) {
		t.Fatalf("missing = %v, %v", got, err)
	}
}

func TestSaveCrawlSnapshotLargeCommentSnapshot(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	comments := make([]models.Comment, 2000)
	for i := range comments {
		comments[i] = models.Comment{Cid: int32(i + 1), Pid: 1}
	}
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1}}, comments); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveCrawlSnapshot([]models.Post{{Pid: 1}}, comments[1:], nil); err != nil {
		t.Fatal(err)
	}
	var deleted []int32
	if err := d.db.Model(&models.Comment{}).Where("deleted_at > 0").Pluck("cid", &deleted).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deleted, []int32{1}) {
		t.Fatalf("deleted = %v", deleted)
	}
}

func TestSaveCrawlResultKeepsPartialComments(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1}}, []models.Comment{{Cid: 1, Pid: 1}, {Cid: 2, Pid: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1}}, []models.Comment{{Cid: 2, Pid: 1}}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := d.db.Model(&models.Comment{}).Where("pid = ? AND deleted_at = 0", 1).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("partial result left %d active comments", count)
	}
}

func TestSaveCrawlSnapshotIgnoresEmbeddedAssociations(t *testing.T) {
	d, cleanup := setupTestDB(t)
	defer cleanup()
	if err := d.SaveCrawlResult([]models.Post{{Pid: 1}}, []models.Comment{{Cid: 1, Pid: 1, Text: "preserved"}}); err != nil {
		t.Fatal(err)
	}
	quoteID := int32(99)
	posts := []models.Post{{Pid: 1, Comments: []models.Comment{{Cid: 98, Pid: 1}}}}
	comments := []models.Comment{{Cid: 2, Pid: 1, QuoteID: &quoteID, Quote: &models.Comment{Cid: 99, Pid: 1, Text: "quoted-only"}}}
	if err := d.SaveCrawlSnapshot(posts, comments, nil); err != nil {
		t.Fatal(err)
	}
	var stored []models.Comment
	if err := d.db.Order("cid").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("embedded comments persisted: %#v", stored)
	}
	if stored[0].DeletedAt == 0 || stored[0].Text != "preserved" || stored[1].Cid != 2 || stored[1].DeletedAt != 0 || stored[1].QuoteID == nil || *stored[1].QuoteID != quoteID {
		t.Fatalf("unexpected snapshot: %#v", stored)
	}
}
