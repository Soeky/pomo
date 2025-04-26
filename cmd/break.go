package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var breakCmd = &cobra.Command{
	Use:   "break [Dauer]",
	Short: "Startet eine Pausensession",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		session.StartBreak(args)
	},
}

func init() {
	rootCmd.AddCommand(breakCmd)
}
