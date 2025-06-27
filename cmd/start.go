package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [duration (optional)] [topic]",
	Short: "starts a work session",
	Long:  "starts a work session with [topic]. If you exceed the duration you gave, it will just continue. The default duration can be set via pomo set.",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		session.StartFocus(args)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
