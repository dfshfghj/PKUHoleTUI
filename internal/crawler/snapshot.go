package crawler

import (
	"fmt"
	"log"

	"treehole/internal/client"
	"treehole/internal/db"
	"treehole/internal/models"
)

// commentListPageLimit is the observed server cap for comment/list pages.
const commentListPageLimit = 100

// fetchAllComments pages until an empty or short page. The response total is
// not the comment count (it tracks the request's page size), so it cannot be
// used for completeness checks. A changing or incomplete snapshot must never
// reconcile deletions.
func fetchAllComments(c *client.Client, pid int32, pageSize int) ([]models.Comment, error) {
	return fetchCommentsFrom(c, pid, nil, 0, pageSize)
}

// fetchCommentsFrom continues an embedded comment list. List pages are
// ascending by cid, so [0:len(initial)) must match page 1; the overlap is
// verified before appending. It stops at the target count or a short page.
func fetchCommentsFrom(c *client.Client, pid int32, initial []models.Comment, target, pageSize int) ([]models.Comment, error) {
	if pageSize <= 0 {
		pageSize = 200
	}
	comments := append([]models.Comment(nil), initial...)
	seen := make(map[int32]bool, len(comments))
	for _, comment := range comments {
		seen[comment.Cid] = true
	}
	// Embedded lists can exceed the comment/list cap; align continuation
	// requests so page numbers stay consistent with the accepted prefix.
	if len(initial) > 0 && pageSize > commentListPageLimit {
		pageSize = commentListPageLimit
	}
	reductions := 0
	for target <= 0 || len(comments) < target {
		// Recompute from the consumed offset when reducing an odd page size.
		// The containing page overlaps the accepted prefix; verify it below.
		offset := len(comments)
		page := offset/pageSize + 1
		pageOffset := (page - 1) * pageSize
		type commentPage struct {
			comments []models.Comment
		}
		result, err := retryTimeout(c, fmt.Sprintf("comments pid=%d page=%d limit=%d", pid, page, pageSize), func(attemptClient *client.Client) (commentPage, error) {
			batch, _, err := attemptClient.ListCommentsV3(pid, page, pageSize, 0, 1)
			return commentPage{comments: batch}, err
		})
		batch := result.comments
		if err != nil {
			if isServerTimeout(err) && pageSize > 1 && reductions < maxPageReductions {
				log.Printf("[Crawler] 服务端超时，缩小 comments pid=%d page=%d limit=%d -> %d", pid, page, pageSize, pageSize/2)
				pageSize /= 2
				reductions++
				continue
			}
			return nil, err
		}
		if len(batch) == 0 {
			return comments, nil
		}
		// The server may cap limit. Adopt the returned size on the first page
		// instead of treating a capped page as the final one.
		if offset == 0 && len(batch) < pageSize {
			pageSize = len(batch)
		}
		if len(batch) > pageSize {
			return nil, fmt.Errorf("comment page %d returned %d items for page size %d", page, len(batch), pageSize)
		}
		for i, comment := range batch {
			if comment.Cid <= 0 || comment.Pid != pid {
				return nil, fmt.Errorf("invalid comment identity: pid=%d cid=%d", comment.Pid, comment.Cid)
			}
			if pageOffset+i < offset {
				if comments[pageOffset+i].Cid != comment.Cid {
					return nil, fmt.Errorf("comment recovery boundary changed at offset %d", pageOffset+i)
				}
				continue
			}
			if seen[comment.Cid] {
				return nil, fmt.Errorf("overlapping comment pages: cid=%d", comment.Cid)
			}
			seen[comment.Cid] = true
			comments = append(comments, comment)
		}
		if len(batch) < pageSize {
			return comments, nil
		}
	}
	return comments, nil
}

// Only reconcile bounded, descending PID pages. Pinned posts do not define
// range boundaries. The session passes the preceding page only when adjacent.
func missingPostsInPage(database *db.Database, posts, previous []models.Post) ([]int32, error) {
	if len(posts) == 0 {
		return nil, nil
	}
	observed := make([]int32, 0, len(posts)+len(previous))
	var low, high, last int32
	for _, batch := range [][]models.Post{previous, posts} {
		for _, post := range batch {
			observed = append(observed, post.Pid)
			if post.IsTop {
				continue
			}
			if post.Pid <= 0 {
				return nil, fmt.Errorf("invalid post pid: %d", post.Pid)
			}
			if last > 0 && post.Pid > last {
				// Offset pages can overlap while new posts arrive. Avoid inferring any
				// deletion from an ambiguous ordering; observed data can still be saved.
				return nil, nil
			}
			if high == 0 {
				high = post.Pid
			}
			low = post.Pid
			last = post.Pid
		}
	}
	if low == 0 || low == high {
		return nil, nil
	}
	return database.FindMissingPostPIDs(low, high, observed)
}
