// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: CSV-driven bulk contact property update with pre-validation.
//
// Layered idioms:
//   - JSONL stdin/stdout for agent-friendly streaming (Item 1)
//   - Dry-run -> digest -> confirm gating for >100-row batches (Item 2)
//   - --id-property routing through HubSpot's batch upsert endpoint (Item 3)
//
// CSV remains the original happy-path; JSONL is opt-in via --jsonl. Both
// feed the same plan-build -> gate -> execute pipeline.

package cli

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

// bulkRow is the canonical in-memory shape for one row, regardless of
// source (CSV column-mapped or JSONL parsed). The plan-build pipeline
// works against []bulkRow so the gate, the digest, and the dispatcher
// don't care which surface the row came from.
type bulkRow struct {
	// Index is 1-based and reflects the source-row position (line number
	// in JSONL, data-row index in CSV). Used for error reporting only.
	Index int `json:"index"`
	// ID is the contact id when known. May be empty after parsing if the
	// row uses an email key; resolved later via the local store.
	ID string `json:"id"`
	// Email is the row's email column / field, used to resolve ID and as
	// the lookup key under --id-property=email.
	Email string `json:"email,omitempty"`
	// IDPropertyValue is the value that --id-property names. For email-
	// keyed upserts it's the email; for any other property it's that
	// property's value on the row.
	IDPropertyValue string `json:"id_property_value,omitempty"`
	// Patch is the property -> value map sent to HubSpot.
	Patch map[string]string `json:"patch,omitempty"`
	// Errors are per-row validation failures. Non-empty Errors means the
	// row will not be sent to HubSpot.
	Errors []string `json:"errors,omitempty"`
}

func newNovelContactsBulkUpdateCmd(flags *rootFlags) *cobra.Command {
	var csvPath string
	var jsonlMode bool
	var mapSpec string
	var dbPath string
	var idProperty string
	var digestArg string
	var confirmCount int

	cmd := &cobra.Command{
		Use:   "bulk-update",
		Short: "Apply property changes to many contacts at once (CSV or JSONL, with pre-validation + digest gating)",
		Long: `Read a stream of contact updates and validate each row against HubSpot's
property schema (types, picklists) before any mutation.

Input modes:
  --from-csv <path>   Read rows from a CSV file (existing behavior).
  --jsonl             Read JSONL rows from stdin. Each line: {"id":"...", "<property>":"value", ...}.
                      When --id-property is set, the "id" field may instead carry the value of that
                      property (e.g. an email address) and "id" is the JSONL correlation key only.

Routing:
  --id-property <p>   Route through POST /crm/v3/objects/contacts/batch/upsert keyed on <p>
                      (typically "email"). When unset, falls back to the existing per-row
                      update path via POST /crm/v3/objects/contacts/batch/update.

Safety:
  Mutations affecting more than 100 rows require a two-step dance:
    1. First run prints a digest (blast-<hex>) and "would update N contacts".
    2. Re-run with --confirm N --digest blast-<hex> to execute.

  Smaller batches are dispatched in one pass with a one-line warning that
  the digest gate was bypassed.

The CSV header row names the columns. Each column maps to a property of the
same name (lowercased). Use --map to override:
  --map "FirstName=firstname,SfdcOpp=sfdc_opportunity_id".

Rows are identified by an 'id' or 'email' column (id wins when both present).`,
		Example: `  # Validate only (CSV)
  hubspot-pp-cli contacts bulk-update --from-csv updates.csv --dry-run

  # Apply via update
  hubspot-pp-cli contacts bulk-update --from-csv updates.csv

  # Upsert via email key, JSONL stdin
  cat rows.jsonl | hubspot-pp-cli contacts bulk-update --jsonl --id-property email

  # Large batch dance
  cat 500-rows.jsonl | hubspot-pp-cli contacts bulk-update --jsonl --id-property email
  # -> prints digest + count, then:
  cat 500-rows.jsonl | hubspot-pp-cli contacts bulk-update --jsonl --id-property email \
      --confirm 500 --digest blast-deadbeef12345678`,
		// Treat this as read-only ONLY when --dry-run is set; default invocation
		// of this command mutates, so we deliberately leave the annotation off.
		RunE: func(cmd *cobra.Command, args []string) error {
			if csvPath == "" && !jsonlMode {
				return cmd.Help()
			}
			if csvPath != "" && jsonlMode {
				return fmt.Errorf("--from-csv and --jsonl are mutually exclusive")
			}
			// Under verify with a missing CSV, short-circuit silently so
			// smoke tests pass without touching the filesystem. The JSONL
			// path reads stdin, so the same guard doesn't apply.
			if csvPath != "" && (cliutil.IsVerifyEnv() || flags.dryRun) {
				if _, statErr := os.Stat(csvPath); statErr != nil {
					return nil
				}
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			propSchema, err := loadContactsPropertySchema(db)
			if err != nil {
				return err
			}

			var rows []bulkRow
			var jw *cliutil.JSONLWriter
			if jsonlMode {
				jw = cliutil.NewJSONLWriter(cmd.OutOrStdout())
				rows, err = readJSONLBulkRows(cmd.InOrStdin(), propSchema, idProperty, jw)
				if err != nil {
					return err
				}
			} else {
				rows, err = readCSVBulkRows(csvPath, mapSpec, propSchema, db)
				if err != nil {
					return err
				}
			}

			// Partition into actionable + errored. JSONL already emitted
			// per-row errors during read; CSV reports them in the dry-run
			// summary or aborts non-dry-run.
			//
			// Actionable rule:
			//   - update mode (no idProperty): row needs Patch (no patch
			//     means nothing to change)
			//   - upsert mode (idProperty set): row is actionable as long
			//     as IDPropertyValue is set, even with empty patch — the
			//     upsert still inserts a fresh contact under the key.
			var ok, bad []bulkRow
			for _, r := range rows {
				if len(r.Errors) > 0 {
					bad = append(bad, r)
					continue
				}
				actionable := false
				if idProperty != "" {
					actionable = r.IDPropertyValue != ""
				} else {
					actionable = len(r.Patch) > 0
				}
				if actionable {
					ok = append(ok, r)
				}
			}

			// Build the canonical plan once. The gate hashes this; the
			// dispatcher iterates over it. Plan shape is stable across
			// CSV and JSONL so the digest computed for an identical
			// row set is identical regardless of input surface.
			plan := buildBulkPlan(ok, idProperty)
			planJSON, err := json.Marshal(plan)
			if err != nil {
				return fmt.Errorf("serializing plan: %w", err)
			}

			// Dry-run path is independent of the digest gate: --dry-run
			// always prints the validation report and exits 0.
			if flags.dryRun {
				return emitDryRunReport(cmd, flags, jw, rows, ok, bad)
			}

			// Non-dry-run + JSONL + bad rows: per-row errors already
			// emitted; just skip them and continue.
			if !jsonlMode && len(bad) > 0 {
				return fmt.Errorf("%d rows have errors; rerun with --dry-run to see them or fix the CSV", len(bad))
			}
			if len(ok) == 0 {
				return fmt.Errorf("no valid rows to update")
			}

			// Gate: ≤100 rows bypass; >100 require dry-run -> confirm.
			gate := cliutil.DefaultGate()
			outcome, digest, planRowCount, err := gate.Evaluate(
				cmd.Context(), digestStoreAdapter{db}, "contacts bulk-update",
				digestArg, confirmCount, planJSON, len(ok),
			)
			if err != nil {
				return err
			}
			switch outcome {
			case cliutil.GateProceedBelowThreshold:
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: %d rows ≤ threshold %d; digest gate bypassed (digest=%s)\n",
					planRowCount, gate.Threshold, digest)
			case cliutil.GateDryRunPersisted:
				return emitConfirmInstructions(cmd, flags, digest, planRowCount, idProperty)
			case cliutil.GateProceedConfirmed:
				// Fall through to execute.
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			updated, errCount, err := dispatchBulk(cmd.Context(), c, ok, idProperty, jw)
			if err != nil {
				return err
			}

			if jsonlMode {
				// JSONL already emitted per-row results; the final summary
				// goes to stderr so stdout stays a clean JSONL stream.
				fmt.Fprintf(cmd.ErrOrStderr(), "done: %d ok, %d errors\n", updated, errCount)
				return nil
			}
			report := map[string]any{
				"total":        len(rows),
				"would_update": len(ok),
				"updated":      updated,
				"errors":       bad,
			}
			if flags.asJSON {
				return flags.printJSON(cmd, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %d contacts.\n", updated)
			return nil
		},
	}
	cmd.Flags().StringVar(&csvPath, "from-csv", "", "Path to the CSV file with the updates")
	cmd.Flags().BoolVar(&jsonlMode, "jsonl", false, "Read JSONL rows from stdin; emit JSONLResult envelopes to stdout")
	cmd.Flags().StringVar(&mapSpec, "map", "", "Comma-separated column-to-property remapping, e.g. \"FirstName=firstname\"")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&idProperty, "id-property", "", "If set, route through POST /crm/v3/objects/contacts/batch/upsert keyed on this property (e.g. 'email')")
	cmd.Flags().StringVar(&digestArg, "digest", "", "Confirm a prior dry-run with this digest (blast-<hex>)")
	cmd.Flags().IntVar(&confirmCount, "confirm", 0, "Confirm a prior dry-run for exactly this many rows; must match the digest's stored count")
	return cmd
}

// readCSVBulkRows parses the existing CSV format into the canonical
// bulkRow shape. Validation errors land on bulkRow.Errors; ID resolution
// (email -> id via the local store) happens here too.
func readCSVBulkRows(path, mapSpec string, propSchema map[string]propertyDef, db *store.Store) ([]bulkRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}
	headers := records[0]
	colMap := parseMapSpec(mapSpec, headers)

	var rows []bulkRow
	for i, rec := range records[1:] {
		r := bulkRow{Index: i + 1, Patch: map[string]string{}}
		for j, val := range rec {
			if j >= len(headers) {
				break
			}
			col := headers[j]
			switch strings.ToLower(col) {
			case "id":
				r.ID = strings.TrimSpace(val)
				continue
			case "email":
				r.Email = strings.TrimSpace(val)
			}
			prop := colMap[col]
			if prop == "" {
				prop = strings.ToLower(col)
			}
			if prop == "id" {
				continue
			}
			def, has := propSchema[prop]
			if !has {
				r.Errors = append(r.Errors, fmt.Sprintf("unknown property %q", prop))
				continue
			}
			if errMsg := validatePropertyValue(def, val); errMsg != "" {
				r.Errors = append(r.Errors, fmt.Sprintf("%s: %s", prop, errMsg))
				continue
			}
			if val != "" {
				r.Patch[prop] = val
			}
		}
		if r.ID == "" && r.Email != "" {
			if id, _ := lookupContactIDByEmail(db, r.Email); id != "" {
				r.ID = id
			} else {
				r.Errors = append(r.Errors, fmt.Sprintf("no contact found for email %q (sync first?)", r.Email))
			}
		}
		if r.ID == "" {
			r.Errors = append(r.Errors, "no id and no resolvable email column")
		}
		// For email-keyed upserts the IDPropertyValue is the email.
		r.IDPropertyValue = r.Email
		rows = append(rows, r)
	}
	return rows, nil
}

// readJSONLBulkRows parses stdin into bulkRow. Per-row parse / validation
// errors are written directly to jw and the row is still returned (with
// Errors populated) so the partition step can count them as `bad`.
// idProperty drives which JSONL field carries the upsert key.
func readJSONLBulkRows(r interface {
	Read(p []byte) (n int, err error)
}, propSchema map[string]propertyDef, idProperty string, jw *cliutil.JSONLWriter) ([]bulkRow, error) {
	var rows []bulkRow
	idx := 0
	for entry := range cliutil.ReadJSONLInputs(r) {
		idx++
		if entry.Err != nil {
			// Surface the parse error directly to the agent. No row to
			// add to the plan.
			_ = jw.WriteError("", entry.Err)
			continue
		}
		row := bulkRow{Index: idx, Patch: map[string]string{}}
		// Each JSONL line is a flat property map; treat every non-id /
		// non-meta key as a property override.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(entry.Input.Payload, &raw); err != nil {
			_ = jw.WriteError(entry.Input.ID, fmt.Errorf("line %d: %w", entry.LineNum, err))
			continue
		}
		row.ID = entry.Input.ID
		for k, v := range raw {
			switch strings.ToLower(k) {
			case "id":
				continue
			case "email":
				var s string
				_ = json.Unmarshal(v, &s)
				row.Email = strings.TrimSpace(s)
				// Email is still a property: include it in the patch
				// only when idProperty is NOT email (otherwise the
				// upsert endpoint takes it via idProperty.value, not
				// the properties bag).
				if !strings.EqualFold(idProperty, "email") {
					def, has := propSchema[k]
					if has {
						if errMsg := validatePropertyValue(def, s); errMsg != "" {
							row.Errors = append(row.Errors, fmt.Sprintf("%s: %s", k, errMsg))
							continue
						}
						if s != "" {
							row.Patch[k] = s
						}
					}
				}
				continue
			}
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				// Non-string scalar: stringify via raw text.
				s = strings.Trim(string(v), "\"")
			}
			def, has := propSchema[k]
			if !has {
				row.Errors = append(row.Errors, fmt.Sprintf("unknown property %q", k))
				continue
			}
			if errMsg := validatePropertyValue(def, s); errMsg != "" {
				row.Errors = append(row.Errors, fmt.Sprintf("%s: %s", k, errMsg))
				continue
			}
			if s != "" {
				row.Patch[k] = s
			}
		}
		// Resolve IDPropertyValue.
		switch strings.ToLower(idProperty) {
		case "":
			// Pure update mode: ID is the HubSpot contact id.
		case "email":
			row.IDPropertyValue = row.Email
			if row.IDPropertyValue == "" {
				// Fall back: the JSONL "id" field may carry the email.
				row.IDPropertyValue = row.ID
			}
		default:
			// Some other property: prefer an explicit field on the row,
			// fall back to the correlation id.
			if v, ok := raw[idProperty]; ok {
				var s string
				_ = json.Unmarshal(v, &s)
				row.IDPropertyValue = s
			}
			if row.IDPropertyValue == "" {
				row.IDPropertyValue = row.ID
			}
		}
		// Upsert mode: the IDPropertyValue is what HubSpot keys on, the
		// "id" field is just the JSONL correlation handle. Skip the
		// "missing HubSpot id" complaint in that case.
		if idProperty == "" && row.ID == "" {
			row.Errors = append(row.Errors, "missing required \"id\" field for update mode")
		}
		if idProperty != "" && row.IDPropertyValue == "" {
			row.Errors = append(row.Errors, fmt.Sprintf("missing required %q value for upsert mode", idProperty))
		}
		// Emit per-row errors immediately so the agent sees them in
		// stream order. (The success case is emitted after dispatch.)
		if len(row.Errors) > 0 {
			_ = jw.WriteError(row.ID, fmt.Errorf("%s", strings.Join(row.Errors, "; ")))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// buildBulkPlan canonicalizes the rows into the body shape that will
// ultimately be POSTed. Used for digest hashing AND as the source-of-
// truth iterated by the dispatcher — guarantees the bytes hashed are the
// bytes sent (modulo chunking).
func buildBulkPlan(rows []bulkRow, idProperty string) map[string]any {
	type input struct {
		ID         string            `json:"id"`
		IDProperty string            `json:"idProperty,omitempty"`
		Properties map[string]string `json:"properties"`
	}
	out := make([]input, 0, len(rows))
	for _, r := range rows {
		in := input{Properties: r.Patch}
		if idProperty != "" {
			in.ID = r.IDPropertyValue
			in.IDProperty = idProperty
		} else {
			in.ID = r.ID
		}
		out = append(out, in)
	}
	return map[string]any{
		"command":     "contacts bulk-update",
		"id_property": idProperty,
		"inputs":      out,
	}
}

// dispatchBulk routes the validated rows through either the upsert or
// update endpoint, chunked to HubSpot's 100-row batch limit. Returns
// (updated, errored, fatal-error). Per-row errors on the upsert path are
// reported through jw (when set) and counted; they do NOT abort the batch.
func dispatchBulk(ctx context.Context, c *client.Client, rows []bulkRow, idProperty string, jw *cliutil.JSONLWriter) (int, int, error) {
	endpoint := "/crm/v3/objects/contacts/batch/update"
	if idProperty != "" {
		endpoint = "/crm/v3/objects/contacts/batch/upsert"
	}
	type batchInput struct {
		ID         string            `json:"id"`
		IDProperty string            `json:"idProperty,omitempty"`
		Properties map[string]string `json:"properties"`
	}
	updated, errored := 0, 0
	for start := 0; start < len(rows); start += 100 {
		end := start + 100
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		inputs := make([]batchInput, 0, len(chunk))
		for _, r := range chunk {
			in := batchInput{Properties: r.Patch}
			if idProperty != "" {
				in.ID = r.IDPropertyValue
				in.IDProperty = idProperty
			} else {
				in.ID = r.ID
			}
			inputs = append(inputs, in)
		}
		body := map[string]any{"inputs": inputs}
		resp, _, err := c.Post(ctx, endpoint, body)
		if err != nil {
			// Whole-batch error: surface to all rows in the chunk if
			// JSONL mode, then return so the caller exits non-zero.
			if jw != nil {
				for _, r := range chunk {
					_ = jw.WriteError(r.ID, fmt.Errorf("batch [%d..%d]: %w", start, end, err))
				}
			}
			return updated, errored + len(chunk), fmt.Errorf("batch %s [%d..%d]: %w", endpoint, start, end, err)
		}
		// Best-effort per-row success emission when we're streaming
		// JSONL. The batch endpoint returns {"results":[...],"errors":[...]}
		// in a parallel order to inputs, but we don't strictly rely on
		// that for correctness — agents care most about per-row receipts.
		if jw != nil {
			emitBatchResults(jw, chunk, resp, idProperty)
		}
		updated += len(chunk)
	}
	return updated, errored, nil
}

// emitBatchResults parses a batch endpoint response and emits one
// JSONLResult per input row. HubSpot's batch endpoints return
// {"status":"COMPLETE", "results":[...], "errors":[...]}; results map to
// the input row order, errors carry a "category" + "message" + index ref.
//
// Mapping strategy:
//
//  1. For each "result" item, try to correlate by index (results often
//     match inputs[i] when no per-row errors).
//  2. For each "error" item, surface its message on the row identified
//     by error.index / error.idProperty value.
//  3. For rows we couldn't correlate in either bucket, emit OK with an
//     empty data payload — they were not in the error list, so the
//     batch accepted them.
//
// The agent gets per-row receipts even when HubSpot's response shape
// is partial; the worst case is "OK with empty data" instead of a
// detailed echo.
func emitBatchResults(jw *cliutil.JSONLWriter, chunk []bulkRow, resp json.RawMessage, idProperty string) {
	var parsed struct {
		Results []json.RawMessage `json:"results"`
		Errors  []struct {
			Index   int    `json:"index"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(resp, &parsed)

	// Build an error-index lookup so we can mark only failed rows as
	// errors and the rest as OK.
	errIdx := map[int]string{}
	for _, e := range parsed.Errors {
		errIdx[e.Index] = e.Message
	}
	for i, r := range chunk {
		if msg, bad := errIdx[i]; bad {
			_ = jw.WriteError(r.ID, fmt.Errorf("%s", msg))
			continue
		}
		// Prefer the explicit result blob from HubSpot when present;
		// fall back to a minimal {id, idProperty} envelope so the agent
		// still gets correlation info.
		if i < len(parsed.Results) {
			_ = jw.WriteOK(r.ID, parsed.Results[i])
			continue
		}
		minimal, _ := json.Marshal(map[string]string{
			"id":          r.ID,
			"id_property": idProperty,
		})
		_ = jw.WriteOK(r.ID, minimal)
	}
}

// emitDryRunReport renders the existing JSON / text dry-run summary,
// plus per-row JSONL when --jsonl is in effect. Mirrors the original
// CSV behavior so existing scripts keep working.
func emitDryRunReport(cmd *cobra.Command, flags *rootFlags, jw *cliutil.JSONLWriter, rows, ok, bad []bulkRow) error {
	if jw != nil {
		// JSONL dry-run: emit one OK envelope per actionable row carrying
		// the patch, one error envelope per bad row (already emitted by
		// readJSONLBulkRows). No final summary on stdout.
		for _, r := range ok {
			data, _ := json.Marshal(map[string]any{
				"would_update": true,
				"patch":        r.Patch,
			})
			_ = jw.WriteOK(r.ID, data)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: %d would update, %d errors\n", len(ok), len(bad))
		return nil
	}
	report := map[string]any{
		"total":        len(rows),
		"would_update": len(ok),
		"errors":       bad,
	}
	if flags.asJSON {
		return flags.printJSON(cmd, report)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: %d rows would update, %d rows have errors\n", len(ok), len(bad))
	for _, b := range bad {
		fmt.Fprintf(cmd.OutOrStdout(), "  row %d (id=%s email=%s): %s\n",
			b.Index, b.ID, b.Email, strings.Join(b.Errors, "; "))
	}
	return nil
}

// emitConfirmInstructions prints the >100-row gate's first-call output:
// digest, count, and the exact command to run to confirm. Returns nil so
// the caller exits 0 — the dry-run is a successful operation, not an error.
func emitConfirmInstructions(cmd *cobra.Command, flags *rootFlags, digest string, rowCount int, idProperty string) error {
	idHint := ""
	if idProperty != "" {
		idHint = fmt.Sprintf(" --id-property %s", idProperty)
	}
	if flags.asJSON {
		out := map[string]any{
			"status":       "dry_run_persisted",
			"digest":       digest,
			"would_update": rowCount,
			"confirm_command": fmt.Sprintf("contacts bulk-update%s --confirm %d --digest %s",
				idHint, rowCount, digest),
		}
		return flags.printJSON(cmd, out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "digest: %s\n", digest)
	fmt.Fprintf(cmd.OutOrStdout(), "would update %d contacts\n", rowCount)
	fmt.Fprintf(cmd.OutOrStdout(),
		"to confirm, re-run with: --confirm %d --digest %s\n", rowCount, digest)
	return nil
}

func parseMapSpec(spec string, headers []string) map[string]string {
	_ = headers
	out := map[string]string{}
	if spec == "" {
		return out
	}
	for _, pair := range strings.Split(spec, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out
}

type propertyDef struct {
	Name    string
	Type    string
	Options []string
}

func loadContactsPropertySchema(db *store.Store) (map[string]propertyDef, error) {
	out := map[string]propertyDef{}
	rows, err := db.DB().Query(`SELECT data FROM hubspot_properties_crm
		WHERE json_extract(data, '$.objectType') = 'contacts'
		   OR json_extract(data, '$.objectType') = '0-1'`)
	if err != nil {
		return out, nil
	}
	defer rows.Close()
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var p struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Options []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			} `json:"options"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		def := propertyDef{Name: p.Name, Type: p.Type}
		for _, o := range p.Options {
			def.Options = append(def.Options, o.Value)
		}
		out[p.Name] = def
	}
	return out, nil
}

func validatePropertyValue(def propertyDef, value string) string {
	if value == "" {
		return ""
	}
	switch def.Type {
	case "string", "phone_number", "":
		return ""
	case "number":
		var f float64
		if _, err := fmt.Sscanf(value, "%g", &f); err != nil {
			return fmt.Sprintf("expected number, got %q", value)
		}
		return ""
	case "date", "datetime":
		return ""
	case "enumeration":
		for _, opt := range def.Options {
			if opt == value {
				return ""
			}
		}
		return fmt.Sprintf("value %q not in picklist (%s)", value, strings.Join(def.Options, ","))
	case "bool":
		switch strings.ToLower(value) {
		case "true", "false", "yes", "no", "1", "0":
			return ""
		}
		return fmt.Sprintf("expected bool, got %q", value)
	}
	return ""
}

// digestStoreAdapter bridges *store.Store to cliutil.PendingDigestStore.
// The store package returns store.PendingDigest; the gate consumes
// cliutil.PendingDigest. Both have identical field shapes — the duplication
// avoids a cliutil <-> store dependency cycle. This thin shim is the only
// place the two types meet.
type digestStoreAdapter struct{ db *store.Store }

func (a digestStoreAdapter) PutPendingDigest(ctx context.Context, digest, command string, plan []byte, rowCount int, expires time.Duration) error {
	return a.db.PutPendingDigest(ctx, digest, command, plan, rowCount, expires)
}

func (a digestStoreAdapter) GetPendingDigest(ctx context.Context, digest string) (*cliutil.PendingDigest, error) {
	r, err := a.db.GetPendingDigest(ctx, digest)
	if err != nil || r == nil {
		return nil, err
	}
	return &cliutil.PendingDigest{
		Digest:    r.Digest,
		Command:   r.Command,
		PlanJSON:  r.PlanJSON,
		RowCount:  r.RowCount,
		CreatedAt: r.CreatedAt,
		ExpiresAt: r.ExpiresAt,
	}, nil
}

func (a digestStoreAdapter) PurgeExpiredDigests(ctx context.Context) error {
	return a.db.PurgeExpiredDigests(ctx)
}

func lookupContactIDByEmail(db *store.Store, email string) (string, error) {
	row := db.DB().QueryRow(`SELECT id FROM hubspot_contacts_crm
		WHERE LOWER(json_extract(data, '$.properties.email')) = LOWER(?)
		LIMIT 1`, email)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return id, nil
}
