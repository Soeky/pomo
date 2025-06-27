package cmd

import (
	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var correctCmd = &cobra.Command{
	Use:   "correct [start|break] [time into the past] [topic]",
	Short: "corrects the start of a session back in time",
	Long: `
  pomo correct works as follows:
  pomo correct start 10m newTopic => 
  it stops the previous session at now-10m and starts the current work session at now-10m

  This command is there to adjust the last session because sometimes you may forget to use pomo start or pomo break.
  This way it is also not possible to have 2 sessions in parallel, which wouldn't make sense anyway.
	`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		session.HandleCorrectCommand(args)
	},
}

func init() {
	rootCmd.AddCommand(correctCmd)
}
