package db

import (
	"fmt"

	"treehole/internal/models"
)

// PIDSample measures locally observed live posts over a PID interval. Missing
// local records may reflect incomplete crawling as well as remote deletion.
type PIDSample struct {
	MinPID    int32
	MaxPID    int32
	LivePosts int64
}

func (d *Database) GetPostPIDSample(targetPID, latestPID int32) (PIDSample, error) {
	if targetPID <= 0 || latestPID <= 0 {
		return PIDSample{}, fmt.Errorf("PID must be positive")
	}
	if targetPID > latestPID {
		return PIDSample{}, nil
	}
	var result struct {
		MinPID    int32 `gorm:"column:min_pid"`
		MaxPID    int32 `gorm:"column:max_pid"`
		LivePosts int64 `gorm:"column:live_posts"`
	}
	err := d.db.Model(&models.Post{}).
		Select("COALESCE(MIN(pid), 0) AS min_pid, COALESCE(MAX(pid), 0) AS max_pid, COALESCE(SUM(CASE WHEN deleted_at = 0 THEN 1 ELSE 0 END), 0) AS live_posts").
		Where("pid >= ? AND pid <= ?", targetPID, latestPID).Scan(&result).Error
	return PIDSample(result), err
}
