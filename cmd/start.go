package cmd

import (
	"fmt"
	"os"

	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [duration (optional)] [topic]",
	Short: "starts a work session",
	Long:  "starts a work session with [topic]. If you exceed the duration you gave, it will just continue. The default duration can be set via pomo set.",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		res, err := session.StartFocus(args)
		if err != nil {
			fmt.Println("❌ error starting the work session:", err)
			os.Exit(1)
		}
		if res.StoppedPrevious {
			fmt.Println("previous session has been stopped")
		}
		fmt.Printf("🍅 work session started: \"%s\" for %s (ID %d)\n", res.Topic, session.FormatShortDuration(res.Duration), res.ID)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
