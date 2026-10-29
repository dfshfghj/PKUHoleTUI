package main

import (
	"fmt"

	"treehole/internal/crawler"

	"github.com/spf13/cobra"
)

func newCrawlerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawler",
		Short: "Run the crawler",
		Long:  `启动 PKU Hole 爬虫，支持一次性抓取、无限抓取和循环抓取。`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := crawler.ValidatePostPageSize(postBatchSize); err != nil {
				return fmt.Errorf("--post-batch-size: %w", err)
			}
			if err := crawler.ValidateCommentLimit(commentLimit); err != nil {
				return fmt.Errorf("--comment-limit: %w", err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
	}

	cmd.Flags().IntVar(&startPage, "start-page", 1, "first page to crawl")
	cmd.Flags().IntVar(&pagesPerRound, "pages", 0, "number of pages to crawl per round; 0 means until the first empty page")
	cmd.Flags().BoolVar(&loopCrawl, "loop", false, "continuously repeat crawl rounds")
	cmd.Flags().IntVar(&loopDelay, "loop-delay", 60, "seconds between crawl rounds")
	cmd.Flags().IntVar(&pageDelay, "page-delay", 1, "seconds between page requests")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume from saved crawl state")
	cmd.Flags().BoolVar(&saveRawJSON, "save-raw-json", false, "save raw API responses as JSON files")
	cmd.Flags().IntVar(&postBatchSize, "post-batch-size", 100, "posts requested per API call (1..100, server cap)")
	cmd.Flags().IntVar(&commentLimit, "comment-limit", 100, "comments embedded per post in the post list (0..100, server cap); remaining pages are fetched until reply is reached")
	cmd.Flags().BoolVar(&fetchImages, "fetch-images", false, "download images from image posts")
	cmd.Flags().BoolVar(&convertWebp, "convert-webp", true, "convert downloaded images to WebP format")
	cmd.Flags().StringVar(&deletedJSONPath, "deleted-json", "", "merge deleted-post gaps into this JSON file while crawling (daily one-off run recommended)")

	cmd.AddCommand(newFetchImagesCmd())
	cmd.AddCommand(newFetchThumbnailsCmd())

	return cmd
}

func newFetchImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch-images",
		Short: "Download missing images from database",
		Long:  `从数据库中查找有图片的帖子和评论，下载缺失的图片到 data/images/。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, cleanup, err := initDB()
			if err != nil {
				return err
			}
			defer cleanup()

			client, _, err := initClientForCrawler()
			if err != nil {
				return err
			}

			crawler.FetchImagesFromDB(client, database, convertWebp)
			return nil
		},
	}

	cmd.Flags().BoolVar(&convertWebp, "convert-webp", true, "convert downloaded images to WebP format")

	return cmd
}

func newFetchThumbnailsCmd() *cobra.Command {
	var startID int
	var endID int
	var convertThumbWebp bool

	cmd := &cobra.Command{
		Use:   "fetch-thumbnails",
		Short: "Download thumbnails by media ID range",
		Long:  `按 media id 区间批量下载缩略图到 data/thumbnails/。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := initClientForCrawler()
			if err != nil {
				return err
			}

			downloaded, skipped, err := crawler.FetchThumbnailsByIDRange(client, startID, endID, convertThumbWebp)
			if err != nil {
				return err
			}

			cmd.Printf("thumbnails completed: downloaded=%d skipped=%d range=%d-%d\n", downloaded, skipped, startID, endID)
			return nil
		},
	}

	cmd.Flags().IntVar(&startID, "start-id", 0, "starting media id (inclusive)")
	cmd.Flags().IntVar(&endID, "end-id", 0, "ending media id (inclusive)")
	cmd.Flags().BoolVar(&convertThumbWebp, "convert-webp", false, "convert downloaded thumbnails to WebP format")
	_ = cmd.MarkFlagRequired("start-id")
	_ = cmd.MarkFlagRequired("end-id")

	return cmd
}
