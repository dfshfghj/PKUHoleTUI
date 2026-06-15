package main

import (
	"treehole/internal/crawler"

	"github.com/spf13/cobra"
)

func newCrawlerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawler",
		Short: "Run the crawler",
		Long:  `启动 PKU Hole 爬虫，支持一次性抓取、无限抓取和监控模式。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
	}

	cmd.Flags().IntVar(&startPage, "start-page", 1, "starting page number")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "max pages to crawl per round (0 = infinite)")
	cmd.Flags().IntVar(&pageInterval, "page-interval", 1, "seconds between each page crawl")
	cmd.Flags().IntVar(&loopInterval, "loop-interval", 60, "seconds between rounds in loop mode")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume from last crawled page")
	cmd.Flags().IntVarP(&loopPages, "loop-pages", "k", 0, "loop mode: repeatedly crawl first N pages")
	cmd.Flags().BoolVar(&saveJSON, "save-json", false, "save raw API responses to JSON files for analysis")
	cmd.Flags().IntVar(&postsPerReq, "posts-per-request", 200, "max posts per API request")
	cmd.Flags().IntVar(&commentsPerPost, "comments-per-post", 200, "max comments per post in API request")
	cmd.Flags().BoolVar(&fetchImages, "fetch-images", false, "download images from posts with type=\"image\"")
	cmd.Flags().BoolVar(&convertWebp, "convert-webp", true, "convert downloaded images to WebP format")

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
