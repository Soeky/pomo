package cmd

import (
	"github.com/Soeky/pomo/internal/stats"
	"github.com/spf13/cobra"
)

var statCmd = &cobra.Command{
	Use:   "stat [timeframe]",
	Short: "prints work and break stats",
	Long: `
	print stats of different timeframes:
	without arg:
		prints the stats of the current day
	pomo stat day | pomo stat 2025-06-14:
		prints the stats of the current day
	pomo stat week | pomo stat 2025-06-10 2025-07-10:
		prints the stats of the current week
	pomo stat month | pomo stat 2025-06:
		prints the stats of the current month
	pomo stat year | pomo stat 2025:
		prints the stats of the current year
	pomo stat sem:
		prints the stats of the current semester. Semester start can be set in the configs using pomo set
	`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		stats.ShowStats(args)
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}
