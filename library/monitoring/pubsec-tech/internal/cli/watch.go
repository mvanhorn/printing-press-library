// Watch command — transcendence feature 9.
//
// Persistent watchlist that returns only the articles touching the watched
// entity since the last recorded tick. Composes with the news + recipients
// data layers; the diff state lives in the watchlists table.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Persistent watchlist that returns deltas since the last call",
		Long: "Maintains a per-(kind,value) last-tick timestamp in the local store. On each " +
			"invocation, returns articles tagged with the entity since the last tick and " +
			"advances the tick. Use `--peek` to read deltas without advancing the cursor.",
		Example: "  pubsec-tech-pp-cli watch vendor \"Leidos\"\n  pubsec-tech-pp-cli watch vendor \"Leidos\" --peek --json",
	}
	cmd.AddCommand(newWatchVendorCmd(flags))
	cmd.AddCommand(newWatchAgencyCmd(flags))
	return cmd
}

func newWatchVendorCmd(flags *rootFlags) *cobra.Command {
	return newWatchKindCmd(flags, "vendor", "recipient")
}

func newWatchAgencyCmd(flags *rootFlags) *cobra.Command {
	return newWatchKindCmd(flags, "agency", "agency")
}

func newWatchKindCmd(flags *rootFlags, useName, kind string) *cobra.Command {
	var peek bool
	var limit int
	cmd := &cobra.Command{
		Use:         useName + " <name>",
		Short:       "Show new articles touching <name> since the last tick (advances unless --peek)",
		Example:     fmt.Sprintf("  pubsec-tech-pp-cli watch %s \"Leidos\"\n  pubsec-tech-pp-cli watch %s \"GSA\" --peek --json", useName, useName),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			value := args[0]
			var lastTick time.Time
			if !peek {
				lastTick, err = s.SetWatchlistTick(ctx, kind, value)
				if err != nil {
					return err
				}
			}
			if lastTick.IsZero() {
				lastTick = time.Now().Add(-7 * 24 * time.Hour)
			}
			articles, err := s.ArticlesForEntity(ctx, kind, value, lastTick, limit)
			if err != nil {
				return err
			}
			type result struct {
				Kind      string                `json:"kind"`
				Value     string                `json:"value"`
				SinceTick time.Time             `json:"since_tick"`
				Advanced  bool                  `json:"advanced"`
				Count     int                   `json:"count"`
				Articles  []VendorRollupArticle `json:"articles,omitempty"`
			}
			r := result{Kind: kind, Value: value, SinceTick: lastTick, Advanced: !peek, Count: len(articles)}
			for _, a := range articles {
				r.Articles = append(r.Articles, VendorRollupArticle{
					ID: a.ID, SourceID: a.SourceID, Title: a.Title, Link: a.Link, PublishedAt: a.PublishedAt,
				})
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), r, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Watch %s=%s (since %s, advanced=%t): %d articles\n", kind, value, lastTick.Format("2006-01-02 15:04"), r.Advanced, r.Count)
			for _, a := range r.Articles {
				date := ""
				if !a.PublishedAt.IsZero() {
					date = a.PublishedAt.Format("2006-01-02")
				}
				fmt.Fprintf(w, "  - %s  [%s]  %s\n", date, a.SourceID, truncate(a.Title, 100))
			}
			if !flags.asJSON && len(r.Articles) == 0 {
				fmt.Fprintln(w, "(no new articles in window; run `news sync` to refresh)")
			}
			_ = json.RawMessage{}
			return nil
		},
	}
	cmd.Flags().BoolVar(&peek, "peek", false, "Read without advancing the tick (don't mark articles as seen)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum articles to return")
	return cmd
}
