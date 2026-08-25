// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type faultResult struct {
	Query    string       `json:"query"`
	MatchKey string       `json:"match_key"`
	Matches  []supportRef `json:"matches"`
	Scanned  int          `json:"scanned_articles"`
	Note     string       `json:"note,omitempty"`
}

func newNovelFaultCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
		full   bool
	)
	cmd := &cobra.Command{
		Use:   "fault <string>",
		Short: "Paste the literal fault or status string Q-SYS Designer displays and get the article that explains it, plus the models it applies to.",
		Long: strings.Trim(`
Fault takes a status string exactly as Q-SYS Designer prints it and returns the
support article that explains it.

QSC titles those articles with the literal string, then slugifies it. Designer
shows

  Fault - LAN A Streaming Error - Not Connected

and the article lives at

  support.qsys.com/en_US/errorstatus-messages/error-fault-lan-a-streaming-error-not-connected

Case, spacing, separators, and the "error-"/"fault-" classification prefixes all
differ, so a full-text search on the pasted string finds the wrong article or
nothing. This command folds both sides to the same key first: lowercase
alphanumerics joined by single hyphens, with the classification prefixes
stripped. Paste the string with or without its "Fault -" prefix; both match.

Only errorstatus-messages and troubleshooting are searched, and matching is
strict on purpose. An unrecognized string comes back empty with a note rather
than dragging in a plausible-looking article, and exits 3 so a script can tell
"no such fault" from "here is your answer".

Models are resolved locally by scanning the article for product names, so the
list is evidence of what the article mentions, not a vendor applicability
statement.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli fault "LAN A Streaming Error - Not Connected" --agent
  qsys-pp-cli fault "Fault - LAN A Streaming Error - Not Connected" --full
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "string=LAN A Streaming Error - Not Connected",
			"pp:typed-exit-codes":    "0,3",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fault")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a fault or status string is required, e.g. \"LAN A Streaming Error - Not Connected\""))
			}
			if limit <= 0 {
				limit = 5
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			res := faultResult{Query: query, MatchKey: faultKey(query), Matches: make([]supportRef, 0, limit)}

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				res.Note = "no local corpus; run `qsys-pp-cli harvest --only support`"
				return finishFault(cmd, flags, res, false)
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
				res.Note = supportHarvestHint
				return finishFault(cmd, flags, res, false)
			}

			articles, err := loadSupportArticles(ctx, db, faultCategories)
			if err != nil {
				return err
			}
			res.Scanned = len(articles)

			type scored struct {
				row  supportRow
				rank int
			}
			hits := make([]scored, 0, 8)
			for _, a := range articles {
				if rank := faultRank(res.MatchKey, a.Slug, a.Title); rank >= 0 {
					hits = append(hits, scored{row: a, rank: rank})
				}
			}
			sort.SliceStable(hits, func(i, j int) bool { return hits[i].rank > hits[j].rank })
			if len(hits) > limit {
				hits = hits[:limit]
			}

			models, err := productModels(ctx, db)
			if err != nil {
				return err
			}
			for _, h := range hits {
				ref := supportRef{
					Title:            h.row.Title,
					Category:         h.row.Category,
					URL:              h.row.URL,
					Models:           modelsMentioned(models, h.row.Title+" "+h.row.Body),
					DesignerVersions: articleVersions(h.row.Title + " " + h.row.Body),
				}
				ref.Excerpt = excerpt(h.row.Body, excerptLen(full))
				res.Matches = append(res.Matches, ref)
			}

			if len(res.Matches) == 0 {
				res.Note = fmt.Sprintf("no error/status or troubleshooting article matches %q (searched %d articles); check the wording against `qsys-pp-cli support index`", query, res.Scanned)
				return finishFault(cmd, flags, res, true)
			}
			return finishFault(cmd, flags, res, false)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum matching articles to return")
	cmd.Flags().BoolVar(&full, "full", false, "Return the untruncated article text")
	return cmd
}

// finishFault emits the result in whichever format the caller asked for, then
// maps a graceful miss to exit 3. The payload is always written first so a
// machine caller gets parseable output on both paths.
func finishFault(cmd *cobra.Command, flags *rootFlags, res faultResult, notFound bool) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
			return err
		}
	} else {
		w := cmd.OutOrStdout()
		if len(res.Matches) > 0 {
			fmt.Fprintf(w, "%d article(s) match %q (key: %s)\n", len(res.Matches), res.Query, res.MatchKey)
			for _, m := range res.Matches {
				fmt.Fprintf(w, "\n%s  [%s]\n%s\n", m.Title, m.Category, m.URL)
				if len(m.Models) > 0 {
					fmt.Fprintf(w, "models:  %s\n", strings.Join(m.Models, ", "))
				}
				if len(m.DesignerVersions) > 0 {
					fmt.Fprintf(w, "designer: %s\n", strings.Join(m.DesignerVersions, ", "))
				}
				if m.Excerpt != "" {
					fmt.Fprintf(w, "\n%s\n", m.Excerpt)
				}
			}
		}
		if res.Note != "" {
			fmt.Fprintf(w, "note: %s\n", res.Note)
		}
	}
	if notFound {
		return notFoundErr(fmt.Errorf("no support article matches %q", res.Query))
	}
	return nil
}

func excerptLen(full bool) int {
	if full {
		return 0
	}
	return 900
}

// productModels returns the stored model names worth matching article text
// against, longest first so a SKU is preferred over the series it belongs to
// when both appear in one article.
//
// Two filters, both earning their place against the real corpus:
//
//   - is_product, the store's own has-a-spec-sheet signal, OR a row whose
//     model is the prefix of a real product's SKU. The second clause is what
//     keeps CX-Q: the series landing page publishes no PDF, so is_product is
//     0, yet "CX-Q" is exactly the name support articles use.
//   - a digit or a hyphen in the name. The vendor's family index pages parse
//     into pseudo-models (AMPLIFIERS, LOUDSPEAKERS, QIO, OTHER) that match
//     ordinary prose and would report a fault as applying to the catalogue.
func productModels(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(p.model, '') FROM qsys_products p
		WHERE (p.model GLOB '*[0-9]*' OR p.model LIKE '%-%')
		  AND (p.is_product = 1
		       OR EXISTS (SELECT 1 FROM qsys_products q
		                  WHERE q.is_product = 1 AND q.model LIKE p.model || '-%'))`)
	if err != nil {
		return nil, fmt.Errorf("reading product models: %w", err)
	}
	out := make([]string, 0, 256)
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning product model: %w", err)
		}
		if strings.TrimSpace(m) != "" {
			out = append(out, m)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating product models: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out, nil
}

// modelsMentioned returns the stored models an article names. Bounded so a
// page that lists the whole catalogue does not dominate the payload.
func modelsMentioned(models []string, text string) []string {
	const max = 12
	lower := strings.ToLower(text)
	out := make([]string, 0, max)
	for _, m := range models {
		if len(out) >= max {
			break
		}
		if mentionsModel(lower, m) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}
