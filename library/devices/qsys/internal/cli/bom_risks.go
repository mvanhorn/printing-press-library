// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type bomRisksResult struct {
	QDSVersion      string         `json:"qds_version,omitempty"`
	Models          []string       `json:"models"`
	Risks           []supportRef   `json:"risks"`
	CountsByCat     map[string]int `json:"risk_counts_by_category"`
	ModelsWithRisks []string       `json:"models_with_risks"`
	ModelsClear     []string       `json:"models_with_no_risks"`
	Scanned         int            `json:"scanned_articles"`
	Note            string         `json:"note,omitempty"`
}

func newNovelBomRisksCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		qds    string
		limit  int
		full   bool
	)
	cmd := &cobra.Command{
		Use:   "risks [models...]",
		Short: "Surface every known issue, awareness note, and troubleshooting article that touches any model on an equipment list, filtered to a Designer release.",
		Long: strings.Trim(`
Bom risks is the "what will bite us" pass over an equipment list.

For every model on the list it pulls the support articles in the four
categories that describe something going wrong - known-issues, awareness,
troubleshooting, and errorstatus-messages - and returns them deduped, so an
article covering three models on your list appears once with all three named.
The vendor knowledge base can only be browsed one category at a time and
indexes nothing by model, so today this is a manual search per part.

Where bom verify answers "will it run", bom risks answers "what is already
known to go wrong with it".

--qds narrows the result to one Designer release line: 10.0 also keeps articles
naming 10.0.2, and articles naming a different line are dropped. Articles that
name no version are always kept, because most troubleshooting content is
release-agnostic and dropping it would hide the majority of real risk. Omit
--qds to skip release filtering entirely.

Model matching is a local heuristic: an article "touches" a model when it names
it as a standalone token. An empty result means nothing matched, never that
nothing exists.

Accepts models as arguments or newline-separated on stdin, so a BOM export can
be piped straight in.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli bom risks CX-Q TSC-70-G3 --qds 10.0 --agent
  cat bom.txt | qsys-pp-cli bom risks --qds 10.0
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "models=CX-Q TSC-70-G3",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "bom risks")
			}
			models := readModels(cmd, args)
			if len(models) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one model is required, as arguments or on stdin"))
			}
			if limit <= 0 {
				limit = 50
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			res := bomRisksResult{
				QDSVersion:      strings.TrimSpace(qds),
				Models:          models,
				Risks:           make([]supportRef, 0, limit),
				CountsByCat:     map[string]int{},
				ModelsWithRisks: make([]string, 0, len(models)),
				ModelsClear:     make([]string, 0, len(models)),
			}

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				res.ModelsClear = append(res.ModelsClear, models...)
				res.Note = "no local corpus; run `qsys-pp-cli harvest --only support`"
				return finishBomRisks(cmd, flags, res)
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			stored, err := supportHarvested(ctx, db)
			if err != nil {
				return err
			}
			if stored == 0 {
				res.ModelsClear = append(res.ModelsClear, models...)
				res.Note = supportHarvestHint
				return finishBomRisks(cmd, flags, res)
			}

			articles, err := loadSupportArticles(ctx, db, riskCategories)
			if err != nil {
				return err
			}
			res.Scanned = len(articles)

			hit := map[string]bool{}
			for _, a := range articles {
				text := a.Title + " " + a.Body
				lower := strings.ToLower(text)
				matched := make([]string, 0, len(models))
				for _, m := range models {
					if mentionsModel(lower, m) {
						matched = append(matched, m)
					}
				}
				if len(matched) == 0 {
					continue
				}
				versions := articleVersions(text)
				if !versionRelevant(res.QDSVersion, versions) {
					continue
				}
				sort.Strings(matched)
				for _, m := range matched {
					hit[m] = true
				}
				res.CountsByCat[a.Category]++
				res.Risks = append(res.Risks, supportRef{
					Title:            a.Title,
					Category:         a.Category,
					URL:              a.URL,
					Models:           matched,
					DesignerVersions: versions,
					Excerpt:          excerpt(a.Body, excerptLen(full)),
				})
			}
			sort.SliceStable(res.Risks, func(i, j int) bool {
				if res.Risks[i].Category != res.Risks[j].Category {
					return riskCategoryOrder(res.Risks[i].Category) < riskCategoryOrder(res.Risks[j].Category)
				}
				return res.Risks[i].Title < res.Risks[j].Title
			})
			if len(res.Risks) > limit {
				res.Risks = res.Risks[:limit]
				res.Note = fmt.Sprintf("truncated to %d articles; raise --limit to see the rest", limit)
			}
			for _, m := range models {
				if hit[m] {
					res.ModelsWithRisks = append(res.ModelsWithRisks, m)
				} else {
					res.ModelsClear = append(res.ModelsClear, m)
				}
			}
			if len(res.Risks) == 0 && res.Note == "" {
				res.Note = fmt.Sprintf("no known-issue, awareness, troubleshooting, or error/status article names any of these models (%d articles scanned); that is not a guarantee they are risk-free", res.Scanned)
			}
			return finishBomRisks(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().StringVar(&qds, "qds", "", "Limit to articles about this Q-SYS Designer release line, e.g. 10.0 (default: no release filter)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum articles to return")
	cmd.Flags().BoolVar(&full, "full", false, "Return untruncated article text")
	return cmd
}

// riskCategoryOrder sorts the most actionable category first: a known issue in
// the release you are about to ship on outranks a general troubleshooting page.
func riskCategoryOrder(category string) int {
	for i, c := range riskCategories {
		if c == category {
			return i
		}
	}
	return len(riskCategories)
}

func finishBomRisks(cmd *cobra.Command, flags *rootFlags, res bomRisksResult) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), res, flags)
	}
	w := cmd.OutOrStdout()
	header := fmt.Sprintf("%d article(s) across %d model(s)", len(res.Risks), len(res.Models))
	if res.QDSVersion != "" {
		header += " for Q-SYS Designer " + res.QDSVersion
	}
	fmt.Fprintln(w, header)
	for _, r := range res.Risks {
		fmt.Fprintf(w, "\n[%s] %s\n%s\nmodels: %s\n", r.Category, r.Title, r.URL, strings.Join(r.Models, ", "))
	}
	if len(res.ModelsClear) > 0 {
		fmt.Fprintf(w, "\nno articles found for: %s\n", strings.Join(res.ModelsClear, ", "))
	}
	if res.Note != "" {
		fmt.Fprintf(w, "note: %s\n", res.Note)
	}
	return nil
}
