package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newFTSCmd() *cobra.Command {
	var backfill, reindex, showStatus bool
	cmd := &cobra.Command{
		Use:   "fts",
		Short: "SQLite FTS5 全文索引管理（回填/重建/状态）",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, cleanup, err := initDB()
			if err != nil {
				return err
			}
			defer cleanup()

			if !database.FTSEnabled() {
				return fmt.Errorf("当前构建未启用 FTS5，请使用 -tags sqlite_fts5 构建")
			}

			switch {
			case backfill:
				start := time.Now()
				n, err := database.BackfillFTSSearchColumn(func(done, total int) {
					fmt.Printf("\r回填进度: %d/%d (%.1f%%)", done, total, float64(done)/float64(max(total, 1))*100)
				})
				fmt.Println()
				if err != nil {
					return fmt.Errorf("回填 text_search 列失败: %w", err)
				}
				fmt.Printf("回填完成: %d 条帖子, 耗时 %s\n", n, time.Since(start).Round(time.Millisecond))
			case reindex:
				start := time.Now()
				if err := database.RebuildFTSIndex(); err != nil {
					return err
				}
				fmt.Printf("索引重建完成, 耗时 %s\n", time.Since(start).Round(time.Millisecond))
			default:
				st, err := database.FTSStatus()
				if err != nil {
					return err
				}
				pending := st.Posts - st.IndexedRows
				if pending < 0 {
					pending = 0
				}
				fmt.Printf("FTS5 启用: %v\n", st.Enabled)
				fmt.Printf("帖子总数: %d\n", st.Posts)
				fmt.Printf("已索引行数: %d (待回填 %d)\n", st.IndexedRows, pending)
				if st.IndexBytes >= 0 {
					fmt.Printf("索引体积: %.1f MiB\n", float64(st.IndexBytes)/(1024*1024))
				} else {
					fmt.Println("索引体积: 未知（SQLite 未编译 dbstat 扩展）")
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&backfill, "backfill", false, "为存量帖子生成 text_search 分词列并增量入索引")
	cmd.Flags().BoolVar(&reindex, "reindex", false, "从 posts.text_search 全量重建 FTS 索引")
	cmd.Flags().BoolVar(&showStatus, "status", true, "显示索引状态")
	return cmd
}
