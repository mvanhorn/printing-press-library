// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// distinctiveBiomarkerName reports whether a biomarker name is specific enough
// to match against free-text prose without colliding with ordinary English.
// Multi-word names, names with digits, abbreviations/mixed-case (ApoB, ALT,
// PSA), and longer names qualify; short all-lowercase common words (Lead, Iron,
// Zinc) do not — for those, note matching falls back to category only.
func distinctiveBiomarkerName(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	if strings.ContainsAny(q, " -0123456789") {
		return true
	}
	for i, r := range q {
		if i > 0 && r >= 'A' && r <= 'Z' {
			return true // internal uppercase: ApoB, hsCRP, or an all-caps abbreviation
		}
	}
	return len([]rune(q)) >= 5
}

// wholeWordContains reports whether query occurs in text as a whole word
// (case-insensitive). Used for matching a biomarker name in clinician-note
// prose without a short name like "Lead"/"Iron" matching "leading"/"iron-rich".
func wholeWordContains(text, query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return false
	}
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(q) + `\b`)
	if err != nil {
		return strings.Contains(strings.ToLower(text), strings.ToLower(q))
	}
	return re.MatchString(text)
}

// noteRecord mirrors one synced clinician-notes record: a per-requisition entry
// holding a notes[] array of {note, category}. (relevant_biomarkers is present
// in the API but observed null, so it is not relied upon for matching.)
type noteRecord struct {
	Date  string `json:"date"`
	Notes []struct {
		Note     string `json:"note"`
		Category struct {
			CategoryName string `json:"categoryName"`
		} `json:"category"`
	} `json:"notes"`
}

// parseNoteRecords decodes the stored notes payload, tolerating either a bare
// array of records or a {results|data: [...]} envelope.
func parseNoteRecords(raw []byte) []noteRecord {
	var arr []noteRecord
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	var wrapped struct {
		Results []noteRecord `json:"results"`
		Data    []noteRecord `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		if len(wrapped.Results) > 0 {
			return wrapped.Results
		}
		if len(wrapped.Data) > 0 {
			return wrapped.Data
		}
	}
	return nil
}

func newNovelBundleCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "bundle [biomarker]",
		Short:       "Markdown bundle for one biomarker: history + clinician notes + Function-optimal range + recommendations — ready to paste into Claude or ChatGPT",
		Long:        "Composes a single Markdown file pulling everything synced about one biomarker: every measurement across every round, every clinician note that mentions it (FTS5), Function's optimal range, and active recommendations.",
		Example:     "  function-health-pp-cli bundle ApoB\n  function-health-pp-cli bundle hs-CRP --window 3rounds > apoB.md",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			s, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer safeCloseStore(s)

			rows, err := loadAllResults(ctx, s)
			if err != nil {
				return err
			}
			matching := filterByBiomarker(rows, query)
			if window != "" {
				matching = filterByWindow(matching, window)
			}
			if len(matching) == 0 {
				return notFoundErr(fmt.Errorf("no synced results for biomarker %q", query))
			}
			latest := matching[len(matching)-1]

			// Clinician notes. The real shape is a list of per-requisition records,
			// each holding a notes[] array of {note, category}. There is no flat
			// body/biomarker field and no per-note biomarker link — match on the
			// biomarker's category or a name mention in the prose. Note text is
			// HTML-entity-encoded, so decode before rendering.
			noteRow := s.DB().QueryRowContext(ctx, `SELECT data FROM resources WHERE resource_type = 'notes' LIMIT 1`)
			var matchedNotes []string
			{
				var raw []byte
				if err := noteRow.Scan(&raw); err == nil {
					for _, nr := range parseNoteRecords(raw) {
						when := ""
						if nr.Date != "" {
							when = formatDrawDate(nr.Date) + " — "
						}
						for _, inner := range nr.Notes {
							text := strings.TrimSpace(inner.Note)
							if text == "" {
								continue
							}
							catName := inner.Category.CategoryName
							// Category is the reliable structured link (notes are
							// categorized the same way biomarkers are) and is always
							// honored. The prose match is a secondary signal, gated two
							// ways: whole-word only, and only for *distinctive* names —
							// a short common-word biomarker like "Lead"/"Iron" otherwise
							// matches the English verb ("lead to", "iron-rich") and the
							// homograph noise can't be removed by word boundaries alone.
							matchesCategory := latest.Category != "" && strings.EqualFold(catName, latest.Category)
							matchesProse := distinctiveBiomarkerName(query) &&
								(wholeWordContains(text, query) || wholeWordContains(catName, query))
							if matchesCategory || matchesProse {
								prefix := when
								if catName != "" {
									prefix += "[" + catName + "] "
								}
								matchedNotes = append(matchedNotes, prefix+html.UnescapeString(text))
							}
						}
					}
				}
			}

			// Recommendations. The real /recommendations payload is a single
			// envelope of category groups, each item targeting biomarkers by Quest
			// code. Match a recommendation to this biomarker by Quest-code overlap.
			questCodes := map[string]bool{}
			for _, r := range matching {
				if r.QuestCode != "" {
					questCodes[r.QuestCode] = true
				}
			}
			recRow := s.DB().QueryRowContext(ctx, `SELECT data FROM resources WHERE resource_type = 'recommendations' LIMIT 1`)
			var matchedRecs []string
			{
				var raw []byte
				if err := recRow.Scan(&raw); err == nil {
					var env recEnvelope
					if json.Unmarshal(raw, &env) == nil {
						for _, g := range env.Recommendations {
							label := g.DisplayName
							if label == "" {
								label = g.CategoryName
							}
							for _, it := range g.Items {
								for _, code := range it.Biomarkers {
									if questCodes[questCodeString(code)] {
										matchedRecs = append(matchedRecs, "**"+label+"** — "+it.Name)
										break
									}
								}
							}
						}
					}
				}
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "# %s — Function Health context bundle\n\n", latest.BiomarkerName)
			fmt.Fprintf(w, "**Category:** %s  •  **Unit:** %s\n", latest.Category, latest.Unit)
			fmt.Fprintf(w, "**Function-optimal range:** %.2f-%.2f  •  **Quest reference range:** %.2f-%.2f\n\n",
				latest.OptimalLow, latest.OptimalHigh, latest.QuestRangeLow, latest.QuestRangeHigh)

			fmt.Fprintf(w, "## History (%d draws, oldest → newest)\n\n", len(matching))
			fmt.Fprintln(w, "| Draw date | Value | Unit | Status |")
			fmt.Fprintln(w, "|---|---|---|---|")
			for _, r := range matching {
				fmt.Fprintf(w, "| %s | %.2f | %s | %s |\n", formatDrawDate(r.DrawDate), r.Value, r.Unit, r.Status)
			}
			fmt.Fprintln(w)

			if len(matchedNotes) > 0 {
				fmt.Fprintln(w, "## Clinician notes mentioning this biomarker")
				fmt.Fprintln(w)
				for _, n := range matchedNotes {
					fmt.Fprintf(w, "- %s\n", n)
				}
				fmt.Fprintln(w)
			}

			if len(matchedRecs) > 0 {
				fmt.Fprintln(w, "## Function recommendations for this biomarker")
				fmt.Fprintln(w)
				for _, r := range matchedRecs {
					fmt.Fprintf(w, "- %s\n", r)
				}
				fmt.Fprintln(w)
			}

			fmt.Fprintln(w, "## Suggested questions to ask an agent")
			fmt.Fprintln(w)
			fmt.Fprintln(w, "- Given this trend and the supplement stack I told you about, what should I change?")
			fmt.Fprintln(w, "- Which interventions have the most evidence for moving this biomarker toward Function's optimal range?")
			fmt.Fprintln(w, "- What follow-up tests would make sense before my next draw?")
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "", "Restrict to recent rounds (e.g. 3rounds, 1y, 6mo)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	return cmd
}
