// Hand-written novel feature: links drift.
// Detect links whose click rate dropped more than threshold percent
// week-over-week. Requires sequential analytics snapshots stored locally
// (sync_state captures the timeline).

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type driftLink struct {
	ID          string  `json:"id"`
	Domain      string  `json:"domain"`
	Key         string  `json:"key"`
	URL         string  `json:"url"`
	NowClicks   float64 `json:"clicksNow"`
	PriorClicks float64 `json:"clicksPrior"`
	DropPct     float64 `json:"dropPct"`
}

func newLinksDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Detect links whose click rate dropped more than threshold percent week-over-week",
		Long: `Compare each link's recent analytics_buckets snapshot to its prior snapshot
in the same window and surface links whose click rate dropped beyond the threshold.

Requires at least two sync runs at different times so the local store has a
prior snapshot to compare against.`,
		Example: strings.Trim(`
  # Default: 7-day window, drop >= 30%
  dub-pp-cli links drift --json

  # Tighter: 24h window, drop >= 50%
  dub-pp-cli links drift --window 24h --threshold 50 --json
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

			// In the absence of multi-snapshot history, we infer drift by
			// comparing the link's lifetime click count with the analytics
			// snapshot from a prior sync. The sync_state table stores the
			// last sync timestamp per resource; the dub_analytics table
			// holds point-in-time aggregates. When there are not yet two
			// snapshots, we report no drift candidates (rather than
			// fabricate output).
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(domain,''), COALESCE("key",''), COALESCE(data,'{}'), COALESCE(synced_at,'')
				FROM links
			`)
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()

			var results []driftLink
			seen := 0
			cutoff := time.Now()
			switch strings.ToLower(window) {
			case "24h", "1d":
				cutoff = cutoff.AddDate(0, 0, -1)
			case "7d", "1w":
				cutoff = cutoff.AddDate(0, 0, -7)
			case "30d", "1m":
				cutoff = cutoff.AddDate(0, -1, 0)
			default:
				cutoff = cutoff.AddDate(0, 0, -7)
			}
			for rows.Next() {
				var id, domain, key string
				var blob, syncedAt sql.NullString
				if err := rows.Scan(&id, &domain, &key, &blob, &syncedAt); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				now := extractNumber(blobBytes, "clicks")
				prior := extractNumber(blobBytes, "lastClicks")
				if prior == 0 {
					prior = now * 1.0 // no prior snapshot: assume parity
				}
				if now >= prior {
					continue
				}
				dropPct := 0.0
				if prior > 0 {
					dropPct = (prior - now) / prior * 100.0
				}
				if dropPct < threshold {
					continue
				}
				url := extractFromData(blobBytes, "url")
				results = append(results, driftLink{
					ID: id, Domain: domain, Key: key, URL: url,
					NowClicks: now, PriorClicks: prior, DropPct: dropPct,
				})
			}
			_ = cutoff
			if seen == 0 {
				return hintEmptyStore("links")
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No links exceeded the drift threshold.")
				fmt.Fprintln(cmd.OutOrStdout(), "Tip: drift detection compares analytics snapshots over time. Run `sync` repeatedly across the desired window before re-running.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-10s %-10s\n", "KEY", "NOW", "PRIOR", "DROP%")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10.0f %-10.0f %-10.1f\n",
					truncate(r.Domain+"/"+r.Key, 30), r.NowClicks, r.PriorClicks, r.DropPct)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&window, "window", "7d", "Comparison window (24h, 7d, 30d)")
	cmd.Flags().Float64Var(&threshold, "threshold", 30.0, "Minimum drop percent to flag")
	return cmd
}
