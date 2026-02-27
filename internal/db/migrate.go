package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

type migrationFile struct {
	name     string
	filename string
	content  string
	checksum string
}

func RunMigrations(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, m := range migrations {
		existingChecksum, exists, err := getAppliedChecksum(ctx, db, m.name)
		if err != nil {
			return err
		}

		if exists {
			if existingChecksum == "" || existingChecksum == "legacy" || existingChecksum == m.checksum {
				if existingChecksum != m.checksum {
					if _, err := db.ExecContext(ctx, `UPDATE schema_migrations SET checksum = ? WHERE name = ?`, m.checksum, m.name); err != nil {
						return err
					}
				}
				continue
			}
			return fmt.Errorf("migration %s checksum mismatch", m.name)
		}

		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}

		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(name, checksum, applied_at) VALUES (?, ?, ?)`, m.name, m.checksum, time.Now()); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		)`); err != nil {
		return err
	}

	// Compatibility with older table definition.
	if _, err := db.ExecContext(ctx, `ALTER TABLE schema_migrations ADD COLUMN checksum TEXT`); err != nil && !isDuplicateColumnErr(err) {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE schema_migrations
		SET checksum = COALESCE(NULLIF(checksum, ''), 'legacy')
		WHERE checksum IS NULL OR checksum = ''`); err != nil {
		return err
	}
	return nil
}

func loadMigrations() ([]migrationFile, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		raw, err := migrationsFS.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, err
		}

		name := strings.TrimSuffix(entry.Name(), ".up.sql")
		sum := sha256.Sum256(raw)
		files = append(files, migrationFile{
			name:     name,
			filename: entry.Name(),
			content:  string(raw),
			checksum: hex.EncodeToString(sum[:]),
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].filename < files[j].filename })
	return files, nil
}

func getAppliedChecksum(ctx context.Context, db *sql.DB, name string) (string, bool, error) {
	var checksum sql.NullString
	err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE name = ?`, name).Scan(&checksum)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return checksum.String, true, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migrationFile) error {
	switch m.name {
	case "002_sessions_metadata":
		if err := addColumnIfMissing(ctx, db, "sessions", "planned_event_id", "INTEGER"); err != nil {
			return err
		}
		if err := addColumnIfMissing(ctx, db, "sessions", "created_at", "DATETIME"); err != nil {
			return err
		}
		return addColumnIfMissing(ctx, db, "sessions", "updated_at", "DATETIME")
	case "008_unified_events_backfill_and_sync":
		if err := addColumnIfMissing(ctx, db, "events", "timezone", "TEXT NOT NULL DEFAULT 'Local'"); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, m.content)
		return err
	default:
		_, err := db.ExecContext(ctx, m.content)
		return err
	}
}

func addColumnIfMissing(ctx context.Context, db *sql.DB, tableName, columnName, columnType string) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnType))
	return err
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
