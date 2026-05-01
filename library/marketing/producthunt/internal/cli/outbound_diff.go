package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

type outboundDriftRow struct {
	PostID      int64  `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	PreviousURL string `json:"previous_external_url"`
	CurrentURL  string `json:"current_external_url"`
	ChangedAt   string `json:"changed_at"`
	SeenCount   int    `json:"seen_count"`
}

func newOutboundDiffCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "outbound-diff",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List posts whose external URL has changed across snapshots",
		Long: `Find posts whose external landing URL (/r/p/{id} PH redirect) has been
updated across syncs. Signals domain moves, beta-to-launch transitions,
or link swaps.

Current implementation flags any post with seen_count > 1 and last_seen_at
in the window — use it as a candidate set and compare the current_external_url
against archived snapshots for full drift detection.

Run 'sync' on multiple days before using this command; a store with only
one snapshot cannot detect drift.`,
		Example: `  producthunt-pp-cli outbound-diff --since 30d
  producthunt-pp-cli outbound-diff --since 7d --agent --select 'slug,title,current_external_url'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := parseRelativeOrAbsoluteTime(since)
			if err != nil {
				return usageErr(fmt.Errorf("--since: %w", err))
			}
			autoWarm(flags, dbPath)
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			if err := store.EnsurePHTables(db); err != nil {
				return configErr(err)
			}
			drifts, err := db.OutboundDrift(t)
			if err != nil {
				return apiErr(err)
			}
			if limit > 0 && len(drifts) > limit {
				drifts = drifts[:limit]
			}
			out := make([]outboundDriftRow, len(drifts))
			for i, d := range drifts {
				row := outboundDriftRow{
					PostID:      d.PostID,
					Slug:        d.Slug,
					Title:       d.Title,
					PreviousURL: d.OldURL,
					CurrentURL:  d.NewURL,
					ChangedAt:   d.ChangedAt.UTC().Format(time.RFC3339),
				}
				if p, err := db.GetPostByID(d.PostID); err == nil && p != nil {
					row.SeenCount = p.SeenCount
				}
				out[i] = row
			}
			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "30d", "Window for drift candidates (e.g. '7d', '30d', '2026-01-01')")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	return cmd
}
