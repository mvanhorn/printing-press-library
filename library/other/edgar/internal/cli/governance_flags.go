// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli governance-flags <TICKER>` — three independent signals:
// auditor changes (8-K 4.01), restatements (8-K 4.02), late filings (NT 10-K/Q).

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

type governanceFlag struct {
	Accession string `json:"accession"`
	FiledAt   string `json:"filed_at"`
	Detail    string `json:"detail,omitempty"`
	Error     string `json:"error,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type governanceReport struct {
	Ticker         string           `json:"ticker"`
	CIK            string           `json:"cik"`
	AuditorChanges []governanceFlag `json:"auditor_changes"`
	Restatements   []governanceFlag `json:"restatements"`
	LateFilings    []governanceFlag `json:"late_filings"`
}

func newGovernanceFlagsCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "governance-flags <ticker-or-cik>",
		Short:       "Compose 8-K 4.01 (auditor change), 4.02 (restatement), and NT 10-K/Q (late filing) signals",
		Example:     "  edgar-pp-cli governance-flags AAPL --since 2y",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}
			ec, err := resolveCIKOrTicker(cmd.Context(), c, db, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}
			if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
				return classifyAPIError(ferr, flags)
			}
			report := governanceReport{Ticker: ec.Ticker, CIK: ec.CIK,
				AuditorChanges: []governanceFlag{}, Restatements: []governanceFlag{}, LateFilings: []governanceFlag{}}

			// 8-Ks (filter by Item)
			eightKs, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"8-K", "8-K/A"}, sinceISO, 200)
			for _, f := range eightKs {
				cached, _ := db.GetEdgarFiling(cmd.Context(), f.Accession)
				body := cached.BodyText
				if body == "" && cached.PrimaryDocURL != "" {
					body, _ = fetchFilingBody(cmd.Context(), c, db, &cached)
				}
				if body == "" {
					continue
				}
				items := parseEightKItems(body)
				for _, item := range items {
					switch item {
					case "4.01":
						snippet := extractFirstSentenceAfterItem(body, "4.01", 300)
						flag := governanceFlag{Accession: f.Accession, FiledAt: f.FiledAt}
						if snippet == "" {
							flag.Error = "summary_unparseable"
							flag.Reason = "Item 4.01 header present but no extractable summary"
						} else {
							flag.Detail = snippet
						}
						report.AuditorChanges = append(report.AuditorChanges, flag)
					case "4.02":
						snippet := extractFirstSentenceAfterItem(body, "4.02", 300)
						flag := governanceFlag{Accession: f.Accession, FiledAt: f.FiledAt}
						if snippet == "" {
							flag.Error = "summary_unparseable"
							flag.Reason = "Item 4.02 header present but no extractable summary"
						} else {
							flag.Detail = snippet
						}
						report.Restatements = append(report.Restatements, flag)
					}
				}
			}

			// Late filings (NT 10-K, NT 10-Q)
			ntFilings, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"NT 10-K", "NT 10-Q"}, sinceISO, 50)
			for _, f := range ntFilings {
				report.LateFilings = append(report.LateFilings, governanceFlag{
					Accession: f.Accession, FiledAt: f.FiledAt,
					Detail: f.FormType + " filed " + f.FiledAt,
				})
			}
			return emitJSON(cmd, flags, report)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Earliest filing date (ISO or 1y/2y)")
	return cmd
}

// Ensure store import is referenced.
var _ = store.EdgarFiling{}
