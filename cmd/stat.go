package cmd

import (
	"fmt"
	"os"
	"time"

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
		prints the stats of the current semester. Semester start can be set with: pomo config set semester_start YYYY-MM-DD
	pomo stat adherence [range]:
		prints on-time adherence metrics for the current week by default
	pomo stat plan-vs-actual [range]:
		prints plan-vs-actual metrics and drift by domain for the current week by default
	`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		report, err := stats.BuildReport(args, time.Now())
		if err != nil {
			fmt.Println("error at statistics:", err)
			os.Exit(1)
		}
		fmt.Print(stats.RenderReport(report))
	},
}

var statAdherenceCmd = &cobra.Command{
	Use:   "adherence [range]",
	Short: "show on-time adherence for planned events",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		report, err := stats.BuildAdherenceReport(args, time.Now())
		if err != nil {
			fmt.Println("error at adherence statistics:", err)
			os.Exit(1)
		}
		fmt.Print(stats.RenderAdherenceReport(report))
	},
}

var statPlanVsActualCmd = &cobra.Command{
	Use:   "plan-vs-actual [range]",
	Short: "show plan-vs-actual adherence, completion, drift, and balance metrics",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		report, err := stats.BuildPlanVsActualReport(args, time.Now())
		if err != nil {
			fmt.Println("error at plan-vs-actual statistics:", err)
			os.Exit(1)
		}
		fmt.Print(stats.RenderPlanVsActualReport(report))
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
	statCmd.AddCommand(statAdherenceCmd)
	statCmd.AddCommand(statPlanVsActualCmd)
}
