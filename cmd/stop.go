package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stops the current session",
	Run: func(cmd *cobra.Command, args []string) {
		session.StopSession()
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
