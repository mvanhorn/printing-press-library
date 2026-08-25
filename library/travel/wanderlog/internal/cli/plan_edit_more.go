// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelPlanSectionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section",
		Short: "Add, edit, or delete Wanderlog itinerary sections",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlanSectionAddDayCmd(flags))
	cmd.AddCommand(newNovelPlanSectionSetFieldCmd(flags))
	cmd.AddCommand(newNovelPlanSectionDeleteCmd(flags))
	// PATCH(amend-2026-08-23: register plan section swap-days)
	cmd.AddCommand(newNovelPlanSectionSwapDaysCmd(flags))
	return cmd
}

func newNovelPlanSectionAddDayCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, position: -1}
	var date string
	var heading string
	cmd := &cobra.Command{
		Use:     "add-day",
		Short:   "Insert a new day section into an editable Wanderlog plan",
		Example: "  wanderlog-pp-cli plan section add-day --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --date 2026-09-07 --position 11 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(date) == "" {
				return usageErr(errors.New("--date is required"))
			}
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return usageErr(fmt.Errorf("--date must be YYYY-MM-DD: %w", err))
			}
			return runPlanEdit(cmd, flags, opts, "plan section add-day", func(target map[string]any) (planEditBuildResult, error) {
				secs := sections(target)
				idx := normalizeInsertPosition(opts.position, len(secs))
				section := newDaySection(date, heading)
				ops := []map[string]any{{"p": []any{"itinerary", "sections", idx}, "li": section}}
				dayCount := countDaySections(target)
				if intAny(target["days"]) != dayCount+1 {
					ops = append(ops, objectSetOp([]any{"days"}, target["days"], target["days"] != nil, dayCount+1, false))
				}
				if start := stringField(target, "startDate"); start == "" || date < start {
					ops = append(ops, objectSetOp([]any{"startDate"}, target["startDate"], start != "", date, false))
				}
				if end := stringField(target, "endDate"); end == "" || end < date {
					ops = append(ops, objectSetOp([]any{"endDate"}, target["endDate"], end != "", date, false))
				}
				report := baseEditReport("plan section add-day", opts, target)
				report.Section = ptrSectionReport(sectionReportFor(idx, dayCount+1, section))
				report.Operation = "insert day section"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&date, "date", "", "ISO date for the new day section")
	cmd.Flags().StringVar(&heading, "heading", "", "Optional section heading")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based raw section insertion position; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanSectionSetFieldCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var field string
	var value string
	var jsonValue string
	var remove bool
	cmd := &cobra.Command{
		Use:     "set-field",
		Short:   "Set (--field/--value or --json-value) or remove (--remove) a section field such as heading or rich text; previews unless --apply",
		Example: "  wanderlog-pp-cli plan section set-field --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --field heading --value 'Arrival day' --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, newValue, err := parseFieldMutation(field, value, jsonValue, remove)
			if err != nil {
				return usageErr(err)
			}
			if path[0] == "blocks" || path[0] == "id" {
				return usageErr(fmt.Errorf("section field %q is protected; use block commands or raw op for advanced edits", path[0]))
			}
			return runPlanEdit(cmd, flags, opts, "plan section set-field", func(target map[string]any) (planEditBuildResult, error) {
				sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
				if err != nil {
					return planEditBuildResult{}, err
				}
				old, exists := getMapPath(sec.Raw, path)
				ops := []map[string]any{objectSetOp(append([]any{"itinerary", "sections", sec.Index}, pathToAny(path)...), old, exists, newValue, remove)}
				report := baseEditReport("plan section set-field", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Operation = "set section field"
				if remove {
					report.Operation = "remove section field"
				}
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&field, "field", "", "Section field path, e.g. heading, date, text")
	cmd.Flags().StringVar(&value, "value", "", "String value to set")
	cmd.Flags().StringVar(&jsonValue, "json-value", "", "JSON value to set")
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove the field instead of setting it")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanSectionDeleteCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var force bool
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an empty Wanderlog section",
		Example: "  wanderlog-pp-cli plan section delete --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 8 --force --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanEdit(cmd, flags, opts, "plan section delete", func(target map[string]any) (planEditBuildResult, error) {
				sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
				if err != nil {
					return planEditBuildResult{}, err
				}
				if len(sec.Blocks) > 0 && !force {
					return planEditBuildResult{}, errors.New("section contains blocks; rerun with --force after reviewing dry-run output")
				}
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index}, "ld": sec.Raw}}
				if sec.Report.Day > 0 {
					dayCount := countDaySections(target)
					if intAny(target["days"]) != dayCount-1 {
						ops = append(ops, objectSetOp([]any{"days"}, target["days"], target["days"] != nil, dayCount-1, false))
					}
					dates := dayDatesExcludingSection(target, sec.Index)
					if len(dates) > 0 {
						if start := dates[0]; stringField(target, "startDate") != start {
							ops = append(ops, objectSetOp([]any{"startDate"}, target["startDate"], target["startDate"] != nil, start, false))
						}
						if end := dates[len(dates)-1]; stringField(target, "endDate") != end {
							ops = append(ops, objectSetOp([]any{"endDate"}, target["endDate"], target["endDate"] != nil, end, false))
						}
					}
				}
				report := baseEditReport("plan section delete", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Operation = "delete section"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().BoolVar(&force, "force", false, "Allow deleting a non-empty section")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockEditTextCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	var markdown bool
	cmd := &cobra.Command{
		Use:   "edit-text",
		Short: "Replace block note as plain text unless --markdown",
		Example: "  wanderlog-pp-cli plan block edit-text --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --text 'Updated note' --dry-run --agent\n" +
			"  wanderlog-pp-cli plan block edit-text --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --markdown --text $'# Stop\\n- item' --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("text") {
				return usageErr(errors.New("--text is required; pass an empty string deliberately to clear text"))
			}
			newText, stripped, err := blockNoteText(opts.text, markdown)
			if err != nil {
				return usageErr(err)
			}
			return runPlanEdit(cmd, flags, opts, "plan block edit-text", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				old, exists := block["text"]
				ops := []map[string]any{objectSetOp([]any{"itinerary", "sections", sec.Index, "blocks", idx, "text"}, old, exists, newText, false)}
				updated := cloneJSONMap(block)
				updated["text"] = newText
				report := baseEditReport("plan block edit-text", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(updated)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "replace block text"
				report.OpPaths = opPaths(ops)
				report.Stripped = stripped
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.text, "text", "", "Replacement block note text")
	cmd.Flags().BoolVar(&markdown, "markdown", false, "Compile --text as markdown: **bold**, -/* bullets, and # headings as bold label lines. Never emits Quill header attributes")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockRenameCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	var name string
	cmd := &cobra.Command{
		Use:     "rename",
		Short:   "Rename a place or lodging block display name",
		Example: "  wanderlog-pp-cli plan block rename --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 1 --name 'Property' --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				return usageErr(errors.New("--name is required"))
			}
			return runPlanEdit(cmd, flags, opts, "plan block rename", func(target map[string]any) (planEditBuildResult, error) {
				return buildPlanBlockRename(target, opts, name)
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&name, "name", "", "New place or lodging display name")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

// PATCH(amend-2026-08-23: dedicated rename writes JSON0 place.name instead of set-field)
func buildPlanBlockRename(target map[string]any, opts planEditOptions, name string) (planEditBuildResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return planEditBuildResult{}, errors.New("--name is required")
	}
	sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
	if err != nil {
		return planEditBuildResult{}, err
	}
	place := mapField(block, "place")
	if place == nil {
		return planEditBuildResult{}, fmt.Errorf("block %d has no place to rename; use plan block edit-text for notes", intAny(block["id"]))
	}
	old, exists := place["name"]
	ops := []map[string]any{objectSetOp([]any{"itinerary", "sections", sec.Index, "blocks", idx, "place", "name"}, old, exists, name, false)}
	updated := cloneJSONMap(block)
	if updatedPlace := mapField(updated, "place"); updatedPlace != nil {
		updatedPlace["name"] = name
		updated["place"] = updatedPlace
	}
	report := baseEditReport("plan block rename", opts, target)
	report.Section = ptrSectionReport(sec.Report)
	report.Block = summarizeBlock(updated)
	report.BlockID = intAny(block["id"])
	report.BlockIndex = idx
	report.Operation = "rename place"
	report.OpPaths = opPaths(ops)
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

func newNovelPlanBlockSetFieldCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	var field string
	var value string
	var jsonValue string
	var remove bool
	cmd := &cobra.Command{
		Use:     "set-field",
		Short:   "Set (--field/--value or --json-value) or remove (--remove) any non-protected field on a block; previews unless --apply",
		Example: "  wanderlog-pp-cli plan block set-field --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --field startTime --value 09:30 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, newValue, err := parseFieldMutation(field, value, jsonValue, remove)
			if err != nil {
				return usageErr(err)
			}
			if path[0] == "id" || path[0] == "type" || path[0] == "addedBy" {
				return usageErr(fmt.Errorf("block field %q is protected; use raw op for advanced edits", path[0]))
			}
			// PATCH(amend-2026-08-23: place.name is owned by plan block rename)
			if path[0] == "place" {
				return usageErr(fmt.Errorf("block field %q is protected; use plan block rename to change place.name", path[0]))
			}
			return runPlanEdit(cmd, flags, opts, "plan block set-field", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				old, exists := getMapPath(block, path)
				ops := []map[string]any{objectSetOp(append([]any{"itinerary", "sections", sec.Index, "blocks", idx}, pathToAny(path)...), old, exists, newValue, remove)}
				report := baseEditReport("plan block set-field", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "set block field"
				if remove {
					report.Operation = "remove block field"
				}
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&field, "field", "", "Block field path, e.g. startTime, durationMinutes, text")
	cmd.Flags().StringVar(&value, "value", "", "String value to set")
	cmd.Flags().StringVar(&jsonValue, "json-value", "", "JSON value to set")
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove the field instead of setting it")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanRawCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raw",
		Short: "Advanced JSON0 operations for unsupported Wanderlog plan fields",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlanRawOpCmd(flags))
	return cmd
}

func newNovelPlanRawOpCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	var opJSON string
	var opsFile string
	cmd := &cobra.Command{
		Use:   "op",
		Short: "Preview or apply an explicit ShareDB JSON0 operation array from --op or --ops-file; escape hatch for fields no named command covers",
		Example: "  wanderlog-pp-cli plan raw op --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --op '[{\"p\":[\"title\"],\"od\":\"Old\",\"oi\":\"New\"}]' --dry-run --agent\n" +
			"  wanderlog-pp-cli plan raw op --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --ops-file ./ops.json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Dry-run guard first, unconditionally: --ops-file must never be
			// read from disk before the short-circuit. Preview mode is the
			// default for real invocations -- omit --apply -- so --dry-run
			// being a pure no-op here costs no capability.
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"command":         "plan raw op",
					"dry_run":         true,
					"applied":         false,
					"apply_requested": false,
					"ops":             0,
					"warnings":        []string{"global --dry-run set: --op/--ops-file was not read and no operations were applied"},
				}, flags)
			}
			// PATCH(amend-2026-08-23: --ops-file is an alternative to --op)
			ops, err := loadJSON0Ops(opJSON, opsFile)
			if err != nil {
				return usageErr(err)
			}
			return runJSON0OpsEdit(cmd, flags, opts, "plan raw op", ops)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&opJSON, "op", "", "JSON0 operation array to send to ShareDB")
	cmd.Flags().StringVar(&opsFile, "ops-file", "", "Path to a JSON0 operation array; alternative to --op")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the raw op through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newDaySection(date string, heading string) map[string]any {
	return map[string]any{
		"blocks":           []any{},
		"date":             date,
		"heading":          heading,
		"id":               randomWanderlogID(),
		"mode":             "dayPlan",
		"placeMarkerColor": "#46cdcf",
		"placeMarkerIcon":  "map-marker",
		"text":             richText(""),
		"type":             "normal",
	}
}

func sectionReportFor(index int, day int, sec map[string]any) planSectionReport {
	blocks, _ := sec["blocks"].([]any)
	return planSectionReport{Index: index, Day: day, ID: intAny(sec["id"]), Title: firstNonEmpty(stringField(sec, "title"), stringField(sec, "heading")), Mode: stringField(sec, "mode"), Date: stringField(sec, "date"), BlockCount: len(blocks)}
}

func countDaySections(trip map[string]any) int {
	count := 0
	for _, rep := range sectionReports(trip) {
		if rep.Day > count {
			count = rep.Day
		}
	}
	return count
}

func dayDatesExcludingSection(trip map[string]any, excludeIndex int) []string {
	var dates []string
	for idx, secAny := range sections(trip) {
		if idx == excludeIndex {
			continue
		}
		sec, ok := secAny.(map[string]any)
		if !ok || stringField(sec, "mode") != "dayPlan" {
			continue
		}
		if date := stringField(sec, "date"); date != "" {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	return dates
}

func parseFieldMutation(field string, value string, jsonValue string, remove bool) ([]string, any, error) {
	path := parseFieldPath(field)
	if len(path) == 0 {
		return nil, nil, errors.New("--field is required")
	}
	if remove {
		if value != "" || jsonValue != "" {
			return nil, nil, errors.New("--remove cannot be combined with --value or --json-value")
		}
		return path, nil, nil
	}
	if jsonValue != "" {
		var parsed any
		if err := json.Unmarshal([]byte(jsonValue), &parsed); err != nil {
			return nil, nil, fmt.Errorf("parse --json-value: %w", err)
		}
		return path, parsed, nil
	}
	if value == "" {
		return nil, nil, errors.New("--value, --json-value, or --remove is required")
	}
	if path[len(path)-1] == "text" {
		return path, richText(value), nil
	}
	return path, value, nil
}

func parseFieldPath(field string) []string {
	var out []string
	for _, part := range strings.Split(field, ".") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func pathToAny(path []string) []any {
	out := make([]any, 0, len(path))
	for _, part := range path {
		if n, err := strconv.Atoi(part); err == nil {
			out = append(out, n)
		} else {
			out = append(out, part)
		}
	}
	return out
}

func getMapPath(root map[string]any, path []string) (any, bool) {
	var cur any = root
	for _, part := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func objectSetOp(path []any, old any, exists bool, newValue any, remove bool) map[string]any {
	op := map[string]any{"p": path}
	if exists {
		op["od"] = old
	}
	if !remove {
		op["oi"] = newValue
	}
	return op
}
