package cmd

import (
	"github.com/Soeky/pomo/internal/delete"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete ",
	Short: "starts delete session prompt",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		delete.StartDelete()
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
