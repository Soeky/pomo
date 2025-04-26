package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var correctCmd = &cobra.Command{
	Use:   "correct [start|break] [Zeit zurück] [Thema]",
	Short: "Korrigiert den Start einer Session rückwirkend",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		session.HandleCorrectCommand(args)
	},
}

func init() {
	rootCmd.AddCommand(correctCmd)
}
