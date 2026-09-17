// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// pp:data-source auto
func newNovelImportsBlameCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	cmd := &cobra.Command{Use: "blame <jobId>", Short: "For one import job: who got the SMS, who was skipped and why", Long: "Use this command after an import finished to see per-row delivery outcomes. Do NOT use it to wait for an import to finish; use 'imports watch' instead.", Example: "  conduyt-crm-pp-cli imports blame 3b9e4c2d --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"}, RunE: func(cmd *cobra.Command, args []string) error {
		if limit <= 0 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--limit must be positive"))
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "imports blame")
		}
		if len(args) == 0 && !cmd.Flags().Changed("limit") && !cmd.Flags().Changed("db") {
			return cmd.Help()
		}
		if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("jobId is required"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		jobID := args[0]
		rows := make([]map[string]any, 0)
		failures := 0
		var fetchErr error
		failureReasons := make([]string, 0)
		perPage := minInt(200, limit+1)
		var total *int
		rowTabs := map[string]int(nil)
		rowsUnavailable := false
		rowsNote := ""
		progress := newPaginationProgressGuard()
		for page := 1; len(rows) <= limit; page++ {
			data, e := c.Get(ctx, "/imports/"+jobID+"/rows", map[string]string{"page": strconv.Itoa(page), "per_page": strconv.Itoa(perPage)})
			if e != nil {
				fetchErr = e
				failures++
				failureReasons = append(failureReasons, fmt.Sprintf("fetching import rows page %d: %v", page, e))
				break
			}
			batch, decodeErr := objectItems(data, false)
			if decodeErr != nil {
				failures++
				failureReasons = append(failureReasons, fmt.Sprintf("decoding import rows page %d: %v", page, decodeErr))
				break
			}
			for i, row := range batch {
				if decodeErr = validateContactIdentity(row, "import", i); decodeErr != nil {
					failures++
					failureReasons = append(failureReasons, fmt.Sprintf("decoding import rows page %d: %v", page, decodeErr))
					break
				}
			}
			if decodeErr != nil {
				break
			}
			if n, ok := responseTotal(data); ok {
				total = &n
			}
			if page == 1 {
				rowTabs = importRowTabs(data)
			}
			ids := make([]string, len(batch))
			for i := range batch {
				ids[i] = strAny(batch[i], "contactId", "contact_id", "id")
			}
			if guardErr := progress.observe(ids, "", len(batch) == perPage); guardErr != nil {
				failures++
				failureReasons = append(failureReasons, guardErr.Error())
				break
			}
			rows = append(rows, batch...)
			if len(batch) == 0 || len(batch) < perPage {
				break
			}
		}
		partial := failures > 0 || len(rows) > limit || total != nil && *total > limit
		if total != nil && *total > len(rows) && len(rows) <= limit {
			partial = true
			failureReasons = append(failureReasons, fmt.Sprintf("import rows ended after %d rows but response metadata reports total %d", len(rows), *total))
		}
		if len(rows) > limit || total != nil && *total > limit {
			failureReasons = append(failureReasons, "--limit capped the import rows checked")
		}
		if len(rows) > limit {
			rows = rows[:limit]
		}
		if total != nil && rowTabs != nil && rowTabs["all"] > *total {
			rowsUnavailable = true
			rowsNote = "row detail was not retained; delivery is read from the side-effect counters"
		}
		contacts := map[string]map[string]any{}
		if dbPath == "" {
			dbPath = defaultDBPath("conduyt-crm-pp-cli")
		}
		if _, e := os.Stat(dbPath); e == nil {
			db, e := store.OpenWithContext(ctx, dbPath)
			if e != nil {
				if fetchErr == nil {
					fetchErr = e
				}
				return fmt.Errorf("opening local mirror: %w", e)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "contacts")
			hintIfStale(cmd, db, "contacts", flags.maxAge)
			qr, e := db.DB().QueryContext(ctx, `SELECT id,data FROM resources WHERE resource_type='contacts'`)
			if e != nil {
				return e
			}
			for qr.Next() {
				var id string
				var raw []byte
				if e = qr.Scan(&id, &raw); e != nil {
					_ = qr.Close()
					return e
				}
				var m map[string]any
				if decodeErr := json.Unmarshal(raw, &m); decodeErr != nil {
					failures++
					failureReasons = append(failureReasons, fmt.Sprintf("decoding mirrored contact %s: %v", id, decodeErr))
					continue
				}
				contacts[id] = m
			}
			if e = qr.Err(); e != nil {
				failures++
				failureReasons = append(failureReasons, fmt.Sprintf("reading contacts mirror: %v", e))
			}
			if e = qr.Close(); e != nil {
				failures++
				failureReasons = append(failureReasons, fmt.Sprintf("closing contacts mirror rows: %v", e))
			}
		}
		for _, r := range rows {
			id := strAny(r, "contactId", "contact_id")
			if id == "" || contacts[id] != nil {
				continue
			}
			data, e := c.Get(ctx, "/contacts/"+id, nil)
			if e != nil {
				if fetchErr == nil {
					fetchErr = e
				}
				failures++
				failureReasons = append(failureReasons, fmt.Sprintf("fetching contact %s: %v", id, e))
				continue
			}
			items, decodeErr := objectItems(data, true) // GET /contacts/{id} legitimately returns one object.
			if decodeErr != nil {
				failures++
				failureReasons = append(failureReasons, "decoding contact "+id+": "+decodeErr.Error())
				continue
			}
			if len(items) > 0 {
				if strings.TrimSpace(strAny(items[0], "contactId", "contact_id", "id")) == "" {
					items[0]["id"] = id
				}
				if decodeErr = validateContactIdentity(items[0], "contact", 0); decodeErr != nil {
					failures++
					failureReasons = append(failureReasons, "decoding contact "+id+": "+decodeErr.Error())
					continue
				}
				contacts[id] = items[0]
			} else {
				failures++
				failureReasons = append(failureReasons, "decoding contact "+id+": response contained no contact")
			}
		}
		side := blameSideEffects{}
		sideRaw, e := c.Get(ctx, "/imports/"+jobID+"/side-effects", nil)
		if e != nil {
			if fetchErr == nil {
				fetchErr = e
			}
			failures++
			failureReasons = append(failureReasons, fmt.Sprintf("fetching import side effects: %v", e))
		} else {
			var env map[string]any
			if decodeErr := rejectResponseErrorEnvelope(sideRaw); decodeErr != nil {
				failures++
				failureReasons = append(failureReasons, "decoding import side effects: "+decodeErr.Error())
			} else if decodeErr := json.Unmarshal(sideRaw, &env); decodeErr == nil {
				m := unwrapMap(env)
				missing := make([]string, 0, 7)
				for name, spec := range map[string]struct {
					key    string
					legacy string
					target *int
				}{"queued": {"queued", "", &side.Queued}, "retryable": {"retryableFailed", "retryable", &side.Retryable}, "exhausted": {"exhaustedFailed", "exhausted", &side.Exhausted}, "total": {"total", "", &side.Total}, "completed": {"completed", "", &side.Completed}, "failed": {"failed", "", &side.Failed}, "held": {"held", "", &side.Held}} {
					value, counterErr := nonNegativeCounter(m, spec.key)
					if spec.legacy != "" {
						value, counterErr = preferredLegacyCounter(m, spec.key, spec.legacy)
					}
					if value == nil {
						if counterErr != nil {
							missing = append(missing, counterErr.Error())
						} else {
							missing = append(missing, name)
						}
						continue
					}
					*spec.target = *value
				}
				if len(missing) > 0 {
					sort.Strings(missing)
					failures++
					failureReasons = append(failureReasons, "import side effects missing or invalid counters: "+strings.Join(missing, ", "))
				}
				actionRequired, actionPresent, actionErr := preferredLegacyBool(m, "actionRequired", "action_required")
				if actionErr != nil {
					failures++
					failureReasons = append(failureReasons, "import side effects "+actionErr.Error())
				} else if actionPresent {
					side.ActionRequired = &actionRequired
				}
				isComplete, isCompleteErr := requiredBool(m, "isComplete")
				if isCompleteErr != nil {
					failures++
					failureReasons = append(failureReasons, "import side effects "+isCompleteErr.Error())
				} else {
					side.IsComplete = isComplete
					if !isComplete {
						failures++
						failureReasons = append(failureReasons, "import side effects are not complete")
					}
				}
			} else {
				failures++
				failureReasons = append(failureReasons, fmt.Sprintf("decoding import side effects: %v", decodeErr))
			}
		}
		partial = partial || failures > 0
		view := blameView{
			JobID:                     jobID,
			Rows:                      len(rows),
			Partial:                   partial,
			Checked:                   len(rows),
			Total:                     total,
			DeliveryCorrelation:       "none",
			DeliveryCorrelationReason: "GET /reports/sms-delivery returns aggregate totals only (no per-message import, message or contact keys), so delivery is read from the import rows",
			SkippedByReason:           map[string]int{},
			SideEffects:               side,
			RowTabs:                   rowTabs,
			RowsUnavailable:           rowsUnavailable,
			RowsNote:                  rowsNote,
			FetchFailures:             failures,
			Failures:                  failureReasons,
		}
		for i, r := range rows {
			id := strAny(r, "contactId", "contact_id")
			rowStatus, statusErr := preferredLegacyString(r, "status", "outcome")
			if statusErr != nil {
				failures++
				view.Partial = true
				view.FetchFailures = failures
				view.Failures = append(view.Failures, fmt.Sprintf("import row %d status: %v", i+1, statusErr))
				continue
			}
			status := strings.ToLower(rowStatus)
			reason := strAny(r, "reason", "error", "message")
			if status == "delivered" {
				view.Delivered++
				continue
			}
			if reason == "" {
				if contacts[id] == nil {
					reason = "contact_not_found"
				} else {
					reason = status
				}
			}
			if reason == "" {
				reason = "unknown"
			}
			view.SkippedByReason[reason]++
		}
		if failures > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed\n", failures, 1+len(rows))
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			if e := printJSONFiltered(cmd.OutOrStdout(), view, flags); e != nil {
				return e
			}
			if view.Partial {
				if fetchErr != nil {
					return classifyAPIErrorOnly(fetchErr)
				}
				return apiErr(fmt.Errorf("imports blame is incomplete: %s", strings.Join(view.Failures, "; ")))
			}
			return nil
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		if view.Partial {
			fmt.Fprintln(tw, "WARNING: result is partial; one or more rows were capped or fetches failed.")
		}
		actionRequired := "unknown"
		if side.ActionRequired != nil {
			actionRequired = strconv.FormatBool(*side.ActionRequired)
		}
		fmt.Fprintf(tw, "JOB\tROWS\tDELIVERED\tQUEUED\tRETRYABLE\tEXHAUSTED\tACTION REQUIRED\n%s\t%d\t%d\t%d\t%d\t%d\t%s\n", view.JobID, view.Rows, view.Delivered, side.Queued, side.Retryable, side.Exhausted, actionRequired)
		for reason, n := range view.SkippedByReason {
			fmt.Fprintf(tw, "SKIPPED\t%s\t%d\n", reason, n)
		}
		if e := tw.Flush(); e != nil {
			return e
		}
		if view.Partial {
			if fetchErr != nil {
				return classifyAPIErrorOnly(fetchErr)
			}
			return apiErr(fmt.Errorf("imports blame is incomplete: %s", strings.Join(view.Failures, "; ")))
		}
		return nil
	}}
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum import rows to inspect")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path")
	return cmd
}

type blameSideEffects struct {
	Queued         int   `json:"queued"`
	Retryable      int   `json:"retryable"`
	Exhausted      int   `json:"exhausted"`
	Total          int   `json:"total"`
	Completed      int   `json:"completed"`
	Failed         int   `json:"failed"`
	Held           int   `json:"held"`
	ActionRequired *bool `json:"action_required"`
	IsComplete     bool  `json:"is_complete"`
}
type blameView struct {
	JobID                     string           `json:"job_id"`
	Rows                      int              `json:"rows"`
	Partial                   bool             `json:"partial"`
	Checked                   int              `json:"checked"`
	Total                     *int             `json:"total,omitempty"`
	Delivered                 int              `json:"delivered"`
	DeliveryCorrelation       string           `json:"delivery_correlation"`
	DeliveryCorrelationReason string           `json:"delivery_correlation_reason"`
	SkippedByReason           map[string]int   `json:"skipped_by_reason"`
	SideEffects               blameSideEffects `json:"side_effects"`
	RowTabs                   map[string]int   `json:"row_tabs,omitempty"`
	RowsUnavailable           bool             `json:"rows_unavailable"`
	RowsNote                  string           `json:"rows_note,omitempty"`
	FetchFailures             int              `json:"fetch_failures"`
	Failures                  []string         `json:"failures,omitempty"`
}

func importRowTabs(raw json.RawMessage) map[string]int {
	var envelope struct {
		Data struct {
			Tabs map[string]int `json:"tabs"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil
	}
	return envelope.Data.Tabs
}

func responseTotal(raw json.RawMessage) (int, bool) {
	var doc any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if dec.Decode(&doc) != nil {
		return 0, false
	}
	var find func(any) (int, bool)
	find = func(value any) (int, bool) {
		m, ok := value.(map[string]any)
		if !ok {
			return 0, false
		}
		for _, container := range []string{"meta", "pagination"} {
			if nested, ok := m[container].(map[string]any); ok {
				if n, ok := numericInt(nested["total"]); ok {
					return n, true
				}
			}
		}
		if n, ok := numericInt(m["total"]); ok {
			return n, true
		}
		if data, ok := m["data"]; ok {
			return find(data)
		}
		return 0, false
	}
	return find(doc)
}

func numericInt(value any) (int, bool) {
	switch v := value.(type) {
	case json.Number:
		n, err := strconv.Atoi(v.String())
		return n, err == nil
	case float64:
		if v < float64(-int(^uint(0)>>1)-1) || v > float64(int(^uint(0)>>1)) || v != float64(int(v)) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}

func unwrapMap(m map[string]any) map[string]any {
	for {
		d, ok := m["data"].(map[string]any)
		if !ok {
			return m
		}
		m = d
	}
}
func intAny(m map[string]any, keys ...string) int {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case json.Number:
			n, err := strconv.Atoi(v.String())
			if err == nil {
				return n
			}
			// Non-integral JSON numbers are not valid integer counters; try the next alias.
		}
	}
	return 0
}
func boolAny(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if b, ok := m[k].(bool); ok {
			return b
		}
	}
	return false
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
