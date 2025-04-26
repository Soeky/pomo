package cmd

import (
	"github.com/Soeky/pomo/internal/stats"
	"github.com/spf13/cobra"
)

var statCmd = &cobra.Command{
	Use:   "stat [Zeitraum]",
	Short: "Zeigt Fokus- und Pausenstatistiken",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		stats.ShowStats(args)
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}
