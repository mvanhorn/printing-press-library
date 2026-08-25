// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored helpers shared by the scan-backed commands (check, report,
// advice, gate, compare, batch). The upstream API is a single stateless POST,
// so these helpers own calling it, persisting each result to the local scan
// store, and rendering a report.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

// categoryOrder is the canonical display order of the five check categories.
var categoryOrder = []string{"discoverability", "contentAccessibility", "botAccessControl", "discovery", "commerce"}

var categoryLabel = map[string]string{
	"discoverability":      "Discoverability",
	"contentAccessibility": "Content Accessibility",
	"botAccessControl":     "Bot Access Control",
	"discovery":            "API / Auth / MCP",
	"commerce":             "Commerce",
}

// profileCategories maps a site-type profile to the categories it displays.
// "all" (or empty) shows every category.
var profileCategories = map[string][]string{
	"content": {"discoverability", "contentAccessibility", "botAccessControl"},
	"apiapp":  {"discovery", "commerce"},
}

// performScan calls POST /api/scan for url and returns the raw report.
func performScan(ctx context.Context, flags *rootFlags, url string) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, _, err := c.PostWithParams(ctx, "/api/scan", map[string]string{}, map[string]any{"url": url})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	return data, nil
}

// mustJSON marshals v, falling back to an empty array on error so callers can
// pass the result straight to printOutputWithFlags.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("[]")
	}
	return b
}

// loadStore reads all scan records from the default store path.
func loadStore() ([]store.ScanRecord, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, err
	}
	return store.Load(path)
}

// sourceOrDefault returns the active scanner, defaulting to isitagentready.
// newRootCmd's StringVar registration normally fills flags.source in, but a
// rootFlags built directly (tests, or any future caller that bypasses the
// Cobra wiring) would leave it empty — and an empty source silently filters
// the whole scan store to nothing. Default here rather than fail open.
func (f *rootFlags) sourceOrDefault() string {
	if f.source == "" {
		return store.SourceIsItAgentReady
	}
	return f.source
}

// performScanFor fetches a report from the source selected by --source.
func performScanFor(ctx context.Context, flags *rootFlags, url string) (json.RawMessage, error) {
	return performScanForSource(ctx, flags, flags.sourceOrDefault(), url)
}

// performScanForSource fetches a report from an explicit source, decoupled from
// --source. crossref uses it to fetch BOTH scanners for one URL.
func performScanForSource(ctx context.Context, flags *rootFlags, source, url string) (json.RawMessage, error) {
	switch source {
	case store.SourceIsAgentic:
		return fetchAgenticReport(ctx, flags, url)
	default: // isitagentready
		return performScan(ctx, flags, url)
	}
}

// persistScanFor appends a scan to the local store with its provenance, parsing
// per source so each scanner's headline lands in the right field.
func persistScanFor(flags *rootFlags, raw json.RawMessage) {
	persistScanForSource(flags, flags.sourceOrDefault(), raw)
}

func persistScanForSource(flags *rootFlags, source string, raw json.RawMessage) {
	path, err := store.DefaultPath()
	if err != nil {
		return
	}
	switch source {
	case store.SourceIsAgentic:
		rep, err := store.ParseAgenticReport(raw)
		if err != nil {
			return
		}
		score := rep.Score // local so the pointer is stable
		rec := store.ScanRecord{
			URL:        rep.Target,
			Source:     store.SourceIsAgentic,
			ScannedAt:  rep.ScannedAt,
			Score:      &score,
			ScoreLabel: rep.ScoreLabel,
			Raw:        raw,
		}
		if err := store.Append(path, rec); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save scan to local history: %v\n", err)
		}
	default: // isitagentready — mirrors persistScan but labels the source
		rep, err := store.ParseReport(raw)
		if err != nil {
			return
		}
		rec := store.ScanRecord{URL: rep.URL, Source: store.SourceIsItAgentReady, ScannedAt: rep.ScannedAt, Level: rep.Level, LevelName: rep.LevelName, Raw: raw}
		if err := store.Append(path, rec); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save scan to local history: %v\n", err)
		}
	}
}

// loadStoreFor reads the local store filtered to the active --source, so no
// command can compare two scanners' incompatible numbers.
func loadStoreFor(flags *rootFlags) ([]store.ScanRecord, error) {
	return loadStoreForSource(flags, flags.sourceOrDefault())
}

func loadStoreForSource(flags *rootFlags, source string) ([]store.ScanRecord, error) {
	recs, err := loadStore()
	if err != nil {
		return nil, err
	}
	return store.FilterSource(recs, source), nil
}

// buildScanRecord parses a raw report into a persisted ScanRecord with the
// correct headline for its source: level fields for isitagentready, score
// fields for is-agentic. Used by batch to build its fan-out rows carrying the
// right provenance.
func buildScanRecord(flags *rootFlags, raw json.RawMessage) (store.ScanRecord, error) {
	if flags.sourceOrDefault() == store.SourceIsAgentic {
		rep, err := store.ParseAgenticReport(raw)
		if err != nil {
			return store.ScanRecord{}, err
		}
		score := rep.Score
		return store.ScanRecord{URL: rep.Target, Source: store.SourceIsAgentic, ScannedAt: rep.ScannedAt, Score: &score, ScoreLabel: rep.ScoreLabel, Raw: raw}, nil
	}
	rep, err := store.ParseReport(raw)
	if err != nil {
		return store.ScanRecord{}, err
	}
	return store.ScanRecord{URL: rep.URL, Source: store.SourceIsItAgentReady, ScannedAt: rep.ScannedAt, Level: rep.Level, LevelName: rep.LevelName, Raw: raw}, nil
}

// resolveReport returns a raw report for url according to the --data-source
// flag, bounding the work with the root --timeout. Used by view commands
// (report, advice) so they do not re-scan on every call.
func resolveReport(cmd *cobra.Command, flags *rootFlags, url string) (json.RawMessage, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	return resolveReportCtx(ctx, flags, url)
}

// resolveReportCtxForSource is resolveReportCtx with an explicit source instead
// of flags.source. crossref uses it to resolve BOTH scanners for one URL the
// same way the single-source commands resolve one (live / local / auto).
func resolveReportCtxForSource(ctx context.Context, flags *rootFlags, source, url string) (json.RawMessage, error) {
	switch flags.dataSource {
	case "live":
		raw, err := performScanForSource(ctx, flags, source, url)
		if err != nil {
			return nil, err
		}
		persistScanForSource(flags, source, raw)
		return raw, nil
	case "local":
		recs, err := loadStoreForSource(flags, source)
		if err != nil {
			return nil, err
		}
		rec, ok := store.Latest(recs, url)
		if !ok {
			return nil, notFoundErr(fmt.Errorf("no stored %s scan for %s; run 'isitagentready-pp-cli check %s --source %s' first", source, url, url, source))
		}
		return rec.Raw, nil
	default: // auto
		recs, err := loadStoreForSource(flags, source)
		if err != nil {
			return nil, err
		}
		if rec, ok := store.Latest(recs, url); ok {
			return rec.Raw, nil
		}
		raw, err := performScanForSource(ctx, flags, source, url)
		if err != nil {
			return nil, err
		}
		persistScanForSource(flags, source, raw)
		return raw, nil
	}
}

// resolveReportCtx is resolveReport with an explicit context so callers that
// fan out (compare) can share one bounded context: live forces a fresh scan,
// local reads the latest stored scan (and errors if none), auto reads the
// latest stored scan or scans when none exists.
func resolveReportCtx(ctx context.Context, flags *rootFlags, url string) (json.RawMessage, error) {
	switch flags.dataSource {
	case "live":
		raw, err := performScanFor(ctx, flags, url)
		if err != nil {
			return nil, err
		}
		persistScanFor(flags, raw)
		return raw, nil
	case "local":
		recs, err := loadStoreFor(flags)
		if err != nil {
			return nil, err
		}
		rec, ok := store.Latest(recs, url)
		if !ok {
			return nil, notFoundErr(fmt.Errorf("no stored %s scan for %s; run 'isitagentready-pp-cli check %s --source %s' first", flags.sourceOrDefault(), url, url, flags.sourceOrDefault()))
		}
		return rec.Raw, nil
	default: // auto
		recs, err := loadStoreFor(flags)
		if err != nil {
			return nil, err
		}
		if rec, ok := store.Latest(recs, url); ok {
			return rec.Raw, nil
		}
		raw, err := performScanFor(ctx, flags, url)
		if err != nil {
			return nil, err
		}
		persistScanFor(flags, raw)
		return raw, nil
	}
}

// renderScan prints a scan report: raw JSON (with --select/--compact) for
// machine output, or a human summary for a terminal.
func renderScan(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage) error {
	return renderScanFor(cmd, flags, raw, wantsHumanTable(cmd.OutOrStdout(), flags))
}

// renderScanFor is renderScan with the human/machine decision passed in, and
// with the human renderer chosen by --source.
//
// An is-agentic report shares no fields with an isitagentready one, and
// store.ParseReport unmarshals it WITHOUT error into an all-zero Report — so
// routing it through the level renderer printed an empty URL, "Level 0" and
// zero checks instead of the real score and issues. Dispatch on the source
// before rendering, the same way report/crossref already do.
//
// human is a parameter rather than a recomputed wantsHumanTable call so tests
// can exercise the terminal path: wantsHumanTable gates it behind isTerminal,
// which is false for the buffer a test renders into.
func renderScanFor(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage, human bool) error {
	if !human {
		return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
	}
	if flags.sourceOrDefault() == store.SourceIsAgentic {
		// tier "" — check has no --category/--tier filter, so show both tiers.
		return renderAgenticReport(cmd, raw, "")
	}
	return renderScanLevels(cmd, flags, raw)
}

// renderScanLevels prints the isitagentready level-based human summary.
func renderScanLevels(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage) error {
	rep, err := store.ParseReport(raw)
	if err != nil {
		return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, bold(rep.URL))
	if rep.SiteError != nil {
		fmt.Fprintf(out, "  %s HTTP %d %s — the scanner could not fetch this target.\n",
			red("site error:"), rep.SiteError.HTTPStatus, rep.SiteError.StatusText)
		return nil
	}
	fmt.Fprintf(out, "  Level %d — %s\n", rep.Level, rep.LevelName)
	pass, fail, neutral, total := rep.Counts()
	fmt.Fprintf(out, "  Checks: %s, %s, %d neutral (of %d)\n", green(fmt.Sprintf("%d pass", pass)), red(fmt.Sprintf("%d fail", fail)), neutral, total)
	flat := rep.FlatChecks()
	byCat := map[string][]store.CheckRef{}
	for _, c := range flat {
		byCat[c.Category] = append(byCat[c.Category], c)
	}
	for _, cat := range categoryOrder {
		checks := byCat[cat]
		if len(checks) == 0 {
			continue
		}
		p := 0
		for _, c := range checks {
			if c.Status == "pass" {
				p++
			}
		}
		fmt.Fprintf(out, "    %-22s %d/%d pass\n", labelFor(cat), p, len(checks))
	}
	if len(rep.NextLevel.Requirements) > 0 {
		next := rep.NextLevel.Name
		if next == "" {
			next = "next level"
		}
		fmt.Fprintf(out, "  Next: %s needs %d fix(es). Run 'isitagentready-pp-cli advice %s' for the prompts.\n",
			next, len(rep.NextLevel.Requirements), rep.URL)
	}
	return nil
}

func labelFor(cat string) string {
	if l, ok := categoryLabel[cat]; ok {
		return l
	}
	return cat
}

// sortedCategories returns the report's categories in canonical order, with any
// unknown categories appended alphabetically.
func sortedCategories(rep *store.Report) []string {
	present := map[string]bool{}
	for cat := range rep.Checks {
		present[cat] = true
	}
	var out []string
	for _, cat := range categoryOrder {
		if present[cat] {
			out = append(out, cat)
			delete(present, cat)
		}
	}
	var rest []string
	for cat := range present {
		rest = append(rest, cat)
	}
	sort.Strings(rest)
	return append(out, rest...)
}
