// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: since. Lists posts and pages
// modified within a time window, across those types at once, from the local
// mirror. (WooCommerce Store API products carry no timestamps, so they are
// excluded.)

// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "List posts and pages modified within a time window, across those types at once",
		Long: "Show everything modified within a window — posts and pages\n" +
			"pieces — sorted newest first, computed from the local mirror's modified\n" +
			"timestamps. Duration accepts Go durations plus day/week shorthand:\n" +
			"24h, 7d, 2w, 720h.\n\n" +
			"Shop products are excluded because the WooCommerce Store API omits\n" +
			"timestamps. Run `sync` first.",
		Example: "  nisifilters-pp-cli since 7d\n" +
			"  nisifilters-pp-cli since 30d --json\n" +
			"  nisifilters-pp-cli since 24h --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a duration is required (e.g. 7d, 24h, 2w)"))
			}
			dur, err := cliutil.ParseDurationLoose(args[0])
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid duration %q: %w", args[0], err))
			}
			cutoff := time.Now().Add(-dur)

			db, err := fgOpenStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			type changed struct {
				Type     string `json:"type"`
				ID       int    `json:"id"`
				Title    string `json:"title"`
				Modified string `json:"modified"`
				Link     string `json:"link"`
				t        time.Time
			}
			out := []changed{} // non-nil so empty results serialize as [] not null
			synced := 0
			for _, rt := range []string{"posts", "pages"} {
				rows, lerr := db.List(rt, 5000)
				if lerr != nil {
					continue
				}
				synced += len(rows)
				for _, raw := range rows {
					obj := fgDecode(raw)
					ms := fgString(obj, "modified_gmt")
					loc := time.UTC
					if ms == "" {
						ms = fgString(obj, "modified")
						loc = time.Local
					}
					t, perr := fgParseWPTime(ms, loc)
					if perr != nil || t.Before(cutoff) {
						continue
					}
					id, _ := fgInt(obj, "id")
					out = append(out, changed{
						Type:     rt,
						ID:       id,
						Title:    fgPlainTitle(obj),
						Modified: fgString(obj, "modified"),
						Link:     fgString(obj, "link"),
						t:        t,
					})
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].t.After(out[j].t) })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				view := map[string]any{
					"since":   args[0],
					"cutoff":  cutoff.UTC().Format(time.RFC3339),
					"count":   len(out),
					"scanned": synced,
					"items":   out,
				}
				if synced == 0 {
					view["note"] = fgEmptyStoreNote("content")
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			w := cmd.OutOrStdout()
			if synced == 0 {
				fmt.Fprintln(w, fgEmptyStoreNote("content"))
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintf(w, "Nothing modified in the last %s (scanned %d items).\n", args[0], synced)
				return nil
			}
			for _, c := range out {
				fmt.Fprintf(w, "%-10s %s  %s\n", c.Type, c.Modified, c.Title)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum items to return (0 = all)")
	return cmd
}

// fgParseWPTime parses a WordPress timestamp ("2006-01-02T15:04:05", optionally
// with a trailing Z or offset) in the given location.
func fgParseWPTime(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", s)
}
