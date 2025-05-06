package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var correctCmd = &cobra.Command{
	Use:   "correct [start|break] [time into the past] [topic]",
	Short: "corrects the start of a session back in time",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		session.HandleCorrectCommand(args)
	},
}

func init() {
	rootCmd.AddCommand(correctCmd)
}
