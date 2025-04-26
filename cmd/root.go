package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pomo",
	Short: "🍅 Minimalistischer Pomodoro Timer",
	Long:  "Pomo ist ein CLI-Tool für Fokus- und Pausensessions inklusive Statistiken.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
