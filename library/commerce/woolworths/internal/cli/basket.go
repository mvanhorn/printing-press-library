// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. The RunE body below is implemented; do not
// replace it with a scaffold.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
	"github.com/spf13/cobra"
)

// basketHistoryDelta is how a line's current price sits against that same
// product's own recent local history. Nil when nothing has been recorded.
type basketHistoryDelta struct {
	Median   float64 `json:"median"`
	Samples  int     `json:"samples"`
	Window   string  `json:"window"`
	Delta    float64 `json:"delta"`
	DeltaPct float64 `json:"delta_pct"`
}

// basketLine is one line of the list file after resolution.
type basketLine struct {
	Line     int    `json:"line"`
	Query    string `json:"query"`
	Resolved bool   `json:"resolved"`

	Stockcode   string  `json:"stockcode,omitempty"`
	Name        string  `json:"name,omitempty"`
	Brand       string  `json:"brand,omitempty"`
	PackageSize string  `json:"package_size,omitempty"`
	Price       float64 `json:"price,omitempty"`
	IsOnSpecial bool    `json:"is_on_special,omitempty"`
	IsHalfPrice bool    `json:"is_half_price,omitempty"`
	WasPrice    float64 `json:"was_price,omitempty"`
	CupString   string  `json:"cup_string,omitempty"`

	MatchScore  float64             `json:"match_score,omitempty"`
	MatchRank   int                 `json:"match_rank,omitempty"`
	MatchReason string              `json:"match_reason,omitempty"`
	History     *basketHistoryDelta `json:"history,omitempty"`

	// Reason is filled for every unresolved line and is never empty when
	// Resolved is false. An unresolved line contributes nothing to the total.
	Reason string `json:"reason,omitempty"`
	// Fetched is false when the lookup itself failed rather than simply not
	// matching, so a transport failure is never confused with a bad line.
	FetchFailed bool `json:"fetch_failed,omitempty"`
}

type basketPreviousRun struct {
	RanAt        int64   `json:"ran_at"`
	Total        float64 `json:"total"`
	ResolvedRows int     `json:"resolved_lines"`
}

type basketEnvelope struct {
	ListFile        string             `json:"list_file"`
	ListKey         string             `json:"list_key"`
	Lines           []basketLine       `json:"lines"`
	LinesRead       int                `json:"lines_read"`
	LinesSkipped    int                `json:"lines_skipped_blank_or_comment"`
	ResolvedCount   int                `json:"resolved_count"`
	UnresolvedCount int                `json:"unresolved_count"`
	Total           float64            `json:"total"`
	TotalBasis      string             `json:"total_basis"`
	SpecialsCount   int                `json:"specials_count"`
	PreviousRun     *basketPreviousRun `json:"previous_run,omitempty"`
	TotalDelta      float64            `json:"total_delta_vs_previous,omitempty"`
	TotalDeltaPct   float64            `json:"total_delta_pct_vs_previous,omitempty"`
	HistoryAvail    bool               `json:"history_available"`
	Recorded        bool               `json:"recorded"`
	FetchFailures   []wowFetchFailure  `json:"fetch_failures"`
	Note            string             `json:"note,omitempty"`
}

func newNovelBasketCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit  int
		flagDB     string
		flagRecord bool
	)

	cmd := &cobra.Command{
		Use:   "basket <list-file>",
		Short: "Prices a whole shopping list and shows what moved since the last run",
		Long: strings.Trim(`
Prices a plain-text shopping list: one item per line, blank lines and lines
starting with # ignored.

Each line is resolved to a product deterministically — search rank plus token
overlap against the product name, brand and pack size. There is no semantic
matching and no model in the loop, so the same list resolves to the same
products every time, and a line that does not genuinely match anything is
reported as unresolved with the reason rather than being quietly attached to
whatever the search engine happened to return first.

Unresolved lines never enter the total. A line that could not be looked up at
all is marked fetch_failed and is likewise excluded, so a network blip can
never show up as a $0.00 item that makes the basket look cheaper than it is.
The total_basis field always names how many lines the total actually covers.

Where the local price-history database has seen a product before, the line also
carries the delta against that product's own recent median. Each run records
its total against a key derived from the list contents, so the next run over the
same list reports the movement in the basket total.
`, "\n"),
		Example: strings.Trim(`
  # Price a weekly list
  woolworths-pp-cli basket ./groceries.txt

  # Machine-readable, capped at the first 20 lines
  woolworths-pp-cli basket ./groceries.txt --limit 20 --json

  # Price without writing this run's total to the local database
  woolworths-pp-cli basket ./groceries.txt --record=false --agent
`, "\n"),
		// No pp:happy-args: the single positional is a path to a file on the
		// caller's disk, and there is no path the live matrix could supply
		// that would exist. A bare invocation prints help instead.
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// The dry-run short-circuit comes BEFORE any filesystem access so
			// `basket ./nonexistent.txt --dry-run` reports the no-op instead of
			// failing on a file the dry run was never going to read.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, func() string {
					if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
						return "basket " + strings.TrimSpace(args[0])
					}
					return "basket"
				}())
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<list-file> is required"))
			}
			listPath := args[0]
			limit := flagLimit
			if limit <= 0 {
				limit = 100
			}

			// Path admission control. It sits AFTER dryRunOK, so a dry run
			// over a path that does not exist still exits 0, and BEFORE the
			// first byte is read, because this command echoes what it reads.
			if err := basketCheckListPath(listPath); err != nil {
				return err
			}

			items, skipped, err := basketReadList(listPath, limit)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Akamai drops unwarmed POST connections; one GET of /shop primes the jar.
			c.WarmSession(ctx)

			env := basketEnvelope{
				ListFile:      listPath,
				ListKey:       basketListKey(items),
				Lines:         make([]basketLine, 0, len(items)),
				LinesRead:     len(items),
				LinesSkipped:  skipped,
				FetchFailures: make([]wowFetchFailure, 0),
			}

			db, closeDB, haveDB, dbErr := basketOpenDB(ctx, flagDB, flagRecord)
			defer closeDB()
			if dbErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "basket: warning: opening the local price-history database at %s failed: %v\n", basketResolveDBPath(flagDB), dbErr)
			}

			attempted := 0
			var histProbe wowHistoryProbe
			for _, item := range items {
				attempted++
				line := basketResolveLine(ctx, c, item)
				if line.FetchFailed {
					env.FetchFailures = append(env.FetchFailures, wowFetchFailure{
						Kind:  "search",
						Query: basketClipEcho(item.text),
						Page:  1,
						Error: line.Reason,
					})
				}
				if line.Resolved && haveDB {
					hist, herr := basketRecentMedian(ctx, db, line.Stockcode, line.Price)
					if histProbe.record(herr) {
						line.History = hist
					}
				}
				env.Lines = append(env.Lines, line)
			}

			total := 0.0
			for _, line := range env.Lines {
				if !line.Resolved {
					env.UnresolvedCount++
					continue
				}
				env.ResolvedCount++
				total += line.Price
				if line.IsOnSpecial {
					env.SpecialsCount++
				}
			}
			env.Total = roundMoney(total)
			env.TotalBasis = fmt.Sprintf("%d of %d line(s); %d unresolved line(s) contribute nothing and are not counted as $0.00",
				env.ResolvedCount, env.LinesRead, env.UnresolvedCount)

			if haveDB {
				prev, perr := basketPreviousTotal(ctx, db, env.ListKey)
				if histProbe.record(perr) && prev != nil {
					env.PreviousRun = prev
					env.TotalDelta = roundMoney(env.Total - prev.Total)
					if prev.Total > 0 {
						env.TotalDeltaPct = roundMoney((env.Total - prev.Total) / prev.Total * 100)
					}
				}
				if flagRecord && env.ResolvedCount > 0 {
					if rerr := basketRecordRun(ctx, db, env.ListKey, env.Total, env.ResolvedCount, env.UnresolvedCount); rerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "basket: warning: recording this run's total failed, so the next run will report no delta: %v\n", rerr)
					} else {
						env.Recorded = true
					}
				}
			}
			env.HistoryAvail = histProbe.available(haveDB)
			histProbe.warn("basket")

			if env.LinesRead == 0 {
				env.Note = fmt.Sprintf("%s contained no item lines; blank lines and lines starting with # are ignored", listPath)
			} else if env.ResolvedCount == 0 {
				env.Note = fmt.Sprintf("none of the %d line(s) resolved to a product; the total is $0.00 because nothing was priced, not because the shopping is free", env.LinesRead)
			} else if dbErr != nil {
				env.Note = fmt.Sprintf("the local price-history database at %s could not be opened (%v), so per-line medians and the previous-total comparison are unavailable; this is a database error, not an absent history", basketResolveDBPath(flagDB), dbErr)
			} else if !haveDB {
				env.Note = "no local price-history database exists yet; per-line medians and the previous-total comparison are unavailable until 'real-special' has recorded some observations (sync does not build price history on this API)"
			}

			// The denominator is every fetch that was ATTEMPTED, which is one
			// per item line, not the number that resolved to a product: a line
			// can be fetched successfully and still match nothing.
			wowWarnFailures("basket", env.FetchFailures, attempted-len(env.FetchFailures))
			return basketEmit(cmd, flags, env)
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum item lines to read from the list file")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().BoolVar(&flagRecord, "record", true, "Record this run's total locally so the next run can report the delta; --record=false prices without writing")
	return cmd
}

// basketItem is one non-blank, non-comment line of the list file.
type basketItem struct {
	line int
	text string
}

// ---------------------------------------------------------------------------
// List-file admission control.
//
// basket echoes every non-comment line of the file it is handed back to the
// caller — into basketLine.Query, into Reason, and onto stdout. It is also
// registered as an MCP tool whose `path` argument passes straight through from
// an untrusted caller. Without the checks below, "price my shopping list" is an
// arbitrary file read: this CLI's own cookies.json holds w-rctx and
// wow-auth-token, and .env files and ~/.ssh/id_rsa are equally plain text.
//
// Four independent gates, none of which is load-bearing on its own:
// an extension allowlist, a refusal to read out of the CLI's own private
// directories, a regular-file-only check, and a size cap.
// ---------------------------------------------------------------------------

// basketAllowedListExtensions are the only extensions basket will open. The
// allowlist alone blocks cookies.json, .env and id_rsa.
var basketAllowedListExtensions = map[string]bool{
	".txt":  true,
	".md":   true,
	".list": true,
	".csv":  true,
}

// basketMaxListBytes caps the file size basket will read. A shopping list is a
// few kilobytes; a megabyte of it is not a shopping list.
const basketMaxListBytes = 1 << 20

// basketMaxEchoRunes caps how much of a line is echoed back into Query or
// Reason. It counts RUNES, not bytes, so a multi-byte character is never sliced
// in half into invalid UTF-8.
const basketMaxEchoRunes = 120

// basketClipEcho returns at most basketMaxEchoRunes runes of s, marking a clip
// with an ellipsis. The cut is on a rune boundary: product lists carry accented
// characters, and slicing bytes emits invalid UTF-8 into the output.
func basketClipEcho(s string) string {
	runes := []rune(s)
	if len(runes) <= basketMaxEchoRunes {
		return s
	}
	return string(runes[:basketMaxEchoRunes-3]) + "..."
}

// basketCheckListPath decides whether a caller-supplied path may be read at
// all. Every rejection is a usage error (exit 2) naming the reason.
func basketCheckListPath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return usageErr(fmt.Errorf("<list-file> is required"))
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	if !basketAllowedListExtensions[ext] {
		return usageErr(fmt.Errorf("refusing to read %q: basket reads plain-text shopping lists and echoes every line back, so it accepts only .txt, .md, .list and .csv files (this path's extension is %q)", path, ext))
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return usageErr(fmt.Errorf("resolving list file %q: %w", path, err))
	}
	abs = filepath.Clean(abs)
	if kind, protected := basketProtectedDirFor(abs); protected {
		return usageErr(fmt.Errorf("refusing to read %q: it resolves inside this CLI's own %s directory, which holds the saved session cookies, the price-history database and other private state; basket echoes every line it reads back to the caller, so it never reads from there", path, kind))
	}
	// Lstat, not Stat: Stat follows a symlink, so the mode checked would be the
	// target's and the link would decide what gets echoed.
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return usageErr(fmt.Errorf("list file %q does not exist", path))
		}
		return fmt.Errorf("reading list file %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return usageErr(fmt.Errorf("refusing to read %q: it is a symlink, and following it would let the link's target choose what gets echoed back", path))
	}
	if info.IsDir() {
		return usageErr(fmt.Errorf("refusing to read %q: it is a directory, not a shopping list", path))
	}
	if !info.Mode().IsRegular() {
		return usageErr(fmt.Errorf("refusing to read %q: it is not a regular file (mode %s); devices, sockets and named pipes are not shopping lists", path, info.Mode()))
	}
	if info.Size() > basketMaxListBytes {
		return usageErr(fmt.Errorf("refusing to read %q: it is %d bytes, over the %d-byte (1 MiB) cap on a shopping list", path, info.Size(), basketMaxListBytes))
	}
	return nil
}

// basketProtectedDirFor reports whether a cleaned absolute path sits in one of
// the CLI's own private directories, resolved through the SAME lookups the rest
// of the CLI uses for the data directory and defaultDBPath.
func basketProtectedDirFor(abs string) (string, bool) {
	protected := []struct {
		kind    string
		resolve func() (string, error)
	}{
		{"config", cliutil.ConfigDir},
		{"data", cliutil.DataDir},
		{"state", cliutil.StateDir},
		{"cache", cliutil.CacheDir},
	}
	for _, p := range protected {
		dir, err := p.resolve()
		if err != nil {
			continue
		}
		if basketPathWithin(abs, dir) {
			return p.kind, true
		}
	}
	// The directory the default database actually lands in, in case an
	// override moved it out from under the data directory.
	if basketPathWithin(abs, filepath.Dir(defaultDBPath("woolworths-pp-cli"))) {
		return "data", true
	}
	return "", false
}

// basketPathWithin reports whether abs is dir or sits underneath it. Comparison
// is on cleaned absolute paths, case-insensitively on Windows where the
// filesystem is.
func basketPathWithin(abs, dir string) bool {
	cleanDir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil || strings.TrimSpace(dir) == "" {
		return false
	}
	cleanDir = filepath.Clean(cleanDir)
	a, d := abs, cleanDir
	if runtime.GOOS == "windows" {
		a, d = strings.ToLower(a), strings.ToLower(d)
	}
	return a == d || strings.HasPrefix(a, d+string(os.PathSeparator))
}

// basketReadList reads the list file. Called only after the dry-run guard, so a
// dry run never depends on the file existing.
func basketReadList(path string, limit int) ([]basketItem, int, error) {
	f, err := os.Open(path) // #nosec G304 -- the list file is named by the operator on the command line
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, usageErr(fmt.Errorf("list file %q does not exist", path))
		}
		return nil, 0, fmt.Errorf("reading list file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	items := make([]basketItem, 0, limit)
	skipped := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			skipped++
			continue
		}
		if len(items) >= limit {
			break
		}
		items = append(items, basketItem{line: lineNo, text: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, fmt.Errorf("reading list file %q: %w", path, err)
	}
	return items, skipped, nil
}

// basketListKey derives a stable key from the list contents so a run is only
// ever compared against a previous run of the SAME list. Comparing totals
// across two different lists would be a meaningless number presented as a
// trend.
func basketListKey(items []basketItem) string {
	h := sha256.New()
	for _, item := range items {
		_, _ = h.Write([]byte(strings.ToLower(item.text)))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// basketStopwords are dropped from a line before matching. They carry no
// discriminating power and would otherwise force spurious mismatches.
var basketStopwords = map[string]bool{
	"the": true, "of": true, "a": true, "an": true, "and": true, "with": true,
	"x": true, "for": true,
}

// basketTokens splits text into lowercase alphanumeric tokens.
func basketTokens(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		isDigit := r >= '0' && r <= '9'
		isLetter := r >= 'a' && r <= 'z'
		return !isDigit && !isLetter
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" || basketStopwords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// basketTokenMatch reports whether a query token is present in the candidate
// tokens, allowing a prefix match so "egg" matches "eggs". The three-character
// floor stops short tokens from matching almost anything.
func basketTokenMatch(query string, candidate []string) (bool, bool) {
	for _, c := range candidate {
		if c == query {
			return true, true
		}
	}
	if len(query) < 3 {
		return false, false
	}
	for _, c := range candidate {
		if len(c) >= len(query) && strings.HasPrefix(c, query) {
			return true, false
		}
		if len(query) >= len(c) && len(c) >= 3 && strings.HasPrefix(query, c) {
			return true, false
		}
	}
	return false, false
}

// basketResolveLine turns one list line into a priced product, or into an
// explicit unresolved row carrying the reason.
//
// The rule is deliberately strict: EVERY token of the line must appear in the
// candidate's name, brand or pack size. Woolworths search silently ignores
// terms it cannot match — "blorptig cheese" returns ordinary cheese — so
// without this rule a nonsense line would resolve to a real product and be
// priced into the total as though the shopper had asked for it.
func basketResolveLine(ctx context.Context, c *client.Client, item basketItem) basketLine {
	// Every echo of the caller's own line text goes through ppwTruncate, which
	// caps it at basketMaxEchoRunes RUNES. A file this command was pointed at is
	// untrusted input, and its lines end up on stdout verbatim.
	echo := basketClipEcho(item.text)
	line := basketLine{Line: item.line, Query: echo}

	resp, err := wowSearchPage(ctx, c, item.text, 1, wowMaxPageSize, "TraderRelevance")
	if err != nil {
		line.Reason = fmt.Sprintf("lookup failed: %v", err)
		line.FetchFailed = true
		return line
	}
	tiles := resp.tiles()
	if len(tiles) == 0 {
		line.Reason = fmt.Sprintf("search returned no products for %q", echo)
		return line
	}

	queryTokens := basketTokens(item.text)
	if len(queryTokens) == 0 {
		line.Reason = fmt.Sprintf("%q contains no searchable words", echo)
		return line
	}

	bestIdx := -1
	bestScore := math.Inf(-1)
	var bestExact int
	for i, tile := range tiles {
		if !tile.available() {
			continue
		}
		candidate := basketTokens(strings.Join([]string{tile.displayName(), tile.Name, tile.Brand, tile.PackageSize}, " "))
		matchedAll := true
		score := 0.0
		exact := 0
		for _, qt := range queryTokens {
			ok, isExact := basketTokenMatch(qt, candidate)
			if !ok {
				matchedAll = false
				break
			}
			if isExact {
				exact++
				score += 2
			} else {
				score++
			}
		}
		if !matchedAll {
			continue
		}
		// Search rank is a weak, deterministic tie-break: earlier results
		// nudge ahead, but never enough to outrank a better token overlap.
		score -= float64(i) * 0.001
		if score > bestScore {
			bestScore = score
			bestIdx = i
			bestExact = exact
		}
	}

	if bestIdx < 0 {
		line.Reason = fmt.Sprintf("no in-stock result matched every word of %q; Woolworths search ignores terms it cannot match, so the %d result(s) it returned are not this item", echo, len(tiles))
		return line
	}

	tile := tiles[bestIdx]
	line.Resolved = true
	line.Stockcode = tile.stockcode()
	line.Name = tile.displayName()
	line.Brand = tile.Brand
	line.PackageSize = tile.PackageSize
	line.Price = roundMoney(tile.Price)
	line.IsOnSpecial = tile.IsOnSpecial
	line.IsHalfPrice = tile.IsHalfPrice
	line.WasPrice = roundMoney(tile.WasPrice)
	line.CupString = tile.CupString
	line.MatchScore = roundMoney(bestScore)
	line.MatchRank = bestIdx + 1
	line.MatchReason = fmt.Sprintf("all %d query token(s) matched (%d exactly) at search rank %d", len(queryTokens), bestExact, bestIdx+1)
	return line
}

// basketRecentMedian compares a current price against the median of that
// product's own recent recorded prices. The median is used rather than the
// mean so a single half-price week does not drag the baseline down.
func basketRecentMedian(ctx context.Context, db *sql.DB, stockcode string, current float64) (*basketHistoryDelta, error) {
	if db == nil || stockcode == "" {
		return nil, nil
	}
	history, err := store.PriceHistory(ctx, db, stockcode)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	recent := make([]float64, 0, len(history))
	all := make([]float64, 0, len(history))
	for _, obs := range history {
		if obs.Price <= 0 {
			continue
		}
		all = append(all, obs.Price)
		if obs.ObservedAt >= cutoff {
			recent = append(recent, obs.Price)
		}
	}
	sample := recent
	window := "30d"
	if len(sample) == 0 {
		sample = all
		window = "all recorded"
	}
	if len(sample) == 0 {
		return nil, nil
	}
	median := basketMedian(sample)
	out := &basketHistoryDelta{
		Median:  roundMoney(median),
		Samples: len(sample),
		Window:  window,
	}
	if current > 0 {
		out.Delta = roundMoney(current - median)
		if median > 0 {
			out.DeltaPct = roundMoney((current - median) / median * 100)
		}
	}
	return out, nil
}

func basketMedian(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

const basketRunSchema = `
CREATE TABLE IF NOT EXISTS basket_run (
	list_key         TEXT    NOT NULL,
	ran_at           INTEGER NOT NULL,
	total            REAL    NOT NULL,
	resolved_lines   INTEGER NOT NULL DEFAULT 0,
	unresolved_lines INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (list_key, ran_at)
);
CREATE INDEX IF NOT EXISTS idx_basket_run_key ON basket_run (list_key, ran_at DESC);
`

// basketResolveDBPath applies the --db override against the default location.
func basketResolveDBPath(dbPath string) string {
	if resolved := strings.TrimSpace(dbPath); resolved != "" {
		return resolved
	}
	return defaultDBPath("woolworths-pp-cli")
}

// basketOpenDB opens the local database for history lookups and run recording.
// When the database does not exist yet it is only created if this run is going
// to record something; a --record=false run never brings a database into
// existence as a side effect of pricing a list.
//
// The returned error distinguishes "the database could not be opened or
// initialised" from "there is no database yet". Collapsing the two into a bare
// false told every caller that sync had never run, which is a different problem
// with a different fix -- and hid a locked or corrupt file entirely.
func basketOpenDB(ctx context.Context, dbPath string, allowCreate bool) (*sql.DB, func(), bool, error) {
	resolved := basketResolveDBPath(dbPath)
	if _, err := os.Stat(resolved); err != nil {
		if !os.IsNotExist(err) {
			return nil, func() {}, false, fmt.Errorf("checking database %s: %w", resolved, err)
		}
		if !allowCreate {
			// Genuinely absent, and this run must not create one: not an error.
			return nil, func() {}, false, nil
		}
	}
	s, err := store.OpenWithContext(ctx, resolved)
	if err != nil {
		return nil, func() {}, false, fmt.Errorf("opening database %s: %w", resolved, err)
	}
	if err := store.EnsureWoolworthsTables(ctx, s.DB()); err != nil {
		_ = s.Close()
		return nil, func() {}, false, fmt.Errorf("creating price-history tables in %s: %w", resolved, err)
	}
	if _, err := s.DB().ExecContext(ctx, basketRunSchema); err != nil {
		_ = s.Close()
		return nil, func() {}, false, fmt.Errorf("creating basket_run table in %s: %w", resolved, err)
	}
	return s.DB(), func() { _ = s.Close() }, true, nil
}

// basketPreviousTotal returns the most recent recorded run for this list, or
// nil when there is none. Nullable columns scan through sql.Null* and the
// result set is drained and closed before returning so the caller is free to
// issue the follow-up write.
func basketPreviousTotal(ctx context.Context, db *sql.DB, listKey string) (*basketPreviousRun, error) {
	if db == nil || listKey == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT ran_at, COALESCE(total, 0), COALESCE(resolved_lines, 0)
		FROM basket_run
		WHERE list_key = ?
		ORDER BY ran_at DESC
		LIMIT 1`, listKey)
	if err != nil {
		return nil, fmt.Errorf("basket previous total: query: %w", err)
	}
	var out *basketPreviousRun
	for rows.Next() {
		var (
			ranAt    sql.NullInt64
			total    sql.NullFloat64
			resolved sql.NullInt64
		)
		if err := rows.Scan(&ranAt, &total, &resolved); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("basket previous total: scan: %w", err)
		}
		out = &basketPreviousRun{
			RanAt:        ranAt.Int64,
			Total:        roundMoney(total.Float64),
			ResolvedRows: int(resolved.Int64),
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("basket previous total: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("basket previous total: close: %w", err)
	}
	return out, nil
}

// basketRecordRun stores this run's total so the next run over the same list
// can report the movement.
func basketRecordRun(ctx context.Context, db *sql.DB, listKey string, total float64, resolved, unresolved int) error {
	if db == nil || listKey == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, `
		INSERT OR REPLACE INTO basket_run (list_key, ran_at, total, resolved_lines, unresolved_lines)
		VALUES (?,?,?,?,?)`, listKey, time.Now().Unix(), total, resolved, unresolved)
	if err != nil {
		return fmt.Errorf("basket record run: %w", err)
	}
	return nil
}

func basketEmit(cmd *cobra.Command, flags *rootFlags, env basketEnvelope) error {
	out := cmd.OutOrStdout()
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, env, flags)
	}
	if len(env.Lines) == 0 {
		fmt.Fprintf(out, "%s\n", env.Note)
		return nil
	}
	rows := make([]map[string]any, 0, len(env.Lines))
	for _, line := range env.Lines {
		row := map[string]any{"line": line.Line, "item": line.Query}
		if !line.Resolved {
			row["product"] = "UNRESOLVED"
			row["price"] = ""
			row["note"] = line.Reason
			rows = append(rows, row)
			continue
		}
		row["product"] = line.Name
		row["price"] = fmt.Sprintf("$%.2f", line.Price)
		row["special"] = line.IsOnSpecial
		if line.History != nil {
			row["vs_median"] = fmt.Sprintf("%+.2f (%.1f%%)", line.History.Delta, line.History.DeltaPct)
		}
		rows = append(rows, row)
	}
	if err := printAutoTable(out, rows); err != nil {
		return err
	}
	fmt.Fprintf(out, "\ntotal: $%.2f over %s\n", env.Total, env.TotalBasis)
	if env.PreviousRun != nil {
		fmt.Fprintf(out, "previous run %s: $%.2f (%+.2f, %+.1f%%)\n",
			time.Unix(env.PreviousRun.RanAt, 0).Format(time.RFC3339), env.PreviousRun.Total, env.TotalDelta, env.TotalDeltaPct)
	} else {
		fmt.Fprintln(out, "no previous run recorded for this list yet; run basket again to see the movement")
	}
	if env.Note != "" {
		fmt.Fprintf(out, "%s\n", env.Note)
	}
	return nil
}
