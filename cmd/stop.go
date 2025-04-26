package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stoppt die aktuelle Session",
	Run: func(cmd *cobra.Command, args []string) {
		session.StopSession()
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
