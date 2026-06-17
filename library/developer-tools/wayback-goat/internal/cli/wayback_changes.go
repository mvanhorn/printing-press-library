// Copyright 2026 Alex Bresler and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-written: the `changes` content-change analytic.
//
// `changes` collapses a URL's Wayback Machine capture history by content digest
// and reports each point where the page content actually changed. The first
// capture is the baseline (first-seen); every later digest flip is a change
// event. This is the analysis the Wayback web UI and URL-listing tools do not
// surface, and it is the reason this CLI exists beyond a thin index wrapper.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wayback-goat/internal/source/wayback"
)

func newChangesCmd(flags *rootFlags) *cobra.Command {
	var matchType, from, to string
	var limit int
	var allStatus bool

	cmd := &cobra.Command{
		Use:   "changes <url>",
		Short: "Report when a URL's archived content actually changed (digest-diff).",
		Long: "Collapse a URL's Wayback Machine capture history by content digest and report " +
			"each point where the page content changed. The first capture is the baseline " +
			"(first-seen); every later digest flip is a change event. By default only HTTP 200 " +
			"captures are considered, so a transient redirect or 404 snapshot is not mistaken " +
			"for a content change.",
		Example: "  wayback-goat-pp-cli changes example.com\n" +
			"  wayback-goat-pp-cli changes https://example.com/pricing --json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.TrimSpace(args[0])
			if !looksLikeURLOrHost(target) {
				return fmt.Errorf("invalid url %q: expected a domain or URL (for example example.com or https://example.com/path)", args[0])
			}
			c := wayback.NewClient()
			caps, err := c.Captures(cmd.Context(), target, wayback.CapturesOptions{
				MatchType: matchType,
				From:      from,
				To:        to,
				Limit:     limit,
				Status200: !allStatus,
			})
			if err != nil {
				return err
			}
			firstSeen, changes := wayback.Changes(caps)

			if flags.asJSON {
				out := map[string]any{
					"url":      target,
					"captures": len(caps),
					"changes":  changes,
				}
				if firstSeen != nil {
					out["first_seen"] = firstSeen.Timestamp
				} else {
					out["first_seen"] = nil
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			if firstSeen == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "no captures for %s\n", target)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d captures, first seen %s, %d content change(s)\n",
				target, len(caps), fmtWaybackDate(firstSeen.Timestamp), len(changes))
			for _, ch := range changes {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s… → %s…  (HTTP %s)\n",
					fmtWaybackDate(ch.Timestamp), shortDigest(ch.PrevDigest), shortDigest(ch.NewDigest), ch.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&matchType, "match-type", "exact", "match scope: exact|prefix|host|domain")
	cmd.Flags().StringVar(&from, "from", "", "lower-bound timestamp (YYYYMMDD...)")
	cmd.Flags().StringVar(&to, "to", "", "upper-bound timestamp (YYYYMMDD...)")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit captures scanned (0 = all)")
	cmd.Flags().BoolVar(&allStatus, "all-status", false, "include non-200 captures (default: 200 only)")
	return cmd
}

// looksLikeURLOrHost rejects arguments that cannot be a web address so an
// obviously-invalid token fails fast with a non-zero exit instead of being sent
// to the archive as a doomed query that returns an empty (but successful) result.
func looksLikeURLOrHost(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		return true
	}
	return strings.Contains(s, ".")
}

func fmtWaybackDate(ts string) string {
	t, err := time.Parse("20060102150405", ts)
	if err != nil {
		return ts
	}
	return t.Format("2006-01-02")
}

func shortDigest(d string) string {
	if len(d) > 8 {
		return d[:8]
	}
	return d
}
