// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

// Package cli — bill / bill --reconcile.
//
// `bill` is the smart user-facing front-end on top of the generated
// /resellers/{id}/bill/ endpoint:
//   - resolves the reseller ID automatically from /current-user/ so MSP
//     owners do not have to look it up.
//   - plain mode just renders the bill (table / json / csv).
//   - --reconcile mode joins the bill against a CSV the MSP exports from
//     their own invoicing system and surfaces every drift line — clients
//     being undercharged, overcharged, missing on either side.
//
// Money is held as integer cents end-to-end. Floats are only ever used at
// the parse boundary (CSV ingest, API JSON decode) and converted to cents
// at first opportunity. The display formatter divides by 100 once, at the
// edge.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/cliutil"
)

// reconcileThresholdCents is the absolute-value cents floor below which a
// drift is treated as rounding noise and not flagged in the NOTE column.
// $5.00 = 500 cents matches the brief.
const reconcileThresholdCents = 500

// billLineItem is a tolerant decode of one row from /resellers/{id}/bill/.
// The Servosity API may emit either a flat array of lines or an object
// wrapping a "results" / "items" / "line_items" key, so the caller pulls
// out the array shape before unmarshaling.
type billLineItem struct {
	CompanyID   json.RawMessage `json:"company_id"`
	CompanyName string          `json:"company_name"`
	Amount      json.RawMessage `json:"amount"`
}

// reconcileRow is one joined company across Servosity charge vs. MSP
// invoice. Either side can be zero (missing on that side).
type reconcileRow struct {
	CompanyID     string `json:"company_id,omitempty"`
	CompanyName   string `json:"company_name"`
	ServosityCent int64  `json:"servosity_cents"`
	InvoicedCent  int64  `json:"invoiced_cents"`
	DeltaCent     int64  `json:"delta_cents"`
	Note          string `json:"note,omitempty"`
}

// reconcileReport is the JSON envelope when --format json is requested
// (or --json is set). Totals are surfaced separately so spreadsheet
// pipelines don't have to re-sum.
type reconcileReport struct {
	Month  string                 `json:"month"`
	Totals map[string]int64       `json:"totals"`
	Rows   []reconcileRow         `json:"rows"`
	Meta   map[string]interface{} `json:"meta,omitempty"`
}

func newBillCmd(flags *rootFlags) *cobra.Command {
	var reconcilePath string
	var month string
	var format string

	cmd := &cobra.Command{
		Use:   "bill",
		Short: "Pull your monthly Servosity bill — with optional reconcile against your client invoicing",
		Long: `Pull the MSP's monthly Servosity bill, with two modes:

  bill                       — show the bill (table | json | csv).
  bill --reconcile <csv>     — compare line-by-line against a CSV of what
                                you've invoiced YOUR clients and surface
                                the drift (over/under-charges, missing rows).

The CSV must have header columns: company_id,company_name,invoiced_amount.
Amounts may be formatted with or without a leading "$" (e.g. "128.50" or
"$128.50"). All money is computed in integer cents to avoid float drift.`,
		Example: `  # Show this month's Servosity bill
  servosity-msp-cli bill

  # Last month, JSON for piping into jq
  servosity-msp-cli bill --month 2026-04 --format json

  # Reconcile against your invoicing system
  servosity-msp-cli bill --reconcile ./invoices-2026-05.csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --format early so usage errors don't waste an API call.
			format = strings.ToLower(strings.TrimSpace(format))
			switch format {
			case "", "table":
				format = "table"
			case "json", "csv":
				// ok
			default:
				return usageErr(fmt.Errorf("--format must be one of: table, json, csv (got %q)", format))
			}

			// Default month = current month in YYYY-MM. Validate any user value.
			if strings.TrimSpace(month) == "" {
				month = time.Now().Format("2006-01")
			} else {
				if _, err := time.Parse("2006-01", month); err != nil {
					return usageErr(fmt.Errorf("--month must be YYYY-MM (got %q): %w", month, err))
				}
			}

			// Verify-friendly: short-circuit BEFORE any IO. The printing-press
			// verify pipeline runs every hand-written command with --dry-run;
			// this guard keeps it green. The CSV existence check below is
			// IO and must come AFTER the dry-run short-circuit.
			if dryRunOK(flags) {
				return nil
			}

			// --reconcile <path>: fail fast on a missing file BEFORE we
			// burn an API call. Exit 2 (usage / input validation).
			if reconcilePath != "" {
				if _, err := os.Stat(reconcilePath); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return usageErr(fmt.Errorf("--reconcile file does not exist: %s", reconcilePath))
					}
					return usageErr(fmt.Errorf("--reconcile file %s: %w", reconcilePath, err))
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: resolve reseller ID. The legacy resolveResellerID
			// here probed /current-user/, which doesn't expose the
			// reseller field on partner-scoped tokens. cliutil.ResolveResellerID
			// derives it from the first company's reseller URL field,
			// with SERVOSITY_MSP_RESELLER_ID as an override.
			resellerInt64, err := cliutil.ResolveResellerID(cmd.Context(), c)
			if err != nil {
				return fmt.Errorf("resolving reseller ID: %w", err)
			}
			resellerID := strconv.FormatInt(resellerInt64, 10)

			// Step 2: pull the bill. The OpenAPI path doesn't expose a
			// month query param explicitly; pass it through anyway so the
			// API gets the hint when it does support it, and we surface
			// the requested month in our output envelope regardless.
			billPath := replacePathParam("/resellers/{id}/bill/", "id", resellerID)
			params := map[string]string{}
			if month != "" {
				params["month"] = month
			}
			data, _, err := resolveRead(cmd.Context(), c, flags, "bill", false, billPath, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Plain bill mode: pass the API response through the standard
			// format pipeline. Honor --format as a sugar for the global
			// --json / --csv flags so users don't have to mix flags.
			if reconcilePath == "" {
				return renderPlainBill(cmd.OutOrStdout(), data, format, flags)
			}

			// Reconcile mode.
			lines, err := extractBillLines(data)
			if err != nil {
				return apiErr(fmt.Errorf("decoding bill response: %w", err))
			}
			invoices, err := readInvoicesCSV(reconcilePath)
			if err != nil {
				return usageErr(fmt.Errorf("reading %s: %w", reconcilePath, err))
			}
			rows := joinAndScore(lines, invoices)
			report := buildReport(month, rows)

			return renderReconcile(cmd.OutOrStdout(), report, format, flags)
		},
	}

	cmd.Flags().StringVar(&reconcilePath, "reconcile", "", "Path to invoicing CSV (columns: company_id,company_name,invoiced_amount). Triggers reconcile mode.")
	cmd.Flags().StringVar(&month, "month", "", "Billing period in YYYY-MM (default: current month)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table | json | csv")

	return cmd
}

// resolveResellerID pulls /current-user/ and digs the reseller ID out of
// the response. The Servosity API has historically wrapped the ID under
// either "reseller" (object with "id") or "reseller_id" (scalar) — we
// tolerate both rather than break if the shape shifts.
func resolveResellerID(c interface {
	GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error)
}) (string, error) {
	raw, err := c.GetWithHeaders("/current-user/", map[string]string{}, nil)
	if err != nil {
		return "", err
	}
	// /current-user/ might be returned as an array (paginated list with a
	// single user) or a flat object. Normalize.
	var asObj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asObj); err != nil {
		var asArr []map[string]json.RawMessage
		if err2 := json.Unmarshal(raw, &asArr); err2 != nil || len(asArr) == 0 {
			return "", fmt.Errorf("current-user response not an object or non-empty array")
		}
		asObj = asArr[0]
	}
	// Try common shapes in order.
	if v, ok := asObj["reseller_id"]; ok {
		if s := scalarToString(v); s != "" {
			return s, nil
		}
	}
	if v, ok := asObj["reseller"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(v, &inner); err == nil {
			if idRaw, ok := inner["id"]; ok {
				if s := scalarToString(idRaw); s != "" {
					return s, nil
				}
			}
		}
		// Sometimes reseller is itself just an ID scalar.
		if s := scalarToString(v); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("could not find reseller id on /current-user/ response (no 'reseller_id' or 'reseller.id' field)")
}

// scalarToString unwraps a JSON number or string into a flat string. Used
// because the API isn't internally consistent about whether IDs are int or
// UUID-string.
func scalarToString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var out string
		if err := json.Unmarshal(raw, &out); err == nil {
			return out
		}
	}
	return s
}

// extractBillLines decodes /resellers/{id}/bill/ into a flat slice of
// line items. The endpoint can return either a top-level array or an
// object wrapping the array under one of several keys — tolerate both.
func extractBillLines(data json.RawMessage) ([]billLineItem, error) {
	if len(data) == 0 {
		return nil, nil
	}
	// Try flat array first.
	var arr []billLineItem
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	// Try wrapped object.
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, err
	}
	for _, key := range []string{"results", "items", "line_items", "lines", "bill"} {
		if v, ok := wrap[key]; ok {
			if err := json.Unmarshal(v, &arr); err == nil {
				return arr, nil
			}
		}
	}
	return nil, fmt.Errorf("bill response shape not recognized (expected array or {results|items|line_items|lines|bill: [...]})")
}

// readInvoicesCSV parses the MSP's invoicing CSV. Header row is REQUIRED
// and must include company_id, company_name, invoiced_amount (case-insens,
// order-agnostic so MSPs can hand us whatever their accounting tool
// exports without re-arranging columns).
func readInvoicesCSV(path string) (map[string]reconcileRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	// Allow some sloppy exports (variable field counts inside reason cols).
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("CSV is empty")
		}
		return nil, err
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"company_id", "company_name", "invoiced_amount"} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("CSV missing required header column %q (got %v)", required, header)
		}
	}

	out := map[string]reconcileRow{}
	lineNum := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			return nil, fmt.Errorf("CSV line %d: %w", lineNum, err)
		}
		if len(rec) == 0 {
			continue
		}
		cid := strings.TrimSpace(rec[idx["company_id"]])
		if cid == "" {
			// Skip blank rows silently — common when MSPs hand-export.
			continue
		}
		amtStr := rec[idx["invoiced_amount"]]
		cents, err := parseMoneyToCents(amtStr)
		if err != nil {
			return nil, fmt.Errorf("CSV line %d: invoiced_amount %q: %w", lineNum, amtStr, err)
		}
		row := reconcileRow{
			CompanyID:    cid,
			CompanyName:  strings.TrimSpace(rec[idx["company_name"]]),
			InvoicedCent: cents,
		}
		out[cid] = row
	}
	return out, nil
}

// parseMoneyToCents accepts "$128.50" / "128.50" / "128" / "1,234.56" /
// "(45.00)" (accounting negative) and returns integer cents. Negative
// amounts are allowed (credits). Rounding is to nearest cent.
func parseMoneyToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	negative := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		negative = true
		s = s[1 : len(s)-1]
	}
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		negative = !negative
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("empty amount after strip")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	cents := int64(math.Round(f * 100))
	if negative {
		cents = -cents
	}
	return cents, nil
}

// joinAndScore folds bill line items and the invoice CSV into a single
// row-per-company set, computes the delta in cents, and assigns the NOTE
// column based on which side is missing (or whether |delta| crosses the
// threshold).
func joinAndScore(billLines []billLineItem, invoices map[string]reconcileRow) []reconcileRow {
	// Index bill side by company_id.
	billByID := map[string]reconcileRow{}
	for _, line := range billLines {
		cid := scalarToString(line.CompanyID)
		cents, err := billAmountToCents(line.Amount)
		if err != nil {
			// Skip un-parseable bill lines but keep going — a single
			// weird row shouldn't blow up the whole reconcile.
			continue
		}
		row := billByID[cid]
		row.CompanyID = cid
		if row.CompanyName == "" {
			row.CompanyName = line.CompanyName
		}
		row.ServosityCent += cents
		billByID[cid] = row
	}

	// Union of all company IDs.
	idSet := map[string]struct{}{}
	for cid := range billByID {
		idSet[cid] = struct{}{}
	}
	for cid := range invoices {
		idSet[cid] = struct{}{}
	}

	rows := make([]reconcileRow, 0, len(idSet))
	for cid := range idSet {
		b := billByID[cid]
		inv := invoices[cid]
		row := reconcileRow{
			CompanyID:     cid,
			CompanyName:   firstNonEmpty(inv.CompanyName, b.CompanyName),
			ServosityCent: b.ServosityCent,
			InvoicedCent:  inv.InvoicedCent,
		}
		row.DeltaCent = row.InvoicedCent - row.ServosityCent
		row.Note = classifyDrift(row)
		rows = append(rows, row)
	}

	// Sort by |delta| descending so the worst drift is at the top —
	// stable secondary sort by company_id for deterministic output.
	sort.SliceStable(rows, func(i, j int) bool {
		ai, aj := absInt64(rows[i].DeltaCent), absInt64(rows[j].DeltaCent)
		if ai != aj {
			return ai > aj
		}
		return rows[i].CompanyID < rows[j].CompanyID
	})
	return rows
}

// billAmountToCents pulls the Amount field out of a bill line item. The
// API may surface this as a number, a string, or wrap it inside an object
// (we've seen "amount" return as {"value": "12.50", "currency": "USD"} on
// some Servosity-adjacent endpoints, so be tolerant).
func billAmountToCents(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	// Number or quoted-number scalar.
	s := scalarToString(raw)
	if s != "" && s != "{" && !strings.HasPrefix(s, "{") {
		return parseMoneyToCents(s)
	}
	// Nested object.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"value", "amount", "total", "cents"} {
			if v, ok := obj[key]; ok {
				return parseMoneyToCents(scalarToString(v))
			}
		}
	}
	return 0, fmt.Errorf("unrecognized amount shape: %s", string(raw))
}

func classifyDrift(row reconcileRow) string {
	switch {
	case row.ServosityCent == 0 && row.InvoicedCent != 0:
		return "invoiced but no Servosity record"
	case row.InvoicedCent == 0 && row.ServosityCent != 0:
		return "Servosity billing not invoiced"
	case absInt64(row.DeltaCent) <= reconcileThresholdCents:
		return ""
	case row.DeltaCent > 0:
		return "client overcharged"
	case row.DeltaCent < 0:
		return "client undercharged (lost revenue)"
	}
	return ""
}

func buildReport(month string, rows []reconcileRow) reconcileReport {
	totals := map[string]int64{"servosity": 0, "invoiced": 0, "delta": 0}
	for _, r := range rows {
		totals["servosity"] += r.ServosityCent
		totals["invoiced"] += r.InvoicedCent
	}
	totals["delta"] = totals["invoiced"] - totals["servosity"]
	return reconcileReport{Month: month, Totals: totals, Rows: rows}
}

// renderPlainBill is the no-reconcile path: render the raw bill response
// in the requested format. Honors --format as a sugar over the global
// --json / --csv flags so the user gets the format they asked for even if
// they didn't combine flags.
func renderPlainBill(w io.Writer, data json.RawMessage, format string, flags *rootFlags) error {
	switch format {
	case "json":
		// Pretty JSON, no envelope — this is the "smart" command, not the
		// generated raw one, so the user expects clean output they can
		// pipe to jq without unwrapping a provenance layer.
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			// Fall back to raw bytes if it isn't valid JSON.
			_, err := w.Write(data)
			return err
		}
		return enc.Encode(v)
	case "csv":
		// Reuse the standard pipeline by forcing the flag and routing
		// through printOutputWithFlags — it knows how to flatten arrays.
		orig := flags.csv
		flags.csv = true
		defer func() { flags.csv = orig }()
		return printOutputWithFlags(w, data, flags)
	default: // "table"
		// Standard auto-table over an array of objects.
		var items []map[string]any
		if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
			return printAutoTable(w, items)
		}
		// Fallback for object-shaped responses.
		return printOutputWithFlags(w, data, flags)
	}
}

// renderReconcile writes the report in the requested format.
func renderReconcile(w io.Writer, report reconcileReport, format string, flags *rootFlags) error {
	// --json wins over --format=table (global flag is more specific).
	if flags.asJSON {
		format = "json"
	} else if flags.csv {
		format = "csv"
	}
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "csv":
		cw := csv.NewWriter(w)
		defer cw.Flush()
		if err := cw.Write([]string{"company_id", "company_name", "servosity", "invoiced", "delta", "note"}); err != nil {
			return err
		}
		for _, r := range report.Rows {
			if err := cw.Write([]string{
				r.CompanyID,
				r.CompanyName,
				formatCents(r.ServosityCent),
				formatCents(r.InvoicedCent),
				formatCentsSigned(r.DeltaCent),
				r.Note,
			}); err != nil {
				return err
			}
		}
		// Totals row.
		return cw.Write([]string{
			"", "TOTAL",
			formatCents(report.Totals["servosity"]),
			formatCents(report.Totals["invoiced"]),
			formatCentsSigned(report.Totals["delta"]),
			"",
		})
	default: // table
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "COMPANY\tSERVOSITY\tINVOICED\tDELTA\tNOTE")
		for _, r := range report.Rows {
			label := r.CompanyName
			if r.CompanyID != "" {
				label = fmt.Sprintf("%s (%s)", r.CompanyName, r.CompanyID)
			}
			if r.CompanyName == "" && r.CompanyID != "" {
				label = fmt.Sprintf("(%s)", r.CompanyID)
			}
			fmt.Fprintf(tw, "%s\t$%s\t$%s\t%s\t%s\n",
				label,
				formatCents(r.ServosityCent),
				formatCents(r.InvoicedCent),
				signedDollar(r.DeltaCent),
				r.Note,
			)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(w, "\nTOTAL servosity: $%s  invoiced: $%s  delta: %s\n",
			formatCents(report.Totals["servosity"]),
			formatCents(report.Totals["invoiced"]),
			signedDollar(report.Totals["delta"]),
		)
		return nil
	}
}

// formatCents turns integer cents into a "128.50" string (no sign, no $).
func formatCents(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	whole := c / 100
	frac := c % 100
	if neg {
		return fmt.Sprintf("-%d.%02d", whole, frac)
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}

func formatCentsSigned(c int64) string {
	if c >= 0 {
		return "+" + formatCents(c)
	}
	return formatCents(c)
}

// signedDollar formats a signed delta with a leading $ inside the sign:
// "+$21.50" / "-$15.00".
func signedDollar(c int64) string {
	if c >= 0 {
		return "+$" + formatCents(c)
	}
	return "-$" + formatCents(-c)
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
