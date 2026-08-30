// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// digest.go implements the `digest` command: a compact "what's notable" snapshot
// for a condition or free-text search term. It reports total trials found,
// recruiting count, the newest N trials, and any recently-terminated ones —
// using only ClinicalTrials.gov registry signals.
//
// This is a read-only point-in-time snapshot. It does not persist state and
// does not diff against a previous run (see `watch` for change-feed behaviour).
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// digestFetchCap is the maximum number of trials fetched (and reflected in the
// totals) for a single digest. One page is enough for a "what's notable" read.
const digestFetchCap = 200

// digestTrial is a condensed trial row in the digest output.
type digestTrial struct {
	NCTID      string `json:"id"`
	Title      string `json:"title,omitempty"`
	Status     string `json:"status,omitempty"`
	Phase      string `json:"phase,omitempty"`
	Sponsor    string `json:"sponsor,omitempty"`
	LastUpdate string `json:"last_update,omitempty"`
}

// digestView is the JSON shape `digest` emits.
type digestView struct {
	Term        string        `json:"term"`
	Total       int           `json:"total"`
	Recruiting  int           `json:"recruiting"`
	Newest      []digestTrial `json:"newest,omitempty"`
	RecentlyTerminated []digestTrial `json:"recently_terminated,omitempty"`
	Note        string        `json:"note,omitempty"`
}

// pp:data-source live
func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "digest <term>",
		Short: "Compact 'what's notable' snapshot for a condition or search term: total, recruiting count, newest trials, recently-terminated ones.",
		Long: "Fetches trials matching a condition or free-text term and produces a compact\n" +
			"digest: total matched, how many are actively recruiting, the N most recently\n" +
			"updated trials (newest first), and any that have been terminated or withdrawn.\n\n" +
			"This is a READ-ONLY point-in-time snapshot. No state is stored between runs;\n" +
			"use the `watch` command for a persistent change feed.",
		Example: "  clinical-trials-pp-cli digest \"type 2 diabetes\"\n" +
			"  clinical-trials-pp-cli digest alzheimer --limit 5 --json",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would produce a condition digest from ClinicalTrials.gov")
				return nil
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a term argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch up to digestFetchCap trials for the term; one page is enough.
			trials, err := ctgovFetch(ctx, c, ctgovParams("term", term), digestFetchCap, 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(trials) == 0 {
				return noResultsErr(term)
			}

			view := buildDigest(term, trials, limit)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderDigestHuman(cmd, view)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Max trials to show in newest and recently-terminated lists (min 1)")
	return cmd
}

// buildDigest is the pure core of the `digest` command: it partitions the
// matched trials into a recruiting count and a stopped subset, then assembles
// the newest and recently-stopped lists (each capped at limit). It performs no
// I/O so it can be unit-tested directly.
func buildDigest(term string, trials []Trial, limit int) digestView {
	recruitingCount := 0
	var stopped []Trial
	for _, t := range trials {
		upper := strings.ToUpper(t.Status)
		if upper == "RECRUITING" || upper == "ENROLLING_BY_INVITATION" {
			recruitingCount++
		}
		if isTerminalStatus(t.Status) {
			stopped = append(stopped, t)
		}
	}

	// Newest N: sort a copy of all trials by LastUpdate desc, take up to limit.
	sorted := make([]Trial, len(trials))
	copy(sorted, trials)
	newest := topDigestTrials(sorted, limit)

	// Recently-stopped: same treatment over the stopped subset.
	recentlyTerminated := topDigestTrials(stopped, limit)

	note := ""
	if len(trials) >= digestFetchCap {
		note = fmt.Sprintf("result set capped at %d trials; totals reflect the first %d matched", digestFetchCap, digestFetchCap)
	}

	return digestView{
		Term:               term,
		Total:              len(trials),
		Recruiting:         recruitingCount,
		Newest:             newest,
		RecentlyTerminated: recentlyTerminated,
		Note:               note,
	}
}

// topDigestTrials sorts a copy of the given trials by LastUpdate descending
// (newest first) and returns at most limit of them as condensed digest rows.
// Callers pass a slice they own; the sort is in place on that slice.
func topDigestTrials(trials []Trial, limit int) []digestTrial {
	sort.SliceStable(trials, func(i, j int) bool {
		return trials[i].LastUpdate > trials[j].LastUpdate
	})
	if len(trials) > limit {
		trials = trials[:limit]
	}
	out := make([]digestTrial, 0, len(trials))
	for _, t := range trials {
		out = append(out, digestTrial{
			NCTID:      t.NCTID,
			Title:      t.Title,
			Status:     t.Status,
			Phase:      t.Phase,
			Sponsor:    t.Sponsor,
			LastUpdate: t.LastUpdate,
		})
	}
	return out
}

func renderDigestHuman(cmd *cobra.Command, v digestView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Digest: %q\n", v.Term)
	fmt.Fprintf(w, "  Total matched: %d    Recruiting: %d\n\n", v.Total, v.Recruiting)

	if len(v.Newest) > 0 {
		fmt.Fprintln(w, "Newest (by last update):")
		for _, t := range v.Newest {
			fmt.Fprintf(w, "  %-14s %-18s %-10s %s\n",
				t.NCTID, truncate(t.Status, 18), truncate(t.Phase, 10), truncate(t.Title, 60))
		}
		fmt.Fprintln(w)
	}

	if len(v.RecentlyTerminated) > 0 {
		fmt.Fprintln(w, "Recently stopped (terminated/withdrawn/suspended):")
		for _, t := range v.RecentlyTerminated {
			fmt.Fprintf(w, "  %-14s %-18s %s\n",
				t.NCTID, truncate(t.Status, 18), truncate(t.Title, 60))
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "No recently stopped (terminated/withdrawn/suspended) trials in this result set.")
	}

	if v.Note != "" {
		fmt.Fprintf(w, "note: %s\n", v.Note)
	}
	return nil
}
