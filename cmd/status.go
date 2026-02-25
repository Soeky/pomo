package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/Soeky/pomo/internal/status"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "prints the current status of the session",
	Run: func(cmd *cobra.Command, args []string) {
		res, err := status.CurrentStatus(time.Now())
		if err != nil {
			fmt.Println("error finding session:", err)
			os.Exit(1)
		}
		if !res.Active {
			fmt.Println("📭 no active session.")
			return
		}
		fmt.Printf("%s %s\n", res.Emoji, res.Formatted)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
