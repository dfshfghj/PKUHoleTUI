//go:build !withserver

package main

import (
	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	return nil
}

func runServer() error {
	return nil
}
