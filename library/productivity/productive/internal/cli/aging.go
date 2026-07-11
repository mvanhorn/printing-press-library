// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. Implemented body — regen preserves this file.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// agingBucket is one AR aging band. Amounts are integer cents.
type agingBucket struct {
	Bucket      string `json:"bucket"`
	Count       int    `json:"count"`
	UnpaidCents int64  `json:"unpaid_cents"`
}

// pp:data-source live
func newNovelAgingCmd(flags *rootFlags) *cobra.Command {
	var flagAsOf, flagCompany string
	var flagMaxPages int

	cmd := &cobra.Command{
		Use:   "aging",
		Short: "Bucket unpaid invoice amounts by age past their due date.",
		Long: "Scans regular invoices with an outstanding balance and buckets amount_unpaid (cents) by " +
			"days past pay_on relative to --as-of: current (not yet due), 1-30, 31-60, 61-90, 90+. " +
			"Reads live invoices and aggregates locally.",
		Example:     "  productive-pp-cli aging --as-of 2026-07-04 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan unpaid invoices and bucket by age")
				return nil
			}
			asOf := time.Now()
			if flagAsOf != "" {
				t, err := time.Parse("2006-01-02", flagAsOf)
				if err != nil {
					return usageErr(fmt.Errorf("--as-of must be YYYY-MM-DD: %w", err))
				}
				asOf = t
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			buckets := []*agingBucket{
				{Bucket: "current"},
				{Bucket: "1-30"},
				{Bucket: "31-60"},
				{Bucket: "61-90"},
				{Bucket: "90+"},
			}
			place := func(daysOverdue int, unpaid int64) {
				var b *agingBucket
				switch {
				case daysOverdue <= 0:
					b = buckets[0]
				case daysOverdue <= 30:
					b = buckets[1]
				case daysOverdue <= 60:
					b = buckets[2]
				case daysOverdue <= 90:
					b = buckets[3]
				default:
					b = buckets[4]
				}
				b.Count++
				b.UnpaidCents += unpaid
			}

			page := 1
			scanned := 0
			truncated := false
			for page <= flagMaxPages {
				params := map[string]string{
					"filter[invoice_type][eq]": "1",
					"page[size]":               "200",
					"page[number]":             strconv.Itoa(page),
				}
				if flagCompany != "" {
					params["filter[company][eq]"] = flagCompany
				}
				body, err := c.Get(ctx, "/invoices", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var doc japiDoc
				if err := json.Unmarshal(body, &doc); err != nil {
					return fmt.Errorf("parsing invoices response: %w", err)
				}
				for _, inv := range doc.Data {
					scanned++
					unpaid := rowMoneyCents(inv.Attributes, "amount_unpaid")
					if unpaid <= 0 {
						continue
					}
					payOn := rowString(inv.Attributes, "pay_on")
					days := 0
					if payOn != "" {
						if due, perr := time.Parse("2006-01-02", payOn); perr == nil {
							days = int(asOf.Sub(due).Hours() / 24)
						}
					}
					place(days, unpaid)
				}
				total, ok := metaInt(doc.Meta, "total_pages")
				if ok {
					if page >= total {
						break
					}
				} else if len(doc.Data) < 200 {
					// No total_pages in meta: fall back to the full-page
					// heuristic (page[size] is 200 above) — a short page means
					// the end. Otherwise aging would scan only page 1 and
					// report unpaid buckets that are too low.
					break
				}
				if page >= flagMaxPages {
					// Hit the scan cap with more pages available: stop, but flag
					// the result as incomplete so the totals are not reported as
					// complete when more unpaid invoices may exist.
					truncated = true
					break
				}
				page++
			}

			var totalUnpaid int64
			var totalCount int
			for _, b := range buckets {
				totalUnpaid += b.UnpaidCents
				totalCount += b.Count
			}
			out := make([]agingBucket, 0, len(buckets))
			for _, b := range buckets {
				out = append(out, *b)
			}
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			if truncated {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: aging stopped at the --max-scan-pages cap (%d pages, 200 invoices/page); more unpaid invoices may exist and the totals below may be understated. Re-run with a higher --max-scan-pages.\n", flagMaxPages)
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{
				"source": "live", "as_of": asOf.Format("2006-01-02"),
				"invoices_scanned": scanned, "unpaid_invoices": totalCount, "total_unpaid_cents": totalUnpaid,
				"scan_truncated": truncated, "max_scan_pages": flagMaxPages,
			})
		},
	}
	cmd.Flags().StringVar(&flagAsOf, "as-of", "", "Reference date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&flagCompany, "company", "", "Restrict to a company id")
	cmd.Flags().IntVar(&flagMaxPages, "max-scan-pages", 25, "Maximum invoice pages to scan (200/page)")
	return cmd
}
