package cli

// pp:data-source local

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// nccplExportRecord is one exported observation.
//
// The four coordinate fields are the store's own primary key plus the vintage
// stamp; payload is the upstream object byte-for-byte as it was stored. Nothing is
// reshaped, rounded or renamed on the way out, so a consumer can reconstruct the
// row exactly and an export can be diffed against the store that produced it.
type nccplExportRecord struct {
	Resource   string          `json:"resource"`
	Date       string          `json:"date"`
	Key        string          `json:"key"`
	ObservedAt string          `json:"observed_at"`
	Payload    json.RawMessage `json:"payload"`
}

// nccplExportResource is the per-resource summary of what was written.
type nccplExportResource struct {
	Resource  string `json:"resource"`
	Rows      int    `json:"rows"`
	Dates     int    `json:"dates"`
	FirstDate string `json:"first_date,omitempty"`
	LastDate  string `json:"last_date,omitempty"`
}

// nccplExportView is the summary envelope. It is deliberately separate from the
// dump itself: when the dump goes to stdout the summary goes to stderr, so a
// caller piping the export never has to strip a report out of the data.
type nccplExportView struct {
	Format    string                `json:"format"`
	Output    string                `json:"output"`
	From      string                `json:"from,omitempty"`
	To        string                `json:"to,omitempty"`
	Resources []nccplExportResource `json:"resources"`
	TotalRows int                   `json:"total_rows"`
	DryRun    bool                  `json:"dry_run,omitempty"`
	Action    string                `json:"action,omitempty"`
	Would     string                `json:"would,omitempty"`
	DBPath    string                `json:"db_path"`
	Records   []nccplExportRecord   `json:"records,omitempty"`
	Note      string                `json:"note,omitempty"`
}

// nccplExportCSVHeader is the CSV column order, and also the field order used to
// build every CSV row, so the two can never drift.
var nccplExportCSVHeader = []string{"resource", "date", "key", "observed_at", "payload"}

func newExportCmd(flags *rootFlags) *cobra.Command {
	var (
		outputPath   string
		format       string
		resourcesCSV string
		fromDate     string
		toDate       string
		dbPath       string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Dump the whole local store as JSONL or CSV, one line per stored observation",
		Long: strings.Trim(`
Write every observation held in the local store -- or the subset selected by
--resources, --from and --to -- to stdout or to a file, one record per line.

This is the bulk counterpart to 'panel'. panel takes ONE resource and reshapes it
into long format for a regression; export takes the WHOLE store and emits it
verbatim: resource, settlement date, row key, the observed_at vintage, and the
upstream payload exactly as it was stored. Nothing is reshaped, rounded or renamed,
so an export can be diffed against the store that produced it.

Export reads and never writes to the store, and never fetches. It reports what is
stored and nothing else. A session that was never fetched is simply absent from the
output: no date is interpolated, no value is carried forward, no row is synthesised
to fill a hole. Run 'coverage' to see which sessions are actually held before reading
anything into a short export.

Round-trip, honestly: this does NOT round-trip through 'import'. Each JSONL line is
a valid JSON object, so import will parse it -- but import POSTs every line to the
upstream API path for the named resource, and every NCCPL /api/* path is a read
endpoint that takes a settlement date. There is no write API to import into, so
importing an export sends rows at a read endpoint and puts nothing back in the local
store. The command that loads data back into the store is 'ingest', which takes a
JSON array of upstream row objects for one (resource, date):

  nccpl-pp-cli export --resources var-margins --from 2026-09-04 --to 2026-09-04 \
    | jq -s 'map(.payload)' > var-margins-2026-09-04.json
  nccpl-pp-cli ingest var-margins-2026-09-04.json --resource var-margins --date 2026-09-04

That path was measured, not assumed: resource, date, row key and payload come back
byte-for-byte identical. observed_at does not, by design -- ingest stamps the vintage
at which the RECEIVING store first saw the row, and a copy must not inherit a
first-observation time it did not earn.

Output routing: --format chooses the dump format (jsonl or csv); the global --json,
--plain, --compact and --select apply to the SUMMARY, not to the dump. With --output
set, the dump goes to the file and the summary to stdout. Without it, the dump goes
to stdout and the summary to stderr.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli export --output store.jsonl
  nccpl-pp-cli export --resources fipi,lipi --from 2026-08-01 --to 2026-09-04
  nccpl-pp-cli export --format csv --resources var-margins --output var-margins.csv
  nccpl-pp-cli export --resources mts --dry-run --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--format=jsonl",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			dryRun := dryRunOK(flags)
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("export takes no positional arguments; got %q", strings.Join(args, " ")))
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format != "jsonl" && format != "csv" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown --format %q; valid: jsonl, csv", format))
			}
			resources, err := nccplSearchResources(resourcesCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if err := nccplSearchDateBounds(fromDate, toDate); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			toStdout := strings.TrimSpace(outputPath) == "" || strings.TrimSpace(outputPath) == "-"
			target := "stdout"
			if !toStdout {
				target = outputPath
			}
			// When the caller asked for machine-readable output and named no file,
			// stdout must carry exactly one JSON document -- so the records are
			// inlined into the summary rather than streamed alongside it. This is
			// what `panel` and `search` already do, and it keeps `export --json`
			// parseable instead of emitting a bare JSONL stream with the summary
			// on a different stream.
			// Gate on the caller explicitly asking for JSON, not on
			// wantsHumanTable: that returns false for any non-TTY, which would
			// turn an ordinary `export --format jsonl > out.jsonl` pipe into a
			// single JSON document and break the streaming contract.
			inlineJSON := toStdout && !dryRun && (flags.asJSON || flags.agent)
			// Otherwise the dump owns stdout when no file was named, so the summary
			// moves to stderr to keep the data stream clean. A dry-run writes no
			// dump at all, so its report belongs on stdout like every other
			// command's.
			summaryW := cmd.OutOrStdout()
			if toStdout && !dryRun && !inlineJSON {
				summaryW = cmd.ErrOrStderr()
			}

			view := nccplExportView{
				Format:    format,
				Output:    target,
				From:      fromDate,
				To:        toDate,
				Resources: make([]nccplExportResource, 0),
				DBPath:    dbPath,
			}
			if dryRun {
				view.DryRun = true
				view.Action = "export"
			}
			syncHint := fmt.Sprintf("run: nccpl-pp-cli sync --resources fipi --latest-only --db %s\n", dbPath)

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s", dbPath, syncHint)
				view.Note = "no local store to export; nothing was written"
				return nccplWriteExportSummary(summaryW, flags, view)
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			ready, err := store.NCCPLSchemaReady(ctx, db)
			if err != nil {
				return err
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\n%s", dbPath, syncHint)
				view.Note = "no NCCPL observations stored yet; nothing was written"
				return nccplWriteExportSummary(summaryW, flags, view)
			}

			// With no --resources filter, dump only what the store actually holds
			// rather than every registered resource name, so the summary lists real
			// coverage instead of a wall of empty rows.
			if len(resources) == 0 {
				resources, err = store.NCCPLStoredResources(ctx, db)
				if err != nil {
					return err
				}
			}

			var (
				sink io.Writer = cmd.OutOrStdout()
				file *os.File
			)
			if !dryRun && !toStdout {
				file, err = os.Create(filepath.Clean(outputPath)) // #nosec G304 -- writing to the user-specified --output path is this flag's documented purpose.
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer func() { _ = file.Close() }()
				sink = file
			}
			buffered := bufio.NewWriter(sink)
			enc := json.NewEncoder(buffered)
			cw := csv.NewWriter(buffered)
			if !dryRun && format == "csv" {
				if err := cw.Write(nccplExportCSVHeader); err != nil {
					return fmt.Errorf("writing %s: %w", target, err)
				}
			}

			for _, name := range resources {
				obs, err := store.NCCPLObservations(ctx, db, name, fromDate, toDate)
				if err != nil {
					return err
				}
				if len(obs) == 0 {
					continue
				}
				summary := nccplExportResource{
					Resource:  name,
					Rows:      len(obs),
					FirstDate: obs[0].Date,
					LastDate:  obs[len(obs)-1].Date,
				}
				seen := make(map[string]bool, len(obs))
				for _, o := range obs {
					if !seen[o.Date] {
						seen[o.Date] = true
						summary.Dates++
					}
					if dryRun {
						continue
					}
					rec := nccplExportRecord{
						Resource:   o.Resource,
						Date:       o.Date,
						Key:        o.Key,
						ObservedAt: o.ObservedAt,
						Payload:    nccplExportPayload(o.Payload),
					}
					if inlineJSON {
						view.Records = append(view.Records, rec)
						continue
					}
					if format == "csv" {
						if err := cw.Write([]string{rec.Resource, rec.Date, rec.Key, rec.ObservedAt, string(rec.Payload)}); err != nil {
							return fmt.Errorf("writing %s: %w", target, err)
						}
						continue
					}
					if err := enc.Encode(rec); err != nil {
						return fmt.Errorf("writing %s: %w", target, err)
					}
				}
				view.Resources = append(view.Resources, summary)
				view.TotalRows += summary.Rows
			}

			if !dryRun {
				cw.Flush()
				if err := cw.Error(); err != nil {
					return fmt.Errorf("writing %s: %w", target, err)
				}
				if err := buffered.Flush(); err != nil {
					return fmt.Errorf("writing %s: %w", target, err)
				}
				if file != nil {
					if err := file.Close(); err != nil {
						return fmt.Errorf("closing %s: %w", target, err)
					}
				}
			}

			switch {
			case dryRun:
				view.Would = fmt.Sprintf("write %d %s record(s) from %d resource(s) to %s; nothing was written and the store was only read",
					view.TotalRows, format, len(view.Resources), target)
			case view.TotalRows == 0:
				view.Note = "nothing stored matched this filter; check 'coverage' for which sessions are held before reading anything into an empty export"
			case inlineJSON:
				view.Note = "exported what is stored, inlined under records because --json was set without --output; sessions that were never fetched are absent, not zero -- run 'coverage' to see which are held. Pass --output to stream the chosen --format to a file instead."
			default:
				view.Note = "exported what is stored; sessions that were never fetched are absent, not zero -- run 'coverage' to see which are held"
			}
			return nccplWriteExportSummary(summaryW, flags, view)
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Write the dump to this file; empty or - means stdout")
	cmd.Flags().StringVar(&format, "format", "jsonl", "Dump format: jsonl (one JSON record per line) or csv")
	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to export; empty means every stored resource")
	cmd.Flags().StringVar(&fromDate, "from", "", "Earliest settlement date to export (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Latest settlement date to export (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// nccplExportPayload returns the stored payload as raw JSON.
//
// Everything the sync and ingest paths store went through json.Marshal, so the
// fallback should never fire; it exists so that one unparseable row -- a
// hand-edited store, a future schema change -- degrades to a quoted string on its
// own line instead of making the entire export unparseable.
func nccplExportPayload(payload string) json.RawMessage {
	if json.Valid([]byte(payload)) {
		return json.RawMessage(payload)
	}
	quoted, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return json.RawMessage(quoted)
}

// nccplWriteExportSummary renders the run summary through the CLI's normal output
// rules, on whichever stream the dump did not claim.
func nccplWriteExportSummary(w io.Writer, flags *rootFlags, view nccplExportView) error {
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if view.DryRun {
		fmt.Fprintf(w, "dry-run: would %s\n", view.Would)
		return nil
	}
	if len(view.Resources) > 0 {
		fmt.Fprintf(w, "%-20s %-9s %-7s %-12s %s\n", "RESOURCE", "ROWS", "DATES", "FIRST", "LAST")
		for _, r := range view.Resources {
			fmt.Fprintf(w, "%-20s %-9d %-7d %-12s %s\n", r.Resource, r.Rows, r.Dates, r.FirstDate, r.LastDate)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "exported %d row(s) from %d resource(s) as %s to %s\n",
		view.TotalRows, len(view.Resources), view.Format, view.Output)
	if view.Note != "" {
		fmt.Fprintf(w, "%s\n", view.Note)
	}
	return nil
}
