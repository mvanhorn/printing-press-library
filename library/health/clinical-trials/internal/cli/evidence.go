// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// evidence.go implements the `evidence` command: link a trial to its
// publications and citation context. It searches PubMed (for indexed papers
// referencing the NCT id) and OpenAlex (for works and their citation counts),
// degrading gracefully if either source is unavailable so a partial answer still
// returns rather than failing the whole command.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/source"

	"github.com/spf13/cobra"
)

// isRateLimit reports whether err is (or wraps) an upstream 429 throttle, which
// must surface to the caller rather than being swallowed as a partial result.
func isRateLimit(err error) bool {
	var rl *cliutil.RateLimitError
	return errors.As(err, &rl)
}

type evidencePub struct {
	PMID  string `json:"pmid,omitempty"`
	Title string `json:"title,omitempty"`
	Year  string `json:"year,omitempty"`
	URL   string `json:"url,omitempty"`
}

type evidenceWork struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Year    int    `json:"year,omitempty"`
	CitedBy int    `json:"cited_by_count"`
	DOI     string `json:"doi,omitempty"`
}

type evidenceView struct {
	Query         string         `json:"query"`
	TrialTitle    string         `json:"trial_title,omitempty"`
	PubMedCount   int            `json:"pubmed_count"`
	Publications  []evidencePub  `json:"publications,omitempty"`
	OpenAlexCount int            `json:"openalex_count"`
	Works         []evidenceWork `json:"works,omitempty"`
	TotalCitedBy  int            `json:"total_cited_by"`
	Note          string         `json:"note,omitempty"`
	Errors        []string       `json:"errors,omitempty"`
}

// pp:data-source live
func newNovelEvidenceCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "evidence <nct-id-or-term>",
		Short: "Link a trial to its publications and citation context via PubMed and OpenAlex.",
		Long: "Search PubMed and OpenAlex for publications referencing a trial (by NCT id, or\n" +
			"any free-text term) and report the matched papers plus their citation context.\n" +
			"If one source is unavailable the other's results still return.",
		Example: "  clinical-trials-pp-cli evidence NCT04280705 --json\n" +
			"  clinical-trials-pp-cli evidence NCT07011732 --limit 15",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would link a trial to its publications via PubMed and OpenAlex")
				return nil
			}
			term := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			src := flags.sourceClient()

			view := evidenceView{Query: term}

			// Best-effort trial title for context when the term is an NCT id.
			if looksLikeNCT(term) {
				if c, err := flags.newClient(); err == nil {
					if t, ok, err := ctgovFetchOne(ctx, c, term); err == nil && ok {
						view.TrialTitle = t.Title
					}
				}
			}

			pm, err := source.SearchPubMed(ctx, src, term, limit, ncbiKey())
			if err != nil {
				// A throttle must surface; other PubMed failures degrade to a
				// partial answer with OpenAlex still attempted.
				if isRateLimit(err) {
					return classifyAPIError(err, flags)
				}
				view.Errors = append(view.Errors, "pubmed: "+truncate(err.Error(), 120))
			} else {
				view.PubMedCount = pm.Count
				for _, p := range pm.Items {
					view.Publications = append(view.Publications, evidencePub{PMID: p.PMID, Title: p.Title, Year: p.Year, URL: p.URL})
				}
			}

			oa, err := source.SearchOpenAlex(ctx, src, term, limit, openAlexMailto())
			if err != nil {
				if isRateLimit(err) {
					return classifyAPIError(err, flags)
				}
				view.Errors = append(view.Errors, "openalex: "+truncate(err.Error(), 120))
			} else {
				view.OpenAlexCount = oa.Count
				for _, w := range oa.Items {
					view.Works = append(view.Works, evidenceWork{ID: w.ID, Title: w.Title, Year: w.Year, CitedBy: w.CitedBy, DOI: w.DOI})
					view.TotalCitedBy += w.CitedBy
				}
			}

			if len(view.Publications) == 0 && len(view.Works) == 0 && len(view.Errors) == 0 {
				view.Note = "no publications found referencing this trial yet"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderEvidenceHuman(cmd, view); err != nil {
				return err
			}
			// grep-style: zero publications and zero works with no source errors
			// is an empty result, so exit non-zero. A source error is a degraded
			// run, not an empty one, so it does not flip the exit here.
			if len(view.Publications) == 0 && len(view.Works) == 0 && len(view.Errors) == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results per source")
	return cmd
}

// looksLikeNCT reports whether s is shaped like a ClinicalTrials.gov id.
func looksLikeNCT(s string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "NCT")
}

func renderEvidenceHuman(cmd *cobra.Command, v evidenceView) error {
	w := cmd.OutOrStdout()
	header := v.Query
	if v.TrialTitle != "" {
		header = fmt.Sprintf("%s — %s", v.Query, truncate(v.TrialTitle, 70))
	}
	fmt.Fprintf(w, "Evidence — %s\n", header)
	fmt.Fprintf(w, "  PubMed: %d matches   OpenAlex: %d works   total citations: %d\n\n", v.PubMedCount, v.OpenAlexCount, v.TotalCitedBy)
	if len(v.Publications) > 0 {
		fmt.Fprintln(w, "Publications (PubMed):")
		for _, p := range v.Publications {
			fmt.Fprintf(w, "  [%s] %s (%s)\n      %s\n", p.PMID, truncate(p.Title, 70), p.Year, p.URL)
		}
		fmt.Fprintln(w)
	}
	if len(v.Works) > 0 {
		fmt.Fprintln(w, "Works (OpenAlex, by citations):")
		for _, work := range v.Works {
			fmt.Fprintf(w, "  %-60s %dy  cited %d\n", truncate(work.Title, 60), work.Year, work.CitedBy)
		}
	}
	for _, e := range v.Errors {
		fmt.Fprintf(w, "\nwarning: %s\n", e)
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}
