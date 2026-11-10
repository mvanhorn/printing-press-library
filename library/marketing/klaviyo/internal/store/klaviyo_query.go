// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *Store) queryReadOnly(query string, args ...any) (*sql.Rows, error) {
	if err := ValidateReadOnlyQuery(query); err != nil {
		return nil, err
	}
	reader, err := OpenReadOnlyContext(context.Background(), s.path)
	if err != nil {
		return nil, err
	}
	// An active Rows keeps its connection alive until Rows.Close.
	defer reader.Close()
	return reader.db.Query(query, args...)
}

func ValidateReadOnlyQuery(query string) error {
	first, ended := true, false
	for i := 0; i < len(query); {
		switch {
		case strings.ContainsRune(" \t\r\n\f", rune(query[i])):
			i++
			continue
		case strings.HasPrefix(query[i:], "--"):
			if end := strings.IndexByte(query[i:], '\n'); end >= 0 {
				i += end + 1
				continue
			}
			i = len(query)
			continue
		case strings.HasPrefix(query[i:], "/*"):
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				return fmt.Errorf("unterminated SQL comment")
			}
			i += end + 4
			continue
		}
		if ended {
			return fmt.Errorf("only one SQL statement is allowed")
		}
		if first {
			start := i
			for i < len(query) && ((query[i] >= 'a' && query[i] <= 'z') || (query[i] >= 'A' && query[i] <= 'Z')) {
				i++
			}
			keyword := strings.ToUpper(query[start:i])
			if keyword != "SELECT" && keyword != "WITH" {
				return fmt.Errorf("only SELECT or WITH queries are allowed")
			}
			first = false
			continue
		}
		switch query[i] {
		case ';':
			ended = true
		case '\'', '"', '`', '[':
			quote := query[i]
			if quote == '[' {
				quote = ']'
			}
			i++
			for {
				if i == len(query) {
					return fmt.Errorf("unterminated SQL quote")
				}
				if query[i] == quote {
					if quote != ']' && i+1 < len(query) && query[i+1] == quote {
						i += 2
						continue
					}
					break
				}
				i++
			}
		}
		i++
	}
	if first {
		return fmt.Errorf("only SELECT or WITH queries are allowed")
	}
	return nil
}
