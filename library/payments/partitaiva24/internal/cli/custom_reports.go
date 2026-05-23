// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: shared helpers for hand-authored transcendence commands.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/partitaiva24/internal/store"

	"github.com/spf13/cobra"
)

func openCLIStore(cmd *cobra.Command) (*store.Store, error) {
	return openStore(cmd.Context())
}

func money2(v float64) float64 {
	return math.Round(v*100) / 100
}

func pct1(v float64) float64 {
	return math.Round(v*10) / 10
}

func parseYMD(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func quarterBounds(q string) (int, time.Time, time.Time, error) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(q)), "-Q")
	if len(parts) != 2 {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("period must look like 2026-Q2")
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("invalid year in %q", q)
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 1 || n > 4 {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("invalid quarter in %q", q)
	}
	startMonth := time.Month((n-1)*3 + 1)
	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 3, 0)
	return year, start, end, nil
}

func periodWhere(period string) (string, []any, error) {
	p := strings.TrimSpace(period)
	if p == "" {
		return "", nil, fmt.Errorf("provide --period")
	}
	if len(p) == 4 {
		if _, err := strconv.Atoi(p); err != nil {
			return "", nil, fmt.Errorf("invalid period %q", period)
		}
		return "strftime('%Y', date) = ?", []any{p}, nil
	}
	if len(p) == 7 && p[4] == '-' && p[5] != 'Q' && p[5] != 'q' {
		if _, err := time.Parse("2006-01", p); err != nil {
			return "", nil, fmt.Errorf("invalid period %q", period)
		}
		return "strftime('%Y-%m', date) = ?", []any{p}, nil
	}
	if strings.Contains(strings.ToUpper(p), "-Q") {
		_, start, end, err := quarterBounds(p)
		if err != nil {
			return "", nil, err
		}
		return "date >= ? AND date <= ?", []any{start.Format("2006-01-02"), end.Format("2006-01-02")}, nil
	}
	return "", nil, fmt.Errorf("period must be YYYY, YYYY-MM, or YYYY-Qn")
}

func nullableString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullableFloat(n sql.NullFloat64) float64 {
	if n.Valid {
		return n.Float64
	}
	return 0
}

func firstJSONText(raw string, paths ...string) string {
	if raw == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return ""
	}
	for _, p := range paths {
		cur := v
		ok := true
		for _, part := range strings.Split(p, ".") {
			m, isMap := cur.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			cur, ok = m[part]
			if !ok {
				break
			}
		}
		if ok {
			switch x := cur.(type) {
			case string:
				if strings.TrimSpace(x) != "" {
					return strings.TrimSpace(x)
				}
			case float64:
				return strconv.FormatFloat(x, 'f', -1, 64)
			case bool:
				return strconv.FormatBool(x)
			}
		}
	}
	return ""
}

func firstJSONFloat(raw string, paths ...string) float64 {
	for _, p := range paths {
		s := firstJSONText(raw, p)
		if s == "" {
			continue
		}
		if v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); err == nil {
			return v
		}
	}
	return 0
}

func homeExpanded(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}
