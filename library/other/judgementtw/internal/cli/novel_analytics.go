// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/extract"
	"judgementtw-pp-cli/internal/judicial"
	"judgementtw-pp-cli/internal/source/fjud"
)

// newSentencesCmd builds 'sentences --statute X [--court Y] [--year Z]' — the
// sentencing-distribution analytics novel feature.
func newSentencesCmd(flags *rootFlags) *cobra.Command {
	var statute, court string
	var year int
	cmd := &cobra.Command{
		Use:   "sentences",
		Short: "Sentencing distribution for a statute (synced criminal judgments only)",
		Long: `Aggregate the 主文 (verdict) sentence patterns parsed at sync-time for every
locally-synced judgment that cites the given statute. Returns a count, min /
median / max prison-months, fine breakdown, and a histogram.`,
		Example: `  judgementtw-pp-cli sentences --statute 毒品危害防制條例 --court TPH --json
  judgementtw-pp-cli sentences --statute 刑法 --year 115 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if statute == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			stats, err := judicial.AggregateSentences(ctx, db, statute, court, year)
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), stats, flags)
		},
	}
	cmd.Flags().StringVar(&statute, "statute", "", "Statute name (e.g. 毒品危害防制條例) — REQUIRED")
	cmd.Flags().StringVar(&court, "court", "", "Filter by court code")
	cmd.Flags().IntVar(&year, "year", 0, "Filter by ROC year")
	return cmd
}

// newAppealChainCmd builds 'appeal-chain <jid>' — the appeal-chain walker
// novel feature.
func newAppealChainCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "appeal-chain [jid]",
		Short: "Walk the appeal chain (lower → appellate → supreme) for the same matter",
		Long: `Given a JID, find every locally-synced judgment that shares the same case-character
and adjacent year. Sorts by court hierarchy rank so the chain reads
district → high → supreme.

Note: appellate matching uses (case-character, year ±1) — Taiwan's case numbering
restarts per court. Truly precise matching requires party-name extraction and is
left to future work.`,
		Example:     `  judgementtw-pp-cli appeal-chain TPHM,110,毒抗,1212,20210831,1 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			parsed, err := extract.Parse(args[0])
			if err != nil {
				return usageErr(err)
			}
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.QueryContext(ctx,
				`SELECT id FROM judgments WHERE id LIKE ? OR id LIKE ?`,
				"%,"+intToStr(parsed.Year-1)+","+parsed.CaseChar+",%",
				"%,"+intToStr(parsed.Year)+","+parsed.CaseChar+",%")
			if err != nil {
				return err
			}
			defer rows.Close()
			var ids []string
			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err != nil {
					return err
				}
				ids = append(ids, s)
			}
			// Sort by court-hierarchy rank ASC, then JDate ASC.
			sort.Slice(ids, func(i, j int) bool {
				pi, _ := extract.Parse(ids[i])
				pj, _ := extract.Parse(ids[j])
				if pi == nil || pj == nil {
					return ids[i] < ids[j]
				}
				ri, rj := extract.CourtHierarchyRank(pi.Court), extract.CourtHierarchyRank(pj.Court)
				if ri != rj {
					return ri < rj
				}
				return pi.JDate < pj.JDate
			})
			out := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				p, _ := extract.Parse(id)
				if p == nil {
					continue
				}
				out = append(out, map[string]any{
					"jid":        id,
					"court":      p.Court,
					"court_name": fjud.CourtName(p.Court),
					"case_type":  p.CaseType,
					"case_char":  p.CaseChar,
					"year":       p.Year,
					"no":         p.No,
					"jdate":      p.JDate,
					"hierarchy":  extract.CourtHierarchyRank(p.Court),
				})
			}
			return emitJSON(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// newRelatedCmd builds 'related <jid>' — Jaccard similarity over citations.
func newRelatedCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var limit int
	cmd := &cobra.Command{
		Use:   "related [jid]",
		Short: "Find synced judgments with overlapping statute citations",
		Long: `Computes Jaccard similarity over the citation set of <jid> against every other
synced judgment. Filters to the same court tier and ±2 years; returns the top-N
matches above --threshold.`,
		Example:     `  judgementtw-pp-cli related TPSM,115,台抗,703,20260430,1 --threshold 0.3 --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			parsed, err := extract.Parse(args[0])
			if err != nil {
				return usageErr(err)
			}
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			seedCites, err := judicial.CitationsOf(ctx, db, args[0])
			if err != nil {
				return err
			}
			if len(seedCites) == 0 {
				return emitJSON(cmd.OutOrStdout(), []any{}, flags)
			}
			seedSet := make(map[string]struct{}, len(seedCites))
			for _, c := range seedCites {
				seedSet[c.Statute+"#"+intToStr(c.Article)] = struct{}{}
			}
			ids, err := listJudgmentIDs(ctx, db, "", "", 0)
			if err != nil {
				return err
			}
			seedTier := extract.CourtHierarchyRank(parsed.Court)
			type scored struct {
				JID   string  `json:"jid"`
				Score float64 `json:"score"`
				Court string  `json:"court"`
				Year  int     `json:"year"`
			}
			var out []scored
			for _, other := range ids {
				if other == args[0] {
					continue
				}
				op, err := extract.Parse(other)
				if err != nil {
					continue
				}
				if extract.CourtHierarchyRank(op.Court) != seedTier {
					continue
				}
				if abs(op.Year-parsed.Year) > 2 {
					continue
				}
				cs, _ := judicial.CitationsOf(ctx, db, other)
				if len(cs) == 0 {
					continue
				}
				inter := 0
				union := len(seedSet)
				for _, c := range cs {
					k := c.Statute + "#" + intToStr(c.Article)
					if _, ok := seedSet[k]; ok {
						inter++
					} else {
						union++
					}
				}
				if union == 0 {
					continue
				}
				j := float64(inter) / float64(union)
				if j < threshold {
					continue
				}
				out = append(out, scored{
					JID:   other,
					Score: j,
					Court: op.Court,
					Year:  op.Year,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return emitJSON(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 0.2, "Minimum Jaccard similarity (0..1)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

// newCaseTypesCmd builds 'case-types list' (per-court 字別 catalog) and
// 'case-types courts' (all 41 courts).
func newCaseTypesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "case-types",
		Short: "Enumerate Taiwan court taxonomies (字別 case-characters and 41 courts)",
	}
	cmd.AddCommand(newCaseTypesListCmd(flags))
	cmd.AddCommand(newCaseTypesCourtsCmd(flags))
	return cmd
}

func newCaseTypesListCmd(flags *rootFlags) *cobra.Command {
	var court string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List 字別 case-characters across the local corpus, grouped by court",
		Long: `Aggregates the 案件字別 column across every locally-synced judgment, grouped
by court. Use --court to narrow to a single court. The result includes one
sample JID per (court, case-character) pair so agents can pick a representative
case to inspect.`,
		Example:     `  judgementtw-pp-cli case-types list --court TPH --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			ids, err := listJudgmentIDs(ctx, db, court, "", 0)
			if err != nil {
				return err
			}
			type bucket struct {
				Court    string `json:"court"`
				CaseChar string `json:"case_char"`
				Count    int    `json:"count"`
				Sample   string `json:"sample_jid"`
			}
			by := make(map[string]*bucket)
			for _, id := range ids {
				p, err := extract.Parse(id)
				if err != nil {
					continue
				}
				k := p.Court + "/" + p.CaseChar
				if _, ok := by[k]; !ok {
					by[k] = &bucket{Court: p.Court, CaseChar: p.CaseChar, Sample: id}
				}
				by[k].Count++
			}
			out := make([]bucket, 0, len(by))
			for _, b := range by {
				out = append(out, *b)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Court == out[j].Court {
					return out[i].Count > out[j].Count
				}
				return out[i].Court < out[j].Court
			})
			return emitJSON(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&court, "court", "", "Restrict to a single court code")
	return cmd
}

func newCaseTypesCourtsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "courts",
		Short:       "List all 41 Taiwan courts with codes and Chinese names",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return emitJSON(cmd.OutOrStdout(), fjud.Courts, flags)
		},
	}
	return cmd
}

// newKnowledgeLinkCmd builds 'knowledge link <par>' — cross-source bridge.
// Registered as a subcommand of `knowledge` in judicial_knowledge.go.
func newKnowledgeLinkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link [par]",
		Short: "Find synced judgments that cite the same statutes as a knowledge-base commentary",
		Long: `Fetch a Knowledge Base case commentary by its par-token, extract every statute
reference it makes, then return every locally-synced judgment whose own citation
set overlaps. The only meaningful join across the two judicial.gov.tw sub-sites.`,
		Example:     `  judgementtw-pp-cli knowledge link H4sF6HdN%2fbyjjMYJ42ZPATLh%2fu2Al%2f83pT2w0OTOytP6IvrcKVCjLQ%3d%3d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			c := fjudkmClient(flags)
			doc, err := c.Get(ctx, args[0])
			if err != nil {
				return err
			}
			cites := extract.ExtractCitations(doc.Body)
			if len(cites) == 0 {
				return emitJSON(cmd.OutOrStdout(), map[string]any{
					"par":         args[0],
					"title":       doc.Title,
					"citations":   []any{},
					"linked_jids": []any{},
				}, flags)
			}
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			seen := make(map[string]struct{})
			var linked []string
			for _, ct := range cites {
				rows, err := db.QueryContext(ctx,
					`SELECT jid FROM citations WHERE statute = ?`, ct.Statute)
				if err != nil {
					return err
				}
				for rows.Next() {
					var j string
					if err := rows.Scan(&j); err != nil {
						rows.Close()
						return err
					}
					if _, dup := seen[j]; !dup {
						seen[j] = struct{}{}
						linked = append(linked, j)
					}
				}
				rows.Close()
			}
			payload := map[string]any{
				"par":         args[0],
				"title":       doc.Title,
				"citations":   cites,
				"linked_jids": linked,
			}
			return emitJSON(cmd.OutOrStdout(), payload, flags)
		},
	}
	return cmd
}

// intToStr is a tiny helper so call-sites stay free of strconv noise.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// avoid an "imported and not used" error when callers move to constants.
var _ = strings.HasPrefix
