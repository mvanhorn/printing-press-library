package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newDriftCmd returns the `drift` command — report network changes since a
// time window, based on the `updatedAt` field in synced entity payloads.
//
// The simplest definition for "what changed" with a single sync history: an
// entity whose payload `updatedAt` falls within the --since window. Each
// entity has been observed at least once at sync time.
func newDriftCmd(flags *rootFlags) *cobra.Command {
	var since string
	var entityType string
	var limit int
	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "Report network entities updated since a time window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Walk locally-synced entities and surface those whose API-side
updatedAt timestamp falls within the --since window. Combined with a
periodic 'sync', this is your "what changed on the network" view.

Run 'sync' first; this command reads the local store.`,
		Example: strings.Trim(`
  # Collections updated in the last 7 days
  postman-explore-pp-cli drift --since 7d --type collection

  # APIs updated in the last 30 days, JSON output
  postman-explore-pp-cli drift --since 30d --type api --json --limit 25`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if entityType != "" && !validEntityType(entityType) {
				return usageErr(fmt.Errorf("invalid --type %q (use one of collection, workspace, api, flow, or omit for all)", entityType))
			}
			if since == "" {
				return usageErr(fmt.Errorf("--since is required (e.g. 7d, 24h, 30d)"))
			}
			window, err := parseDriftWindow(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window)

			db, err := openLocalStore(flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var items []json.RawMessage
			if entityType != "" {
				items, err = queryNetworkEntities(db, entityType, "")
			} else {
				items, err = queryAllNetworkEntities(db, "")
			}
			if err != nil {
				return err
			}

			type change struct {
				ID          string `json:"id"`
				EntityType  string `json:"entityType"`
				Name        string `json:"name"`
				UpdatedAt   string `json:"updatedAt"`
				CreatedAt   string `json:"createdAt"`
				IsNew       bool   `json:"isNew"`
				RedirectURL string `json:"redirectURL,omitempty"`
			}
			out := make([]change, 0)
			for _, raw := range items {
				var doc struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					EntityType  string `json:"entityType"`
					UpdatedAt   string `json:"updatedAt"`
					CreatedAt   string `json:"createdAt"`
					RedirectURL string `json:"redirectURL"`
				}
				if err := json.Unmarshal(raw, &doc); err != nil {
					continue
				}
				updated, ok := parseTime(doc.UpdatedAt)
				if !ok {
					continue
				}
				if updated.Before(cutoff) {
					continue
				}
				created, _ := parseTime(doc.CreatedAt)
				out = append(out, change{
					ID:          stringify(doc.ID),
					EntityType:  doc.EntityType,
					Name:        doc.Name,
					UpdatedAt:   doc.UpdatedAt,
					CreatedAt:   doc.CreatedAt,
					IsNew:       !created.Before(cutoff),
					RedirectURL: doc.RedirectURL,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if len(out) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd, []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no entities updated in this window — run 'sync' or widen --since"))
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd, out, flags)
			}
			tbl := make([][]string, 0, len(out))
			for _, c := range out {
				marker := "~"
				if c.IsNew {
					marker = "+"
				}
				tbl = append(tbl, []string{
					marker,
					c.EntityType,
					c.Name,
					c.UpdatedAt,
					c.RedirectURL,
				})
			}
			return flags.printTable(cmd, []string{"", "Type", "Name", "UpdatedAt", "URL"}, tbl)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window to consider (e.g. 7d, 24h, 30d)")
	cmd.Flags().StringVar(&entityType, "type", "", "Restrict to a single entity type (omit for all)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	return cmd
}

// parseDriftWindow accepts shorthand like "7d", "24h", "30m". Returns an
// error message that names the bad input so the caller can surface it.
func parseDriftWindow(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if len(s) >= 2 && s[len(s)-1] == 'd' {
		var n int
		if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &n); err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return 0, fmt.Errorf("could not parse --since %q (use forms like 7d, 24h, 30m)", s)
}

// parseTime accepts the ISO-8601 forms the proxy returns.
func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
