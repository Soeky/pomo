package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/spf13/cobra"
)

var (
	upgradeVersion      string
	upgradeSkipBackup   bool
	upgradeSkipSelf     bool
	upgradeSkipMigrate  bool
	upgradeSkipFinalize bool
)

const upgradeModulePath = "github.com/Soeky/pomo"

var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Aliases: []string{"update"},
	Short:   "upgrade CLI binary and run one-time major upgrade tasks",
	Long: `Upgrade performs a full CLI/data upgrade flow.

By default it:
1) applies database migrations,
2) finalizes v2 cutover (one-time legacy backfill + disables legacy sync triggers),
3) updates the CLI via "go install github.com/Soeky/pomo@<version>".

Use flags to skip specific steps.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !upgradeSkipBackup {
			backupPath, err := backupDatabaseFile(db.GetDBPath())
			if err != nil {
				fmt.Println("❌ database backup step failed:", err)
				os.Exit(1)
			}
			fmt.Printf("✅ database backup created: %s\n", backupPath)
		} else {
			fmt.Println("ℹ️ skipped backup step (--skip-backup)")
		}

		if !upgradeSkipMigrate {
			if err := db.RunMigrations(context.Background(), db.DB); err != nil {
				fmt.Println("❌ migration step failed:", err)
				os.Exit(1)
			}
			fmt.Println("✅ database migrations applied")
		} else {
			fmt.Println("ℹ️ skipped migration step (--skip-migrate)")
		}

		if !upgradeSkipFinalize {
			result, err := db.FinalizeV2Cutover(context.Background(), db.DB)
			if err != nil {
				fmt.Println("❌ v2 finalization failed:", err)
				os.Exit(1)
			}
			if result.AlreadyFinalized {
				fmt.Println("✅ v2 finalization already completed")
			} else {
				fmt.Printf("✅ v2 finalization completed (sessions=%d, planned=%d, session_backfilled=%d, planned_backfilled=%d, triggers_dropped=%d)\n",
					result.SessionsRows, result.PlannedEventsRows, result.SessionBackfilledRows, result.PlannedBackfilledRows, result.DroppedCompatibilitySync)
			}
		} else {
			fmt.Println("ℹ️ skipped v2 finalization (--skip-finalize)")
		}

		if !upgradeSkipSelf {
			if err := runGoInstallUpgrade(upgradeVersion); err != nil {
				fmt.Println("❌ self-update step failed:", err)
				fmt.Println("   You can still update manually with:")
				fmt.Printf("   go install %s@%s\n", upgradeModulePath, normalizeUpgradeVersion(upgradeVersion))
				os.Exit(1)
			}
			fmt.Printf("✅ CLI updated via go install (%s@%s)\n", upgradeModulePath, normalizeUpgradeVersion(upgradeVersion))
		} else {
			fmt.Println("ℹ️ skipped self-update step (--skip-self)")
		}
	},
}

func runGoInstallUpgrade(version string) error {
	goCmd, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go toolchain not found in PATH")
	}
	target := fmt.Sprintf("%s@%s", upgradeModulePath, normalizeUpgradeVersion(version))
	cmd := exec.Command(goCmd, "install", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func normalizeUpgradeVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "latest"
	}
	return strings.TrimSpace(v)
}

func backupDatabaseFile(dbPath string) (string, error) {
	if strings.TrimSpace(dbPath) == "" {
		return "", fmt.Errorf("empty database path")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return "", err
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupPath := filepath.Clean(dbPath) + ".bak." + timestamp

	src, err := os.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	if err := dst.Sync(); err != nil {
		return "", err
	}
	return backupPath, nil
}

func init() {
	upgradeCmd.Flags().StringVar(&upgradeVersion, "version", "latest", "module version/tag to install (for example: latest, v2.0.0)")
	upgradeCmd.Flags().BoolVar(&upgradeSkipBackup, "skip-backup", false, "skip database backup step")
	upgradeCmd.Flags().BoolVar(&upgradeSkipSelf, "skip-self", false, "skip CLI self-update step")
	upgradeCmd.Flags().BoolVar(&upgradeSkipMigrate, "skip-migrate", false, "skip database migration step")
	upgradeCmd.Flags().BoolVar(&upgradeSkipFinalize, "skip-finalize", false, "skip one-time v2 cutover finalization step")
	rootCmd.AddCommand(upgradeCmd)
}
