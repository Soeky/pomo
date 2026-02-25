package cmd

import (
	"fmt"
	"os"

	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var breakCmd = &cobra.Command{
	Use:   "break [duration]",
	Short: "starts a break session",
	Long:  "starts a break session and stops the session before. break sessions have no topic in the database",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, err := session.StartBreak(args)
		if err != nil {
			fmt.Println("❌ error starting break session:", err)
			os.Exit(1)
		}
		if res.StoppedPrevious {
			fmt.Println("previous session has been stopped")
		}
		fmt.Printf("💤 break started for %s (ID %d)\n", session.FormatShortDuration(res.Duration), res.ID)
	},
}

func init() {
	rootCmd.AddCommand(breakCmd)
}
