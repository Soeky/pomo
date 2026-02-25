package cmd

import (
	"fmt"
	"os"

	"github.com/Soeky/pomo/internal/session"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stops the current session",
	Run: func(cmd *cobra.Command, args []string) {
		res, err := session.StopSession()
		if err != nil {
			fmt.Println("❌ error while stopping:", err)
			os.Exit(1)
		}
		if res.Stopped {
			fmt.Println("🛑 session has been stopped")
			return
		}
		fmt.Println("📭 no active session.")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
