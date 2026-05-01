package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

type makerTallyPayload struct {
	Author      string `json:"author"`
	TotalCount  int    `json:"total_count"`
	UniquePosts int    `json:"unique_posts"`
}

func newMakersCmd(flags *rootFlags) *cobra.Command {
	var top int
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "makers",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Top authors (makers and hunters) across all synced snapshots",
		Long: `Aggregate the author field across every snapshot in the window. Returns
each author with total_count (how many snapshot appearances they logged)
and unique_posts (how many distinct slugs they're credited on).

Requires at least one snapshot in the window. An empty result window
returns [].`,
		Example: `  # Top 10 authors in the last 30 days
  producthunt-pp-cli makers --since 30d --top 10

  # Agent-friendly payload
  producthunt-pp-cli makers --since 7d --agent --select 'author,total_count'

  # All time
  producthunt-pp-cli makers --since 10y --top 50 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if top <= 0 {
				top = 10
			}
			sinceT := time.Time{}
			if since != "" {
				t, err := parseRelativeOrAbsoluteTime(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				sinceT = t
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

			tallies, err := db.TopAuthorsSince(sinceT, top)
			if err != nil {
				return apiErr(err)
			}
			out := make([]makerTallyPayload, len(tallies))
			for i, t := range tallies {
				out[i] = makerTallyPayload{
					Author:      t.Author,
					TotalCount:  t.Count,
					UniquePosts: t.Unique,
				}
			}
			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().IntVar(&top, "top", 10, "Max authors to return")
	cmd.Flags().StringVar(&since, "since", "", "Aggregate only snapshots taken after this time (e.g. '30d'); empty = all time")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	return cmd
}
