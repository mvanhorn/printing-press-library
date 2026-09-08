// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// exportResources is the fixed set of dumpable store resource keys, plus the
// "coverage" pseudo-resource that dumps the fetch ledger instead of
// observations. Naming them explicitly rather than reading DISTINCT resource
// from the table means a mirror that was never backfilled still reports a
// usable choice list in the usage error.
var exportResources = []string{
	"daily-returns",
	"daily-nav",
	"daily-pricing",
	"daily-payout",
	"daily-ter",
	"monthly",
	"allocation",
	"coverage",
}

const exportResourceCoverage = "coverage"

// The mirror tables are named here because export reads them through its own
// streaming cursor rather than store.LoadMUFAP*; see exportScanObs for why.
const (
	exportObsTable      = "mufap_obs"
	exportCoverageTable = "mufap_coverage"
)

// Identity columns lead every record so a loader can key on them without
// hunting through MUFAP's own header labels, which differ per tab.
var exportObsBaseColumns = []string{"date", "row_key", "observed_at"}

var exportCoverageBaseColumns = []string{"resource", "date", "row_count", "fetched_at"}

// exportRow is one flattened output record: the identity columns merged with
// the stored payload's own keys.
type exportRow map[string]any

// exportWhere builds the shared (resource, date range) predicate. An empty
// resource matches every resource, which is how the coverage ledger is dumped
// whole -- the same rule store.LoadMUFAPCoverage follows.
func exportWhere(resource, from, to string) (string, []any) {
	q := " WHERE 1=1"
	args := make([]any, 0, 3)
	if resource != "" {
		q += " AND resource = ?"
		args = append(args, resource)
	}
	if from != "" {
		q += " AND date >= ?"
		args = append(args, from)
	}
	if to != "" {
		q += " AND date <= ?"
		args = append(args, to)
	}
	return q, args
}

// exportMirrorHasTable reports whether a MUFAP mirror table exists. The mirror
// file is shared with the platform's own tables, so it can exist carrying no
// MUFAP schema at all (a `teach` run creates the file), and a read-only handle
// cannot create one. Probing sqlite_master turns that into the empty dump plus
// the single command that fixes it, the shape panel.go already uses.
func exportMirrorHasTable(ctx context.Context, s *store.Store, table string) (bool, error) {
	if s == nil || s.DB() == nil {
		return false, nil
	}
	var name sql.NullString
	err := s.DB().QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("export: reading mirror schema: %w", err)
	}
	return name.Valid, nil
}

// exportScanObs streams observations straight off the SQLite cursor instead of
// draining them into a slice first. store.LoadMUFAPObs materializes every row
// including its full payload JSON, and a multi-year daily-returns dump is
// hundreds of funds x thousands of dates: holding all of that before the first
// byte reaches stdout is exactly the failure mode a bulk load into an external
// research database hits. The row cap rides in the SQL as LIMIT rather than a
// post-load slice, so --limit 5 reads five rows instead of the whole table.
//
// The callback must not touch the store: the read-only handle is a small
// SQLite pool and this cursor stays open across every call. The CSV header
// pass therefore re-runs the scan rather than nesting a query inside it.
func exportScanObs(ctx context.Context, s *store.Store, resource, from, to string, limit int, fn func(store.MUFAPObs) error) error {
	if s == nil || s.DB() == nil {
		return nil
	}
	where, args := exportWhere(resource, from, to)
	q := `SELECT date, row_key, payload, observed_at FROM ` + exportObsTable + where + ` ORDER BY date, row_key`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("export: query observations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		o := store.MUFAPObs{Resource: resource}
		var payload, observed sql.NullString
		if err := rows.Scan(&o.Date, &o.Key, &payload, &observed); err != nil {
			return fmt.Errorf("export: scan observation: %w", err)
		}
		o.Payload = payload.String
		o.ObservedAt = observed.String
		if err := fn(o); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("export: iterate observations: %w", err)
	}
	return nil
}

// exportScanCoverage streams the fetch ledger under the same cursor rules as
// exportScanObs.
func exportScanCoverage(ctx context.Context, s *store.Store, resource, from, to string, limit int, fn func(store.MUFAPCoverage) error) error {
	if s == nil || s.DB() == nil {
		return nil
	}
	where, args := exportWhere(resource, from, to)
	q := `SELECT resource, date, row_count, fetched_at FROM ` + exportCoverageTable + where + ` ORDER BY resource, date`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("export: query coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c store.MUFAPCoverage
		var n sql.NullInt64
		var fetched sql.NullString
		if err := rows.Scan(&c.Resource, &c.Date, &n, &fetched); err != nil {
			return fmt.Errorf("export: scan coverage: %w", err)
		}
		// RowCount 0 is a real value -- "fetched and legitimately empty" --
		// so a NULL is read as 0 rather than dropping the ledger row.
		c.RowCount = int(n.Int64)
		c.FetchedAt = fetched.String
		if err := fn(c); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("export: iterate coverage: %w", err)
	}
	return nil
}

// exportCountRows counts the rows a dump would emit without reading any of
// them. Only the human path needs this: the guidance shown for an empty range
// has to be decided before a CSV header is written, and "is it empty" is the
// one question a stream cannot answer before it starts.
func exportCountRows(ctx context.Context, s *store.Store, table, resource, from, to string, limit int) (int, error) {
	if s == nil || s.DB() == nil {
		return 0, nil
	}
	where, args := exportWhere(resource, from, to)
	var n int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("export: count rows: %w", err)
	}
	if limit > 0 && n > limit {
		n = limit
	}
	return n, nil
}

// exportSelection turns --select into a top-level key allow-list. A dump is a
// stream of flat records rather than one JSON document, so printJSONFiltered
// cannot run over it; honouring --select here per record is what keeps the flag
// from being accepted and then silently ignored. Only the leading path segment
// can match, since the records have no nested objects.
func exportSelection(sel string) map[string]bool {
	keep := make(map[string]bool)
	for _, part := range strings.Split(sel, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, "."); i >= 0 {
			part = part[:i]
		}
		if part == "" {
			continue
		}
		keep[strings.ToLower(part)] = true
	}
	if len(keep) == 0 {
		return nil
	}
	return keep
}

// exportKeyKept applies the allow-list through the same case- and
// kebab-insensitive matcher the rest of the CLI uses, so --select row-key finds
// "row_key" and --select "fund name" finds MUFAP's "Fund Name" header label.
func exportKeyKept(keep map[string]bool, key string) bool {
	if keep == nil {
		return true
	}
	return matchSelectSegment(key, keep, nil) != ""
}

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagFormat string
	var flagDB string
	var flagLimit int
	var flagRawValues bool

	cmd := &cobra.Command{
		Use:   "dump [resource]",
		Short: "Dump the stored panel, allocation or coverage rows to stdout as JSONL or CSV.",
		Long: `Bulk-dumps the local mirror to stdout for loading into an external research
database. Nothing is written to disk and nothing is fetched from MUFAP: this
reads only what backfill already stored.

Resources:

  daily-returns  daily-nav  daily-pricing  daily-payout  daily-ter
  monthly        allocation
  coverage       the fetch ledger (resource, date, row_count, fetched_at)
                 rather than observations

Each observation record carries date, row_key and observed_at, then the stored
payload's own fields flattened in alongside them. The payload keys are MUFAP's
header labels verbatim ("Fund Name", "30 Days", "Validity Date", ...), so they
differ per tab and have widened over the years; the CSV header is therefore the
union of every emitted row's keys, not the first row's.

Dates are the row's OWN normalized validity date, not the date it was fetched
on, so --from/--to select observations rather than fetch batches.

MUFAP writes every negative in ACCOUNTING NOTATION and never with a minus sign:
"(4.97)" means -4.97. Payload cells are therefore DECODED by default -- a cell
MUFAP wrote as a number is emitted as a real JSON number, and as a plain signed
decimal in CSV -- while a cell that is not a number ("N/A", "-", fund names,
ratings, "Sep 04, 2026") passes through as the text MUFAP published. Measured
on the 2026-09-04 daily-returns export: 96 of 388 YTD cells were parenthesised
and none carried a minus sign, so a plain float parse fails on 97 of them and
the 291 it keeps average +9.29 where all 387 reporting rows average +2.45 --
a 6.8pp overstatement made of exactly the funds that fell. --raw-values turns the
decoding off and emits MUFAP's original text verbatim; the identity columns
(date, row_key, observed_at) are never rewritten either way.

Decoding is decided per cell by VALUE, not by column name. Measured over the
912 rows in the local mirror, Sector, Category, Fund Name, Rating and Validity
Date never parse as a number; Benchmark is the single exception ("N/A" on 907
rows, a bare "12"/"23" on three), so that one column can carry either a string
or a number.

Rows are streamed off the database a record at a time and --limit is applied in
SQL, so the dump never sizes with the table.

Output flags: the body is always the selected --format, in every output mode.
--select narrows the columns of each record (identity columns included) and
--csv is a synonym for --format csv and --json/--agent for --format json;
--compact, --plain and --quiet reshape a single JSON document and so do not
apply to a line-oriented dump. Pass --format jsonl explicitly to keep the
line-oriented dump under --json.

The dump goes to stdout, so redirect it to capture a file:
    mufap-pp-cli dump daily-returns --from 2016-08-22 --to 2026-09-04 > returns.jsonl
    mufap-pp-cli dump allocation --format csv > allocation.csv
The runnable examples below carry no redirection on purpose: they are executed
verbatim by the verification harness, which passes arguments directly rather
than through a shell, so a ">" would arrive as a positional argument.`,
		Example: `  mufap-pp-cli dump daily-returns --from 2026-09-01 --to 2026-09-04 --limit 5
  mufap-pp-cli dump daily-returns --from 2026-09-01 --to 2026-09-04 --format json --agent
  mufap-pp-cli dump allocation --format csv --limit 20
  mufap-pp-cli dump coverage --from 2026-08-01 --to 2026-09-04`,
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "resource=daily-returns;--from=2026-09-01;--to=2026-09-04;--limit=5",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a resource argument is required: want one of %s", strings.Join(exportResources, ", ")))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("export takes exactly one resource; got %d", len(args)))
			}
			resource := strings.TrimSpace(args[0])
			known := false
			for _, r := range exportResources {
				if r == resource {
					known = true
					break
				}
			}
			if !known {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown resource %q: want one of %s", resource, strings.Join(exportResources, ", ")))
			}

			format := strings.ToLower(strings.TrimSpace(flagFormat))
			if format == "" {
				// The global --csv flag asks for the same thing as
				// --format csv; honouring it here keeps the two from
				// silently disagreeing.
				format = "jsonl"
				if flags != nil {
					switch {
					case flags.csv:
						format = "csv"
					case flags.asJSON || flags.agent:
						// --json/--agent asks for ONE parseable document, and
						// newline-delimited records are not one: an empty
						// mirror yields zero lines, which is a correct dump but
						// unparseable JSON. Honouring the flag here keeps the
						// promise the flag makes, the same way --csv already
						// selects the csv body.
						format = "json"
					}
				}
			}
			if format != "jsonl" && format != "csv" && format != "json" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be jsonl, json or csv; got %q", flagFormat))
			}
			if flagLimit < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be zero or positive; got %d", flagLimit))
			}

			const exportDateLayout = "2006-01-02"
			for _, b := range []struct {
				name, val string
			}{{"--from", flagFrom}, {"--to", flagTo}} {
				if b.val == "" {
					continue
				}
				if _, err := time.Parse(exportDateLayout, b.val); err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("%s must be YYYY-MM-DD; got %q", b.name, b.val))
				}
			}
			if flagFrom != "" && flagTo != "" && flagTo < flagFrom {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s", flagFrom, flagTo))
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("mufap-pp-cli")
			}

			baseColumns := exportObsBaseColumns
			table := exportObsTable
			// The ledger is dumped for every resource, so its scan carries no
			// resource predicate.
			scanResource := resource
			if resource == exportResourceCoverage {
				baseColumns = exportCoverageBaseColumns
				table = exportCoverageTable
				scanResource = ""
			}

			var exportKeep map[string]bool
			if flags != nil && flags.selectFields != "" {
				exportKeep = exportSelection(flags.selectFields)
			}
			if exportKeep != nil {
				// The identity columns are part of the record, so the same
				// allow-list has to narrow them: a base column the user did
				// not select would otherwise stay in the CSV header as a
				// column of blanks.
				kept := make([]string, 0, len(baseColumns))
				for _, b := range baseColumns {
					if exportKeyKept(exportKeep, b) {
						kept = append(kept, b)
					}
				}
				baseColumns = kept
			}

			// An absent mirror file and a mirror carrying no MUFAP schema are
			// the same state: an empty dump, not a failure. Both flow through
			// the ordinary dump path below so all three empty cases (no file,
			// no table, no rows in range) emit one document shape in the
			// selected --format -- a bare CSV header, or zero JSONL lines.
			mirrorEmpty := false
			var s *store.Store
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
				mirrorEmpty = true
			} else {
				opened, openErr := store.OpenReadOnlyContext(ctx, dbPath)
				if openErr != nil {
					return fmt.Errorf("export: open %s: %w", dbPath, openErr)
				}
				defer func() { _ = opened.Close() }()
				s = opened
				has, tableErr := exportMirrorHasTable(ctx, opened, table)
				if tableErr != nil {
					return tableErr
				}
				if !has {
					// Wording matches panel.go/rates.go, narrowed for the
					// ledger because "observations" is the wrong noun there.
					missing := "observations"
					if resource == exportResourceCoverage {
						missing = "coverage ledger"
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "no MUFAP %s in %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", missing, dbPath)
					mirrorEmpty = true
				}
			}

			exportDecodePayload := func(raw string) (map[string]any, bool) {
				if strings.TrimSpace(raw) == "" {
					return nil, false
				}
				dec := json.NewDecoder(strings.NewReader(raw))
				// UseNumber keeps allocation amounts byte-identical on the way
				// out; float64 round-tripping would reformat MUFAP's figures.
				dec.UseNumber()
				var m map[string]any
				if err := dec.Decode(&m); err != nil {
					return nil, false
				}
				return m, true
			}

			exportFlattenObs := func(o store.MUFAPObs) exportRow {
				rec := exportRow{
					"date":        o.Date,
					"row_key":     o.Key,
					"observed_at": o.ObservedAt,
				}
				payload, ok := exportDecodePayload(o.Payload)
				if !ok {
					// Carry an unparseable payload through verbatim rather
					// than dropping the row: the loader can still see it.
					rec["payload"] = o.Payload
					return rec
				}
				for k, v := range payload {
					key := k
					if key == "date" || key == "row_key" || key == "observed_at" {
						key = "payload_" + key
					}
					// MUFAP's accounting negatives are decoded here rather
					// than left for the loader; see dumpPanelDecodeCell.
					rec[key] = dumpPanelDecodeCell(v, flagRawValues)
				}
				return rec
			}

			exportFlattenCoverage := func(c store.MUFAPCoverage) exportRow {
				return exportRow{
					"resource":   c.Resource,
					"date":       c.Date,
					"row_count":  c.RowCount,
					"fetched_at": c.FetchedAt,
				}
			}

			// selectMatched tracks whether --select ever named a real column,
			// so a typo reports rather than quietly emitting empty records.
			selectMatched := false
			exportProject := func(rec exportRow) exportRow {
				if exportKeep == nil {
					return rec
				}
				narrowed := make(exportRow, len(rec))
				for k, v := range rec {
					if exportKeyKept(exportKeep, k) {
						narrowed[k] = v
						selectMatched = true
					}
				}
				return narrowed
			}

			// exportForEach walks the selected rows straight off the database
			// cursor, flattening one at a time so no full dump is ever
			// materialized. It is re-runnable: the CSV header pass calls it
			// once for the key union and again for the rows.
			exportForEach := func(fn func(rec exportRow) error) error {
				if mirrorEmpty {
					return nil
				}
				if resource == exportResourceCoverage {
					return exportScanCoverage(ctx, s, scanResource, flagFrom, flagTo, flagLimit, func(c store.MUFAPCoverage) error {
						return fn(exportProject(exportFlattenCoverage(c)))
					})
				}
				return exportScanObs(ctx, s, scanResource, flagFrom, flagTo, flagLimit, func(o store.MUFAPObs) error {
					return fn(exportProject(exportFlattenObs(o)))
				})
			}

			exportOrderColumns := func(present map[string]bool) []string {
				cols := make([]string, 0, len(present)+len(baseColumns))
				seen := make(map[string]bool, len(baseColumns))
				for _, b := range baseColumns {
					cols = append(cols, b)
					seen[b] = true
				}
				rest := make([]string, 0, len(present))
				for k := range present {
					if !seen[k] {
						rest = append(rest, k)
					}
				}
				sort.Strings(rest)
				return append(cols, rest...)
			}

			exportCell := func(v any) string {
				switch t := v.(type) {
				case nil:
					return ""
				case string:
					return t
				case json.Number:
					return t.String()
				case int:
					return strconv.Itoa(t)
				case bool:
					return strconv.FormatBool(t)
				default:
					if b, mErr := json.Marshal(t); mErr == nil {
						return string(b)
					}
					return fmt.Sprintf("%v", t)
				}
			}

			exportWriteJSONLine := func(w *bufio.Writer, rec exportRow) error {
				// Keys are ordered explicitly (identity columns first, then
				// the payload's own keys sorted) so successive lines are
				// diffable; encoding/json on a map would sort every key
				// including the identity ones.
				seen := make(map[string]bool, len(baseColumns))
				cols := make([]string, 0, len(rec))
				for _, b := range baseColumns {
					if _, ok := rec[b]; ok {
						cols = append(cols, b)
						seen[b] = true
					}
				}
				rest := make([]string, 0, len(rec))
				for k := range rec {
					if !seen[k] {
						rest = append(rest, k)
					}
				}
				sort.Strings(rest)
				cols = append(cols, rest...)

				if err := w.WriteByte('{'); err != nil {
					return err
				}
				for i, c := range cols {
					if i > 0 {
						if err := w.WriteByte(','); err != nil {
							return err
						}
					}
					kb, err := json.Marshal(c)
					if err != nil {
						return err
					}
					if _, err := w.Write(kb); err != nil {
						return err
					}
					if err := w.WriteByte(':'); err != nil {
						return err
					}
					vb, err := json.Marshal(rec[c])
					if err != nil {
						return err
					}
					if _, err := w.Write(vb); err != nil {
						return err
					}
				}
				if err := w.WriteByte('}'); err != nil {
					return err
				}
				return w.WriteByte('\n')
			}

			written := 0
			exportDump := func(w io.Writer) error {
				written = 0
				if format == "json" {
					// One array document. jsonl exists to be piped into a
					// line-oriented loader and correctly emits nothing for an
					// empty mirror, but that leaves an agent with no parseable
					// result at all -- every sibling command answers an empty
					// mirror with a shaped empty payload. This format is that
					// answer, still written a row at a time rather than
					// buffered, so a multi-year dump does not have to fit in
					// memory to be valid JSON.
					// The envelope carries its own provenance -- which
					// resource and window produced these rows -- so an empty
					// result still says WHAT is empty. A bare [] cannot
					// distinguish "no daily-returns in this window" from "no
					// allocation rows", which is the same fetched-and-empty
					// vs never-fetched confusion the coverage ledger exists to
					// prevent. Siblings answer with a shaped envelope; so does
					// this.
					header := fmt.Sprintf(`{"resource":%s,"from":%s,"to":%s,"rows":[`,
						exportJSONString(resource), exportJSONString(flagFrom), exportJSONString(flagTo))
					if _, err := io.WriteString(w, header); err != nil {
						return err
					}
					first := true
					if err := exportForEach(func(rec exportRow) error {
						if !first {
							if _, err := io.WriteString(w, ","); err != nil {
								return err
							}
						}
						first = false
						enc, err := json.Marshal(rec)
						if err != nil {
							return err
						}
						if _, err := w.Write(enc); err != nil {
							return err
						}
						written++
						return nil
					}); err != nil {
						return err
					}
					_, err := fmt.Fprintf(w, `],"count":%d}`+"\n", written)
					return err
				}
				if format == "csv" {
					// CSV has to be rectangular, so the header is the union of
					// every emitted row's keys. Row one is not enough: MUFAP
					// widened the daily panel over time (older validity dates
					// carry no "2 Years"/"3 Years"), and the allocation
					// payload gained fields, so a header taken from the first
					// record would silently truncate later ones. The union
					// pass re-queries the rows instead of buffering them so the
					// dump itself is still written a row at a time.
					present := make(map[string]bool, 32)
					if err := exportForEach(func(rec exportRow) error {
						for k := range rec {
							present[k] = true
						}
						return nil
					}); err != nil {
						return err
					}
					cols := exportOrderColumns(present)
					cw := csv.NewWriter(w)
					if err := cw.Write(cols); err != nil {
						return err
					}
					cells := make([]string, len(cols))
					if err := exportForEach(func(rec exportRow) error {
						for i, c := range cols {
							cells[i] = exportCell(rec[c])
						}
						if err := cw.Write(cells); err != nil {
							return err
						}
						written++
						return nil
					}); err != nil {
						// Flush what was written so a truncated dump is
						// visibly truncated rather than silently short.
						cw.Flush()
						return err
					}
					cw.Flush()
					return cw.Error()
				}
				bw := bufio.NewWriter(w)
				if err := exportForEach(func(rec exportRow) error {
					if err := exportWriteJSONLine(bw, rec); err != nil {
						return err
					}
					written++
					return nil
				}); err != nil {
					_ = bw.Flush()
					return err
				}
				return bw.Flush()
			}

			// A --select that named nothing real would otherwise emit a stream
			// of empty records with no explanation; filterFields warns the same
			// way for the document-shaped commands.
			exportWarnSelect := func() {
				if exportKeep == nil || selectMatched || written == 0 {
					return
				}
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --select %q matched no fields; identity fields for %s: %s (payload fields are MUFAP's own header labels)\n",
					flags.selectFields, resource, strings.Join(exportCoverageOrObsColumns(resource), ", "))
			}

			out := cmd.OutOrStdout()
			// --format governs the body in every output mode: a bulk dump
			// exists to be piped, and collapsing it into one JSON document
			// would defeat both the streaming and the line-oriented load.
			if !wantsHumanTable(out, flags) {
				if err := exportDump(out); err != nil {
					return err
				}
				exportWarnSelect()
				return nil
			}

			if mirrorEmpty {
				// The remedy already went to stderr above; a human gets no
				// empty dump on top of it.
				return nil
			}
			count, countErr := exportCountRows(ctx, s, table, scanResource, flagFrom, flagTo, flagLimit)
			if countErr != nil {
				return countErr
			}
			if count == 0 {
				window := "the whole mirror"
				if flagFrom != "" || flagTo != "" {
					window = fmt.Sprintf("%s..%s", flagFrom, flagTo)
				}
				fmt.Fprintf(out, "no %s rows for %s in %s\n", resource, window, dbPath)
				fmt.Fprintln(out, "run: mufap-pp-cli coverage --resource "+resource+" --from <date> --to <date>")
				return nil
			}
			if err := exportDump(out); err != nil {
				return err
			}
			// The count goes to stderr so a redirected stdout stays a clean
			// dump even when the command was run from a terminal. It reports
			// what was actually written, not what was counted, so a short dump
			// cannot read as a complete one.
			fmt.Fprintf(cmd.ErrOrStderr(), "exported %d of %d %s row(s) as %s\n", written, count, resource, format)
			// The ledger carries no MUFAP payload cells, so the value
			// convention is only worth stating for observation dumps.
			if resource != exportResourceCoverage {
				if flagRawValues {
					fmt.Fprintln(cmd.ErrOrStderr(), "values are MUFAP's own text: negatives are parenthesised, so \"(4.97)\" means -4.97")
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "numeric cells decoded: MUFAP's accounting negatives -- \"(4.97)\" -- are emitted as -4.97; --raw-values keeps the original text")
				}
			}
			exportWarnSelect()
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earliest observation date YYYY-MM-DD (inclusive); empty is open-ended")
	cmd.Flags().StringVar(&flagTo, "to", "", "Latest observation date YYYY-MM-DD (inclusive); empty is open-ended")
	cmd.Flags().StringVar(&flagFormat, "format", "", "Output format: jsonl (default, one record per line for piping), json (one array document, parseable in a single read), or csv")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum rows to emit; 0 means no cap")
	cmd.Flags().BoolVar(&flagRawValues, "raw-values", false, "Emit payload cells as MUFAP's own text, accounting negatives included (\"(4.97)\" for -4.97). Off by default: numeric cells are decoded to real numbers.")
	return cmd
}

// exportCoverageOrObsColumns names the identity columns a resource emits, for
// the --select warning.
func exportCoverageOrObsColumns(resource string) []string {
	if resource == exportResourceCoverage {
		return exportCoverageBaseColumns
	}
	return exportObsBaseColumns
}

// exportJSONString renders s as a JSON string literal, falling back to an
// empty literal if it somehow cannot be marshalled, so the envelope stays
// parseable no matter what a resource or date flag carries.
func exportJSONString(s string) string {
	enc, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(enc)
}

// dumpPanelDecodeCell decodes one stored MUFAP payload cell for the two
// row-level exports. `dump` and `panel` share it so both speak exactly one
// value convention; rawValues short-circuits it for callers who want MUFAP's
// text verbatim.
//
// MUFAP writes every negative in ACCOUNTING NOTATION and never with a minus
// sign -- "(4.97)" means -4.97 -- so emitting payload cells as published hands
// a research loader numeric columns it cannot parse. Measured on the 388-row
// 2026-09-04 daily-returns export: 96 of 388 YTD cells were parenthesised and
// none carried a minus, so a plain float parse fails on 97 of them and the 291
// it keeps average +9.29 where all 387 reporting rows average +2.45. Across
// both mirrored dates the same rule decodes 9,627 cells, 1,102 of them
// accounting negatives. The bias is not random -- it
// drops exactly the left tail -- and it is the same quirk that once inverted
// the reported sign of the equity market (endpoint contract, section I).
//
// The decision is per VALUE, not per column name: whatever mufap.ParseNumber
// accepts becomes a number, and everything else -- "N/A", "-", "", fund names,
// ratings, "Sep 04, 2026" -- is returned untouched, so a non-reporting fund is
// never coerced to zero. Measured over the 912 rows in the local mirror,
// Sector, Category, Fund Name, Rating and Validity Date never parse as a
// number; Benchmark is the one exception ("N/A" on 907 rows, a bare "12"/"23"
// on three), which is why the rule is not expressed as a column allow-list.
// Values that are not strings (allocation amounts already stored as JSON
// numbers) are handed straight back so they stay byte-identical.
func dumpPanelDecodeCell(v any, rawValues bool) any {
	if rawValues {
		return v
	}
	s, ok := v.(string)
	if !ok {
		return v
	}
	f, ok := mufap.ParseNumber(s)
	if !ok {
		return v
	}
	return dumpPanelNumericLiteral(s, f)
}

// dumpPanelNumericLiteral renders a decoded cell as a JSON number literal.
//
// MUFAP's own digits are kept whenever they already form a legal JSON number
// once the thousands separators, a trailing "%" and the accounting parentheses
// are resolved, so a NAV of "10.4570" keeps its trailing zero and "(1,236.28)"
// becomes -1236.28 rather than a reformatted float. Anything that would not be
// legal JSON on its own (a leading zero, a bare ".5") falls back to the
// shortest round-tripping form of the parsed value, which keeps the emitted
// document parseable in every case.
func dumpPanelNumericLiteral(orig string, v float64) json.Number {
	t := strings.TrimSpace(strings.ReplaceAll(orig, ",", ""))
	t = strings.TrimSpace(strings.TrimSuffix(t, "%"))
	if len(t) > 1 && strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		t = strings.TrimSpace(t[1 : len(t)-1])
		t = "-" + strings.TrimSpace(strings.TrimSuffix(t, "%"))
	}
	if json.Valid([]byte(t)) {
		if parsed, err := strconv.ParseFloat(t, 64); err == nil && parsed == v {
			return json.Number(t)
		}
	}
	return json.Number(strconv.FormatFloat(v, 'f', -1, 64))
}
