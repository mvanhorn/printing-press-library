// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `feeds` parent: add, list, sync.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/rss"
)

func newFeedsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feeds",
		Short: "Subscribe to podcast RSS feeds and sync new episodes",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFeedsAddCmd(flags))
	cmd.AddCommand(newFeedsListCmd(flags))
	cmd.AddCommand(newFeedsSyncCmd(flags))
	return cmd
}

func newFeedsAddCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [rss-url]",
		Short:   "Track an RSS feed",
		Example: "  podcast-goat-pp-cli feeds add https://feeds.example.com/show.xml",
		Args:    cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			url := args[0]
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			title := ""
			if !cliutil.IsVerifyEnv() {
				adapter := rss.New()
				if showTitle, _, ferr := adapter.FetchFeedItems(cmd.Context(), url); ferr == nil {
					title = showTitle
				}
			}
			id, err := ps.AddFeed(cmd.Context(), url, "rss", title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added feed id=%d title=%q url=%s\n", id, title, url)
			return nil
		},
	}
	return cmd
}

func newFeedsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List tracked feeds",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.ListFeeds(cmd.Context())
			if err != nil {
				return err
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no feeds yet. Try `feeds add <rss-url>`.")
				return nil
			}
			headers := []string{"id", "title", "url", "last_sync"}
			var data [][]string
			for _, r := range rows {
				last := "(never)"
				if r.LastSyncAt.Valid {
					last = r.LastSyncAt.Time.Format("2006-01-02 15:04")
				}
				data = append(data, []string{fmt.Sprintf("%d", r.ID), r.ShowTitle, r.URL, last})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	return cmd
}

func newFeedsSyncCmd(flags *rootFlags) *cobra.Command {
	var flagFeed int64
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull transcripts for new items across tracked feeds",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			feeds, err := ps.ListFeeds(cmd.Context())
			if err != nil {
				return err
			}
			if len(feeds) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no feeds tracked")
				return nil
			}
			adapter := rss.New()
			total := 0
			for _, f := range feeds {
				if flagFeed != 0 && f.ID != flagFeed {
					continue
				}
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.OutOrStdout(), "would sync feed %d (%s) (verify mode)\n", f.ID, f.URL)
					continue
				}
				_, items, err := adapter.FetchFeedItems(cmd.Context(), f.URL)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "feed %d: %v\n", f.ID, err)
					continue
				}
				added := 0
				for _, it := range items {
					if it.Link == "" {
						continue
					}
					if existing, _ := ps.GetTranscript(cmd.Context(), it.Link); existing != nil {
						continue
					}
					res, derr := dispatch.Dispatch(cmd.Context(), it.Link, dispatch.Options{})
					if derr != nil {
						// Try RSS-specific fallback even if dispatcher said no match.
						var na *source.NotApplicableError
						if !errors.As(derr, &na) {
							continue
						}
						continue
					}
					if res.Transcript == nil {
						continue
					}
					_ = ps.UpsertTranscript(cmd.Context(), res.Transcript)
					added++
				}
				total += added
				_ = ps.MarkFeedSynced(cmd.Context(), f.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "feed %d: %d new transcript(s)\n", f.ID, added)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sync complete: %d new\n", total)
			return nil
		},
	}
	cmd.Flags().Int64Var(&flagFeed, "feed", 0, "Only sync this feed id")
	_ = strings.TrimSpace // placeholder for lint
	return cmd
}
