package crawler

import (
	"fmt"
	"log"
	"math"

	"treehole/internal/client"
	"treehole/internal/db"
	"treehole/internal/models"
)

type PageEstimate struct {
	Page           int
	DeletedDensity float64
	Sample         db.PIDSample
}

// EstimatePostPage supplies a search seed, never proof of a page boundary.
// Sparse local history can overestimate deletion density; LocatePostPage
// verifies the estimate against actual remote PID ranges.
func EstimatePostPage(targetPID, latestPID int32, limit int, sample db.PIDSample) (PageEstimate, error) {
	result := PageEstimate{Page: 1, Sample: sample}
	if targetPID <= 0 || latestPID <= 0 {
		return result, fmt.Errorf("PID must be positive")
	}
	if err := ValidatePostPageSize(limit); err != nil {
		return result, err
	}
	if sample.LivePosts < 0 || (sample.MinPID != 0 || sample.MaxPID != 0) && (sample.MinPID <= 0 || sample.MaxPID < sample.MinPID || sample.LivePosts > int64(sample.MaxPID)-int64(sample.MinPID)+1) {
		return result, fmt.Errorf("invalid PID sample")
	}
	if sample.MinPID == 0 && sample.MaxPID == 0 && sample.LivePosts != 0 {
		return result, fmt.Errorf("invalid empty PID sample")
	}
	if targetPID >= latestPID {
		return result, nil
	}
	liveFraction := 1.0
	if sample.MinPID > 0 {
		liveFraction = float64(sample.LivePosts) / float64(int64(sample.MaxPID)-int64(sample.MinPID)+1)
	}
	result.DeletedDensity = 1 - liveFraction
	postsBefore := int64(float64(int64(latestPID)-int64(targetPID)) * liveFraction)
	result.Page = int(postsBefore/int64(limit)) + 1
	return result, nil
}

// LocatePostPage finds the target or its insertion page in the descending live
// PID stream. A deleted target between pages belongs to the older page; callers
// covering an interval should include its preceding page to overlap that gap.
// It projects pages from the observed PID span (Newton step) and refines with
// a secant line, so a good estimate usually converges in a few probes.
// Requests are bounded and errors return directly, without retry loops.
func LocatePostPage(c *client.Client, database *db.Database, targetPID, latestPID int32, limit int) (int, error) {
	sample := db.PIDSample{}
	if database != nil {
		var err error
		sample, err = database.GetPostPIDSample(targetPID, latestPID)
		if err != nil {
			return 0, err
		}
	}
	return locatePostPage(targetPID, latestPID, limit, sample, func(page, size int) ([]models.Post, error) {
		posts, _, err := c.ListPostsV3(client.V3ListPostsParams{Page: page, Limit: size, CommentLimit: 0})
		return posts, err
	})
}

func locatePostPage(targetPID, latestPID int32, limit int, sample db.PIDSample, probe func(int, int) ([]models.Post, error)) (int, error) {
	estimate, err := EstimatePostPage(targetPID, latestPID, limit, sample)
	if err != nil {
		return 0, err
	}
	// Positive, unique PIDs bound the maximum number of live posts, even when
	// no database sample exists. The additional page is the empty sentinel.
	lo, hi := 1, int((int64(latestPID)-1)/int64(limit))+2
	page := estimate.Page
	if page < lo {
		page = lo
	}
	if page > hi {
		page = hi
	}

	type anchor struct {
		page int
		pid  int32
	}
	var previous anchor
	havePrevious := false
	firstEmpty := 0

	insertion := func() int {
		if firstEmpty > 1 && lo >= firstEmpty {
			return firstEmpty - 1
		}
		if lo < 1 {
			return 1
		}
		return lo
	}

	for probes := 0; probes < 24; probes++ {
		posts, err := probe(page, limit)
		if err != nil {
			return 0, fmt.Errorf("locate PID %d on page %d: %w", targetPID, page, err)
		}
		var newest, oldest int32
		for _, post := range posts {
			if post.IsTop {
				continue
			}
			if post.Pid <= 0 || oldest > 0 && post.Pid >= oldest {
				return 0, fmt.Errorf("locate PID: page %d is not strictly descending", page)
			}
			if newest == 0 {
				newest = post.Pid
			}
			oldest = post.Pid
		}
		if newest == 0 {
			if len(posts) > 0 {
				return 0, fmt.Errorf("locate PID: page %d contains only pinned posts", page)
			}
			log.Printf("[Locator] target PID %d: page=%d empty", targetPID, page)
			// Empty pages are past the end of the list.
			if firstEmpty == 0 || page < firstEmpty {
				firstEmpty = page
			}
			hi = page - 1
			if lo > hi {
				return insertion(), nil
			}
			page = lo + (hi-lo)/2
			continue
		}
		log.Printf("[Locator] target PID %d: page=%d -> PID=%d-%d", targetPID, page, oldest, newest)
		if targetPID >= oldest && targetPID <= newest {
			return page, nil
		}

		current := anchor{page: page, pid: oldest}
		if targetPID > newest {
			hi = page - 1
		} else {
			lo = page + 1
		}
		if lo > hi {
			return insertion(), nil
		}

		next := 0
		if havePrevious && previous.page != current.page && previous.pid != current.pid {
			// Secant through (pid, page): pages descend as PIDs grow.
			y := float64(current.page) + float64(int64(targetPID)-int64(current.pid))*
				float64(current.page-previous.page)/float64(int64(current.pid)-int64(previous.pid))
			next = int(math.Round(y))
		} else {
			// Newton step from the PID span this page covers.
			span := int64(newest) - int64(oldest)
			if span < 1 {
				span = 1
			}
			if targetPID > newest {
				step := int((int64(targetPID) - int64(newest) + span - 1) / span)
				next = page - step
			} else {
				step := int((int64(oldest) - int64(targetPID) + span - 1) / span)
				next = page + step
			}
		}
		if next < lo {
			next = lo
		}
		if next > hi {
			next = hi
		}
		if next == page {
			if lo < hi {
				next = lo + (hi-lo)/2
			}
			if next == page {
				return insertion(), nil
			}
		}
		previous, havePrevious = current, true
		page = next
	}
	return 0, fmt.Errorf("locate PID %d exceeded 24 probes", targetPID)
}
