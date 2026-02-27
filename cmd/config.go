package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/Soeky/pomo/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "manage pomo configuration values",
	Long: `Manage configuration values used by CLI, web, and scheduler behavior.
Use:
  pomo config list
  pomo config get <key>
  pomo config set <key> <value>
  pomo config describe [key]`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all config values",
	Run: func(cmd *cobra.Command, args []string) {
		vals := config.ListValues()
		keys := make([]string, 0, len(vals))
		for k := range vals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s=%s\n", k, vals[k])
		}
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "get one config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		v, err := config.GetValue(args[0])
		if err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
		fmt.Println(v)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "set one config value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.SetValue(args[0], args[1]); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
	},
}

var configDescribeCmd = &cobra.Command{
	Use:   "describe [key]",
	Short: "show key descriptions and examples",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			d, err := config.DescribeKey(args[0])
			if err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
			fmt.Printf("%s\nexample: %s\n", d.Description, d.Example)
			return
		}

		for _, key := range config.KnownKeys() {
			d, err := config.DescribeKey(key)
			if err != nil {
				continue
			}
			fmt.Printf("%s\n  %s\n  example: %s\n", key, d.Description, d.Example)
		}
	},
}

func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configDescribeCmd)
	rootCmd.AddCommand(configCmd)
}
