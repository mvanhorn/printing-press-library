// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/spf13/cobra"
)

type planHistoryOptions struct {
	planEditOptions
	limit      int
	recordID   string
	includeOps bool
}

type planEditJournalRecord struct {
	ID         string           `json:"id"`
	TargetKey  string           `json:"target_key"`
	Command    string           `json:"command"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at,omitempty"`
	Version    int              `json:"version"`
	Status     string           `json:"status"`
	ForwardOps []map[string]any `json:"forward_ops"`
	InverseOps []map[string]any `json:"inverse_ops"`
	OpPaths    []string         `json:"op_paths,omitempty"`
	Report     planEditReport   `json:"report,omitempty"`
}

func newNovelPlanHistoryCmd(flags *rootFlags) *cobra.Command {
	opts := planHistoryOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2}, limit: 20}
	cmd := &cobra.Command{Use: "history", Short: "List local undo/redo journal entries for a Wanderlog plan", Example: "  wanderlog-pp-cli plan history --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts.planEditOptions)
		if err != nil {
			return usageErr(err)
		}
		recs, path, err := readPlanEditJournal(c)
		if err != nil {
			return err
		}
		filtered := filterJournalByPlan(recs, key)
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
		if opts.limit > 0 && len(filtered) > opts.limit {
			filtered = filtered[:opts.limit]
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan history", "target_key": key, "journal_path": path, "entries": summarizeJournalRecords(filtered, opts.includeOps)}, flags)
	}}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	cmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum journal entries to show; 0 shows all")
	cmd.Flags().BoolVar(&opts.includeOps, "include-ops", false, "Include forward_ops and inverse_ops on each journal entry")
	return cmd
}

func newNovelPlanUndoCmd(flags *rootFlags) *cobra.Command {
	opts := planHistoryOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2, applyRetries: 2}}
	cmd := &cobra.Command{Use: "undo", Short: "Undo the latest applied ShareDB edit from the local journal", Example: "  wanderlog-pp-cli plan undo --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		return runPlanHistoryMutation(cmd, flags, opts, "undo")
	}}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	cmd.Flags().StringVar(&opts.recordID, "record-id", "", "Specific journal record id to undo; default is latest applied entry")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the undo through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanRedoCmd(flags *rootFlags) *cobra.Command {
	opts := planHistoryOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2, applyRetries: 2}}
	cmd := &cobra.Command{Use: "redo", Short: "Redo the latest undone ShareDB edit from the local journal", Example: "  wanderlog-pp-cli plan redo --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		return runPlanHistoryMutation(cmd, flags, opts, "redo")
	}}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	cmd.Flags().StringVar(&opts.recordID, "record-id", "", "Specific journal record id to redo; default is latest undone entry")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the redo through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func runPlanHistoryMutation(cmd *cobra.Command, flags *rootFlags, opts planHistoryOptions, action string) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	key, err := resolveEditablePlanKey(opts.planEditOptions)
	if err != nil {
		return usageErr(err)
	}
	recs, path, err := readPlanEditJournal(c)
	if err != nil {
		return err
	}
	idx, rec, err := pickJournalRecord(recs, key, opts.recordID, action)
	if err != nil {
		if !opts.apply || flags.dryRun {
			out := map[string]any{
				"command":         "plan " + action,
				"target_key":      key,
				"journal_path":    path,
				"available":       false,
				"apply_requested": opts.apply,
				"dry_run":         true,
				"applied":         false,
				"message":         err.Error(),
			}
			if flags.dryRun {
				out["warnings"] = []string{"global --dry-run set: no edit will be applied"}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		}
		return usageErr(err)
	}
	ops := rec.InverseOps
	newStatus := "undone"
	commandName := "plan undo"
	operation := "undo journal entry"
	if action == "redo" {
		ops = rec.ForwardOps
		newStatus = "applied"
		commandName = "plan redo"
		operation = "redo journal entry"
	}
	if len(ops) == 0 {
		return usageErr(fmt.Errorf("journal record %s has no %s operations", rec.ID, action))
	}
	report := planEditReport{Command: commandName, TargetKey: key, ApplyRequested: opts.apply, DryRun: !opts.apply || flags.dryRun, Operation: operation, OpPaths: opPaths(ops)}
	out := map[string]any{"command": commandName, "target_key": key, "journal_path": path, "record": summarizeJournalRecord(rec, false), "ops": ops, "op_paths": opPaths(ops), "apply_requested": opts.apply, "dry_run": report.DryRun, "applied": false}
	if !opts.apply || flags.dryRun {
		if flags.dryRun {
			out["warnings"] = []string{"global --dry-run set: no edit will be applied"}
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}
	if err := requireCookie(c); err != nil {
		return authErr(err)
	}
	result, version, err := applyPlanEditViaShareDBWithRetry(ctx, c, key, opts.clientSchemaVersion, opts.applyRetries, func(target map[string]any) (planEditBuildResult, error) {
		return planEditBuildResult{Ops: cloneOps(ops), Report: report}, nil
	})
	if err != nil {
		return apiErr(err)
	}
	now := time.Now().UTC()
	recs[idx].Status = newStatus
	recs[idx].UpdatedAt = now
	if err := writePlanEditJournal(c, recs); err != nil {
		result.Report.Warnings = append(result.Report.Warnings, fmt.Sprintf("edit applied but failed to update local undo journal: %v", err))
	}
	result.Report.TargetKey = key
	result.Report.Version = version
	result.Report.ApplyRequested = true
	result.Report.Applied = true
	result.Report.DryRun = false
	return printPlanEditReport(cmd, flags, result.Report)
}

func recordPlanEditJournal(c *client.Client, targetKey string, commandName string, version int, result planEditBuildResult) error {
	inverse, err := invertJSON0Ops(result.Ops)
	if err != nil {
		return err
	}
	recs, _, err := readPlanEditJournal(c)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rec := planEditJournalRecord{
		ID:         fmt.Sprintf("%d-%d", now.UnixNano(), randomWanderlogID()),
		TargetKey:  targetKey,
		Command:    commandName,
		CreatedAt:  now,
		Version:    version,
		Status:     "applied",
		ForwardOps: cloneOps(result.Ops),
		InverseOps: inverse,
		OpPaths:    opPaths(result.Ops),
		Report:     result.Report,
	}
	recs = append(recs, rec)
	return writePlanEditJournal(c, recs)
}

func planEditJournalPath(c *client.Client) (string, error) {
	if c != nil && c.Config != nil && strings.TrimSpace(c.Config.Path) != "" {
		return filepath.Join(filepath.Dir(c.Config.Path), "edit-journal.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "wanderlog-pp-cli", "edit-journal.json"), nil
}

func readPlanEditJournal(c *client.Client) ([]planEditJournalRecord, string, error) {
	path, err := planEditJournalPath(c)
	if err != nil {
		return nil, "", err
	}
	// #nosec G304 -- path is not user input: planEditJournalPath derives it
	// from the CLI's own config directory (or $HOME/.config/wanderlog-pp-cli)
	// and appends a fixed basename.
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, path, nil
	}
	var recs []planEditJournalRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, path, fmt.Errorf("parse edit journal %s: %w", path, err)
	}
	return recs, path, nil
}

func writePlanEditJournal(c *client.Client, recs []planEditJournalRecord) error {
	path, err := planEditJournalPath(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func filterJournalByPlan(recs []planEditJournalRecord, targetKey string) []planEditJournalRecord {
	out := []planEditJournalRecord{}
	for _, rec := range recs {
		if rec.TargetKey == targetKey {
			out = append(out, rec)
		}
	}
	return out
}

func pickJournalRecord(recs []planEditJournalRecord, targetKey string, recordID string, action string) (int, planEditJournalRecord, error) {
	wantStatus := "applied"
	if action == "redo" {
		wantStatus = "undone"
	}
	for i := len(recs) - 1; i >= 0; i-- {
		rec := recs[i]
		if rec.TargetKey != targetKey {
			continue
		}
		if recordID != "" && rec.ID != recordID {
			continue
		}
		if rec.Status != wantStatus {
			if recordID != "" {
				return -1, planEditJournalRecord{}, fmt.Errorf("record %s is %s, not %s", recordID, rec.Status, wantStatus)
			}
			continue
		}
		return i, rec, nil
	}
	if recordID != "" {
		return -1, planEditJournalRecord{}, fmt.Errorf("journal record %s not found for target plan", recordID)
	}
	return -1, planEditJournalRecord{}, fmt.Errorf("no %s journal entry available for target plan", wantStatus)
}

func summarizeJournalRecords(recs []planEditJournalRecord, includeOps bool) []map[string]any {
	out := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		out = append(out, summarizeJournalRecord(rec, includeOps))
	}
	return out
}

func summarizeJournalRecord(rec planEditJournalRecord, includeOps bool) map[string]any {
	out := map[string]any{
		"id":         rec.ID,
		"target_key": rec.TargetKey,
		"command":    rec.Command,
		"created_at": rec.CreatedAt,
		"updated_at": rec.UpdatedAt,
		"version":    rec.Version,
		"status":     rec.Status,
		"op_paths":   rec.OpPaths,
	}
	// PATCH(amend-2026-08-23: omit forward/inverse ops unless --include-ops)
	if includeOps {
		out["forward_ops"] = rec.ForwardOps
		out["inverse_ops"] = rec.InverseOps
	}
	return out
}

func invertJSON0Ops(ops []map[string]any) ([]map[string]any, error) {
	inverse := make([]map[string]any, 0, len(ops))
	for i := len(ops) - 1; i >= 0; i-- {
		op, err := invertJSON0Op(ops[i])
		if err != nil {
			return nil, fmt.Errorf("invert op %d: %w", i, err)
		}
		inverse = append(inverse, op)
	}
	return inverse, nil
}

func invertJSON0Op(op map[string]any) (map[string]any, error) {
	path, ok := op["p"].([]any)
	if !ok || len(path) == 0 {
		return nil, errors.New("operation missing path p")
	}
	inv := map[string]any{"p": cloneAnySlice(path)}
	li, hasLI := op["li"]
	ld, hasLD := op["ld"]
	oi, hasOI := op["oi"]
	od, hasOD := op["od"]
	if hasLI || hasLD {
		if hasLI {
			inv["ld"] = cloneJSONValue(li)
		}
		if hasLD {
			inv["li"] = cloneJSONValue(ld)
		}
		return inv, nil
	}
	if hasOI || hasOD {
		if hasOI {
			inv["od"] = cloneJSONValue(oi)
		}
		if hasOD {
			inv["oi"] = cloneJSONValue(od)
		}
		return inv, nil
	}
	return nil, fmt.Errorf("unsupported or non-invertible JSON0 op keys %v", sortedMapKeys(op))
}

func cloneOps(ops []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(ops))
	for _, op := range ops {
		cloned, _ := cloneJSONValue(op).(map[string]any)
		out = append(out, cloned)
	}
	return out
}

func cloneAnySlice(in []any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = cloneJSONValue(v)
	}
	return out
}

func cloneJSONValue(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var _ = context.Background
