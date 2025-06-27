package cmd

import (
	"github.com/Soeky/pomo/internal/stats"
	"github.com/spf13/cobra"
)

var statCmd = &cobra.Command{
	Use:   "stat [day|week|month|year|sem]",
	Short: "prints work and break stats",
	Long: `
  prints some statistics like avg and sum of your work and break sessions aggregated by topic.
  It is case sensitive, thus test and Test are two different topics. 
  
  To use pomo stat sem you have to set the semester start with pomo set or manually in the config file.
  If you want to calculate other statistics, the database is located in ~/.local/share/pomo/pomo.db. (be careful there is no backup yet)
	`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		stats.ShowStats(args)
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}
