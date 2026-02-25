package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

type sqlResult struct {
	Columns []string
	Rows    [][]string
	Message string
	Error   string
}

func (s *Server) sqlPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tables, err := listTables(r.Context(), s.sqlDB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "sql.html", map[string]any{
		"Tables":                tables,
		"ActivePage":            "sql",
		"PageTitle":             "SQL Workspace",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) sqlQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(r.FormValue("query"))
	writeMode := r.FormValue("write_mode") == "1"
	result := sqlResult{}

	if query == "" {
		result.Error = "query is required"
		s.renderSQLResult(w, result)
		return
	}
	if strings.Count(query, ";") > 1 || (strings.Count(query, ";") == 1 && !strings.HasSuffix(query, ";")) {
		result.Error = "only one SQL statement is allowed"
		s.renderSQLResult(w, result)
		return
	}
	query = strings.TrimSuffix(query, ";")

	if !writeMode && !isReadOnlySQL(query) {
		result.Error = "read-only mode blocks mutating SQL; enable write mode to continue"
		s.renderSQLResult(w, result)
		return
	}

	if isReadOnlySQL(query) {
		if s.sqlDB == nil {
			result.Error = "database is not initialized"
			s.renderSQLResult(w, result)
			return
		}
		rows, err := s.sqlDB.QueryContext(r.Context(), query)
		if err != nil {
			result.Error = err.Error()
			s.renderSQLResult(w, result)
			return
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			result.Error = err.Error()
			s.renderSQLResult(w, result)
			return
		}
		result.Columns = cols
		for rows.Next() {
			scans := make([]any, len(cols))
			vals := make([]sql.NullString, len(cols))
			for i := range vals {
				scans[i] = &vals[i]
			}
			if err := rows.Scan(scans...); err != nil {
				result.Error = err.Error()
				s.renderSQLResult(w, result)
				return
			}
			line := make([]string, len(cols))
			for i := range vals {
				if vals[i].Valid {
					line[i] = vals[i].String
				} else {
					line[i] = "NULL"
				}
			}
			result.Rows = append(result.Rows, line)
		}
		if err := rows.Err(); err != nil {
			result.Error = err.Error()
			s.renderSQLResult(w, result)
			return
		}
		result.Message = fmt.Sprintf("%d rows", len(result.Rows))
		s.renderSQLResult(w, result)
		return
	}

	if s.sqlDB == nil {
		result.Error = "database is not initialized"
		s.renderSQLResult(w, result)
		return
	}
	execRes, err := s.sqlDB.ExecContext(r.Context(), query)
	if err != nil {
		result.Error = err.Error()
		s.renderSQLResult(w, result)
		return
	}
	affected, _ := execRes.RowsAffected()
	result.Message = fmt.Sprintf("write query executed: %d row(s) affected", affected)
	_ = s.store.InsertAuditLog(r.Context(), "sql", 0, "execute", map[string]string{"query": query}, map[string]any{"rows_affected": affected}, "web")
	s.renderSQLResult(w, result)
}

func (s *Server) renderSQLResult(w http.ResponseWriter, r sqlResult) {
	if err := s.tmpl.ExecuteTemplate(w, "sql-result.html", r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
