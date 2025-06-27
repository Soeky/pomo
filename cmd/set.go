package cmd

import (
	"fmt"
	"os"

	"github.com/Soeky/pomo/internal/config"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value (default_fokus, default_break, semester_start)",
	Long:  "Sets the entries of the default config file in ~/.config/pomo/config.json",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		err := config.HandleSetCommand(args)
		if err != nil {
			fmt.Println("error in set cmd", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
