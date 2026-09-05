package cli

// pp:data-source local

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// Browser-captured ingest.
//
// NCCPL sits behind a Cloudflare clearance gate that cannot be replayed by any
// non-browser client. This was established empirically, not assumed: a byte-exact
// TLS fingerprint match to the operator's own Chrome (identical ja4, ja4_r,
// peetprint and HTTP/2 Akamai fingerprint), sending the exact header set and live
// clearance cookies, over both HTTP/2 and HTTP/3, receives the same challenge as
// sending no cookies at all. See proofs/cloudflare-investigation.md.
//
// So the browser becomes the CAPTURE step rather than the runtime: the operator
// browses NCCPL normally, exports what the page already fetched, and this command
// folds it into the same local store the automated paths write. Every downstream
// command -- panel, verify, coverage, universe, leverage, risk-changes -- then works
// on it unchanged, because they read the store, not the network.
//
// Accepts three shapes:
//   - a DevTools HAR export (.har)
//   - a raw API response body, i.e. {"success":true,"<envelope>":[...]}
//   - an array of rows on its own
type nccplIngestResource struct {
	Resource string `json:"resource"`
	Date     string `json:"date"`
	Rows     int    `json:"rows"`
	Source   string `json:"source"`
}

type nccplIngestView struct {
	Ingested  []nccplIngestResource `json:"ingested"`
	TotalRows int                   `json:"total_rows"`
	Skipped   []string              `json:"skipped,omitempty"`
	DBPath    string                `json:"db_path"`
	Note      string                `json:"note,omitempty"`
}

func newNCCPLIngestCmd(flags *rootFlags) *cobra.Command {
	var (
		resource   string
		date       string
		dbPath     string
		stdinInput bool
	)

	cmd := &cobra.Command{
		Use:   "ingest [file...]",
		Short: "Load browser-captured NCCPL responses into the local store",
		Long: strings.Trim(`
Fold NCCPL data captured through a browser into the same local store the automated
paths write, so every other command works on it unchanged.

Why this exists: NCCPL's Cloudflare clearance cannot be replayed by any non-browser
HTTP client. That is a measured result, not an assumption -- an exact TLS fingerprint
match to your own Chrome, with valid clearance cookies, over both HTTP/2 and HTTP/3,
receives the same challenge as sending no cookies at all. Your browser still reaches
the site perfectly, so it does the fetching and this command does the parsing.

How to use it:
  1. Open https://www.nccpl.com.pk/market-information and click the tabs you want
     (VAR margins, MTS/MFS/MSF open positions, SLB, settlement).
  2. In DevTools > Network, right-click and "Save all as HAR with content".
  3. Run: nccpl-pp-cli ingest capture.har

A HAR is matched to resources automatically by request path and request body date.
For a raw JSON body saved by hand, pass --resource and --date so it can be filed.

To pipe one captured response body straight in, with no temp file, use --stdin:
  pbpaste | nccpl-pp-cli ingest --stdin --resource var-margins --date 2026-09-04
--stdin reads a single HAR or JSON body from standard input and files it exactly as a
file argument would; it can be combined with file arguments, and stdin is read first.

Nothing is interpolated or invented: only dates actually present in the capture are
recorded, and each gets a coverage entry exactly as a live fetch would.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli ingest capture.har
  nccpl-pp-cli ingest var-margins.json --resource var-margins --date 2026-09-04
  nccpl-pp-cli ingest *.har --json
`, "\n"),
		Annotations: map[string]string{
			// ingest reads a browser-captured HAR from disk and writes its rows into the local
			// store. It makes no network call at all.
			"mcp:local-write": "true",
			"pp:happy-args":   "--resource=var-margins;--date=2026-09-04",
			// A runnable fixture: with --stdin the happy path files a real body into
			// the local store instead of dry-running against a capture.har that does
			// not exist, so the command is actually exercised.
			"pp:happy-stdin": `{"success":true,"margins":[{"symbol":"HUBC","var_value":"15.5","hair_cut":"20.0","26week_avg":"11.3616","acc_qty%":"0.0"}]}`,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ingest")
			}
			if len(args) == 0 && !stdinInput {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give at least one HAR or JSON file to ingest, or pass --stdin to read one from stdin"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureNCCPLSchema(ctx, db); err != nil {
				return err
			}

			view := nccplIngestView{
				Ingested: make([]nccplIngestResource, 0),
				Skipped:  make([]string, 0),
				DBPath:   dbPath,
			}
			observed := time.Now()

			// sources pairs each payload with the label reported as its Source, so a
			// stdin payload flows through exactly the same parse and store path as a
			// file. Reading stdin first keeps ordering deterministic.
			type ingestSource struct {
				label string
				raw   []byte
			}
			sources := make([]ingestSource, 0, len(args)+1)
			if stdinInput {
				raw, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if len(bytes.TrimSpace(raw)) == 0 {
					return usageErr(fmt.Errorf("--stdin was given but stdin was empty"))
				}
				sources = append(sources, ingestSource{label: "stdin", raw: raw})
			}
			for _, path := range args {
				raw, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("reading %s: %w", path, err)
				}
				sources = append(sources, ingestSource{label: filepath.Base(path), raw: raw})
			}

			for _, src := range sources {
				raw := src.raw
				base := src.label
				var batches []nccplIngestBatch
				if looksLikeHAR(raw) {
					found, skips, harErr := nccplBatchesFromHAR(raw)
					if harErr != nil {
						return fmt.Errorf("%s: %w", base, harErr)
					}
					batches = found
					// Entries the HAR parser refused are reported, not dropped. A capture
					// that quietly ingested nothing is indistinguishable from a day the
					// market published nothing, and only one of those is something the
					// operator can act on.
					for _, s := range skips {
						view.Skipped = append(view.Skipped, fmt.Sprintf("%s: %s", base, s))
					}
				} else {
					b, err := nccplBatchFromBody(raw, resource, date)
					if err != nil {
						view.Skipped = append(view.Skipped, fmt.Sprintf("%s: %v", base, err))
						continue
					}
					batches = []nccplIngestBatch{b}
				}
				for _, b := range batches {
					if len(b.Rows) == 0 {
						continue
					}
					if err := store.SaveNCCPLDate(ctx, db, b.Resource, b.Date, b.Rows, observed); err != nil {
						return err
					}
					view.Ingested = append(view.Ingested, nccplIngestResource{
						Resource: b.Resource, Date: b.Date, Rows: len(b.Rows), Source: base,
					})
					view.TotalRows += len(b.Rows)
				}
			}

			sort.Slice(view.Ingested, func(i, j int) bool {
				if view.Ingested[i].Resource != view.Ingested[j].Resource {
					return view.Ingested[i].Resource < view.Ingested[j].Resource
				}
				return view.Ingested[i].Date < view.Ingested[j].Date
			})
			if view.TotalRows == 0 {
				if len(view.Skipped) > 0 {
					view.Note = fmt.Sprintf("nothing ingested: %d input(s) were refused. See \"skipped\" for why each one was not stored -- a body or capture entry that cannot be identified is never filed under a guess.", len(view.Skipped))
				} else {
					view.Note = "nothing ingested: the capture held no recognisable NCCPL /api/*/data responses. Re-export with \"Save all as HAR with content\" so response bodies are included."
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, r := range view.Ingested {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %5d rows  (%s)\n", r.Resource, r.Date, r.Rows, r.Source)
			}
			for _, s := range view.Skipped {
				fmt.Fprintf(cmd.ErrOrStderr(), "skipped %s\n", s)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\ningested %d row(s) into %s\n", view.TotalRows, dbPath)
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "Resource name for a raw JSON body (ignored for HAR files)")
	cmd.Flags().StringVar(&date, "date", "", "Settlement date YYYY-MM-DD for a raw JSON body (ignored for HAR files)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&stdinInput, "stdin", false, "Read one HAR or JSON body from stdin instead of (or in addition to) file arguments")
	return cmd
}

type nccplIngestBatch struct {
	Resource string
	Date     string
	Rows     []store.NCCPLRow
}

func looksLikeHAR(raw []byte) bool {
	var probe struct {
		Log *struct {
			Entries []json.RawMessage `json:"entries"`
		} `json:"log"`
	}
	return json.Unmarshal(raw, &probe) == nil && probe.Log != nil
}

// nccplBatchFromBody parses a single saved response body.
func nccplBatchFromBody(raw []byte, resource, date string) (nccplIngestBatch, error) {
	if strings.TrimSpace(resource) == "" || strings.TrimSpace(date) == "" {
		return nccplIngestBatch{}, fmt.Errorf("raw JSON body needs --resource and --date")
	}
	res, ok := nccplResourceByName(resource)
	if !ok {
		return nccplIngestBatch{}, fmt.Errorf("unknown resource %q", resource)
	}
	rows, err := nccplRowsFromEnvelope(raw, res)
	if err != nil {
		return nccplIngestBatch{}, err
	}
	return nccplIngestBatch{Resource: res.Name, Date: date, Rows: rows}, nil
}

// nccplKnownEnvelopes is the fixed, exhaustive list of envelope keys this API is
// known to use, deduplicated in registry order. It is the ONLY set of names the
// fallback in nccplRowsFromEnvelope will read, and its order never varies, so the
// same body always yields the same rows.
var nccplKnownEnvelopes = func() []string {
	out := make([]string, 0, len(nccplResources))
	seen := make(map[string]bool, len(nccplResources))
	for _, r := range nccplResources {
		if r.Envelope == "" || seen[r.Envelope] {
			continue
		}
		seen[r.Envelope] = true
		out = append(out, r.Envelope)
	}
	return out
}()

// nccplRowsFromEnvelope pulls the row array out of a response body.
//
// Exactly three shapes are accepted, tried in this order:
//
//  1. a bare array of row objects;
//  2. an object carrying THIS resource's documented envelope key;
//  3. an object carrying exactly one of the API's own other envelope keys, which is
//     what a body hand-saved from a sibling endpoint -- or upstream envelope drift --
//     looks like.
//
// Anything else is refused, and the caller reports the refusal in its skipped list.
//
// Case 3 used to be a walk over every key in the body, taking the first array-valued
// one under ANY name. Go randomises map iteration order, so a body holding several
// arrays produced a different resource's rows from run to run, and an `errors` array
// -- or an unrelated `data` array pasted from another endpoint -- was stored as
// authoritative observations for the named resource and date. A body whose contents
// cannot be identified must not become data, so the allow-list is closed, the order
// is fixed, and two candidates at once is an ambiguity that refuses rather than
// guesses. Refusing costs one observation; guessing corrupts the series.
func nccplRowsFromEnvelope(raw []byte, res nccplResource) ([]store.NCCPLRow, error) {
	objs, err := nccplRowObjectsFromEnvelope(raw, res)
	if err != nil {
		return nil, err
	}
	rows := make([]store.NCCPLRow, 0, len(objs))
	seen := map[string]bool{}
	for i, o := range objs {
		enc, err := json.Marshal(o)
		if err != nil {
			continue
		}
		rows = append(rows, store.NCCPLRow{Key: nccplRowKey(res, o, i, seen), Payload: string(enc)})
	}
	return rows, nil
}

// nccplRowObjectsFromEnvelope resolves the row array, or explains why it will not.
func nccplRowObjectsFromEnvelope(raw []byte, res nccplResource) ([]map[string]any, error) {
	var objs []map[string]any
	if err := json.Unmarshal(raw, &objs); err == nil {
		return objs, nil
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("not JSON")
	}
	if payload, ok := env[res.Envelope]; ok {
		if err := json.Unmarshal(payload, &objs); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", res.Envelope, err)
		}
		return objs, nil
	}

	// The documented key is absent. Consider only the API's own other envelope
	// names, in their fixed registry order, and only where the value is genuinely a
	// JSON array -- a null or an object under a known name is not row data.
	matched := make([]string, 0, 2)
	decoded := make(map[string][]map[string]any, 2)
	for _, key := range nccplKnownEnvelopes {
		if key == res.Envelope {
			continue
		}
		payload, ok := env[key]
		if !ok || !nccplIsJSONArray(payload) {
			continue
		}
		var candidate []map[string]any
		if err := json.Unmarshal(payload, &candidate); err != nil {
			continue
		}
		matched = append(matched, key)
		decoded[key] = candidate
	}
	switch len(matched) {
	case 1:
		return decoded[matched[0]], nil
	case 0:
		return nil, fmt.Errorf("no %q array in body (keys present: %s)", res.Envelope, nccplEnvelopeKeyList(env))
	default:
		return nil, fmt.Errorf("body has no %q array and carries more than one known envelope (%s), so which endpoint it came from is ambiguous",
			res.Envelope, strings.Join(matched, ", "))
	}
}

// nccplIsJSONArray reports whether a raw envelope value is a JSON array.
func nccplIsJSONArray(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '['
}

// nccplEnvelopeKeyList renders the keys a refused body did carry, sorted so the
// message is reproducible and bounded so a stray large object cannot flood the
// skipped list.
func nccplEnvelopeKeyList(env map[string]json.RawMessage) string {
	if len(env) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	const maxKeys = 12
	if len(keys) > maxKeys {
		keys = append(keys[:maxKeys:maxKeys], fmt.Sprintf("... and %d more", len(keys)-maxKeys))
	}
	return strings.Join(keys, ", ")
}
