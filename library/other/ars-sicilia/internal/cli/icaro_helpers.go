// Helpers that bridge generated Cobra commands to the hand-rolled icaroclient.
// Each <archivio>_cerca.go / <archivio>_get.go file delegates here so the
// search-engine logic lives in one place.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/cliutil"
	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

// cercaParams collects the cleaned param map the icaroclient expects, plus
// the few search-time tunables the CLI exposes.
type cercaParams struct {
	Params   map[string]string
	ISISRaw  string
	Limit    int
	MaxPages int
}

// runCerca executes a search against an archive and emits JSON or table-shaped
// output according to flags. archiveSlug names one of the entries in
// internal/icaroclient/archives.go (e.g. "leggi", "ddl").
func runCerca(cmd *cobra.Command, flags *rootFlags, archiveSlug string, p cercaParams) error {
	arc := icaro.BySlug(archiveSlug)
	if arc == nil {
		return fmt.Errorf("unknown archive slug: %q", archiveSlug)
	}
	if flags.dryRun {
		return emitDryRun(cmd, *arc, p)
	}
	if cliIsVerify() {
		return emitDryRun(cmd, *arc, p)
	}
	// Default MaxPages: if Limit is set and small, one page is enough; if
	// caller asked for >50, fan out multiple pages (Icaro paginates ~10/pg).
	maxPages := p.MaxPages
	if maxPages == 0 {
		if p.Limit > 10 {
			maxPages = (p.Limit + 9) / 10
		} else {
			maxPages = 1
		}
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params:   normalizeParams(*arc, p.Params),
		ISISRaw:  p.ISISRaw,
		Limit:    p.Limit,
		MaxPages: maxPages,
	})
	if err != nil {
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(fmt.Errorf("ricerca %s: %w", arc.Slug, err))
		}
		return fmt.Errorf("ricerca %s: %w", arc.Slug, err)
	}
	return emitRecords(cmd, flags, *arc, recs)
}

// runGet fetches and emits a single document. Get needs a fresh session, so
// we Search first with a narrow query that pins the record, then GetDoc on
// the returned docID. For the typical case where the caller passes legisl
// and numero, the query is `<legisl>.LEGISL E <numero>.<KEY>` where KEY is
// the archive-specific id field.
func runGet(cmd *cobra.Command, flags *rootFlags, archiveSlug string, legisl, numero int) error {
	arc := icaro.BySlug(archiveSlug)
	if arc == nil {
		return fmt.Errorf("unknown archive slug: %q", archiveSlug)
	}
	if flags.dryRun || cliIsVerify() {
		out := map[string]any{
			"archive": arc.Slug,
			"legisl":  legisl,
			"numero":  numero,
			"dry_run": true,
			"would_fetch": fmt.Sprintf("%s/icaro/doc%s-1.jsp?icaDocId=N&legisl=%d&numero=%d",
				icaro.DefaultBaseURL, arc.ID, legisl, numero),
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}
	params := map[string]string{}
	if legisl > 0 {
		params["legisl"] = fmt.Sprintf("%d", legisl)
	}
	if numero > 0 {
		params["numero"] = fmt.Sprintf("%d", numero)
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params: normalizeParams(*arc, params),
		Limit:  1,
	})
	if err != nil {
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(fmt.Errorf("locating document: %w", err))
		}
		return fmt.Errorf("locating document: %w", err)
	}
	if len(recs) == 0 {
		return fmt.Errorf("nessun documento trovato per legisl=%d numero=%d in %s", legisl, numero, arc.Slug)
	}
	doc, err := c.GetDoc(ctx, *arc, recs[0].DocID)
	if err != nil {
		return err
	}
	// Merge the short-list fields into the doc so callers see legisl, atto, etc.
	for k, v := range recs[0].Fields {
		if _, exists := doc.Fields[k]; !exists {
			doc.Fields[k] = v
		}
	}
	if recs[0].Excerpt != "" && doc.Body == "" {
		doc.Body = recs[0].Excerpt
	}
	return writeJSON(cmd.OutOrStdout(), doc)
}

// normalizeParams rewrites a few flag inputs to the shape the portal expects:
//   - dates given as YYYY-MM-DD become DD.MM.YYYY (Icaro's storage format)
//   - whitespace is trimmed
func normalizeParams(arc icaro.Archive, in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch k {
		case "data":
			if iso := strings.SplitN(v, "-", 3); len(iso) == 3 {
				v = fmt.Sprintf("%s.%s.%s", iso[2], iso[1], iso[0])
			}
		}
		out[k] = v
	}
	return out
}

// emitDryRun prints the would-be query without hitting the network, useful
// for --dry-run flows and Printing Press verify checks.
func emitDryRun(cmd *cobra.Command, arc icaro.Archive, p cercaParams) error {
	expr := icaro.BuildQuery(arc, normalizeParams(arc, p.Params), p.ISISRaw)
	out := map[string]any{
		"archive":     arc.Slug,
		"archive_id":  arc.ID,
		"isis_query":  expr,
		"would_fetch": fmt.Sprintf("%s/icaro/default.jsp?icaDB=%s&icaQuery=%s", icaro.DefaultBaseURL, arc.ID, expr),
		"dry_run":     true,
	}
	return writeJSON(cmd.OutOrStdout(), out)
}

// emitRecords prints search records honoring --json/--csv/table formats.
// When the user did not pass --json explicitly and stdout is a TTY, we
// produce a small table; otherwise we default to JSON for pipe friendliness.
func emitRecords(cmd *cobra.Command, flags *rootFlags, arc icaro.Archive, recs []icaro.Record) error {
	out := cmd.OutOrStdout()
	asJSON := flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain)
	if asJSON {
		// Convert to a flat shape: {doc_id, title, excerpt, url, <fields...>}.
		flat := make([]map[string]any, 0, len(recs))
		for _, r := range recs {
			row := map[string]any{
				"doc_id":  r.DocID,
				"title":   r.Title,
				"excerpt": r.Excerpt,
				"url":     r.URL,
			}
			for k, v := range r.Fields {
				row[strings.ToLower(strings.TrimSuffix(k, "."))] = v
			}
			flat = append(flat, row)
		}
		return writeJSON(out, flat)
	}
	if flags.csv {
		return writeRecordsCSV(out, arc, recs)
	}
	// Table view (default for TTY).
	if len(recs) == 0 {
		fmt.Fprintln(out, "Nessun risultato.")
		return nil
	}
	for _, r := range recs {
		fmt.Fprintf(out, "#%d  %s\n", r.DocID, r.Title)
		for i, col := range arc.Columns {
			if i == len(arc.Columns)-1 {
				continue // last col is the title block, already printed
			}
			if v, ok := r.Fields[col]; ok {
				fmt.Fprintf(out, "  %-10s %s\n", col, v)
			}
		}
		if r.Excerpt != "" {
			fmt.Fprintf(out, "  %s\n", r.Excerpt)
		}
		fmt.Fprintln(out)
	}
	return nil
}

func writeRecordsCSV(out io.Writer, arc icaro.Archive, recs []icaro.Record) error {
	// Header
	hdr := []string{"doc_id", "title", "excerpt", "url"}
	for _, c := range arc.Columns {
		hdr = append(hdr, strings.ToLower(strings.TrimSuffix(c, ".")))
	}
	for i, h := range hdr {
		if i > 0 {
			fmt.Fprint(out, ",")
		}
		fmt.Fprint(out, csvEscape(h))
	}
	fmt.Fprintln(out)
	for _, r := range recs {
		row := []string{fmt.Sprintf("%d", r.DocID), r.Title, r.Excerpt, r.URL}
		for _, c := range arc.Columns {
			row = append(row, r.Fields[c])
		}
		for i, v := range row {
			if i > 0 {
				fmt.Fprint(out, ",")
			}
			fmt.Fprint(out, csvEscape(v))
		}
		fmt.Fprintln(out)
	}
	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// cliIsVerify mirrors cliutil.IsVerifyEnv so callers can short-circuit
// outbound network calls during Printing Press verify runs.
func cliIsVerify() bool {
	return cliutil.IsVerifyEnv()
}

// itoa is a tiny shorthand so cerca-wrapper commands don't need strconv.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// atoiArg parses a positional CLI argument as an int, returning a
// human-friendly Italian error when the input is malformed.
func atoiArg(s, name string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil {
		return 0, fmt.Errorf("argomento %q non valido (atteso numero intero): %s", name, s)
	}
	return n, nil
}
