package cmd

import (
	"github.com/Soeky/pomo/internal/status"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "prints the current status of the session",
	Run: func(cmd *cobra.Command, args []string) {
		status.ShowStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
