package crawler

import (
	"treehole/internal/client"
	"treehole/internal/db"
	"treehole/internal/models"
)

// CrawlSession keeps page continuity within one sequential crawl round.
// Start a new session for each round; sessions are not safe for concurrent use.
type CrawlSession struct {
	lastPage          int
	previousPagePosts []models.Post
}

func NewCrawlSession() *CrawlSession {
	return &CrawlSession{}
}

// FetchAndSave only uses the preceding page as deletion evidence when both
// pages were fetched and saved successfully in consecutive calls.
func (s *CrawlSession) FetchAndSave(c *client.Client, database *db.Database, page int, saveJSON bool, postLimit int, commentLimit int, fetchImages bool, convertWebp bool) (FetchResult, error) {
	if page != s.lastPage+1 {
		s.previousPagePosts = nil
	}
	result, err := fetchAndSave(c, database, page, saveJSON, postLimit, commentLimit, fetchImages, convertWebp, s.previousPagePosts)
	if err != nil {
		s.lastPage = 0
		s.previousPagePosts = nil
		return result, err
	}
	s.lastPage = page
	s.previousPagePosts = append([]models.Post(nil), result.Posts...)
	return result, nil
}
