package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [Dauer] [Thema]",
	Short: "Startet eine Fokus-Session",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		session.StartFocus(args)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
