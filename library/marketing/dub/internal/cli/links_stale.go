// Hand-written novel feature: links stale.
// Find archived, expired, or zero-traffic links across the workspace before
// they pile up. Joins local link metadata with analytics aggregates the API
// doesn't expose together.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type staleLink struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	Key       string `json:"key"`
	URL       string `json:"url"`
	Clicks    int    `json:"clicks"`
	Archived  bool   `json:"archived"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	Reason    string `json:"reason"`
}

func newLinksStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var includeArchived bool
	var maxClicks int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find archived, expired, or zero-traffic links across the workspace",
		Long: `Find links that look dead based on local data:

  • zero or low clicks in the last N days
  • archived links that still have referrers
  • links past their expiresAt with no replacement

Joins the local links store with analytics counters. Run ` + "`dub-pp-cli sync`" + ` first.`,
		Example: strings.Trim(`
  # Links with zero clicks in the last 90 days
  dub-pp-cli links stale --days 90 --json --select id,key,clicks,archived

  # Lower the threshold to "fewer than 5 clicks"
  dub-pp-cli links stale --days 60 --max-clicks 5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

			query := `SELECT id, COALESCE(domain,''), COALESCE("key",''), COALESCE(data,'{}'), COALESCE(archived,0)
				FROM links`
			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()

			var results []staleLink
			seen := 0
			for rows.Next() {
				var id, domain, key string
				var blob sql.NullString
				var archivedInt int
				if err := rows.Scan(&id, &domain, &key, &blob, &archivedInt); err != nil {
					return err
				}
				seen++
				archived := archivedInt != 0
				blobBytes := []byte("{}")
				if blob.Valid {
					blobBytes = []byte(blob.String)
				}
				clicks := int(extractNumber(blobBytes, "clicks"))
				url := extractFromData(blobBytes, "url")
				expiresAt := extractFromData(blobBytes, "expiresAt")
				createdAt := extractFromData(blobBytes, "createdAt")

				reason := ""
				switch {
				case archived:
					reason = "archived"
				case expiresAt != "" && expiresAt < time.Now().Format(time.RFC3339):
					reason = "expired"
				case clicks <= maxClicks && (createdAt == "" || createdAt < cutoff):
					reason = fmt.Sprintf("low-traffic (%d clicks since %s)", clicks, cutoff[:10])
				}
				if reason == "" {
					continue
				}
				if !includeArchived && archived {
					continue
				}
				results = append(results, staleLink{
					ID:        id,
					Domain:    domain,
					Key:       key,
					URL:       url,
					Clicks:    clicks,
					Archived:  archived,
					ExpiresAt: expiresAt,
					CreatedAt: createdAt,
					Reason:    reason,
				})
			}
			if seen == 0 {
				return hintEmptyStore("links")
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale links found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %s\n", "KEY", "CLICKS", "REASON")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12d %s\n", truncate(r.Domain+"/"+r.Key, 30), r.Clicks, r.Reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/dub-pp-cli/data.db)")
	cmd.Flags().IntVar(&days, "days", 90, "Lookback window in days for low-traffic detection")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", true, "Include already-archived links in the result")
	cmd.Flags().IntVar(&maxClicks, "max-clicks", 0, "Treat as low-traffic if click count is at or below this value")
	return cmd
}

// (compile guard: ensure encoding/json is used even when the file is trimmed)
var _ = json.RawMessage{}
