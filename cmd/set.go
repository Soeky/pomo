package cmd

import (
	"fmt"
	"os"

	"github.com/Soeky/pomo/internal/config"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "deprecated alias for `pomo config set`",
	Long: `Deprecated alias for:
  pomo config set <key> <value>

Use these commands instead:
  pomo config list
  pomo config get <key>
  pomo config describe [key]
  pomo config set <key> <value>`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️ `pomo set` is deprecated and will be removed in a future major release.")
		fmt.Println("   Use `pomo config set <key> <value>` (discover keys via `pomo config list` / `pomo config describe`).")
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
