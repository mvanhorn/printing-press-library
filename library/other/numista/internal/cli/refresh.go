// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

var refreshTypeRE = regexp.MustCompile(`^/types/([0-9]+)(?:$|/)`)

// PATCH: hand-written stale cache refresh command promised by README Highlights.
func newRefreshCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var typeID int64
	var older string
	var field string

	cmd := &cobra.Command{
		Use:     "refresh [--all] [--type-id N#] [--older 30d] [--field prices|issues|all] [--dry-run]",
		Short:   "Refresh cached Numista data — re-fetch only fields that change (prices, issues), leaving cataloger-set identity untouched.",
		Example: "  numista-pp-cli refresh --all --older 30d --dry-run --json\n  numista-pp-cli refresh --type-id 11013 --field prices --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}
			if typeID > 0 && all {
				fmt.Fprintln(os.Stderr, "warning: --type-id wins over --all")
			}
			if !flags.dryRun && !all && typeID == 0 {
				return usageErr(fmt.Errorf("refresh needs one of --all or --type-id in live mode"))
			}
			if field != "prices" && field != "issues" && field != "all" {
				return usageErr(fmt.Errorf("--field must be one of prices, issues, all; got %q", field))
			}
			age, err := parseAgeDuration(older)
			if err != nil {
				return usageErr(err)
			}
			stale, err := staleRefreshEntries(cmd.Context(), time.Now().Add(-age), typeID, field)
			if err != nil {
				return err
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), stale, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			refreshed := 0
			errs := 0
			for _, entry := range stale {
				for _, path := range entry.SuggestedRefreshPaths {
					params := map[string]string{}
					if strings.HasSuffix(path, "/prices") {
						params["currency"] = "EUR"
					}
					if _, err := c.GetNoCache(path, params); err != nil {
						errs++
						continue
					}
					refreshed++
				}
			}
			q, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"refreshed":   refreshed,
				"errors":      errs,
				"quota_after": q,
			}, flags)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Refresh all stale cached types")
	cmd.Flags().Int64Var(&typeID, "type-id", 0, "Refresh one type ID")
	cmd.Flags().StringVar(&older, "older", "30d", "Staleness threshold (Nd, Nh, Nm, or Go duration)")
	cmd.Flags().StringVar(&field, "field", "prices", "Field to refresh: prices, issues, all")
	return cmd
}

type refreshEntry struct {
	TypeID                int64    `json:"type_id"`
	LastSeen              string   `json:"last_seen"`
	SuggestedRefreshPaths []string `json:"suggested_refresh_paths"`
}

func staleRefreshEntries(ctx context.Context, cutoff time.Time, onlyTypeID int64, field string) ([]refreshEntry, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("numista-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer s.Close()
	rows, err := s.DB().QueryContext(ctx,
		`SELECT endpoint, MAX(called_at)
		 FROM lookup_log
		 WHERE method = 'GET'
		   AND is_valid_request = 1
		   AND endpoint LIKE '/types/%'
		 GROUP BY endpoint`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byType := map[int64]*refreshEntry{}
	for rows.Next() {
		var endpoint string
		var last sql.NullString
		if err := rows.Scan(&endpoint, &last); err != nil {
			return nil, err
		}
		tid, ok := parseTypeIDFromEndpoint(endpoint)
		if !ok || (onlyTypeID > 0 && tid != onlyTypeID) || !last.Valid {
			continue
		}
		lastSeen, err := parseSQLiteTime(last.String)
		if err != nil || lastSeen.After(cutoff) {
			continue
		}
		if !refreshFieldAllowsEndpoint(field, endpoint) {
			continue
		}
		ent := byType[tid]
		if ent == nil {
			ent = &refreshEntry{TypeID: tid, LastSeen: lastSeen.UTC().Format(time.RFC3339)}
			byType[tid] = ent
		}
		if lastSeen.Before(mustParseRFC3339(ent.LastSeen)) {
			ent.LastSeen = lastSeen.UTC().Format(time.RFC3339)
		}
		ent.SuggestedRefreshPaths = append(ent.SuggestedRefreshPaths, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]refreshEntry, 0, len(byType))
	for _, ent := range byType {
		sort.Strings(ent.SuggestedRefreshPaths)
		out = append(out, *ent)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TypeID < out[j].TypeID })
	return out, nil
}

func parseAgeDuration(s string) (time.Duration, error) {
	if s == "" {
		return 30 * 24 * time.Hour, nil
	}
	unit := s[len(s)-1:]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err == nil {
		switch unit {
		case "d":
			return time.Duration(n) * 24 * time.Hour, nil
		case "h":
			return time.Duration(n) * time.Hour, nil
		case "m":
			return time.Duration(n) * time.Minute, nil
		}
	}
	return time.ParseDuration(s)
}

func parseTypeIDFromEndpoint(endpoint string) (int64, bool) {
	m := refreshTypeRE.FindStringSubmatch(endpoint)
	if len(m) != 2 {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil && id > 0
}

func refreshFieldAllowsEndpoint(field, endpoint string) bool {
	isType := !strings.Contains(strings.TrimPrefix(endpoint, "/types/"), "/")
	isIssues := strings.HasSuffix(endpoint, "/issues")
	isPrices := strings.HasSuffix(endpoint, "/prices")
	switch field {
	case "prices":
		return isPrices
	case "issues":
		return isType || isIssues
	case "all":
		return isType || isIssues || isPrices
	default:
		return false
	}
}

func parseSQLiteTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", s)
}

func mustParseRFC3339(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
