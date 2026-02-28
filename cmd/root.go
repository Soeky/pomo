package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pomo",
	Short: "🍅 local-first time management CLI",
	Long: `Pomo is a local-first time management CLI and web app.

Core command groups:
  pomo start|break|stop|status
  pomo event add|list|recur|dep
  pomo plan target|constraint|generate|status
  pomo config get|set|list|describe
  pomo web start|stop|status|logs|hosts-check

Topic delimiter examples:
  pomo start 50m Math::DiscreteProbability
  pomo start "Math\\::History::Week 1"   # escaped delimiter in domain

Discover the recommended daily flow:
  pomo help workflow

Compatibility alias (deprecated, still supported):
  pomo set <key> <value>   # use "pomo config set" instead`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
