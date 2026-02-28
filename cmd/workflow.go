package cmd

import "github.com/spf13/cobra"

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "show a recommended daily pomo workflow",
	Long: `Recommended daily workflow:
  1) quick plan review
     pomo plan status --from 2026-03-01T00:00 --to 2026-03-08T00:00
  2) set or adjust workload targets
     pomo plan target list --active-only
     pomo plan target add --domain Math --subtopic Discrete --cadence weekly --hours 8
  3) verify scheduler constraints
     pomo plan constraint show
  4) preview and apply schedule generation
     pomo plan generate --from 2026-03-01T00:00 --to 2026-03-08T00:00 --dry-run
     pomo plan generate --from 2026-03-01T00:00 --to 2026-03-08T00:00 --replace
  5) execute sessions during the day
     pomo start 50m Math::DiscreteProbability
     pomo break 10m
  6) inspect progress at the end of day
     pomo stat day
     pomo plan status --from 2026-03-01T00:00 --to 2026-03-08T00:00

Use this guide with:
  pomo help workflow`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}
