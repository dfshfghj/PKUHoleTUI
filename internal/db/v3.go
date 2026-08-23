package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"treehole/internal/models"
)

func (d *Database) GetPostsV3Page(page, limit int, keyword string, kind, label *int) ([]models.Post, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	var posts []models.Post
	var err error
	if keyword != "" {
		if q, ok := d.buildFTSQuery(keyword); ok {
			matchExpr := fmt.Sprintf("text_search : (%s)", q.match)
			if q.pid != nil {
				matchExpr += fmt.Sprintf(" AND rowid = %d", q.pid)
			}
			if kind != nil {
				matchExpr += fmt.Sprintf(" AND kind : \"%d\"", *kind)
			}
			if label != nil {
				matchExpr += fmt.Sprintf(" AND label : \"%d\"", *label)
			}
			rowsSQL := fmt.Sprintf(
				"SELECT %s FROM posts p WHERE p.pid IN (SELECT rowid FROM %s WHERE posts_fts MATCH ? ORDER BY rowid DESC LIMIT ? OFFSET ?) ORDER BY p.pid DESC",
				qualifiedPostColumns("p"), ftsTableName,
			)
			args := []interface{}{matchExpr, limit, (page - 1) * limit}
			err = d.db.Raw(rowsSQL, args...).Scan(&posts).Error
			if err != nil {
				return nil, 0, err
			}
			return posts, placeholderTotal(page, limit, len(posts)), nil
		}
	}

	build := func() *gorm.DB {
		q := d.db.Model(&models.Post{})
		if kind != nil {
			q = q.Where("kind = ?", *kind)
		}
		if label != nil {
			q = q.Where("label = ?", *label)
		}
		if keyword != "" {
			q = applyPostSearch(q, keyword)
		}
		return q
	}
	err = build().Select(postListColumns).Order("pid DESC").Offset((page - 1) * limit).Limit(limit).Find(&posts).Error
	return posts, placeholderTotal(page, limit, len(posts)), err
}

func placeholderTotal(page, limit, got int) int {
	total := (page-1)*limit + got
	if got == limit {
		total++
	}
	return total
}

func (d *Database) GetCommentsByPidPage(pid int32, page, limit int, sortAsc bool) ([]models.Comment, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	order := "ASC"
	if !sortAsc {
		order = "DESC"
	}
	var comments []models.Comment
	err := d.db.Model(&models.Comment{}).
		Preload("Quote").
		Where("pid = ?", pid).
		Order("cid " + order).
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&comments).Error
	return comments, err
}

func (d *Database) CountCommentsByPid(pid int32) (int64, error) {
	return d.cachedCount(fmt.Sprintf("v3cmts:%d", pid), func() (int64, error) {
		var total int64
		err := d.db.Model(&models.Comment{}).Where("pid = ?", pid).Count(&total).Error
		return total, err
	})
}

func (d *Database) GetCommentsForPids(pids []int32, limitPerPid int) (map[int32][]models.Comment, error) {
	out := make(map[int32][]models.Comment, len(pids))
	if len(pids) == 0 || limitPerPid <= 0 {
		return out, nil
	}
	placeholders := make([]string, len(pids))
	args := make([]interface{}, 0, len(pids)+1)
	for i, pid := range pids {
		placeholders[i] = "?"
		args = append(args, pid)
	}
	args = append(args, limitPerPid)
	sql := fmt.Sprintf(
		`SELECT cid, pid, anonymous, hidden, identity_show, identity_type, is_lz,
		       mention, name_tag, reward_good, text, timestamp, quote_id, media_ids
		 FROM (
		   SELECT *, ROW_NUMBER() OVER (PARTITION BY pid ORDER BY cid ASC) AS rn
		   FROM comments WHERE pid IN (%s)
		 ) WHERE rn <= ? ORDER BY pid DESC, cid ASC`,
		strings.Join(placeholders, ","),
	)
	var rows []models.Comment
	if err := d.db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	quoteIDs := make([]int32, 0, len(rows))
	for _, r := range rows {
		if r.QuoteID != nil {
			quoteIDs = append(quoteIDs, *r.QuoteID)
		}
		out[r.Pid] = append(out[r.Pid], r)
	}
	if len(quoteIDs) > 0 {
		qph := make([]string, len(quoteIDs))
		qargs := make([]interface{}, len(quoteIDs))
		for i, id := range quoteIDs {
			qph[i] = "?"
			qargs[i] = id
		}
		var quotes []models.Comment
		if err := d.db.Model(&models.Comment{}).
			Where(fmt.Sprintf("cid IN (%s)", strings.Join(qph, ",")), qargs...).
			Find(&quotes).Error; err == nil {
			qm := make(map[int32]*models.Comment, len(quotes))
			for i := range quotes {
				qm[quotes[i].Cid] = &quotes[i]
			}
			for pid := range out {
				cs := out[pid]
				for i := range cs {
					if cs[i].QuoteID != nil {
						cs[i].Quote = qm[*cs[i].QuoteID]
					}
				}
			}
		}
	}
	return out, nil
}
