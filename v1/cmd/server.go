//go:build withserver

package main

import (
	"fmt"
	"log"

	"treehole/server"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var (
	serverPort string
	serverHost string
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the API server",
		Long:  `启动 PKU Hole API 服务器，提供 RESTful 接口。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	}

	cmd.Flags().StringVarP(&serverPort, "port", "p", "8081", "server port")
	cmd.Flags().StringVar(&serverHost, "host", "0.0.0.0", "server host")

	return cmd
}

func runServer() error {
	database, cleanup, err := initDB()
	if err != nil {
		return err
	}
	defer cleanup()

	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	server.Init(r, database)

	addr := fmt.Sprintf("%s:%s", serverHost, serverPort)
	log.Printf("Starting PKU Hole API server on %s...", addr)
	log.Printf("API endpoints:")
	log.Printf("  GET http://%s:%s/help", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/posts?begin=0&limit=25", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/post/:pid", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/comment?cid=123", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/comments/:pid?begin=0&limit=25&sort=0", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/health", serverHost, serverPort)

	if err := r.Run(addr); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
