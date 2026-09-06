// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
)

// One object per block makes conflicting edits explicit. Named fields describe
// intent; callers never supply JSON0 paths or code. Nil means not requested.
type planBlockChange struct {
	BlockID  int     `json:"block_id"`
	Markdown *string `json:"markdown,omitempty"`
	Name     *string `json:"name,omitempty"`
	Start    *string `json:"start,omitempty"`
	End      *string `json:"end,omitempty"`
	Duration *int    `json:"duration_minutes,omitempty"`
}

func newNovelPlanEditCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	var filename string
	cmd := &cobra.Command{Use: "edit", Short: "Preview or atomically apply named block changes from a JSON file", Args: cobra.NoArgs,
		Long: "Validate a JSON array of named changes against the current plan and preview the affected fields. Each object requires a unique block_id and at least one of markdown, name, start, end, or duration_minutes. For example: [{\"block_id\":123,\"markdown\":\"**Bring tickets**\",\"start\":\"09:00\",\"duration_minutes\":60}]. Replace the illustrative id with an actual plan block id. Unknown fields and duplicate ids are errors. --dry-run still reads and validates the file; --apply submits all valid changes together.",
		Example: "  # Save your reviewed changes as changes.json; use actual block ids from plan outline.\n" +
			"  wanderlog-pp-cli plan edit --target-key YOUR_TRIP_KEY --changes-file changes.json --dry-run --agent\n" +
			"  # Apply the same reviewed file to the same target.\n" +
			"  wanderlog-pp-cli plan edit --target-key YOUR_TRIP_KEY --changes-file changes.json --apply --agent",

		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(filename) == "" {
				return usageErr(fmt.Errorf("--changes-file is required"))
			}
			data, err := os.ReadFile(filename)
			if err != nil {
				return usageErr(fmt.Errorf("read changes file: %w", err))
			}
			changes, err := parsePlanChanges(data)
			if err != nil {
				return usageErr(err)
			}
			return runPlanEdit(cmd, flags, opts, "plan edit", func(target map[string]any) (planEditBuildResult, error) {
				return buildPlanChanges(target, opts, changes)
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&filename, "changes-file", "", "JSON array: one object per block_id with markdown, name, start, end, or duration_minutes")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply all changes as one ShareDB operation; default validates and previews")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func parsePlanChanges(data []byte) ([]planBlockChange, error) {
	if err := validatePlanChangeKeys(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var changes []planBlockChange
	if err := decoder.Decode(&changes); err != nil {
		return nil, fmt.Errorf("changes-file must be a JSON array of named block changes: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("changes-file must contain exactly one JSON array")
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("changes-file must contain at least one change")
	}
	if len(changes) > 500 {
		return nil, fmt.Errorf("changes-file exceeds 500 blocks; split into deliberate batches")
	}
	seen := map[int]bool{}
	for i, c := range changes {
		if c.BlockID <= 0 {
			return nil, fmt.Errorf("change %d: block_id must be positive", i+1)
		}
		if seen[c.BlockID] {
			return nil, fmt.Errorf("change %d: duplicate block_id %d; combine fields in one object", i+1, c.BlockID)
		}
		seen[c.BlockID] = true
		if c.Markdown == nil && c.Name == nil && c.Start == nil && c.End == nil && c.Duration == nil {
			return nil, fmt.Errorf("change %d: supply markdown, name, start, end, or duration_minutes", i+1)
		}
		if c.Name != nil && strings.TrimSpace(*c.Name) == "" {
			return nil, fmt.Errorf("change %d: name cannot be empty", i+1)
		}
		if c.Start != nil && !validClock(*c.Start) {
			return nil, fmt.Errorf("change %d: start must be HH:MM", i+1)
		}
		if c.End != nil && !validClock(*c.End) {
			return nil, fmt.Errorf("change %d: end must be HH:MM", i+1)
		}
		if c.Duration != nil && (*c.Duration < 0 || *c.Duration > 1440) {
			return nil, fmt.Errorf("change %d: duration_minutes must be between 0 and 1440", i+1)
		}
	}
	return changes, nil
}

func buildPlanChanges(target map[string]any, opts planEditOptions, changes []planBlockChange) (planEditBuildResult, error) {
	var ops []map[string]any
	report := baseEditReport("plan edit", opts, target)
	for _, change := range changes {
		sec, block, index, err := resolveUniquePlanBlock(target, change.BlockID)
		if err != nil {
			return planEditBuildResult{}, err
		}
		base := []any{"itinerary", "sections", sec.Index, "blocks", index}
		updated := cloneJSONMap(block)
		set := func(field string, value any) {
			if reflect.DeepEqual(block[field], value) {
				return
			}
			old, exists := block[field]
			ops = append(ops, objectSetOp(append(append([]any{}, base...), field), old, exists, value, false))
			updated[field] = value
		}
		if change.Markdown != nil {
			text, stripped, err := blockNoteText(*change.Markdown, true)
			if err != nil {
				return planEditBuildResult{}, fmt.Errorf("block %d markdown: %w", change.BlockID, err)
			}
			set("text", text)
			report.Stripped = append(report.Stripped, stripped...)
		}
		if change.Name != nil {
			name := strings.TrimSpace(*change.Name)
			place := mapField(block, "place")
			if place == nil {
				return planEditBuildResult{}, fmt.Errorf("block %d has no place to rename", change.BlockID)
			}
			if stringField(place, "name") != name {
				old, exists := place["name"]
				path := append(append([]any{}, base...), "place", "name")
				ops = append(ops, objectSetOp(path, old, exists, name, false))
				mapField(updated, "place")["name"] = name
			}
		}
		schedule := map[string]any{}
		if change.Start != nil {
			schedule["startTime"] = *change.Start
		}
		if change.End != nil {
			schedule["endTime"] = *change.End
		}
		if change.Duration != nil {
			schedule["durationMinutes"] = *change.Duration
		}
		if len(schedule) > 0 {
			scheduleOps, scheduled, err := buildScheduleOps(updated, base, schedule, false)
			if err != nil {
				return planEditBuildResult{}, fmt.Errorf("block %d schedule: %w", change.BlockID, err)
			}
			ops = append(ops, scheduleOps...)
			updated = scheduled
		}
		report.Changes = append(report.Changes, map[string]any{"block_id": change.BlockID, "section": sec.Report, "before": planChangePreview(block, change), "after": planChangePreview(updated, change)})
	}
	report.Operation = "update blocks atomically"
	report.OpPaths = opPaths(ops)
	if len(ops) == 0 {
		return planEditBuildResult{}, fmt.Errorf("changes already match the plan; no edits needed")
	}
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

// encoding/json normally accepts duplicate keys and treats null pointers as
// absent. Reject both so a reviewed file cannot quietly override its own intent.
func validatePlanChangeKeys(data []byte) error {
	var objects []json.RawMessage
	if err := json.Unmarshal(data, &objects); err != nil {
		return fmt.Errorf("changes-file must be a JSON array: %w", err)
	}
	for i, object := range objects {
		decoder := json.NewDecoder(bytes.NewReader(object))
		token, err := decoder.Token()
		if err != nil || token != json.Delim('{') {
			return fmt.Errorf("change %d must be an object", i+1)
		}
		seen := map[string]bool{}
		for decoder.More() {
			token, err = decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("change %d has an invalid key", i+1)
			}
			switch key {
			case "block_id", "markdown", "name", "start", "end", "duration_minutes":
			default:
				return fmt.Errorf("change %d: unknown field %q", i+1, key)
			}
			if seen[key] {
				return fmt.Errorf("change %d: duplicate field %q", i+1, key)
			}
			seen[key] = true
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("change %d: %s cannot be null; omit unchanged fields or use an empty markdown string to clear the note", i+1, key)
			}
		}
	}
	return nil
}

// Preview only the fields touched by this change. Renaming one stop should not
// echo its entire note, hotel metadata and opening hours twice.
func planChangePreview(block map[string]any, change planBlockChange) map[string]any {
	out := map[string]any{"name": firstNonEmpty(stringField(mapField(block, "place"), "name"), stringField(block, "title"), stringField(block, "name"))}
	if change.Markdown != nil {
		out["markdown"] = readableBlockMarkdown(block)
	}
	if change.Start != nil || change.End != nil || change.Duration != nil {
		for _, field := range []string{"startTime", "endTime", "durationMinutes"} {
			out[field] = block[field]
		}
	}
	return out
}
