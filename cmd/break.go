package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var breakCmd = &cobra.Command{
	Use:   "break [duration]",
	Short: "starts a break session",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		session.StartBreak(args)
	},
}

func init() {
	rootCmd.AddCommand(breakCmd)
}
