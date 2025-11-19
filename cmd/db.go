package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Soeky/pomo/internal/db"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "opens an interactive SQL shell to the pomo database",
	Long:  "Opens an interactive SQL shell (sqlite3) connected to the pomo database.",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath := db.GetDBPath()

		// Check if sqlite3 is available
		sqlite3Cmd, err := exec.LookPath("sqlite3")
		if err != nil {
			fmt.Println("❌ sqlite3 command not found. Please install sqlite3 to use this command.")
			fmt.Printf("Database location: %s\n", dbPath)
			os.Exit(1)
		}

		// Check if database file exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Printf("❌ Database file not found at: %s\n", dbPath)
			fmt.Println("Run any pomo command first to initialize the database.")
			os.Exit(1)
		}

		// Execute sqlite3 with the database path
		sqlite3 := exec.Command(sqlite3Cmd, dbPath)
		sqlite3.Stdin = os.Stdin
		sqlite3.Stdout = os.Stdout
		sqlite3.Stderr = os.Stderr

		if err := sqlite3.Run(); err != nil {
			fmt.Printf("❌ Error running sqlite3: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
}
