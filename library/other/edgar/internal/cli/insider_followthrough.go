// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli insider-followthrough <TICKER>` — pair senior-officer
// discretionary sales ≥$1M with material 8-K filings in the next 90 days.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

type followthroughReport struct {
	Ticker       string              `json:"ticker"`
	CIK          string              `json:"cik"`
	Pairs        []followthroughPair `json:"pairs"`
	Form4Skipped Form4SkipReport     `json:"form4_skipped"`
}

type followthroughPair struct {
	Sale struct {
		Reporter string  `json:"reporter"`
		Title    string  `json:"title,omitempty"`
		Date     string  `json:"date"`
		Shares   float64 `json:"shares"`
		ValueUSD float64 `json:"value_usd"`
	} `json:"sale"`
	Subsequent8K *struct {
		Accession  string   `json:"accession"`
		FiledAt    string   `json:"filed_at"`
		Items      []string `json:"items"`
		IsMaterial bool     `json:"is_material"`
	} `json:"subsequent_8k,omitempty"`
	DaysBetween int `json:"days_between"`
}

func newInsiderFollowthroughCmd(flags *rootFlags) *cobra.Command {
	var since string
	var threshold float64
	var maxForm4 int
	cmd := &cobra.Command{
		Use:         "insider-followthrough <ticker-or-cik>",
		Short:       "Pair senior-officer code-S sales ≥$1M with material 8-Ks in the following 90 days",
		Example:     "  edgar-pp-cli insider-followthrough AAPL --since 2y",
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
			if since == "" {
				since = "2y"
			}
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}
			if threshold == 0 {
				threshold = 1_000_000
			}

			// Ensure form 4 and 8-K data are present
			skipRep, err := ingestForm4ForCIK(cmd.Context(), c, db, ec.CIK, sinceISO, maxForm4)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			warnForm4Skips(skipRep, ec.Ticker, ec.CIK)

			// Senior discretionary sales ≥ threshold
			txs, err := db.ListEdgarInsiderTransactions(cmd.Context(), ec.CIK, sinceISO, true)
			if err != nil {
				return err
			}

			// All 8-Ks for the issuer in window
			eightKs, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"8-K", "8-K/A"}, sinceISO, 500)

			var out []followthroughPair
			for _, t := range txs {
				if t.TransactionCode != "S" || t.ValueUSD < threshold {
					continue
				}
				saleDate, err := time.Parse("2006-01-02", t.TransactionDate)
				if err != nil {
					continue
				}
				endDate := saleDate.AddDate(0, 0, 90)
				var matched *store.EdgarFiling
				var matchedItems []string
				var matchedMaterial bool
				// PATCH: ListEdgarFilings returns DESC by filed_at; pick the chronologically
				// earliest material 8-K after the sale via findEarliestMaterial8K.
				itemsFor := func(i int) []string {
					f := eightKs[i]
					cached, _ := db.GetEdgarFiling(cmd.Context(), f.Accession)
					body := cached.BodyText
					if body == "" && cached.PrimaryDocURL != "" {
						body, _ = fetchFilingBody(cmd.Context(), c, db, &cached)
					}
					return parseEightKItems(body)
				}
				if idx, items, ok := findEarliestMaterial8K(eightKs, saleDate, endDate, itemsFor); ok {
					matched = &eightKs[idx]
					matchedItems = items
					matchedMaterial = true
				}
				pair := followthroughPair{}
				pair.Sale.Reporter = t.ReporterName
				pair.Sale.Title = t.ReporterTitle
				pair.Sale.Date = t.TransactionDate
				pair.Sale.Shares = t.Shares
				pair.Sale.ValueUSD = t.ValueUSD
				if matched != nil {
					pair.Subsequent8K = &struct {
						Accession  string   `json:"accession"`
						FiledAt    string   `json:"filed_at"`
						Items      []string `json:"items"`
						IsMaterial bool     `json:"is_material"`
					}{Accession: matched.Accession, FiledAt: matched.FiledAt, Items: matchedItems, IsMaterial: matchedMaterial}
					filedAt, _ := time.Parse("2006-01-02", matched.FiledAt)
					pair.DaysBetween = int(filedAt.Sub(saleDate).Hours() / 24)
				}
				out = append(out, pair)
			}
			return emitJSON(cmd, flags, followthroughReport{
				Ticker:       ec.Ticker,
				CIK:          ec.CIK,
				Pairs:        out,
				Form4Skipped: skipRep,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "2y", "Earliest sale date")
	cmd.Flags().Float64Var(&threshold, "threshold", 1_000_000, "Minimum sale value (USD)")
	cmd.Flags().IntVar(&maxForm4, "max-form4", DefaultMaxForm4, "Cap on Form 4 filings ingested in the window; truncation is surfaced as form4_truncated + form4_total_in_window. 0 disables the cap.")
	return cmd
}

// findEarliestMaterial8K returns the index, items, and ok flag for the chronologically
// earliest material 8-K filed in (saleDate, endDate]. eightKs is expected DESC by filed_at
// (the order ListEdgarFilings returns), so iteration walks from the tail forward.
// An 8-K is "material" if it carries any item other than 9.01.
func findEarliestMaterial8K(
	eightKs []store.EdgarFiling,
	saleDate, endDate time.Time,
	itemsFor func(int) []string,
) (int, []string, bool) {
	for i := len(eightKs) - 1; i >= 0; i-- {
		f := eightKs[i]
		filedAt, err := time.Parse("2006-01-02", f.FiledAt)
		if err != nil {
			continue
		}
		if filedAt.Before(saleDate) || filedAt.After(endDate) {
			continue
		}
		items := itemsFor(i)
		for _, it := range items {
			if it != "9.01" {
				return i, items, true
			}
		}
	}
	return -1, nil, false
}
