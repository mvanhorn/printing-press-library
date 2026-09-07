// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cdcparse"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cdcpdf"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

type checkResult struct {
	Name       string  `json:"check_name"`
	Tested     int     `json:"tested"`
	Violations int     `json:"violations"`
	WorstDelta float64 `json:"worst_delta,omitempty"`
	Severity   string  `json:"severity"`
	Detail     string  `json:"detail"`
}

type verifyRowsView struct {
	Vintage       string        `json:"vintage"`
	Part          string        `json:"part"`
	Source        string        `json:"source"`
	Layout        string        `json:"layout"`
	Title         string        `json:"title"`
	Producer      string        `json:"producer"`
	Backend       string        `json:"extract_backend"`
	Pages         int           `json:"pages"`
	RowsParsed    int           `json:"rows_parsed"`
	ISINsInText   int           `json:"isins_present_in_text"`
	CoveragePct   float64       `json:"extraction_coverage_pct"`
	Checks        []checkResult `json:"checks"`
	ParseFindings int           `json:"parse_findings"`
	Stored        bool          `json:"stored"`
	Note          string        `json:"note"`
}

var isinInText = regexp.MustCompile(`\b[A-Z]{2}[A-Z0-9]{9}[0-9]\b`)
var vintageRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func newNovelVerifyRowsCmd(flags *rootFlags) *cobra.Command {
	var (
		vintage string
		part    string
		url     string
		file    string
		strict  bool
		noStore bool
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "rows",
		Short: "Prove the PDF extraction is trustworthy by asserting the report's own percentage column against its own share and capital columns.",
		Long: strings.Trim(`
Extract a penetration report and audit it against itself.

No reference parser for these PDFs exists anywhere, so internal consistency is
the only available correctness bar. Three checks run:

  pct_matches_ratio   the report's own "% of shares available with ref: to paid
                      up capital" must equal shares_in_cds / paid_up_incl * 100
  excl_le_incl        paid-up capital excluding GoP holding must never exceed
                      paid-up capital including it
  isin_check_digit    every Security Id must satisfy ISO 6166

EXTRACTION COVERAGE IS MEASURED FROM OUTSIDE THE PARSER: ISINs found in the raw
text are counted independently and compared against rows actually emitted, so a
parser that quietly drops a layout cannot report success. Measured on the live
2025-11-30 vintage: part A 99.80%, part B 100.00%. The archived 2024 vintage
reaches only ~50% because Excel-era PDFs render the S.No column as a separate
text block, which interleaves records; that number is REPORTED, not hidden.

The check that once "failed" on 154 rows was a parser bug of exactly this kind:
a generic regex compared a capital column against a percentage column. The
source data is clean to rounding precision.

Extraction needs macOS PDFKit for print-driver vintages (everything from 2025
on). The pure-Go fallback reads only Excel-era files, so on other platforms this
command fails honestly rather than returning empty rows.
`, "\n"),
		Example: "  cdc-pakistan-pp-cli verify rows --vintage 2025-11-30 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"mcp:local-write":     "true",
			"pp:happy-args":       "--vintage=2025-11-30;--timeout=30m",
			"pp:typed-exit-codes": "0,2,3,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify rows")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if file == "" && url == "" && vintage == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("one of --vintage, --url or --file is required"))
			}
			if vintage != "" && !vintageRe.MatchString(vintage) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--vintage must be YYYY-MM-DD, got %q", vintage))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}

			// Resolve the bytes: a local file, an explicit URL, or a vintage
			// looked up in the document index.
			var raw []byte
			var srcLabel string
			switch {
			case file != "":
				b, err := os.ReadFile(file)
				if err != nil {
					return usageErr(fmt.Errorf("reading --file: %w", err))
				}
				raw, srcLabel = b, file
				if part == "" {
					part = cdcparse.ReportPart(file)
				}
			default:
				target := url
				if target == "" {
					t, p, err := resolveVintageURL(ctx, dbPath, vintage, part)
					if err != nil {
						return err
					}
					if t == "" {
						// Nothing to audit yet. Emit an empty typed result and a
						// stderr hint naming the command that populates it --
						// never a usage or API error.
						fmt.Fprintf(cmd.ErrOrStderr(),
							"no penetration report in the local index at %s\n"+
								"the series lives in the `miscellaneous` category and only ONE live vintage exists at a time (CDC removes prior months)\n"+
								"run: cdc-pakistan-pp-cli coverage map --category miscellaneous --probe --db %s\n", dbPath, dbPath)
						empty := &verifyRowsView{
							Vintage: vintage, Part: part, Checks: []checkResult{},
							Note: "no penetration report available locally; nothing audited. Run `coverage map --category miscellaneous --probe`, or pass --url/--file.",
						}
						if !wantsHumanTable(cmd.OutOrStdout(), flags) {
							return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
						}
						fmt.Fprintln(cmd.OutOrStdout(), empty.Note)
						return nil
					}
					target, part = t, p
				}
				if part == "" {
					part = cdcparse.ReportPart(target)
				}
				cl, err := LoadClearance(flags)
				if err != nil {
					return err
				}
				if err := cl.CheckMargin(30 * time.Second); err != nil {
					return err
				}
				f := newClearanceFetcher(cl, 2*time.Minute)
				_, b, err := f.GetBinary(ctx, target)
				if err != nil {
					return err
				}
				raw, srcLabel = b, target
			}

			tmp, err := os.CreateTemp("", "cdc-vintage-*.pdf")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			if _, err := tmp.Write(raw); err != nil {
				tmp.Close()
				return err
			}
			tmp.Close()

			res, err := cdcpdf.Extract(ctx, tmp.Name())
			if err != nil {
				return apiErr(fmt.Errorf("extracting %s: %w", filepath.Base(srcLabel), err))
			}
			parsed := cdcpdf.ParseRows(res.Text)

			present := map[string]bool{}
			for _, m := range isinInText.FindAllString(res.Text, -1) {
				present[m] = true
			}
			view := &verifyRowsView{
				Vintage: vintage, Part: part, Source: srcLabel,
				Layout: string(parsed.Layout), Title: parsed.Title,
				Producer: res.Producer, Backend: res.Backend, Pages: res.Pages,
				RowsParsed: len(parsed.Rows), ISINsInText: len(present),
				ParseFindings: len(parsed.Findings),
			}
			if len(present) > 0 {
				view.CoveragePct = math.Round(10000*float64(len(parsed.Rows))/float64(len(present))) / 100
			}
			view.Checks = runRowChecks(parsed.Rows)

			if !noStore && vintage != "" {
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return configErr(fmt.Errorf("opening store: %w", err))
				}
				defer db.Close()
				if err := store.EnsureCDCSchema(ctx, db); err != nil {
					return err
				}
				if err := storeVintage(ctx, db, view, parsed, res); err != nil {
					return err
				}
				view.Stored = true
			}

			errs := 0
			for _, c := range view.Checks {
				if c.Severity == "error" && c.Violations > 0 {
					errs++
				}
			}
			view.Note = fmt.Sprintf("extraction coverage %.2f%% measured from outside the parser (%d rows emitted / %d ISINs present in raw text); %d parse findings; %d failing checks",
				view.CoveragePct, view.RowsParsed, view.ISINsInText, view.ParseFindings, errs)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				renderVerifyRows(cmd, view)
			}
			if strict && (errs > 0 || view.CoveragePct < 99) {
				return notFoundErr(fmt.Errorf("--strict: %d failing check(s) and %.2f%% coverage", errs, view.CoveragePct))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vintage, "vintage", "", "vintage date YYYY-MM-DD; resolved against the local document index")
	cmd.Flags().StringVar(&part, "part", "", "report part: A (ordinary/preference/modaraba) or B (open-end/ETF funds)")
	cmd.Flags().StringVar(&url, "url", "", "explicit report URL instead of resolving a vintage")
	cmd.Flags().StringVar(&file, "file", "", "read a already-downloaded PDF from disk instead of fetching")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero on any failing check or sub-99%% coverage")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "audit without writing rows to the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

func runRowChecks(rows []cdcpdf.Row) []checkResult {
	pct := checkResult{Name: "pct_matches_ratio", Severity: "error",
		Detail: "the report's own percentage column must equal shares_in_cds / paid_up_incl * 100"}
	excl := checkResult{Name: "excl_le_incl", Severity: "error",
		Detail: "paid-up capital excluding GoP must never exceed paid-up capital including GoP"}
	isin := checkResult{Name: "isin_check_digit", Severity: "error",
		Detail: "every Security Id must satisfy the ISO 6166 check digit"}

	for _, r := range rows {
		isin.Tested++
		if !isinCheckDigitOK(r.ISIN) {
			isin.Violations++
		}
		if r.PaidUpIncl != nil && r.PaidUpExcl != nil {
			excl.Tested++
			if *r.PaidUpExcl > *r.PaidUpIncl+1e-6 {
				excl.Violations++
			}
		}
		if r.PctIncl != nil && r.PaidUpIncl != nil && r.SharesInCDS != nil && *r.PaidUpIncl != 0 {
			pct.Tested++
			d := math.Abs(*r.SharesInCDS / *r.PaidUpIncl * 100 - *r.PctIncl)
			if d > pct.WorstDelta {
				pct.WorstDelta = math.Round(d*10000) / 10000
			}
			// 0.05pp tolerance: CDC publishes the percentage rounded to 2dp.
			if d > 0.05 {
				pct.Violations++
			}
		}
	}
	return []checkResult{pct, excl, isin}
}

// isinCheckDigitOK validates an ISIN per ISO 6166 (Luhn over the
// letter-to-digit expansion).
func isinCheckDigitOK(s string) bool {
	if len(s) != 12 {
		return false
	}
	var digits []int
	for _, ch := range s[:11] {
		switch {
		case ch >= '0' && ch <= '9':
			digits = append(digits, int(ch-'0'))
		case ch >= 'A' && ch <= 'Z':
			v := int(ch-'A') + 10
			digits = append(digits, v/10, v%10)
		default:
			return false
		}
	}
	sum, dbl := 0, true
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if dbl {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		dbl = !dbl
		sum += d
	}
	want := (10 - sum%10) % 10
	return int(s[11]-'0') == want
}

// resolveVintageURL returns ("", "", nil) when the local index simply has no
// penetration report yet. That is an EMPTY LOCAL-CACHE STATE, not a failure: the
// series lives in one category, only one live vintage exists at a time, and a
// fresh install legitimately has none. Callers emit an empty typed result.
func resolveVintageURL(ctx context.Context, dbPath, vintage, part string) (string, string, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", "", nil
	}
	db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_documents")
	if oerr != nil {
		return "", "", oerr
	}
	if !ready {
		return "", "", nil
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx,
		`SELECT url, title FROM cdc_documents WHERE event_kind IS NOT NULL ORDER BY url`)
	if err != nil {
		return "", "", apiErr(err)
	}
	defer rows.Close()
	var candidates []string
	for rows.Next() {
		var u, t string
		if err := rows.Scan(&u, &t); err != nil {
			return "", "", apiErr(err)
		}
		if !cdcparse.IsPenetrationReport(t) {
			continue
		}
		candidates = append(candidates, u)
	}
	if len(candidates) == 0 {
		return "", "", nil
	}
	for _, u := range candidates {
		p := cdcparse.ReportPart(u)
		if part != "" && !strings.EqualFold(p, part) {
			continue
		}
		return u, p, nil
	}
	return candidates[0], cdcparse.ReportPart(candidates[0]), nil
}

func storeVintage(ctx context.Context, db *store.Store, view *verifyRowsView,
	parsed *cdcpdf.Parsed, res *cdcpdf.Result) error {
	now := time.Now().UTC().Format(time.RFC3339)
	fp := fmt.Sprintf("%s|%s|%d|%d", parsed.Layout, parsed.Title, parsed.Layout.NumericFields(), res.Pages)
	if _, err := db.DB().ExecContext(ctx,
		`INSERT INTO cdc_vintages (vintage_date, part, url, layout, title, producer, backend,
		   pages, rows_parsed, isins_in_text, coverage_pct, schema_fingerprint, fetched_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(vintage_date, part) DO UPDATE SET
		   url=excluded.url, layout=excluded.layout, title=excluded.title,
		   producer=excluded.producer, backend=excluded.backend, pages=excluded.pages,
		   rows_parsed=excluded.rows_parsed, isins_in_text=excluded.isins_in_text,
		   coverage_pct=excluded.coverage_pct, schema_fingerprint=excluded.schema_fingerprint,
		   fetched_at=excluded.fetched_at`,
		view.Vintage, view.Part, view.Source, string(parsed.Layout), parsed.Title,
		res.Producer, res.Backend, res.Pages, len(parsed.Rows), view.ISINsInText,
		view.CoveragePct, fp, now); err != nil {
		return apiErr(fmt.Errorf("storing vintage: %w", err))
	}

	const q = `INSERT INTO cdc_penetration_rows
	  (vintage_date, part, isin, seq, name, name_markers, symbol, live_date, maturity_date,
	   status, row_layout, shares_in_cds, market_value, paid_up_incl_gop, pct_incl_gop,
	   paid_up_excl_gop, pct_excl_gop, extract_backend)
	  VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	  ON CONFLICT(vintage_date, part, isin) DO UPDATE SET
	    shares_in_cds=excluded.shares_in_cds, paid_up_incl_gop=excluded.paid_up_incl_gop,
	    pct_incl_gop=excluded.pct_incl_gop, paid_up_excl_gop=excluded.paid_up_excl_gop,
	    status=excluded.status, row_layout=excluded.row_layout`
	for _, r := range parsed.Rows {
		if _, err := db.DB().ExecContext(ctx, q, view.Vintage, view.Part, r.ISIN, r.Seq,
			r.Name, strings.Join(r.NameMarkers, " "), r.Symbol, r.LiveDate, r.MaturityDate,
			r.Status, string(r.Layout), r.SharesInCDS, r.MarketValue, r.PaidUpIncl,
			r.PctIncl, r.PaidUpExcl, r.PctExcl, res.Backend); err != nil {
			return apiErr(fmt.Errorf("storing row %s: %w", r.ISIN, err))
		}
	}

	// Findings go to the shared substrate so --strict emit paths can refuse.
	for _, f := range parsed.Findings {
		if _, err := db.DB().ExecContext(ctx,
			`INSERT INTO cdc_data_quality_findings (surface, vintage, subject, check_name, severity, detail, found_at)
			 VALUES ('penetration', ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(surface, vintage, subject, check_name) DO UPDATE SET detail=excluded.detail, found_at=excluded.found_at`,
			view.Vintage, f.ISIN, f.CheckName, f.Severity, f.Detail, now); err != nil {
			return apiErr(fmt.Errorf("storing finding: %w", err))
		}
	}
	for _, c := range view.Checks {
		if c.Violations == 0 {
			continue
		}
		if _, err := db.DB().ExecContext(ctx,
			`INSERT INTO cdc_data_quality_findings (surface, vintage, subject, check_name, severity, detail, found_at)
			 VALUES ('penetration', ?, '', ?, ?, ?, ?)
			 ON CONFLICT(surface, vintage, subject, check_name) DO UPDATE SET detail=excluded.detail, found_at=excluded.found_at`,
			view.Vintage, c.Name, c.Severity,
			fmt.Sprintf("%d of %d rows violated: %s", c.Violations, c.Tested, c.Detail), now); err != nil {
			return apiErr(err)
		}
	}
	return nil
}

func renderVerifyRows(cmd *cobra.Command, v *verifyRowsView) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "vintage %s part %s  layout=%s\n", v.Vintage, v.Part, v.Layout)
	fmt.Fprintf(w, "  title    : %s\n", v.Title)
	fmt.Fprintf(w, "  producer : %s   backend=%s pages=%d\n", v.Producer, v.Backend, v.Pages)
	fmt.Fprintf(w, "  coverage : %.2f%%  (%d rows / %d ISINs in raw text)\n", v.CoveragePct, v.RowsParsed, v.ISINsInText)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %-20s %8s %11s %s\n", "CHECK", "TESTED", "VIOLATIONS", "WORST")
	for _, c := range v.Checks {
		worst := ""
		if c.WorstDelta > 0 {
			worst = fmt.Sprintf("%.4fpp", c.WorstDelta)
		}
		fmt.Fprintf(w, "  %-20s %8d %11d %s\n", c.Name, c.Tested, c.Violations, worst)
	}
	fmt.Fprintf(w, "\n%s\n", v.Note)
}
