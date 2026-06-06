// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/cliutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/spf13/cobra"
)

func newNovelExportPdfForDoctorCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	var memberName string
	var dob string
	var dbPath string
	var onlyOutOfRange bool
	var section string

	cmd := &cobra.Command{
		Use:         "pdf-for-doctor",
		Short:       "Render a Function-branded multi-round lab-history PDF with your name and date of birth, suitable for emailing to your personal physician",
		Long:        "Renders a multi-round, per-category lab-history PDF from the local SQLite store. Includes member name and date of birth in the header (pulled from the synced /user endpoint or supplied via --name and --dob).\n\nBy default every biomarker is included. Narrow the report with --out-of-range (only biomarkers outside Function's optimal range in their most recent draw) and/or --section (only categories whose name contains the given text, e.g. 'Liver' or 'Cardiovascular'). The two filters combine: '--section Liver --out-of-range' yields only out-of-range Liver biomarkers.",
		Example:     "  function-health-pp-cli export pdf-for-doctor --out ~/Downloads/function-history.pdf\n  function-health-pp-cli export pdf-for-doctor --out report.pdf --name 'Marcus Smith' --dob 1978-04-12\n  function-health-pp-cli export pdf-for-doctor --out oor.pdf --out-of-range\n  function-health-pp-cli export pdf-for-doctor --out liver.pdf --section Liver",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if outPath == "" {
				return usageErr(errors.New("--out is required (path to write the PDF)"))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would render PDF to: %s\n", outPath)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
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
			if len(rows) == 0 {
				return noStoreData("export pdf-for-doctor")
			}

			// Apply filters (combinable, AND-ed). Default keeps every row.
			rows = filterResultRows(rows, onlyOutOfRange, section)
			if len(rows) == 0 {
				switch {
				case section != "" && onlyOutOfRange:
					return notFoundErr(fmt.Errorf("no out-of-range biomarkers in a section matching %q", section))
				case section != "":
					return notFoundErr(fmt.Errorf("no biomarkers in a section matching %q (categories are names like 'Cardiovascular' or 'Liver')", section))
				default:
					return notFoundErr(errors.New("no out-of-range biomarkers in the most recent draws"))
				}
			}

			// Look up member profile when name/DOB not supplied.
			if memberName == "" || dob == "" {
				rows, err := s.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'user' LIMIT 1`)
				if err == nil {
					defer rows.Close()
					if rows.Next() {
						var raw []byte
						if err := rows.Scan(&raw); err == nil {
							var profile struct {
								Fname         string `json:"fname"`
								Lname         string `json:"lname"`
								PreferredName string `json:"preferredName"`
								FirstName     string `json:"firstName"` // legacy fallback
								LastName      string `json:"lastName"`
								DOB           string `json:"dob"`
								DateOfBirth   string `json:"dateOfBirth"` // legacy fallback
							}
							_ = json.Unmarshal(raw, &profile)
							if memberName == "" {
								first := profile.Fname
								if first == "" {
									first = profile.PreferredName
								}
								if first == "" {
									first = profile.FirstName
								}
								last := profile.Lname
								if last == "" {
									last = profile.LastName
								}
								memberName = strings.TrimSpace(first + " " + last)
							}
							if dob == "" {
								dob = profile.DOB
								if dob == "" {
									dob = profile.DateOfBirth
								}
								if dob != "" {
									dob = formatDrawDate(dob)
								}
							}
						}
					}
				}
			}

			scope := pdfScopeLabel(onlyOutOfRange, section)
			if err := renderDoctorPDF(outPath, memberName, dob, scope, rows); err != nil {
				return fmt.Errorf("render pdf: %w", err)
			}

			out := map[string]any{
				"status":      "ok",
				"path":        outPath,
				"member":      memberName,
				"dob":         dob,
				"draws_used":  countDistinctRounds(rows),
				"biomarkers":  countDistinctBiomarkers(rows),
				"rendered_at": time.Now().Format(time.RFC3339),
				"filter": map[string]any{
					"out_of_range": onlyOutOfRange,
					"section":      section,
				},
			}
			if flags != nil && flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Wrote %s\n  member: %s  dob: %s  biomarkers: %d  rounds: %d\n",
				outPath, memberName, dob, out["biomarkers"], out["draws_used"])
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output PDF path (required)")
	cmd.Flags().StringVar(&memberName, "name", "", "Override member name (defaults to /user.firstName + lastName)")
	cmd.Flags().StringVar(&dob, "dob", "", "Override date of birth as YYYY-MM-DD (defaults to /user.dateOfBirth)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	cmd.Flags().BoolVar(&onlyOutOfRange, "out-of-range", false, "Only include biomarkers outside Function's optimal range in their most recent draw")
	cmd.Flags().StringVar(&section, "section", "", "Only include categories whose name contains this text (e.g. 'Liver', 'Cardiovascular'); combinable with --out-of-range")
	return cmd
}

// filterResultRows narrows the result set per the combinable PDF flags. A
// biomarker is kept under --out-of-range when its most recent draw is outside
// Function's optimal range (optimalSign != 0); under --section when its
// category name contains the query (case-insensitive). Both filters AND
// together. The zero-value (false, "") returns rows unchanged.
func filterResultRows(rows []resultRow, onlyOutOfRange bool, section string) []resultRow {
	if !onlyOutOfRange && section == "" {
		return rows
	}

	// --out-of-range keys off the most recent draw per biomarker, matching the
	// "Out-of-Function-optimal in most recent draw" summary semantics; a single
	// historical excursion shouldn't pull a now-optimal biomarker into the report.
	oorBiomarker := map[string]bool{}
	if onlyOutOfRange {
		latest := map[string]resultRow{}
		for _, r := range rows {
			k := biomarkerKey(r)
			if existing, ok := latest[k]; !ok || r.DrawDate > existing.DrawDate {
				latest[k] = r
			}
		}
		for k, r := range latest {
			oorBiomarker[k] = optimalSign(r) != 0
		}
	}

	q := strings.ToLower(strings.TrimSpace(section))
	out := rows[:0:0]
	for _, r := range rows {
		if section != "" && !strings.Contains(strings.ToLower(r.Category), q) {
			continue
		}
		if onlyOutOfRange && !oorBiomarker[biomarkerKey(r)] {
			continue
		}
		out = append(out, r)
	}
	return out
}

// biomarkerKey identifies a biomarker across draws, preferring name and
// falling back to ID — mirrors countDistinctBiomarkers.
func biomarkerKey(r resultRow) string {
	if r.BiomarkerName != "" {
		return r.BiomarkerName
	}
	return r.BiomarkerID
}

// pdfScopeLabel renders the header subtitle describing the applied filter.
func pdfScopeLabel(onlyOutOfRange bool, section string) string {
	switch {
	case section != "" && onlyOutOfRange:
		return "Out-of-optimal biomarkers — section: " + section
	case section != "":
		return "Section: " + section
	case onlyOutOfRange:
		return "Out-of-optimal biomarkers"
	default:
		return "Complete lab history"
	}
}

// renderDoctorPDF writes a multi-page branded PDF with member name/DOB header,
// per-category sections, and per-biomarker history rows. scope is a short label
// describing the applied filter (e.g. "Out-of-optimal biomarkers"), shown as a
// subtitle under the title.
func renderDoctorPDF(out, name, dob, scope string, rows []resultRow) error {
	pdf := fpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	now := time.Now().Format("January 2, 2006")

	pdf.AddPage()
	// Header
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(0x12, 0x33, 0x77)
	pdf.Cell(0, 12, "Function Health — Lab History")
	pdf.Ln(14)
	if scope != "" {
		pdf.SetFont("Helvetica", "I", 11)
		pdf.SetTextColor(0x55, 0x55, 0x55)
		pdf.CellFormat(0, 6, scope, "", 1, "L", false, 0, "")
		pdf.Ln(1)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 11)
	if name != "" {
		pdf.CellFormat(0, 7, "Member: "+name, "", 1, "L", false, 0, "")
	}
	if dob != "" {
		pdf.CellFormat(0, 7, "Date of birth: "+dob, "", 1, "L", false, 0, "")
	}
	pdf.CellFormat(0, 7, "Rendered: "+now, "", 1, "L", false, 0, "")
	pdf.SetDrawColor(0x12, 0x33, 0x77)
	pdf.SetLineWidth(0.4)
	pdf.Line(15, pdf.GetY()+2, 200, pdf.GetY()+2)
	pdf.Ln(7)

	// Summary stats
	pdf.SetFont("Helvetica", "B", 13)
	pdf.Cell(0, 8, "Summary")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	rounds := countDistinctRounds(rows)
	biomarkers := countDistinctBiomarkers(rows)
	outOfRange := countOutOfRange(rows)
	pdf.CellFormat(0, 6, fmt.Sprintf("Test rounds analyzed: %d", rounds), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Biomarkers tracked: %d", biomarkers), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Out-of-Function-optimal in most recent draw: %d", outOfRange), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Per-category sections
	byCategory := map[string][]resultRow{}
	for _, r := range rows {
		c := r.Category
		if c == "" {
			c = "Uncategorized"
		}
		byCategory[c] = append(byCategory[c], r)
	}
	categories := make([]string, 0, len(byCategory))
	for c := range byCategory {
		categories = append(categories, c)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(0x12, 0x33, 0x77)
		pdf.Cell(0, 8, cat)
		pdf.Ln(8)
		pdf.SetTextColor(0, 0, 0)

		// Header row
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(0xee, 0xee, 0xf3)
		pdf.CellFormat(60, 6, "Biomarker", "1", 0, "L", true, 0, "")
		pdf.CellFormat(28, 6, "Latest value", "1", 0, "L", true, 0, "")
		pdf.CellFormat(18, 6, "Unit", "1", 0, "L", true, 0, "")
		pdf.CellFormat(28, 6, "Function range", "1", 0, "L", true, 0, "")
		pdf.CellFormat(28, 6, "Quest range", "1", 0, "L", true, 0, "")
		pdf.CellFormat(22, 6, "Status", "1", 1, "L", true, 0, "")

		pdf.SetFont("Helvetica", "", 9)
		groups := groupByBiomarker(byCategory[cat])
		names := make([]string, 0, len(groups))
		for n := range groups {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			series := groups[n]
			latest := series[len(series)-1]
			status := latest.Status
			if status == "" {
				status = "—"
			}
			pdf.CellFormat(60, 5, truncate(n, 30), "1", 0, "L", false, 0, "")
			pdf.CellFormat(28, 5, fmt.Sprintf("%.2f", latest.Value), "1", 0, "L", false, 0, "")
			pdf.CellFormat(18, 5, truncate(latest.Unit, 8), "1", 0, "L", false, 0, "")
			pdf.CellFormat(28, 5, fmt.Sprintf("%.1f-%.1f", latest.OptimalLow, latest.OptimalHigh), "1", 0, "L", false, 0, "")
			pdf.CellFormat(28, 5, fmt.Sprintf("%.1f-%.1f", latest.QuestRangeLow, latest.QuestRangeHigh), "1", 0, "L", false, 0, "")
			pdf.CellFormat(22, 5, truncate(status, 10), "1", 1, "L", false, 0, "")
			if len(series) > 1 {
				pdf.SetFont("Helvetica", "I", 8)
				pdf.SetTextColor(0x55, 0x55, 0x55)
				history := []string{}
				for _, r := range series {
					history = append(history, formatDrawDate(r.DrawDate)+": "+fmt.Sprintf("%.2f", r.Value))
				}
				pdf.CellFormat(184, 4, "    history → "+truncate(strings.Join(history, "  •  "), 130), "", 1, "L", false, 0, "")
				pdf.SetTextColor(0, 0, 0)
				pdf.SetFont("Helvetica", "", 9)
			}
		}
		pdf.Ln(4)
	}

	// Footer
	pdf.SetY(-15)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(0x77, 0x77, 0x77)
	pdf.CellFormat(0, 10, "Generated by function-health-pp-cli — source data: Function Health (https://my.functionhealth.com)", "", 0, "C", false, 0, "")

	return pdf.OutputFileAndClose(out)
}

func countDistinctRounds(rows []resultRow) int {
	seen := map[string]bool{}
	for _, r := range rows {
		if r.RequisitionID != "" {
			seen[r.RequisitionID] = true
		}
	}
	return len(seen)
}

func countDistinctBiomarkers(rows []resultRow) int {
	seen := map[string]bool{}
	for _, r := range rows {
		k := r.BiomarkerName
		if k == "" {
			k = r.BiomarkerID
		}
		if k != "" {
			seen[k] = true
		}
	}
	return len(seen)
}

func countOutOfRange(rows []resultRow) int {
	// Count the most recent draw per biomarker that's out-of-optimal. Key on
	// biomarkerKey (name, falling back to ID) so unnamed biomarkers don't all
	// collapse into a single "" bucket and undercount the summary.
	latest := map[string]resultRow{}
	for _, r := range rows {
		k := biomarkerKey(r)
		if existing, ok := latest[k]; !ok || r.DrawDate > existing.DrawDate {
			latest[k] = r
		}
	}
	n := 0
	for _, r := range latest {
		if optimalSign(r) != 0 {
			n++
		}
	}
	return n
}

var _ = os.Stdout // os imported for verify-env helper
