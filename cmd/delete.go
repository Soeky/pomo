package cmd

import (
	"github.com/Soeky/pomo/internal/delete"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete ",
	Short: "starts delete session prompt",
	Long: `
  Starts the delete session prompt where you can select which sessions you want to delete. 
  Use space or enter to select sessions, c to confirm, y/n to confirm or deny in the end.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		delete.StartDelete()
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
