package cmd

import (
	"fmt"
	"os"

	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [duration] [domain[::subtopic]]",
	Short: "starts a work session",
	Long: `Starts a focus session. Topic format is domain::subtopic.
Examples:
  pomo start 50m Math::Discrete Probability
  pomo start "Applied Mathematics::Numerical Analysis"
  pomo start "Math"                      # stored as Math::General
If duration is omitted, default_focus from config is used.`,
	Args: cobra.ArbitraryArgs,
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
