package cmd

import (
	"github.com/Soeky/pomo/internal/status"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Zeigt die aktuelle Session an",
	Run: func(cmd *cobra.Command, args []string) {
		status.ShowStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
