// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel commands: batch load (journaled $batch submission from JSONL) and
// batch resume (re-submit only the failed ops of a prior journaled batch).
// Priority's $batch has NO rollback, so the journal is the recovery story.

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source live

// batchOp is one operation in a Priority JSON $batch request.
type batchOp struct {
	ID             string            `json:"id"`
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           json.RawMessage   `json:"body,omitempty"`
	DependsOn      []string          `json:"dependsOn,omitempty"`
	AtomicityGroup string            `json:"atomicityGroup,omitempty"`
}

type batchOpResult struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body,omitempty"`
}

type batchRunView struct {
	JournalID int64    `json:"journal_id"`
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Note      string   `json:"note,omitempty"`
	FailedIDs []string `json:"failed_ids,omitempty"`
}

// stdinIsTerminal reports whether stdin is an interactive terminal (vs a pipe).
func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func defaultBatchOpHeaders() map[string]string {
	return map[string]string{
		"content-type":  "application/json; odata.metadata=minimal; odata.streaming=true",
		"odata-version": "4.0",
	}
}

// assignBatchOpDefaults fills headers and collision-free op IDs in place.
// IDs must be assigned BEFORE journaling so pp_batch_ops.op_id matches what
// the server sees and what failed_ids reports.
func assignBatchOpDefaults(ops []batchOp) {
	used := map[string]bool{}
	for i := range ops {
		if ops[i].ID != "" {
			used[ops[i].ID] = true
		}
	}
	next := 1
	for i := range ops {
		if ops[i].Headers == nil {
			ops[i].Headers = defaultBatchOpHeaders()
		}
		if ops[i].ID == "" {
			for used[strconv.Itoa(next)] {
				next++
			}
			ops[i].ID = strconv.Itoa(next)
			used[ops[i].ID] = true
		}
	}
}

// executeBatchChunk posts one $batch payload and returns per-op results.
func executeBatchChunk(ctx context.Context, c *client.Client, ops []batchOp) ([]batchOpResult, error) {
	payload := map[string]any{"requests": ops}
	data, _, err := c.Post(ctx, "/$batch", payload)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Responses []batchOpResult `json:"responses"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing $batch response: %w", err)
	}
	return envelope.Responses, nil
}

// journalBatch writes the journal header and ops, returning the journal id.
func journalBatch(ctx context.Context, db *store.Store, tenant, source string, ops []batchOp) (int64, error) {
	res, err := db.DB().ExecContext(ctx,
		`INSERT INTO pp_batch_journal (tenant, source, total) VALUES (?, ?, ?)`, tenant, source, len(ops))
	if err != nil {
		return 0, fmt.Errorf("creating batch journal: %w", err)
	}
	jid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for i, op := range ops {
		hdrs, _ := json.Marshal(op.Headers)
		deps := strings.Join(op.DependsOn, ",")
		if _, err := db.DB().ExecContext(ctx,
			`INSERT INTO pp_batch_ops (journal_id, op_index, op_id, method, url, body, headers, depends_on, atomicity_group) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			jid, i, op.ID, op.Method, op.URL, string(op.Body), string(hdrs), deps, op.AtomicityGroup); err != nil {
			return 0, fmt.Errorf("journaling op %d: %w", i, err)
		}
	}
	return jid, nil
}

// recordBatchResults updates per-op status/response and journal counters.
func recordBatchResults(ctx context.Context, db *store.Store, jid int64, ops []batchOp, results []batchOpResult) (succeeded, failed int, failedIDs []string, err error) {
	byID := map[string]batchOpResult{}
	for _, r := range results {
		byID[r.ID] = r
	}
	for i, op := range ops {
		r, ok := byID[op.ID]
		if !ok {
			failed++
			failedIDs = append(failedIDs, op.ID)
			if _, err := db.DB().ExecContext(ctx,
				`UPDATE pp_batch_ops SET status = NULL, error = 'no response received for this op id' WHERE journal_id = ? AND op_index = ?`, jid, i); err != nil {
				return 0, 0, nil, err
			}
			continue
		}
		ok2 := r.Status >= 200 && r.Status < 300
		if ok2 {
			succeeded++
		} else {
			failed++
			failedIDs = append(failedIDs, op.ID)
		}
		if _, err := db.DB().ExecContext(ctx,
			`UPDATE pp_batch_ops SET status = ?, response = ? WHERE journal_id = ? AND op_index = ?`,
			r.Status, string(r.Body), jid, i); err != nil {
			return 0, 0, nil, err
		}
	}
	if _, err := db.DB().ExecContext(ctx,
		`UPDATE pp_batch_journal SET succeeded = ?, failed = ? WHERE id = ?`, succeeded, failed, jid); err != nil {
		return 0, 0, nil, err
	}
	return succeeded, failed, failedIDs, nil
}

func runJournaledBatch(cmd *cobra.Command, flags *rootFlags, ops []batchOp, source string) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("priority-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	tenant := tenantKeyFromClient(c)

	const chunkSize = 100 // documented Priority $batch cap
	assignBatchOpDefaults(ops)
	view := batchRunView{Total: len(ops)}
	jid, err := journalBatch(ctx, db, tenant, source, ops)
	if err != nil {
		return err
	}
	view.JournalID = jid
	for start := 0; start < len(ops); start += chunkSize {
		end := start + chunkSize
		if end > len(ops) {
			end = len(ops)
		}
		chunk := ops[start:end]
		results, err := executeBatchChunk(ctx, c, chunk)
		if err != nil {
			// Transport-level failure: mark the whole chunk failed with the error.
			for i := start; i < end; i++ {
				if _, uerr := db.DB().ExecContext(ctx,
					`UPDATE pp_batch_ops SET error = ? WHERE journal_id = ? AND op_index = ?`, "ambiguous-transport: "+truncate(err.Error(), 280), jid, i); uerr != nil {
					return uerr
				}
				view.Failed++
				view.FailedIDs = append(view.FailedIDs, chunk[i-start].ID)
			}
			if _, uerr := db.DB().ExecContext(ctx,
				`UPDATE pp_batch_journal SET succeeded = ?, failed = ? WHERE id = ?`, view.Succeeded, view.Failed, jid); uerr != nil {
				return uerr
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: chunk %d-%d failed at transport level: %v\n", start, end-1, err)
			continue
		}
		s, f, ids, err := recordBatchResults(ctx, db, jid, chunk, results)
		if err != nil {
			return err
		}
		view.Succeeded += s
		view.Failed += f
		view.FailedIDs = append(view.FailedIDs, ids...)
	}
	if view.Failed > 0 {
		view.Note = fmt.Sprintf("%d of %d ops failed; re-run only the failures with: priority-pp-cli batch resume %d", view.Failed, view.Total, jid)
		fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
	}
	if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
		return err
	}
	if view.Failed > 0 {
		return partialFailureErr(fmt.Errorf("%d of %d batch ops failed (journal %d)", view.Failed, view.Total, jid))
	}
	return nil
}

// parseBatchOpsJSONL reads one batch op per line (JSONL) or a single JSON array.
func parseBatchOpsJSONL(r io.Reader) ([]batchOp, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty batch input")
	}
	if strings.HasPrefix(trimmed, "[") {
		var ops []batchOp
		if err := json.Unmarshal([]byte(trimmed), &ops); err != nil {
			return nil, fmt.Errorf("parsing JSON array of ops: %w", err)
		}
		return ops, nil
	}
	var ops []batchOp
	sc := bufio.NewScanner(strings.NewReader(trimmed))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		var op batchOp
		if err := json.Unmarshal([]byte(text), &op); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		ops = append(ops, op)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no batch ops found in input")
	}
	return ops, nil
}

func newNovelBatchLoadCmd(flags *rootFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Submit a journaled, auto-chunked $batch load from a JSONL file or stdin",
		Long: strings.Trim(`
Reads batch operations (one JSON object per line, or one JSON array) from
--file or stdin, splits them into chunks of 100 (Priority's documented cap),
executes each chunk through the throttled client, and journals every op —
including request bodies and responses — to the local SQLite store (plain
text; delete rows from pp_batch_journal/pp_batch_ops to purge). Priority batches have no rollback — after a partial
failure, 'batch resume <journal-id>' re-submits only the failed ops.

Op shape: {"method":"POST","url":"FAMILY_LOG","id":"1","body":{...},"dependsOn":[],"atomicityGroup":""}`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli batch load --file ops.jsonl --dry-run
  cat ops.jsonl | priority-pp-cli batch load
  priority-pp-cli batch load --file ops.jsonl --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && stdinIsTerminal() {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				// File reads stay behind the dry-run gate: preview op counts
				// when the input is readable, else a generic preview.
				var reader io.Reader = cmd.InOrStdin()
				if file != "" {
					f, err := os.Open(file) // #nosec G304 -- path is the user's own --file flag; loading a user-named ops file is this command's purpose
					if err != nil {
						fmt.Fprintln(cmd.OutOrStdout(), "would submit the batch ops from --file in chunks of 100 with per-op journaling")
						return nil
					}
					defer f.Close()
					reader = f
				} else if stdinIsTerminal() {
					fmt.Fprintln(cmd.OutOrStdout(), "would submit batch ops from stdin in chunks of 100 with per-op journaling")
					return nil
				}
				ops, err := parseBatchOpsJSONL(reader)
				if err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), "would submit the batch ops in chunks of 100 with per-op journaling")
					return nil
				}
				chunks := (len(ops) + 99) / 100
				fmt.Fprintf(cmd.OutOrStdout(), "would submit %d ops in %d $batch request(s) with per-op journaling\n", len(ops), chunks)
				return nil
			}
			var reader io.Reader
			source := "stdin"
			if file != "" {
				f, err := os.Open(file) // #nosec G304 -- path is the user's own --file flag; loading a user-named ops file is this command's purpose
				if err != nil {
					return usageErr(fmt.Errorf("opening --file: %w", err))
				}
				defer f.Close()
				reader = f
				source = file
			} else {
				reader = cmd.InOrStdin()
			}
			ops, err := parseBatchOpsJSONL(reader)
			if err != nil {
				return usageErr(err)
			}
			return runJournaledBatch(cmd, flags, ops, source)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSONL file of batch ops (default: stdin)")
	return cmd
}

func newNovelBatchResumeCmd(flags *rootFlags) *cobra.Command {
	var includeAmbiguous bool
	cmd := &cobra.Command{
		Use:   "resume <journal-id>",
		Short: "Re-run only the failed operations from a partially-failed $batch — Priority batches have no rollback",
		Long: strings.Trim(`
Use this command to re-run failed operations from a journaled batch.
Do NOT use it to submit a new batch; use 'batch load' (journaled) or 'batch' (raw --requests) instead.

Reads the journal written by 'batch load', selects ops whose status is missing
or non-2xx, and submits them as a fresh journaled batch.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli batch resume 42
  priority-pp-cli batch resume 42 --dry-run`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("journal id is required"))
			}
			jid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("journal id must be a number: %w", err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return notFoundErr(fmt.Errorf("no local store at %s; nothing to resume (run 'batch load' first)", dbPath))
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}

			var journalTenant string
			if err := db.DB().QueryRowContext(ctx, `SELECT tenant FROM pp_batch_journal WHERE id = ?`, jid).Scan(&journalTenant); err != nil {
				_ = db.Close()
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("journal %d not found; run 'batch load' first", jid))
				}
				return err
			}
			cGuard, err := flags.newClient()
			if err != nil {
				_ = db.Close()
				return err
			}
			if cur := tenantKeyFromClient(cGuard); cur != journalTenant {
				_ = db.Close()
				return usageErr(fmt.Errorf("journal %d was recorded against tenant %s but the current base URL is %s; refusing to replay writes into a different Priority instance", jid, journalTenant, cur))
			}
			rows, err := db.DB().QueryContext(ctx,
				`SELECT op_id, method, url, COALESCE(body,''), COALESCE(headers,''), COALESCE(depends_on,''), COALESCE(atomicity_group,''), COALESCE(error,'')
				 FROM pp_batch_ops WHERE journal_id = ? AND (status IS NULL OR status < 200 OR status >= 300) ORDER BY op_index`, jid)
			if err != nil {
				_ = db.Close()
				return err
			}
			var ops []batchOp
			ambiguousSkipped := 0
			for rows.Next() {
				var op batchOp
				var body, headers, deps, group, opErr string
				if err := rows.Scan(&op.ID, &op.Method, &op.URL, &body, &headers, &deps, &group, &opErr); err != nil {
					_ = rows.Close()
					_ = db.Close()
					return err
				}
				if body != "" {
					op.Body = json.RawMessage(body)
				}
				if headers != "" {
					_ = json.Unmarshal([]byte(headers), &op.Headers)
				}
				if deps != "" {
					op.DependsOn = strings.Split(deps, ",")
				}
				op.AtomicityGroup = group
				// Ambiguous transport failures: Priority may have committed the
				// whole chunk even though the response was lost. Replaying
				// non-idempotent writes would double-execute them, so these
				// ops are excluded unless --include-ambiguous is passed.
				if strings.HasPrefix(opErr, "ambiguous-transport:") && !includeAmbiguous {
					ambiguousSkipped++
					continue
				}
				ops = append(ops, op)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				_ = db.Close()
				return err
			}
			_ = rows.Close()
			_ = db.Close() // runJournaledBatch reopens; avoid two open handles

			if ambiguousSkipped > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d op(s) failed at the transport level and were skipped — the server may have already committed them. Verify tenant state (e.g. 'entity get'), then re-run with --include-ambiguous to replay them.\n", ambiguousSkipped)
			}
			if len(ops) == 0 {
				note := fmt.Sprintf("journal %d has no failed ops to resume", jid)
				if ambiguousSkipped > 0 {
					note = fmt.Sprintf("journal %d has only %d ambiguous transport-failed op(s); verify tenant state and re-run with --include-ambiguous to replay them", jid, ambiguousSkipped)
				}
				view := batchRunView{JournalID: jid, Note: note}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			// Strip dependsOn references to ops that succeeded in the original
			// run (absent from this resume set) — the server would reject them.
			inSet := map[string]bool{}
			for _, op := range ops {
				inSet[op.ID] = true
			}
			for i := range ops {
				var kept []string
				for _, dep := range ops[i].DependsOn {
					if inSet[dep] {
						kept = append(kept, dep)
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: op %s depended on %s which succeeded in the original run; dependency dropped for resume\n", ops[i].ID, dep)
					}
				}
				ops[i].DependsOn = kept
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would re-submit %d failed op(s) from journal %d\n", len(ops), jid)
				return nil
			}
			return runJournaledBatch(cmd, flags, ops, fmt.Sprintf("resume:%d", jid))
		},
	}
	cmd.Flags().BoolVar(&includeAmbiguous, "include-ambiguous", false, "also re-submit ops whose original chunk failed at the transport level (the server may have already committed them — verify before using)")
	return cmd
}
