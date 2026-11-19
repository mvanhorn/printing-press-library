// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type mktGapTarget struct {
	MatchesAll       bool   `json:"matches_all,omitempty"`
	Name             string `json:"name"`
	MatchingListings int    `json:"matching_listings"`
	DistinctShops    int    `json:"distinct_shops"`
	Statement        string `json:"statement"`
}

// mktGapStatement words a result. A zero match is only ever "no listing found
// in P pages of T terms"; this command never claims "no competition".
func mktGapStatement(n, shops, pages, terms int) string {
	if n == 0 {
		return fmt.Sprintf("no listing found in %s of %s", mktCount(pages, "page"), mktCount(terms, "term"))
	}
	return fmt.Sprintf("%s from %s found in %s of %s", mktCount(n, "listing"), mktCount(shops, "shop"), mktCount(pages, "page"), mktCount(terms, "term"))
}

// mktGapsHappyStdin is the synthetic rules document the live-dogfood happy path
// pipes on stdin (the fixture must be valid JSON; the rules parser accepts the
// JSON spelling of the schema as well as the YAML subset).
const mktGapsHappyStdin = `{"targets":[{"name":"linen apron","include":["linen apron"],"exclude":[]}]}`

func newNovelMarketGapsCmd(flags *rootFlags) *cobra.Command {
	// etsy-ext: inline inputs remain exposed over MCP.
	var flagRules, flagControl, flagRulesJSON, flagControlIDs string
	var flagStdin bool
	var flagSnapshot int64

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Check which product classes you defined have no listing in the sampled results, with the evidence limits stated.",
		// etsy-ext: document inline alternatives for MCP callers.
		Long: `Check which product classes you defined have no listing in the sampled results, with the evidence limits stated.

Rules file (YAML; pass --rules - or --stdin to read it from stdin, where the JSON spelling of the same schema is also accepted):
  targets:
    - name: "linen apron"
      include: ["apron", "pinafore"]
      exclude: ["toddler"]
Patterns are case-insensitive regexes matched against title plus tags. A target with no match reports
exactly "no listing found in P pages of T terms"; this command never says "no competition".
--stdin is the same as --rules - (combining --stdin with a --rules FILE path is a usage error).
--control takes a file of listing ids that should be visible (one per line); when not all were seen,
gap claims are downgraded because the sample may have missed them.
Rules allow at most 256 include/exclude/all patterns total, each at most 512 bytes.

Rules accept all: ["linen", "apron"] to require BOTH patterns (AND), alongside any include and no exclude.
fields defaults to [title, tags]; [title] matches only the title (YAML scalar title is accepted).
Empty, duplicate, unknown fields and [tags] alone are usage errors. fold: true lowercases text and
collapses runs outside Unicode letters, digits and % to one space; patterns are not folded.
Literal punctuation removed by fold is rejected with the removed characters identified. fold defaults false.
Targets with neither include nor all patterns match every listing not excluded; matches_all and notes flag this.
JSON rejects unknown keys at every level. Rules files accept YAML or JSON.
Inline alternatives: --rules-json accepts JSON only (up to 96 KiB); --control-ids accepts comma-separated positive listing ids (up to 96 KiB).`,
		// etsy-ext: demonstrate inline content without filesystem input.
		Example: "  etsy-pp-cli market gaps --rules classes.yaml\n  etsy-pp-cli market gaps --rules classes.yaml --control known-ids.txt\n  cat classes.yaml | etsy-pp-cli market gaps --stdin\n  etsy-pp-cli market gaps --rules-json '{\"targets\":[{\"name\":\"linen apron\",\"all\":[\"linen\",\"apron\"]}]}' --control-ids 1000000001",
		// etsy-ext: declare filesystem sinks for the MCP walker.
		Annotations: map[string]string{"mcp:read-flags": "rules,control", "mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--rules=-", "pp:happy-stdin": mktGapsHappyStdin},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mktValidateSnapshotFlag(cmd, flagSnapshot); err != nil {
				return err
			}
			// etsy-ext: validate inline alternatives before store access or dry run.
			inlineRules := cmd.Flags().Changed("rules-json")
			inlineControl := cmd.Flags().Changed("control-ids")
			if inlineRules && (cmd.Flags().Changed("rules") || flagStdin) {
				return usageErr(fmt.Errorf("--rules-json conflicts with --rules and --stdin"))
			}
			if inlineControl && cmd.Flags().Changed("control") {
				return usageErr(fmt.Errorf("--control-ids conflicts with --control"))
			}
			var targets []*mktTarget
			var control []string
			var err error
			if inlineRules {
				targets, err = mktInlineRules(flagRulesJSON)
				if err != nil {
					return usageErr(err)
				}
			}
			if inlineControl {
				control, err = mktInlineControl(flagControlIDs)
				if err != nil {
					return usageErr(err)
				}
			}
			if flagStdin {
				if flagRules != "" && flagRules != "-" {
					return usageErr(fmt.Errorf("--stdin conflicts with --rules %q: use --stdin (or --rules -) to read stdin, or --rules FILE to read a file, not both", mktEcho(flagRules)))
				}
				flagRules = "-"
			}
			if dryRunOK(flags) {
				return mktDryRun(cmd, flags, "market gaps")
			}
			if flagRules == "" && !inlineRules {
				if requiredInputWaived() {
					return mktDryRun(cmd, flags, "market gaps")
				}
				return usageErr(fmt.Errorf("--rules FILE (or - / --stdin for stdin) or --rules-json is required"))
			}
			// etsy-ext: use the same compiler for inline and file rules.
			if !inlineRules {
				targets, err = mktLoadRules(flagRules, cmd.InOrStdin())
			}
			if err != nil {
				return usageErr(fmt.Errorf("rules: %w", err))
			}
			if flagControl != "" {
				if control, err = mktReadLines(flagControl); err != nil {
					return usageErr(fmt.Errorf("--control: %w", err))
				}
			}
			db, err := mktOpenRead(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db != nil {
				defer db.Close()
			}
			snap, note, err := mktPickSnapshot(db, flagSnapshot)
			if err != nil {
				return err
			}
			if snap == nil {
				return mktEmitReport(cmd, flags, map[string]any{"results": make([]any, 0), "note": note})
			}
			rows, err := mktLoadResults(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			return mktEmitReport(cmd, flags, mktGapsReport(snap, rows, targets, control, flagControl != "" || inlineControl))
		},
	}
	// etsy-ext: content flags are not filesystem sinks.
	cmd.Flags().StringVar(&flagRulesJSON, "rules-json", "", "Inline JSON targets (maximum 96 KiB; exclusive with --rules and --stdin)")
	cmd.Flags().StringVar(&flagControlIDs, "control-ids", "", "Comma-separated positive listing ids (maximum 96 KiB; exclusive with --control)")
	cmd.Flags().StringVar(&flagRules, "rules", "", "YAML or JSON file defining product-class targets (name, include, exclude, all, fields, fold); - reads the rules from stdin")
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, "Read the rules YAML/JSON from stdin (same as --rules -)")
	cmd.Flags().StringVar(&flagControl, "control", "", "File of listing ids expected to be visible (one per line)")
	cmd.Flags().Int64Var(&flagSnapshot, "snapshot", 0, mktSnapshotUsage)
	mktHideZeroDefault(cmd, "snapshot")
	return cmd
}

func mktGapsReport(snap *mktSnapshot, rows []mktRow, targets []*mktTarget, control []string, hasControl bool) map[string]any {
	uniq := mktBestRows(rows)
	pages, nTerms := snap.Pages, len(snap.Terms)
	res := make([]mktGapTarget, 0, len(targets))
	for _, t := range targets {
		n := 0
		shops := map[int64]bool{}
		for _, r := range uniq {
			if t.matchesRow(r) {
				n++
				if r.ShopID.Valid {
					shops[r.ShopID.Int64] = true
				}
			}
		}
		res = append(res, mktGapTarget{Name: t.Name, MatchesAll: len(t.Include) == 0 && len(t.All) == 0, MatchingListings: n, DistinctShops: len(shops), Statement: mktGapStatement(n, len(shops), pages, nTerms)})
	}
	out := map[string]any{"snapshot_id": snap.ID, "pages": pages, "terms": nTerms, "results": res}
	if advisories := mktMatchesAllNotes(targets); len(advisories) > 0 {
		out["row_notes"] = advisories
	}
	if snap.Status != "complete" {
		out["snapshot_note"] = "snapshot is " + snap.Status + "; absence claims cover only the pages actually fetched"
	}
	if hasControl {
		seen := map[int64]bool{}
		for _, r := range uniq {
			seen[r.ListingID] = true
		}
		hit := 0
		for _, id := range control {
			if n, err := strconv.ParseInt(id, 10, 64); err == nil && seen[n] {
				hit++
			}
		}
		reached := 0.0
		if len(control) > 0 {
			reached = float64(hit) / float64(len(control))
		}
		out["control_reached"] = mktRound(reached, 4)
		out["control_ids"] = len(control)
		out["control_seen"] = hit
		if reached < 1.0 {
			out["gap_claims_downgraded"] = true
			out["note"] = fmt.Sprintf("only %d of %d control listings were visible in this sample, so a 'no listing found' result may reflect sampling limits rather than a real gap", hit, len(control))
		} else {
			out["gap_claims_downgraded"] = false
		}
	}
	return out
}
