// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored invoice numbering audit, sezionale-aware.

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type numberingDuplicate struct {
	Number string   `json:"number"`
	IDs    []string `json:"ids"`
}

type numberingDateDisorder struct {
	PrevID     string `json:"prev_id"`
	PrevNumber string `json:"prev_number"`
	PrevDate   string `json:"prev_date"`
	NextID     string `json:"next_id"`
	NextNumber string `json:"next_number"`
	NextDate   string `json:"next_date"`
	Reason     string `json:"reason"`
}

type numberingUnparseable struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

type numberingSeries struct {
	Name         string                  `json:"name"`
	Count        int                     `json:"count"`
	First        int                     `json:"first"`
	Last         int                     `json:"last"`
	Gaps         []int                   `json:"gaps"`
	Duplicates   []numberingDuplicate    `json:"duplicates"`
	DateDisorder []numberingDateDisorder `json:"date_disorder"`
}

type numberingReport struct {
	Year        int                    `json:"year"`
	Checked     int                    `json:"checked"`
	Series      []numberingSeries      `json:"series"`
	Unparseable []numberingUnparseable `json:"unparseable"`
}

func newNumberingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "numbering",
		Short: "Audit invoice numbering",
		Long:  "Check synced invoice numbers for gaps, duplicates, and date disorder, per sezionale.",
		Example: `  partitaiva24-pp-cli numbering audit --year 2026
  partitaiva24-pp-cli numbering audit --json --select series`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newNumberingAuditCmd(flags))
	return cmd
}

func newNumberingAuditCmd(flags *rootFlags) *cobra.Command {
	year := currentYear()
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit one fiscal year's invoice sequence (per sezionale)",
		Long: `Italian invoicing supports multiple numbering sezionali (e.g. "1", "2", … alongside
"01/INF", "1/PA", "1/EU"). The audit groups invoices by sezionale and runs
gap, duplicate, and date-monotonicity checks separately for each series.`,
		Example: `  partitaiva24-pp-cli numbering audit --year 2026
  partitaiva24-pp-cli numbering audit --year 2026 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT id, number, date FROM invoices WHERE strftime('%Y', date) = ? AND COALESCE(status,'') NOT IN ('draft','cancelled','annullata') ORDER BY date, number`,
				fmt.Sprintf("%04d", year))
			if err != nil {
				return err
			}
			defer rows.Close()

			type inv struct {
				id, number, date, series string
				n                        int
			}
			report := numberingReport{Year: year}
			parsedBySeries := map[string][]inv{}
			seenBySeries := map[string]map[string][]string{}

			for rows.Next() {
				var id, number, date sql.NullString
				if err := rows.Scan(&id, &number, &date); err != nil {
					return err
				}
				report.Checked++
				num := nullableString(number)
				series, n, ok := parseInvoiceNumber(num)
				if !ok {
					report.Unparseable = append(report.Unparseable, numberingUnparseable{ID: nullableString(id), Number: num})
					continue
				}
				if seenBySeries[series] == nil {
					seenBySeries[series] = map[string][]string{}
				}
				seenBySeries[series][num] = append(seenBySeries[series][num], nullableString(id))
				parsedBySeries[series] = append(parsedBySeries[series], inv{
					id: nullableString(id), number: num, date: nullableString(date), series: series, n: n,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			seriesNames := make([]string, 0, len(parsedBySeries))
			for k := range parsedBySeries {
				seriesNames = append(seriesNames, k)
			}
			sort.Strings(seriesNames)

			for _, name := range seriesNames {
				items := parsedBySeries[name]
				sort.Slice(items, func(i, j int) bool { return items[i].n < items[j].n })
				ser := numberingSeries{Name: name, Count: len(items)}
				if len(items) > 0 {
					ser.First = items[0].n
					ser.Last = items[len(items)-1].n
				}
				for num, ids := range seenBySeries[name] {
					if len(ids) > 1 {
						ser.Duplicates = append(ser.Duplicates, numberingDuplicate{Number: num, IDs: ids})
					}
				}
				sort.Slice(ser.Duplicates, func(i, j int) bool { return ser.Duplicates[i].Number < ser.Duplicates[j].Number })
				for i := 1; i < len(items); i++ {
					for missing := items[i-1].n + 1; missing < items[i].n; missing++ {
						ser.Gaps = append(ser.Gaps, missing)
					}
					if items[i].date < items[i-1].date {
						ser.DateDisorder = append(ser.DateDisorder, numberingDateDisorder{
							PrevID: items[i-1].id, PrevNumber: items[i-1].number, PrevDate: items[i-1].date,
							NextID: items[i].id, NextNumber: items[i].number, NextDate: items[i].date,
							Reason: "date went backwards within sezionale " + nonemptyOr(name, "(default)"),
						})
					}
				}
				report.Series = append(report.Series, ser)
			}

			// Honor --human-friendly as an explicit override of the
			// "non-TTY → JSON" default; otherwise fall through to JSON in pipes.
			if humanFriendly {
				return printNumberingHuman(cmd.OutOrStdout(), report)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			return printNumberingHuman(cmd.OutOrStdout(), report)
		},
	}
	cmd.Flags().IntVar(&year, "year", year, "Fiscal year")
	return cmd
}

// parseInvoiceNumber separates an invoice number into its sezionale name and
// the integer position within that series.
//
// Examples:
//
//	"1"          -> ("",    1, true)   // default series
//	"2"          -> ("",    2, true)
//	"2026-001"   -> ("",    1, true)   // year prefix is stripped
//	"01/INF"     -> ("INF", 1, true)
//	"1/A"        -> ("A",   1, true)
//	"PA-3"       -> ("PA",  3, true)
//	"3/EU"       -> ("EU",  3, true)
//	"abc"        -> ("",    0, false)  // no integer to anchor on
func parseInvoiceNumber(raw string) (string, int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, false
	}
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '-' || r == '/' || r == ' ' || r == '.' || r == '_'
	})
	// Some numbers are written as "1A" without a separator — split on the
	// alpha/digit boundary so we can recover both pieces.
	expanded := make([]string, 0, len(tokens)*2)
	for _, t := range tokens {
		expanded = append(expanded, splitAlphaNum(t)...)
	}

	var series string
	var num int
	haveNum := false
	for _, tok := range expanded {
		if tok == "" {
			continue
		}
		if isDigits(tok) {
			// Skip 4-digit year tokens.
			if len(tok) == 4 {
				if y, err := strconv.Atoi(tok); err == nil && y >= 2000 && y <= 2100 {
					continue
				}
			}
			if !haveNum {
				if n, err := strconv.Atoi(tok); err == nil {
					num = n
					haveNum = true
				}
			}
		} else if isAlpha(tok) {
			if series == "" {
				series = strings.ToUpper(tok)
			}
		}
	}
	if !haveNum {
		return "", 0, false
	}
	return series, num, true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

func splitAlphaNum(s string) []string {
	var parts []string
	var cur strings.Builder
	curIsDigit := false
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	for i, r := range s {
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			cur.WriteRune(r)
			curIsDigit = isDigit
			continue
		}
		if isDigit != curIsDigit {
			flush()
			curIsDigit = isDigit
		}
		cur.WriteRune(r)
	}
	flush()
	return parts
}

func nonemptyOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func printNumberingHuman(w interface{ Write([]byte) (int, error) }, r numberingReport) error {
	out := func(format string, args ...any) {
		fmt.Fprintf(w, format, args...)
	}
	out("Year:    %d\n", r.Year)
	out("Checked: %d invoices across %d sezionale(s)\n\n", r.Checked, len(r.Series))
	clean := true
	for _, ser := range r.Series {
		name := nonemptyOr(ser.Name, "(default)")
		out("── Sezionale %s ── %d invoice(s), %d → %d\n", name, ser.Count, ser.First, ser.Last)
		if len(ser.Gaps) == 0 && len(ser.Duplicates) == 0 && len(ser.DateDisorder) == 0 {
			out("    %s clean: sequential, no duplicates, dates monotone\n", green("OK"))
		} else {
			clean = false
			if len(ser.Gaps) > 0 {
				out("    %s gaps: %v\n", red("FAIL"), ser.Gaps)
			}
			if len(ser.Duplicates) > 0 {
				out("    %s duplicates:\n", red("FAIL"))
				for _, d := range ser.Duplicates {
					out("        %s used by %d invoices: %v\n", d.Number, len(d.IDs), d.IDs)
				}
			}
			if len(ser.DateDisorder) > 0 {
				out("    %s date disorder:\n", red("FAIL"))
				for _, dd := range ser.DateDisorder {
					out("        %s (%s) → %s (%s): date went backwards\n",
						dd.PrevNumber, dd.PrevDate, dd.NextNumber, dd.NextDate)
				}
			}
		}
		out("\n")
	}
	if len(r.Unparseable) > 0 {
		clean = false
		out("%s unparseable invoice numbers:\n", red("FAIL"))
		for _, u := range r.Unparseable {
			out("    %s (id %s)\n", u.Number, u.ID)
		}
		out("\n")
	}
	if clean {
		out("%s All sezionali are AdE-compliant for fiscal year %d.\n", green("PASS"), r.Year)
	} else {
		out("Review the issues above. Sezionale separation is legal — the question is whether each sezionale is internally consistent.\n")
	}
	return nil
}
