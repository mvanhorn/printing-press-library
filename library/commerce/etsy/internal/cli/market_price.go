// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type mktPriceGroup struct {
	MatchesAll bool     `json:"matches_all,omitempty"`
	Group      string   `json:"group"`
	N          int      `json:"n"`
	Min        float64  `json:"min"`
	Median     float64  `json:"median"`
	Max        float64  `json:"max"`
	Currency   string   `json:"currency,omitempty"`
	Rho        *float64 `json:"spearman_rho"`
	RhoPairs   int      `json:"rho_pairs"`
	Note       string   `json:"note,omitempty"`
}

// mktPriceGroups groups listings and computes price stats plus Spearman rho
// between price and demand. rho is null (with a note) below minGroup or when
// demand is unavailable/constant.
func mktPriceGroups(uniq []mktRow, dem map[int64]mktListingDemand, groupOf func(mktRow) []string, minGroup int) []mktPriceGroup {
	type bucket struct{ group, currency string }
	members := map[bucket][]mktRow{}
	for _, r := range uniq {
		if !r.Price.Valid {
			continue
		}
		for _, g := range groupOf(r) {
			key := bucket{g, strings.ToUpper(r.Currency)}
			members[key] = append(members[key], r)
		}
	}
	out := make([]mktPriceGroup, 0, len(members))
	for g, rs := range members {
		prices := make([]float64, 0, len(rs))
		var px, dy []float64
		for _, r := range rs {
			prices = append(prices, r.Price.Float64)
			if d, ok := dem[r.ListingID]; ok && d.Collected {
				px = append(px, r.Price.Float64)
				dy = append(dy, float64(d.Reviews))
			}
		}
		sorted := append([]float64(nil), prices...)
		sort.Float64s(sorted)
		pg := mktPriceGroup{Group: g.group, N: len(rs), Min: sorted[0], Median: mktRound(mktMedian(prices), 2), Max: sorted[len(sorted)-1], Currency: g.currency, RhoPairs: len(px)}
		switch {
		case len(rs) < minGroup:
			pg.Note = fmt.Sprintf("group has %s, fewer than --min-group %d; correlation not computed", mktCount(len(rs), "listing"), minGroup)
		case len(px) < minGroup:
			if len(px) == 0 {
				pg.Note = "no listings have review data; correlation not computed (absent demand is not zero)"
			} else {
				verb := "have"
				if len(px) == 1 {
					verb = "has"
				}
				pg.Note = fmt.Sprintf("only %s %s review data, fewer than --min-group %d; correlation not computed (absent demand is not zero)", mktCount(len(px), "listing"), verb, minGroup)
			}
		default:
			if rho, ok := mktSpearman(px, dy); ok {
				v := mktRound(rho, 4)
				pg.Rho = &v
			} else {
				pg.Note = "price or demand is constant in this group, so correlation is undefined"
			}
		}
		out = append(out, pg)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Currency < out[j].Currency
	})
	return out
}

func newNovelMarketPriceCmd(flags *rootFlags) *cobra.Command {
	// etsy-ext: inline rules remain exposed over MCP.
	var flagGroupBy, flagRules, flagRulesJSON, flagWindow, flagCurrency string
	var flagMinGroup int
	var flagSnapshot int64

	cmd := &cobra.Command{
		Use:   "price",
		Short: "Summarize price spread per category or product class and how price relates to demand.",
		// etsy-ext: document inline alternatives for MCP callers.
		Long: `Summarize price spread per category or product class and how price relates to demand.

Groups listings by taxonomy id or by the product classes in a --rules file and reports count, min,
median and max price, plus the Spearman rank correlation between price and recent review volume.
Rules allow at most 256 include/exclude/all patterns total, each at most 512 bytes.

Rules accept all: ["linen", "apron"] to require BOTH patterns (AND), alongside any include and no exclude.
fields defaults to [title, tags]; [title] matches only the title (YAML scalar title is accepted).
Empty, duplicate, unknown fields and [tags] alone are usage errors. fold: true lowercases text and
collapses runs outside Unicode letters, digits and % to one space; patterns are not folded.
Literal punctuation removed by fold is rejected with the removed characters identified. fold defaults false.
Targets with neither include nor all patterns match every listing not excluded; matches_all and notes flag this.
JSON rejects unknown keys at every level. Rules files accept YAML or JSON.
Inline --rules-json accepts the same targets as JSON only (maximum 96 KiB) with --group-by class.
Groups smaller than --min-group get a null correlation and a note, never a guess.`,
		// etsy-ext: demonstrate inline content without filesystem input.
		Example: "  etsy-pp-cli market price --group-by class --rules-json '{\"targets\":[{\"name\":\"linen apron\",\"all\":[\"linen\",\"apron\"]}]}'\n  etsy-pp-cli market price --group-by taxonomy --min-group 15\n  etsy-pp-cli market price --group-by class --rules classes.yaml",
		// etsy-ext: declare filesystem sinks for the MCP walker.
		Annotations: map[string]string{"mcp:read-flags": "rules", "mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mktValidateSnapshotFlag(cmd, flagSnapshot); err != nil {
				return err
			}
			// etsy-ext: validate inline rules before dry run or store access.
			inlineRules := cmd.Flags().Changed("rules-json")
			if inlineRules && cmd.Flags().Changed("rules") {
				return usageErr(fmt.Errorf("--rules-json conflicts with --rules"))
			}
			if inlineRules && flagGroupBy != "class" {
				return usageErr(fmt.Errorf("--rules-json applies only with --group-by class"))
			}
			var targets []*mktTarget
			if inlineRules {
				var err error
				targets, err = mktInlineRules(flagRulesJSON)
				if err != nil {
					return usageErr(err)
				}
			}
			if dryRunOK(flags) {
				return mktDryRun(cmd, flags, "market price")
			}
			if flagGroupBy != "taxonomy" && flagGroupBy != "class" {
				return usageErr(fmt.Errorf("--group-by must be taxonomy or class"))
			}
			if flagGroupBy == "class" {
				if flagRules == "" && !inlineRules {
					return usageErr(fmt.Errorf("--group-by class needs --rules FILE or --rules-json"))
				}
				var err error
				if !inlineRules {
					targets, err = mktLoadRules(flagRules, cmd.InOrStdin())
				}
				if err != nil {
					return usageErr(fmt.Errorf("rules: %w", err))
				}
			}
			window, err := mktParseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("--window: %w", err))
			}
			if flagMinGroup < 2 {
				return usageErr(fmt.Errorf("--min-group must be at least 2"))
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
			dem, err := mktLoadDemand(db.DB(), snap, window)
			if err != nil {
				return err
			}
			groupOf := func(r mktRow) []string {
				if flagGroupBy == "taxonomy" {
					if !r.TaxonomyID.Valid {
						return []string{"unknown"}
					}
					return []string{fmt.Sprint(r.TaxonomyID.Int64)}
				}
				out := make([]string, 0)
				for _, t := range targets {
					if t.matchesRow(r) {
						out = append(out, t.Name)
					}
				}
				return out
			}
			uniq := mktBestRows(rows)
			excluded := 0
			if flagCurrency != "" {
				filtered := make([]mktRow, 0, len(uniq))
				for _, r := range uniq {
					if strings.EqualFold(r.Currency, flagCurrency) {
						filtered = append(filtered, r)
					} else {
						excluded++
					}
				}
				uniq = filtered
			}
			groups := mktPriceGroups(uniq, dem, groupOf, flagMinGroup)
			for i := range groups {
				for _, target := range targets {
					if groups[i].Group == target.Name && len(target.Include) == 0 && len(target.All) == 0 {
						groups[i].MatchesAll = true
					}
				}
			}
			result := map[string]any{
				"excluded_other_currency": excluded,
				"snapshot_id":             snap.ID, "group_by": flagGroupBy, "min_group": flagMinGroup, "results": groups,
			}
			has, err := mktHasReviewData(db.DB(), snap)
			if err != nil {
				return err
			}
			if !has {
				alt, err := mktOlderReviewSnapshot(db.DB(), snap)
				if err != nil {
					return err
				}
				result["note"] = "no review data collected; price-demand correlation not computed — re-run market snapshot with --with-reviews"
				if alt != nil {
					result["note"] = "no review data collected; price-demand correlation not computed — " + mktReviewSnapshotHint(alt.ID)
				}
			}
			keys, texts := []string{}, []string{}
			for _, group := range groups {
				keys = append(keys, group.Group)
				texts = append(texts, group.Note)
			}
			result["row_notes"] = append(mktRowNotes(keys, texts), mktMatchesAllNotes(targets)...)
			return mktEmitReport(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "taxonomy", "Group by taxonomy or class (class needs --rules or --rules-json)")
	cmd.Flags().StringVar(&flagCurrency, "currency", "", "Keep only this currency code (case-insensitive)")
	// etsy-ext: inline content is not a filesystem sink.
	cmd.Flags().StringVar(&flagRulesJSON, "rules-json", "", "Inline JSON targets (maximum 96 KiB; requires --group-by class)")
	cmd.Flags().StringVar(&flagRules, "rules", "", "YAML or JSON classes file (same format as market gaps)")
	cmd.Flags().IntVar(&flagMinGroup, "min-group", 15, "Minimum listings per group before a correlation is computed")
	cmd.Flags().StringVar(&flagWindow, "window", "12mo", "Review window used for demand")
	cmd.Flags().Int64Var(&flagSnapshot, "snapshot", 0, mktSnapshotUsage)
	mktHideZeroDefault(cmd, "snapshot")
	return cmd
}
