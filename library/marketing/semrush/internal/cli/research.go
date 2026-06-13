// research — the magic-recipe workflow. Takes seed keywords (from a CSV/flag),
// runs Keyword Magic Tool for each (with optional PKD against the client
// domain), applies sweet-spot filters, aggregates the results, and emits a
// single deduped JSON array ready to pipe into `sheets push`.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newResearchCmd(_ *rootFlags) *cobra.Command {
	var (
		seedsFlag    string
		seedsFile    string
		domain       string
		database     string
		currency     string
		mode         string
		limit        int
		minVolume    int
		maxKD        int
		excludeBrand bool
		excludeList  string
		dedupe       bool
		sleepMS      int
		sortField    string
		outPath      string
	)
	cmd := &cobra.Command{
		Use:   "research",
		Short: "Magic-recipe keyword research: seeds → KMT → score → dedupe → ready for Sheets",
		Long: "Runs Keyword Magic Tool against each of your seed keywords, applies " +
			"sweet-spot filters (volume / KD / PKD), dedupes across seeds, sorts, " +
			"and outputs a single JSON array. Pipe into 'sheets push' to land it " +
			"in your client template.\n\n" +
			"Seeds come from --seeds (comma-separated) or --seeds-file (CSV or " +
			"newline-separated). When --domain is set, every keyword carries a " +
			"PKD score against that domain.",
		Example: strings.Trim(`
  # Inline seeds → research with sweet-spot filters → pipe to your sheet
  semrush-pp-cli research --seeds "seo audit,site audit,technical seo" \
    --domain client.com --database us \
    --min-volume 100 --max-pkd 30 \
    | semrush-pp-cli sheets push <SHEET_ID> --tab "Research"

  # Seeds from a CSV exported from Google Analytics
  semrush-pp-cli research --seeds-file ga-conversion-pages.csv \
    --domain client.com --database us --min-volume 50

  # All matches per seed (broad + related + questions in one pass)
  semrush-pp-cli research --seeds "tiles" --domain nationaltiles.com.au \
    --database au --currency AUD --mode broad,related,questions
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			seeds, err := loadSeeds(seedsFlag, seedsFile)
			if err != nil {
				return err
			}
			if len(seeds) == 0 {
				return fmt.Errorf("no seeds provided — use --seeds or --seeds-file")
			}
			p := ResearchPipelineParams{
				Seeds:        seeds,
				Modes:        parseModes(mode),
				Domain:       domain,
				Database:     database,
				Currency:     currency,
				Limit:        limit,
				MinVolume:    minVolume,
				MaxKD:        maxKD,
				ExcludeBrand: excludeBrand,
				ExcludeList:  excludeList,
				Dedupe:       dedupe,
				SleepMS:      sleepMS,
				SortField:    sortField,
				LogTo:        cmd.ErrOrStderr(),
			}
			allRows, err := RunResearchPipeline(cmd.Context(), p)
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(allRows, "", "  ")
			if outPath != "" {
				if err := os.WriteFile(outPath, out, 0o644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d rows → %s\n", len(allRows), outPath)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&seedsFlag, "seeds", "", "Comma-separated seed keywords (e.g. 'seo audit,site audit')")
	cmd.Flags().StringVar(&seedsFile, "seeds-file", "", "Path to CSV or newline-separated seed file")
	cmd.Flags().StringVar(&domain, "domain", "", "Target client domain (enables PKD scoring)")
	cmd.Flags().StringVar(&database, "database", "us", "Country database (us, au, uk, ca, etc.)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code (USD, AUD, GBP, EUR, etc.)")
	cmd.Flags().StringVar(&mode, "mode", "broad", "Mode(s) — comma-separated: broad, phrase, related, questions")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max keywords per seed × mode (1-100)")
	cmd.Flags().IntVar(&minVolume, "min-volume", 0, "Filter: minimum search volume")
	cmd.Flags().IntVar(&maxKD, "max-kd", 0, "Filter: maximum Keyword Difficulty (KD%) — 0 to disable")
	cmd.Flags().BoolVar(&excludeBrand, "exclude-self-brand", true, "Drop keywords containing your --domain's brand. Disable with --exclude-self-brand=false.")
	cmd.Flags().StringVar(&excludeList, "exclude", "", "Comma-separated competitor brand terms to exclude (e.g. 'beaumont,bunnings,anaconda')")
	cmd.Flags().BoolVar(&dedupe, "dedupe", true, "Skip keywords already seen across seeds/modes")
	cmd.Flags().IntVar(&sleepMS, "sleep-ms", 250, "Pause between API calls to avoid rate limiting (default 250ms)")
	cmd.Flags().StringVar(&sortField, "sort", "volume", "Sort field: volume, difficulty, domain_difficulty, cpc")
	cmd.Flags().StringVar(&outPath, "output", "", "Write JSON to file instead of stdout")
	return cmd
}

// ResearchPipelineParams is the input shape for RunResearchPipeline. Used by
// the `research` command (its RunE wraps this) and by other commands like
// `client onboard` that need the same seed × mode × filter × dedupe loop.
type ResearchPipelineParams struct {
	Seeds        []string
	Modes        []string
	Domain       string
	Database     string
	Currency     string
	Limit        int
	MinVolume    int
	MaxKD        int
	ExcludeBrand bool
	ExcludeList  string
	Dedupe       bool
	SleepMS      int
	SortField    string
	LogTo        io.Writer // optional progress log (typically os.Stderr or cmd.ErrOrStderr())
}

// kmtNotRankingPosition is SEMrush KMT's sentinel value for "the target
// domain does not rank within the tracked top 100 for this keyword". The
// raw int 255 is a system value, not a real ranking — when this appears,
// the deliverable should show "100+" so a human reader doesn't mistake it
// for an actual rank, and the corresponding traffic estimate should be
// blanked (the small number SEMrush returns alongside is meaningless once
// you know the domain isn't ranking).
const kmtNotRankingPosition = 255

// deriveDisplayFields populates `_position_display`, `_traffic_display`, and
// `_trend_summary` on a KMT row, applying the not-ranking sentinel rule and
// summarizing the 12-month trend array. Called once per row after the KMT
// response is unpacked.
func deriveDisplayFields(r map[string]any) {
	pos := keywordIntField(r, "domain_position")
	if pos == kmtNotRankingPosition {
		r["_position_display"] = "100+"
		r["_traffic_display"] = ""
	} else {
		r["_position_display"] = pos
		if t, ok := r["domain_traffic"]; ok {
			r["_traffic_display"] = t
		}
	}
	r["_trend_summary"] = summarizeTrend(r["trend"])
}

// summarizeTrend compares the last 3 months of the 12-month trend array
// against the prior 9 months and returns a human-readable label.
//
// SEMrush returns `trend` as 12 floats (most recent month last), each a 0..1
// ratio of the keyword's peak monthly volume. The output is one of:
//   - "Growing"   — last 3 months avg ≥ 1.25× prior 9 months
//   - "Declining" — last 3 months avg ≤ 0.75× prior 9 months
//   - "Seasonal"  — large peak/trough swing within 12 months (max/min ratio ≥ 2)
//   - "Stable"    — none of the above
//   - ""          — no trend data
func summarizeTrend(v any) string {
	arr, ok := v.([]any)
	if !ok || len(arr) < 6 {
		return ""
	}
	vals := make([]float64, 0, len(arr))
	for _, x := range arr {
		switch n := x.(type) {
		case float64:
			vals = append(vals, n)
		case int:
			vals = append(vals, float64(n))
		}
	}
	if len(vals) < 6 {
		return ""
	}
	// Last 3 vs prior; require enough history to be meaningful
	splitAt := len(vals) - 3
	if splitAt < 3 {
		return ""
	}
	recentAvg := avgFloat(vals[splitAt:])
	priorAvg := avgFloat(vals[:splitAt])
	// Seasonality: large peak-to-trough swing
	minV, maxV := vals[0], vals[0]
	for _, x := range vals {
		if x < minV {
			minV = x
		}
		if x > maxV {
			maxV = x
		}
	}
	seasonal := minV > 0 && maxV/minV >= 2.0
	switch {
	case priorAvg > 0 && recentAvg/priorAvg >= 1.25:
		return "Growing"
	case priorAvg > 0 && recentAvg/priorAvg <= 0.75:
		return "Declining"
	case seasonal:
		return "Seasonal"
	default:
		return "Stable"
	}
}

func avgFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// IntentCodeToString maps SEMrush KMT intent codes to human-readable labels.
// The intents field in KMT responses is an array of integer codes; this
// function joins multiple intents with a comma if present.
//
// SEMrush intent codes (confirmed via product docs):
//   0 = Commercial    — buying intent, product/category searches
//   1 = Informational — content/learning intent
//   2 = Navigational  — brand searches (going to a specific site)
//   3 = Transactional — high-purchase-intent ("buy X now")
func IntentCodeToString(v any) string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	labels := []string{"Commercial", "Informational", "Navigational", "Transactional"}
	var parts []string
	seen := map[string]bool{}
	for _, raw := range arr {
		var code int
		switch n := raw.(type) {
		case float64:
			code = int(n)
		case int:
			code = n
		default:
			continue
		}
		if code < 0 || code >= len(labels) {
			continue
		}
		if seen[labels[code]] {
			continue
		}
		seen[labels[code]] = true
		parts = append(parts, labels[code])
	}
	return strings.Join(parts, ", ")
}

// RunResearchPipeline executes the seed × mode loop against the KMT gateway,
// applies value + brand filters per call, dedupes across all calls, sorts by
// SortField (default "volume" descending), and returns the aggregated row
// set. Each row carries `_seed` and `_mode` audit-trail fields.
func RunResearchPipeline(ctx context.Context, p ResearchPipelineParams) ([]map[string]any, error) {
	if p.LogTo == nil {
		p.LogTo = io.Discard
	}
	if len(p.Modes) == 0 {
		p.Modes = []string{"broad"}
	}

	fmt.Fprintf(p.LogTo,
		"research: %d seeds × %d mode(s) against domain=%q database=%q (limit=%d, min-vol=%d, max-kd=%d)\n",
		len(p.Seeds), len(p.Modes), p.Domain, p.Database, p.Limit, p.MinVolume, p.MaxKD)

	pause := time.Duration(p.SleepMS) * time.Millisecond
	brandFilter := newBrandFilter(p.Domain, p.ExcludeBrand, p.ExcludeList)
	if len(brandFilter.terms) > 0 {
		fmt.Fprintf(p.LogTo, "research: brand-excluding phrases that contain: %s\n", strings.Join(brandFilter.terms, ", "))
	}

	seen := make(map[string]bool)
	var allRows []map[string]any

	for si, seed := range p.Seeds {
		for mi, m := range p.Modes {
			modeCode, qOnly := resolveModeCode(m)
			params := buildKMTParams(seed, modeCode, qOnly, p.Domain, p.Database, p.Currency, p.Limit, 1)
			fmt.Fprintf(p.LogTo, "  [%d/%d %d/%d] seed=%q mode=%s …", si+1, len(p.Seeds), mi+1, len(p.Modes), seed, m)

			result, err := callKMTGateway(ctx, "ideas.GetKeywords", params, 60*time.Second)
			if err != nil {
				fmt.Fprintf(p.LogTo, " error: %v\n", err)
				continue
			}
			rows := extractKMTRows(result)
			before := len(rows)
			rows = applyResearchFilters(rows, p.MinVolume, p.MaxKD, 0)
			rows, _ = applyBrandFilter(rows, brandFilter)
			after := len(rows)
			for _, r := range rows {
				phrase, _ := r["phrase"].(string)
				if phrase == "" {
					phrase = fmt.Sprintf("%v", r["phrase"])
				}
				if p.Dedupe && seen[phrase] {
					continue
				}
				seen[phrase] = true
				r["_seed"] = seed
				r["_mode"] = m
				// Derive human-readable intent label from the raw `intents`
				// code array. Sheets-writers reference this as the `_type` field.
				r["_type"] = IntentCodeToString(r["intents"])
				// Apply the not-ranking sentinel rule for Position/Est Traffic
				// so the deliverable shows "100+" and a blank traffic cell
				// when the domain doesn't rank in the top 100.
				deriveDisplayFields(r)
				allRows = append(allRows, r)
			}
			fmt.Fprintf(p.LogTo, " %d→%d filtered (total=%d)\n", before, after, len(allRows))
			if pause > 0 && (si+1 < len(p.Seeds) || mi+1 < len(p.Modes)) {
				time.Sleep(pause)
			}
		}
	}

	sortField := p.SortField
	if sortField == "" {
		sortField = "volume"
	}
	sort.SliceStable(allRows, func(i, j int) bool {
		a := numField(allRows[i], sortField)
		b := numField(allRows[j], sortField)
		return a > b
	})
	return allRows, nil
}

// loadSeeds parses seeds from --seeds (comma) and/or --seeds-file (CSV or NL).
// Whitespace trimmed; empty lines and lines starting with '#' ignored.
func loadSeeds(inline, file string) ([]string, error) {
	var seeds []string
	if inline != "" {
		for _, s := range strings.Split(inline, ",") {
			if s = strings.TrimSpace(s); s != "" {
				seeds = append(seeds, s)
			}
		}
	}
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// If CSV, take the first column
			if i := strings.IndexByte(line, ','); i >= 0 {
				line = strings.TrimSpace(line[:i])
			}
			// Strip optional surrounding quotes
			line = strings.Trim(line, `"`)
			if line != "" && !strings.EqualFold(line, "keyword") && !strings.EqualFold(line, "seed") {
				seeds = append(seeds, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	return seeds, nil
}

func parseModes(s string) []string {
	if s == "" {
		return []string{"broad"}
	}
	var out []string
	for _, m := range strings.Split(s, ",") {
		if m = strings.TrimSpace(strings.ToLower(m)); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func resolveModeCode(name string) (int, bool) {
	if name == "questions" {
		return kmtModes["phrase"], true
	}
	if code, ok := kmtModes[name]; ok {
		return code, false
	}
	return kmtModes["broad"], false
}

func buildKMTParams(seed string, mode int, questionsOnly bool, domain, database, currency string, limit, page int) kmtParams {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	orderField := "volume"
	if mode == 3 {
		orderField = "relation_level"
	}
	return kmtParams{
		Mode:     mode,
		Currency: strings.ToUpper(currency),
		Database: database,
		Filter: kmtFilter{
			Phrase:           []any{},
			CompetitionLevel: []any{},
			CPC:              []any{},
			Difficulty:       []any{},
			Results:          []any{},
			SERPFeatures:     []kmtSERPFeatureGroup{{Inverted: false, Value: []any{}}},
			Volume:           []any{},
			WordsCount:       []any{},
			DomainDifficulty: []any{},
		},
		Groups:        []any{},
		Order:         kmtOrder{Direction: 1, Field: orderField},
		GroupsOrder:   kmtOrder{Direction: 1, Field: "count"},
		Phrase:        seed,
		QuestionsOnly: questionsOnly,
		Domain:        domain,
		Page:          kmtPage{Number: page, Size: limit},
	}
}

func extractKMTRows(result any) []map[string]any {
	// kmtgw's ideas.GetKeywords response is sometimes a top-level array,
	// sometimes an object wrapping a "list"/"keywords"/"items" key. Use the
	// flex extractor (shared with keyword-magic) to handle both shapes.
	rows, _ := extractKMTRowsAndBackref(result)
	return rows
}

func castRowSlice(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func applyResearchFilters(rows []map[string]any, minVolume, maxKD, maxPKD int) []map[string]any {
	if minVolume == 0 && maxKD == 0 && maxPKD == 0 {
		return rows
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if minVolume > 0 && keywordIntField(r, "volume", "search_volume", "Volume") < minVolume {
			continue
		}
		if maxKD > 0 && keywordIntField(r, "difficulty", "kd") > maxKD {
			continue
		}
		if maxPKD > 0 && keywordIntField(r, "domain_difficulty") > maxPKD {
			continue
		}
		out = append(out, r)
	}
	return out
}

func numField(r map[string]any, name string) float64 {
	if v, ok := r[name]; ok {
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		}
	}
	return 0
}

// _ keeps context import used if we expand
var _ = context.Background
