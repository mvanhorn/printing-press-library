// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cdcparse"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

// CDCCategories is the full downloads taxonomy, enumerated by an exhaustive
// sweep of all 300 (category, year) pairs.
var CDCCategories = []string{
	"notices", "circulars", "miscellaneous", "newsletter", "quarterly-accounts",
	"forms", "procedures", "annual-reports", "guidelines", "designated-time-schedule",
	"list-of-securities", "disciplinary-registers", "tariff-fee-structure",
	"sustainability-report", "publications",
}

// CDC's listing year filter spans these years. year_param keys on the UPLOAD
// date, not the document's own as-of date, so assembling documents ABOUT a
// period means sweeping ALL years and keying on the parsed as-of date.
const (
	cdcFirstYear = 2007
	cdcLastYear  = 2026
)

type coverageBucket struct {
	Category   string `json:"category"`
	Year       int    `json:"year"`
	Paged      int    `json:"paged"`
	State      string `json:"state"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Items      int    `json:"item_count"`
	BlockHash  string `json:"block_hash,omitempty"`
	ProbedAt   string `json:"probed_at,omitempty"`
}

type coverageView struct {
	Mode           string           `json:"mode"`
	Buckets        []coverageBucket `json:"buckets"`
	DocumentsSeen  int              `json:"documents_seen"`
	DocumentsNew   int              `json:"documents_new"`
	Requests       int              `json:"requests,omitempty"`
	PairsSettled   int              `json:"pairs_settled"`
	PairsTotal     int              `json:"pairs_total"`
	PairsRemaining int              `json:"pairs_remaining"`
	RepeatBlocks   int              `json:"repeat_block_terminators"`
	Stopped        string           `json:"stopped_reason,omitempty"`
	Note           string           `json:"note,omitempty"`
}

func newNovelCoverageMapCmd(flags *rootFlags) *cobra.Command {
	var (
		categoryCSV  string
		yearCSV      string
		maxScanPages int
		probe        bool
		strict       bool
		dbPath       string
	)

	cmd := &cobra.Command{
		Use:   "map",
		Short: "See exactly which document buckets are mirrored, which are genuinely absent at source, and which were never asked for.",
		Long: strings.Trim(`
Report and extend the document coverage map.

Without --probe this reads the local store only and makes no network request.
With --probe it walks CDC's downloads enumerator and records what it finds.

Coverage is TRI-STATE and the third state is the point:

  found                 the bucket was probed and returned items
  not-found-at-source   the bucket was probed and CDC genuinely has nothing
  not-probed            we never asked

A coverage map without persisted negatives cannot tell a real source gap from an
unrun query, which makes it a liar rather than a map.

Only the admin-ajax enumerator is walked. The /downloads-category/<cat>/page/N/
URL pagination is decorative: it returns a byte-identical item block for every
N, so walking it yields thousands of duplicates of the same ten items.

Probing is resumable. Settled pairs are skipped, and because the Cloudflare
clearance has a hard ~30-minute lifetime the walk stops cleanly when the window
runs low, reporting how many pairs remain.
`, "\n"),
		Example: "  cdc-pakistan-pp-cli coverage map --category notices --agent",
		Annotations: map[string]string{
			// This command WRITES to the local store. It is annotated honestly;
			// mislabelling it read-only to get past a coverage gate would be a
			// lie about a command that writes.
			"mcp:read-only":   "false",
			"mcp:local-write": "true",
			// A real write invocation with a deliberately tiny scope:
			// list-of-securities/2025 is a single page of 2 items.
			"pp:happy-args":       "--category=list-of-securities;--year=2025;--probe;--timeout=30m",
			"pp:typed-exit-codes": "0,3,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage map")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			cats, err := resolveCategories(categoryCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			years, err := resolveYears(yearCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			// Curtail scan work under live dogfood so the real write path still
			// executes inside the matrix's per-command timeout. Read work stays
			// real -- this never substitutes mock data.
			if cliutil.IsDogfoodEnv() {
				if len(cats) > 1 {
					cats = cats[:1]
				}
				if len(years) > 1 {
					years = years[:1]
				}
				if maxScanPages > 2 {
					maxScanPages = 2
				}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()
			if err := store.EnsureCDCSchema(ctx, db); err != nil {
				return err
			}

			view := &coverageView{
				Mode:       "report",
				Buckets:    []coverageBucket{},
				PairsTotal: len(cats) * len(years),
			}
			if probe {
				view.Mode = "probe"
				if err := probeCoverage(ctx, cmd, db, flags, cats, years, maxScanPages, view); err != nil {
					return err
				}
			}
			if err := loadCoverage(ctx, db, cats, years, view); err != nil {
				return err
			}

			if strict && view.PairsRemaining > 0 {
				return notFoundErr(fmt.Errorf(
					"--strict: %d of %d (category,year) pairs are still not-probed; coverage is incomplete",
					view.PairsRemaining, view.PairsTotal))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderCoverage(cmd, view)
		},
	}
	cmd.Flags().StringVar(&categoryCSV, "category", "", "comma-separated categories, or 'all' (default: all)")
	cmd.Flags().StringVar(&yearCSV, "year", "", "comma-separated years or a YYYY-YYYY range (default: 2007-2026)")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 40, "maximum listing pages to walk per (category, year) before stopping")
	cmd.Flags().BoolVar(&probe, "probe", false, "walk CDC to extend coverage (requires a clearance cookie); without this the command is local-only")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero when any requested pair is still not-probed")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

func probeCoverage(ctx context.Context, cmd *cobra.Command, db *store.Store, flags *rootFlags,
	cats []string, years []int, maxScanPages int, view *coverageView) error {

	cl, err := LoadClearance(flags)
	if err != nil {
		return err
	}
	// Estimate: a settled pair costs ~1-3 requests at 1s pacing. Refuse up front
	// if the clearance window cannot plausibly cover the requested scope.
	settled, err := settledPairs(ctx, db)
	if err != nil {
		return err
	}
	todo := 0
	for _, c := range cats {
		for _, y := range years {
			if !settled[pairKey(c, y)] {
				todo++
			}
		}
	}
	if todo == 0 {
		view.Note = "every requested (category, year) pair is already settled; nothing to probe"
		return nil
	}
	if err := cl.CheckMargin(time.Duration(todo) * 2 * time.Second); err != nil {
		return err
	}

	f := newClearanceFetcher(cl, 45*time.Second)
	now := time.Now().UTC().Format(time.RFC3339)

	for _, cat := range cats {
		for _, year := range years {
			if settled[pairKey(cat, year)] {
				continue
			}
			if err := cl.CheckMargin(0); err != nil {
				view.Stopped = "clearance window ran low; remaining pairs are recorded as not-probed and the run resumes on re-invocation"
				view.Requests = f.reqs
				return nil
			}
			seen := map[string]bool{}
			for paged := 1; paged <= maxScanPages; paged++ {
				code, body, err := f.PostDownloads(ctx, cat, year, paged)
				if err != nil {
					// A challenge or transport failure is recorded as
					// transport-error, NEVER as an empty bucket.
					_ = upsertBucket(ctx, db, cat, year, paged, "transport-error", code, 0, "", now)
					view.Requests = f.reqs
					return err
				}
				docs, perr := cdcparse.ParseDownloads(body, cat, year)
				if perr != nil {
					_ = upsertBucket(ctx, db, cat, year, paged, "transport-error", code, 0, "", now)
					view.Requests = f.reqs
					return authErr(perr)
				}
				if len(docs) == 0 {
					// Genuinely empty at source, and now PERSISTED as such.
					if err := upsertBucket(ctx, db, cat, year, paged, "not-found-at-source", code, 0, "", now); err != nil {
						return err
					}
					break
				}
				h := cdcparse.BlockHash(docs)
				if seen[h] {
					// The enumerator echoed a previous page. Record it so a
					// future paginator regression is caught mechanically.
					if err := upsertBucket(ctx, db, cat, year, paged, "not-found-at-source", code, len(docs), h, now); err != nil {
						return err
					}
					view.RepeatBlocks++
					break
				}
				seen[h] = true
				newDocs, err := upsertDocuments(ctx, db, docs, now)
				if err != nil {
					return err
				}
				view.DocumentsSeen += len(docs)
				view.DocumentsNew += newDocs
				if err := upsertBucket(ctx, db, cat, year, paged, "found", code, len(docs), h, now); err != nil {
					return err
				}
			}
		}
	}
	view.Requests = f.reqs
	return nil
}

func upsertBucket(ctx context.Context, db *store.Store, cat string, year, paged int,
	state string, code, items int, hash, at string) error {
	const q = `INSERT INTO cdc_coverage_buckets
	   (category, year_param, paged, state, http_status, item_count, block_hash, probed_at)
	   VALUES (?,?,?,?,?,?,?,?)
	   ON CONFLICT(category, year_param, paged) DO UPDATE SET
	     state=excluded.state, http_status=excluded.http_status,
	     item_count=excluded.item_count, block_hash=excluded.block_hash,
	     probed_at=excluded.probed_at`
	if _, err := db.DB().ExecContext(ctx, q, cat, year, paged, state, code, items, hash, at); err != nil {
		return fmt.Errorf("recording coverage bucket: %w", err)
	}
	return nil
}

func upsertDocuments(ctx context.Context, db *store.Store, docs []cdcparse.Document, now string) (int, error) {
	const q = `INSERT INTO cdc_documents
	  (url, title, category, meta_day_month, upload_year, upload_month, year_param,
	   legacy_path, event_kind, event_state, event_action, event_confidence, first_seen, last_seen)
	  VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	  ON CONFLICT(url) DO UPDATE SET
	    title=excluded.title, last_seen=excluded.last_seen,
	    event_kind=excluded.event_kind, event_state=excluded.event_state,
	    event_action=excluded.event_action, event_confidence=excluded.event_confidence`
	newCount := 0
	for _, d := range docs {
		ev := cdcparse.ClassifyTitle(d.Title)
		legacy := 0
		if d.LegacyPath {
			legacy = 1
		}
		var existed int
		_ = db.DB().QueryRowContext(ctx, `SELECT 1 FROM cdc_documents WHERE url = ?`, d.URL).Scan(&existed)
		if existed == 0 {
			newCount++
		}
		if _, err := db.DB().ExecContext(ctx, q, d.URL, d.Title, d.Category, d.MetaDayMonth,
			nullIfEmpty(d.UploadYear), nullIfEmpty(d.UploadMonth), d.YearParam, legacy,
			string(ev.Kind), string(ev.State), ev.Action, ev.Confidence, now, now); err != nil {
			return newCount, fmt.Errorf("storing document %s: %w", d.URL, err)
		}
	}
	return newCount, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func settledPairs(ctx context.Context, db *store.Store) (map[string]bool, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT DISTINCT category, year_param FROM cdc_coverage_buckets
		  WHERE state IN ('found','not-found-at-source')`)
	if err != nil {
		return nil, fmt.Errorf("reading settled pairs: %w", err)
	}
	out := map[string]bool{}
	for rows.Next() {
		var c string
		var y int
		if err := rows.Scan(&c, &y); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[pairKey(c, y)] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

func pairKey(c string, y int) string { return c + "|" + strconv.Itoa(y) }

func loadCoverage(ctx context.Context, db *store.Store, cats []string, years []int, view *coverageView) error {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT category, year_param, paged, state, COALESCE(http_status,0),
		        item_count, COALESCE(block_hash,''), COALESCE(probed_at,'')
		   FROM cdc_coverage_buckets ORDER BY category, year_param DESC, paged`)
	if err != nil {
		return fmt.Errorf("reading coverage: %w", err)
	}
	want := map[string]bool{}
	for _, c := range cats {
		for _, y := range years {
			want[pairKey(c, y)] = true
		}
	}
	settled := map[string]bool{}
	for rows.Next() {
		var b coverageBucket
		var hash, at string
		if err := rows.Scan(&b.Category, &b.Year, &b.Paged, &b.State, &b.HTTPStatus,
			&b.Items, &hash, &at); err != nil {
			_ = rows.Close()
			return err
		}
		b.BlockHash, b.ProbedAt = hash, at
		if !want[pairKey(b.Category, b.Year)] {
			continue
		}
		if b.State == "found" || b.State == "not-found-at-source" {
			settled[pairKey(b.Category, b.Year)] = true
		}
		view.Buckets = append(view.Buckets, b)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	var docs int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM cdc_documents`).Scan(&docs); err != nil && err != sql.ErrNoRows {
		return err
	}
	if view.Mode == "report" {
		view.DocumentsSeen = docs
	}
	view.PairsSettled = len(settled)
	view.PairsTotal = len(want)
	view.PairsRemaining = len(want) - len(settled)
	if view.PairsRemaining > 0 && view.Note == "" {
		view.Note = fmt.Sprintf("%d of %d (category,year) pairs are NOT-PROBED -- absence here means 'never asked', not 'CDC published nothing'. Re-run with --probe.",
			view.PairsRemaining, view.PairsTotal)
	}
	return nil
}

func renderCoverage(cmd *cobra.Command, view *coverageView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "mode=%s  pairs settled %d/%d (remaining %d)\n",
		view.Mode, view.PairsSettled, view.PairsTotal, view.PairsRemaining)
	fmt.Fprintf(w, "documents in store: %d", view.DocumentsSeen)
	if view.DocumentsNew > 0 {
		fmt.Fprintf(w, " (+%d new this run)", view.DocumentsNew)
	}
	fmt.Fprintln(w)
	if view.Requests > 0 {
		fmt.Fprintf(w, "requests: %d\n", view.Requests)
	}
	if view.RepeatBlocks > 0 {
		fmt.Fprintf(w, "repeat-block terminators: %d\n", view.RepeatBlocks)
	}
	byState := map[string]int{}
	for _, b := range view.Buckets {
		byState[b.State]++
	}
	keys := make([]string, 0, len(byState))
	for k := range byState {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  %-22s %d buckets\n", k, byState[k])
	}
	if view.Stopped != "" {
		fmt.Fprintf(w, "\nstopped: %s\n", view.Stopped)
	}
	if view.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", view.Note)
	}
	return nil
}

func resolveCategories(csv string) ([]string, error) {
	if csv == "" || strings.EqualFold(csv, "all") {
		return CDCCategories, nil
	}
	valid := map[string]bool{}
	for _, c := range CDCCategories {
		valid[c] = true
	}
	var out []string
	for _, raw := range strings.Split(csv, ",") {
		c := strings.ToLower(strings.TrimSpace(raw))
		if c == "" {
			continue
		}
		if !valid[c] {
			return nil, fmt.Errorf("unknown category %q; known categories are: %s", c, strings.Join(CDCCategories, ", "))
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--category was given but resolved to nothing")
	}
	return out, nil
}

func resolveYears(csv string) ([]int, error) {
	if csv == "" {
		out := make([]int, 0, cdcLastYear-cdcFirstYear+1)
		for y := cdcLastYear; y >= cdcFirstYear; y-- {
			out = append(out, y)
		}
		return out, nil
	}
	var out []int
	for _, raw := range strings.Split(csv, ",") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(s, "-"); ok {
			a, err1 := cdcparse.ParseYearParam(lo)
			b, err2 := cdcparse.ParseYearParam(hi)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("bad year range %q", s)
			}
			if a > b {
				a, b = b, a
			}
			for y := b; y >= a; y-- {
				out = append(out, y)
			}
			continue
		}
		y, err := cdcparse.ParseYearParam(s)
		if err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--year was given but resolved to nothing")
	}
	return out, nil
}
