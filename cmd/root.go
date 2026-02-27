package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pomo",
	Short: "🍅 minimalistic pomodoro timer",
	Long: `Pomo is a local-first time management CLI and web app.
Core commands:
  pomo start [duration] [domain::subtopic]
  pomo break [duration]
  pomo stat [range]
  pomo config list|get|set|describe`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
