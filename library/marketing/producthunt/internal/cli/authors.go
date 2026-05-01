package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

type coOccurrencePayload struct {
	Other           string `json:"other"`
	SharedSnapshots int    `json:"shared_snapshots"`
}

func newAuthorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "authors",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Query author-derived signals from your local snapshot store",
		Long:  `Parent command for author-oriented queries. See subcommands.`,
		Example: `  producthunt-pp-cli authors related --to 'Ryan Hoover' --since 90d
  producthunt-pp-cli authors --help`,
	}
	cmd.AddCommand(newAuthorsRelatedCmd(flags))
	return cmd
}

func newAuthorsRelatedCmd(flags *rootFlags) *cobra.Command {
	var target string
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "related",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Authors who repeatedly appeared in the same /feed snapshots as a given author",
		Long: `A rough social signal from pure /feed data: for a target author, return
the authors whose posts co-occurred in feed snapshots most often. Computed
from your local snapshot store — more snapshots = sharper signal.`,
		Example: `  producthunt-pp-cli authors related --to 'Ryan Hoover' --since 90d
  producthunt-pp-cli authors related --to 'Ryan Hoover' --agent --select 'other,shared_snapshots'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return usageErr(fmt.Errorf("--to <author-name> is required"))
			}
			var sinceT time.Time
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
			occ, err := db.AuthorsCoOccurring(target, sinceT, limit)
			if err != nil {
				return apiErr(err)
			}
			out := make([]coOccurrencePayload, len(occ))
			for i, o := range occ {
				out[i] = coOccurrencePayload{Other: o.Other, SharedSnapshots: o.SharedSnapshots}
			}
			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().StringVar(&target, "to", "", "Target author name (exact match). Required.")
	cmd.Flags().StringVar(&since, "since", "", "Only consider snapshots taken at or after this time")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max co-occurring authors to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	return cmd
}
