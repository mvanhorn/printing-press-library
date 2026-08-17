package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ValidateReadOnlyQuery permits one SELECT or WITH statement while ignoring
// semicolons inside SQL literals, identifiers, and comments.
func ValidateReadOnlyQuery(query string) error {
	stripped := StripLeadingSQLNoise(query)
	if HasTrailingSQLStatement(stripped) {
		return fmt.Errorf("only a single SELECT or WITH statement is allowed")
	}
	upper := strings.ToUpper(stripped)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT queries are allowed")
	}
	return nil
}

func BoundedReadOnlyQuery(ctx context.Context, db *sql.DB, query string, rowLimit, byteLimit int, timeout time.Duration) ([]string, []map[string]any, bool, error) {
	if len(query) > byteLimit {
		return nil, nil, false, fmt.Errorf("SQL query exceeds the %d-byte limit", byteLimit)
	}
	if err := ValidateReadOnlyQuery(query); err != nil {
		return nil, nil, false, err
	}
	if rowLimit <= 0 {
		rowLimit = 100
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	connection, err := db.Conn(queryCtx)
	if err != nil {
		return nil, nil, false, err
	}
	defer connection.Close()
	if _, err := modernsqlite.Limit(connection, sqlite3.SQLITE_LIMIT_LENGTH, byteLimit); err != nil {
		return nil, nil, false, err
	}
	if _, err := modernsqlite.Limit(connection, sqlite3.SQLITE_LIMIT_SQL_LENGTH, byteLimit); err != nil {
		return nil, nil, false, err
	}
	rows, err := connection.QueryContext(queryCtx, query)
	if err != nil {
		return nil, nil, false, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	results := make([]map[string]any, 0, min(rowLimit, 100))
	materialized := 0
	truncated := false
	for rows.Next() {
		if len(results) >= rowLimit {
			truncated = true
			break
		}
		values, pointers := make([]any, len(columns)), make([]any, len(columns))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, false, err
		}
		row := make(map[string]any, len(columns))
		for index, column := range columns {
			value := values[index]
			if bytes, ok := value.([]byte); ok {
				value = string(bytes)
			}
			materialized += len(column) + len(fmt.Sprint(value))
			if materialized > byteLimit {
				return nil, nil, false, fmt.Errorf("SQL result exceeded the %d-byte materialization budget", byteLimit)
			}
			row[column] = value
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return columns, results, truncated, nil
}

func StripLeadingSQLNoise(query string) string {
	for {
		query = strings.TrimLeft(query, " \t\r\n;")
		switch {
		case strings.HasPrefix(query, "--"):
			if index := strings.IndexByte(query, '\n'); index >= 0 {
				query = query[index+1:]
				continue
			}
			return ""
		case strings.HasPrefix(query, "/*"):
			if index := strings.Index(query[2:], "*/"); index >= 0 {
				query = query[index+4:]
				continue
			}
			return ""
		default:
			return query
		}
	}
}

func HasTrailingSQLStatement(query string) bool {
	var single, double, backtick, bracket, lineComment, blockComment bool
	for index := 0; index < len(query); index++ {
		char := query[index]
		var next byte
		if index+1 < len(query) {
			next = query[index+1]
		}
		switch {
		case lineComment:
			if char == '\n' {
				lineComment = false
			}
			continue
		case blockComment:
			if char == '*' && next == '/' {
				blockComment = false
				index++
			}
			continue
		case single:
			if char == '\'' {
				if next == '\'' {
					index++
				} else {
					single = false
				}
			}
			continue
		case double:
			if char == '"' {
				if next == '"' {
					index++
				} else {
					double = false
				}
			}
			continue
		case backtick:
			if char == '`' {
				if next == '`' {
					index++
				} else {
					backtick = false
				}
			}
			continue
		case bracket:
			if char == ']' {
				bracket = false
			}
			continue
		}
		switch {
		case char == '-' && next == '-':
			lineComment = true
			index++
		case char == '/' && next == '*':
			blockComment = true
			index++
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == '`':
			backtick = true
		case char == '[':
			bracket = true
		case char == ';':
			if StripLeadingSQLNoise(query[index+1:]) != "" {
				return true
			}
			return false
		}
	}
	return false
}
