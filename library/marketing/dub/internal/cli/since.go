// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// since produces a unified change feed across links, customers, partners, and
// commissions whose updatedAt/createdAt falls in the requested window. Reads
// from the local /store cache (store.Open) so the output is offline-friendly.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type sinceItem struct {
	Resource  string `json:"resource"`
	ID        string `json:"id"`
	Action    string `json:"action"` // created | updated
	Timestamp string `json:"timestamp"`
	Summary   string `json:"summary"`
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var resourceFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "since [duration]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Time-windowed change feed — what was created/updated in the last N",
		Long: `Show every link, partner, customer, or commission that was created or updated
within the given duration. Reads from the local store using each row's createdAt
and updatedAt timestamps. Run sync first to refresh.`,
		Example: `  # Everything from the last 24 hours
  dub-pp-cli since 24h --json

  # Just links from the last 7 days
  dub-pp-cli since 7d --resource links

  # Cap to 50 rows
  dub-pp-cli since 1h --limit 50 --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			window := "24h"
			if len(args) > 0 {
				window = args[0]
			}
			windowDur, err := parseWindow(window)
			if err != nil {
				return usageErr(err)
			}
			cutoff := nowFunc().Add(-windowDur)

			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			resources := []string{"links", "partners", "customers", "commissions", "domains", "tags"}
			if resourceFilter != "" {
				resources = []string{resourceFilter}
			}

			out := make([]sinceItem, 0)
			for _, resource := range resources {
				rows, err := db.DB().Query(`SELECT id, data FROM ` + resource)
				if err != nil {
					continue
				}
				for rows.Next() {
					var id, raw string
					if err := rows.Scan(&id, &raw); err != nil {
						continue
					}
					var obj map[string]any
					if err := json.Unmarshal([]byte(raw), &obj); err != nil {
						continue
					}
					createdAt := stringField(obj, "createdAt")
					updatedAt := stringField(obj, "updatedAt")
					if createdAt != "" {
						if t, err := parseTimestamp(createdAt); err == nil && t.After(cutoff) {
							out = append(out, sinceItem{
								Resource:  resource,
								ID:        id,
								Action:    "created",
								Timestamp: createdAt,
								Summary:   summarizeItem(resource, obj),
							})
							continue
						}
					}
					if updatedAt != "" {
						if t, err := parseTimestamp(updatedAt); err == nil && t.After(cutoff) {
							out = append(out, sinceItem{
								Resource:  resource,
								ID:        id,
								Action:    "updated",
								Timestamp: updatedAt,
								Summary:   summarizeItem(resource, obj),
							})
						}
					}
				}
				rows.Close()
			}

			// most recent first
			sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no changes in the last %s\n", window)
				return nil
			}
			headers := []string{"WHEN", "RESOURCE", "ACTION", "ID", "SUMMARY"}
			rowsTbl := make([][]string, 0, len(out))
			for _, it := range out {
				rowsTbl = append(rowsTbl, []string{it.Timestamp, it.Resource, it.Action, it.ID, it.Summary})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}

	cmd.Flags().StringVar(&resourceFilter, "resource", "", "Restrict to a single resource (links, partners, customers, ...)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap output to N rows (0 = no limit)")
	return cmd
}

func summarizeItem(resource string, obj map[string]any) string {
	switch resource {
	case "links":
		short := stringField(obj, "shortLink")
		url := stringField(obj, "url")
		if short != "" && url != "" {
			return short + " -> " + truncRight(url, 40)
		}
		return short
	case "partners":
		name := stringField(obj, "name")
		email := stringField(obj, "email")
		if email != "" {
			return name + " <" + email + ">"
		}
		return name
	case "customers":
		email := stringField(obj, "email")
		if email != "" {
			return email
		}
		return stringField(obj, "name")
	case "commissions":
		amt := intField(obj, "amount")
		status := stringField(obj, "status")
		return fmt.Sprintf("%s %d", status, amt)
	case "domains":
		return stringField(obj, "slug")
	case "tags":
		return stringField(obj, "name")
	}
	return ""
}

func truncRight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], "/") + "…"
}
