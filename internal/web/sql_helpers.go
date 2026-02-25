package web

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

var readOnlyPrefix = regexp.MustCompile(`(?i)^\s*(select|with|explain|pragma)\b`)
var blockedMutating = regexp.MustCompile(`(?i)\b(insert|update|delete|alter|drop|create|replace|truncate|attach|vacuum|reindex)\b`)

func isReadOnlySQL(q string) bool {
	if !readOnlyPrefix.MatchString(q) {
		return false
	}
	return !blockedMutating.MatchString(q)
}

func listTables(ctx context.Context, dbh *sql.DB) ([]string, error) {
	if dbh == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	rows, err := dbh.QueryContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
