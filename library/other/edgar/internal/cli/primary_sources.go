// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli primary-sources <TICKER>` — one-shot LODESTAR primary-source
// bundle: latest 10-K + 4 10-Qs + 8-Ks 90d + Form 4s 12mo + DEF 14A.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

type primarySourcesBundle struct {
	Ticker       string             `json:"ticker"`
	CIK          string             `json:"cik"`
	Name         string             `json:"name,omitempty"`
	TenK         map[string]any     `json:"ten_k,omitempty"`
	TenQ         []map[string]any   `json:"ten_q"`
	EightK       []eightKItemResult `json:"eight_k"`
	Form4        []reporterSummary  `json:"form_4"`
	Form4Skipped Form4SkipReport    `json:"form4_skipped"`
	DEF14A       map[string]any     `json:"def_14a,omitempty"`
}

func newPrimarySourcesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var maxForm4 int
	cmd := &cobra.Command{
		Use:   "primary-sources <ticker-or-cik>",
		Short: "LODESTAR primary-source bundle: 10-K + 4 10-Qs + 8-Ks + Form 4s + DEF 14A in one call",
		Long: `Compose the full LODESTAR primary-source pull for a ticker: latest 10-K (cover +
key Items via sections parser), 4 most-recent 10-Qs, 8-Ks in the trailing 90
days (or --since), Form 4 transactions in the trailing 12 months, and latest
DEF 14A. Applies the sections boundary safety contract: when an Item can't be
parsed unambiguously, that section emits a {"error":"boundary_unverifiable"}
object instead of best-effort text.`,
		Example:     "  edgar-pp-cli primary-sources AAPL --since 90d",
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
				since = "90d"
			}
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}
			form4Since, _ := parseSinceDate("12mo")

			if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
				return classifyAPIError(ferr, flags)
			}

			bundle := primarySourcesBundle{Ticker: ec.Ticker, CIK: ec.CIK, Name: ec.Name}

			// Latest 10-K — parse Items 5 + 9B via sections
			if tenKs, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"10-K", "10-K/A"}, "", 1); len(tenKs) > 0 {
				cached, _ := db.GetEdgarFiling(cmd.Context(), tenKs[0].Accession)
				body := cached.BodyText
				if body == "" {
					body, _ = fetchFilingBody(cmd.Context(), c, db, &cached)
				}
				secs, _ := extractSections(body, []string{"5", "9B"})
				bundle.TenK = map[string]any{
					"accession": tenKs[0].Accession,
					"filed_at":  tenKs[0].FiledAt,
					"sections":  secs,
				}
			}

			// 4 most-recent 10-Qs
			if tenQs, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"10-Q", "10-Q/A"}, "", 4); len(tenQs) > 0 {
				for _, q := range tenQs {
					cached, _ := db.GetEdgarFiling(cmd.Context(), q.Accession)
					body := cached.BodyText
					if body == "" && cached.PrimaryDocURL != "" {
						body, _ = fetchFilingBody(cmd.Context(), c, db, &cached)
					}
					// MD&A is typically Item 2 in a 10-Q
					secs, _ := extractSections(body, []string{"2"})
					bundle.TenQ = append(bundle.TenQ, map[string]any{
						"accession": q.Accession,
						"filed_at":  q.FiledAt,
						"sections":  secs,
					})
				}
			}

			// 8-Ks in window
			if eightKs, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"8-K", "8-K/A"}, sinceISO, 50); len(eightKs) > 0 {
				for _, f := range eightKs {
					cached, _ := db.GetEdgarFiling(cmd.Context(), f.Accession)
					body := cached.BodyText
					if body == "" && cached.PrimaryDocURL != "" {
						body, _ = fetchFilingBody(cmd.Context(), c, db, &cached)
					}
					items := parseEightKItems(body)
					isMaterial := false
					for _, it := range items {
						if it != "9.01" {
							isMaterial = true
							break
						}
					}
					summary := map[string]string{}
					for _, it := range items {
						if validEightKItems[it] {
							summary[it] = extractFirstSentenceAfterItem(body, it, 200)
						}
					}
					bundle.EightK = append(bundle.EightK, eightKItemResult{
						Accession: f.Accession, FiledAt: f.FiledAt,
						Items: items, IsMaterial: isMaterial, SummaryPerItem: summary,
					})
				}
			}

			// Form 4 (12 months) — loud-skip per LODESTAR mandate
			form4Skip, form4Err := ingestForm4ForCIK(cmd.Context(), c, db, ec.CIK, form4Since, maxForm4)
			if form4Err == nil {
				warnForm4Skips(form4Skip, ec.Ticker, ec.CIK)
				bundle.Form4Skipped = form4Skip
				txs, _ := db.ListEdgarInsiderTransactions(cmd.Context(), ec.CIK, form4Since, false)
				byReporter := map[string]*reporterSummary{}
				for _, r := range txs {
					if byReporter[r.ReporterName] == nil {
						byReporter[r.ReporterName] = &reporterSummary{
							Name: r.ReporterName, Title: r.ReporterTitle,
							IsSenior: r.IsSeniorOfficer, IsDirector: r.IsDirector,
						}
					}
					rs := byReporter[r.ReporterName]
					rs.TransactionCount++
					switch r.TransactionCode {
					case "S":
						rs.CodeSShares += r.Shares
						rs.CodeSValue += r.ValueUSD
					case "P":
						rs.CodePShares += r.Shares
						rs.CodePValue += r.ValueUSD
					case "A":
						rs.CodeAShares += r.Shares
					case "F":
						rs.CodeFShares += r.Shares
					default:
						rs.OtherShares += r.Shares
					}
				}
				for _, rs := range byReporter {
					rs.NetDiscretionaryShares = rs.CodePShares - rs.CodeSShares
					rs.NetDiscretionaryValueUSD = rs.CodePValue - rs.CodeSValue
					bundle.Form4 = append(bundle.Form4, *rs)
				}
			}

			// DEF 14A (latest)
			if proxies, _ := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"DEF 14A", "DEFA14A"}, "", 1); len(proxies) > 0 {
				bundle.DEF14A = map[string]any{
					"accession":       proxies[0].Accession,
					"filed_at":        proxies[0].FiledAt,
					"primary_doc_url": proxies[0].PrimaryDocURL,
				}
			}

			return emitJSON(cmd, flags, bundle)
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "8-K window (ISO date or 90d/12mo)")
	cmd.Flags().IntVar(&maxForm4, "max-form4", DefaultMaxForm4, "Cap on Form 4 filings ingested in the 12mo window; truncation is surfaced as form4_truncated + form4_total_in_window. 0 disables the cap.")
	return cmd
}
