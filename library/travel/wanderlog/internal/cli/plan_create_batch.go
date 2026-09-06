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
	"strings"

	"github.com/spf13/cobra"
)

type planCreateEntry struct {
	Ref       string   `json:"ref,omitempty"`
	Type      string   `json:"type"`
	Day       int      `json:"day,omitempty"`
	SectionID int      `json:"section_id,omitempty"`
	PlaceID   string   `json:"place_id,omitempty"`
	Name      *string  `json:"name,omitempty"`
	Text      *string  `json:"text,omitempty"`
	Markdown  *string  `json:"markdown,omitempty"`
	Title     *string  `json:"title,omitempty"`
	Items     []string `json:"items,omitempty"`
	Start     *string  `json:"start,omitempty"`
	End       *string  `json:"end,omitempty"`
	Duration  *int     `json:"duration_minutes,omitempty"`
	Timezone  *string  `json:"timezone,omitempty"`
}

func newNovelPlanBlockAddBatchCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, applyRetries: 0, language: "en"}
	var filename string
	policy := "block"
	cmd := &cobra.Command{Use: "add-batch", Short: "Validate and append places, notes and checklists in one atomic batch", Args: cobra.NoArgs,
		Example: "  wanderlog-pp-cli plan block add-batch --target-key naertjcoixqrgrfc --blocks-file blocks.json --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(filename) == "" {
				return usageErr(fmt.Errorf("--blocks-file is required"))
			}
			f, err := os.Open(filename)
			if err != nil {
				return usageErr(fmt.Errorf("read blocks file: %w", err))
			}
			defer f.Close()
			data, err := io.ReadAll(io.LimitReader(f, 2*1024*1024+1))
			if err != nil {
				return usageErr(err)
			}
			if len(data) > 2*1024*1024 {
				return usageErr(fmt.Errorf("blocks-file exceeds 2 MiB"))
			}
			entries, err := parsePlanCreateEntries(data)
			if err != nil {
				return usageErr(err)
			}
			if err := validateClosedPlacePolicy(policy); err != nil {
				return usageErr(err)
			}
			if _, err := resolveEditablePlanKey(opts); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			stops := []fillDayStop{}
			seen := map[string]bool{}
			for _, entry := range entries {
				if entry.Type == "place" && !seen[entry.PlaceID] {
					stops = append(stops, fillDayStop{PlaceID: entry.PlaceID})
					seen[entry.PlaceID] = true
				}
			}
			places, err := resolveFillDayPlaces(ctx, c, opts, stops)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Creation does not retry on a rejected or uncertain acknowledgement. Inspect
			// the plan before rerunning when the server outcome is unknown.
			return runPlanEditWithClient(cmd, flags, c, opts, "plan block add-batch", func(target map[string]any) (planEditBuildResult, error) {
				return buildPlanCreateBatch(target, opts, entries, places, policy, opts.apply && !flags.dryRun)
			})
		}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&filename, "blocks-file", "", "JSON array; each block has type and day or section_id; places require place_id")
	cmd.Flags().StringVar(&opts.language, "language", "en", "Language for resolved place details")
	cmd.Flags().StringVar(&policy, "closed-place-policy", "block", "How to handle explicitly closed places: block, warn, ignore")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the complete validated batch once; default previews without writing")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func parsePlanCreateEntries(data []byte) ([]planCreateEntry, error) {
	var objects []json.RawMessage
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, fmt.Errorf("blocks-file must contain one JSON array: %w", err)
	}
	if len(objects) == 0 || len(objects) > 500 {
		return nil, fmt.Errorf("blocks-file must contain 1 to 500 blocks")
	}
	allowed := map[string]bool{"ref": true, "type": true, "day": true, "section_id": true, "place_id": true, "name": true, "text": true, "markdown": true, "title": true, "items": true, "start": true, "end": true, "duration_minutes": true, "timezone": true}
	for i, obj := range objects {
		d := json.NewDecoder(bytes.NewReader(obj))
		token, err := d.Token()
		if err != nil || token != json.Delim('{') {
			return nil, fmt.Errorf("block %d must be an object", i+1)
		}
		seen := map[string]bool{}
		for d.More() {
			tok, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := tok.(string)
			if !ok || !allowed[key] {
				return nil, fmt.Errorf("block %d: unknown field %q", i+1, key)
			}
			if seen[key] {
				return nil, fmt.Errorf("block %d: duplicate field %q", i+1, key)
			}
			seen[key] = true
			var value json.RawMessage
			if err := d.Decode(&value); err != nil {
				return nil, err
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("block %d: %s cannot be null; omit unused fields", i+1, key)
			}
		}
	}
	var entries []planCreateEntry
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&entries); err != nil {
		return nil, err
	}
	refs := map[string]bool{}
	for i, entry := range entries {
		if err := validatePlanCreateEntry(entry); err != nil {
			return nil, fmt.Errorf("block %d: %w", i+1, err)
		}
		if entry.Ref != "" {
			if refs[entry.Ref] {
				return nil, fmt.Errorf("block %d: duplicate ref %q", i+1, entry.Ref)
			}
			refs[entry.Ref] = true
		}
	}
	return entries, nil
}

func validatePlanCreateEntry(e planCreateEntry) error {
	if (e.Day > 0) == (e.SectionID > 0) || e.Day < 0 || e.SectionID < 0 {
		return fmt.Errorf("choose exactly one positive day or section_id")
	}
	if e.Text != nil && e.Markdown != nil {
		return fmt.Errorf("choose text or markdown, not both")
	}
	if e.Name != nil && strings.TrimSpace(*e.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if e.Type != "place" && (e.PlaceID != "" || e.Name != nil) {
		return fmt.Errorf("place_id and name apply only to place blocks")
	}
	if e.Type != "checklist" && (e.Title != nil || e.Items != nil) {
		return fmt.Errorf("title and items apply only to checklist blocks")
	}
	switch e.Type {
	case "place":
		if strings.TrimSpace(e.PlaceID) == "" || e.PlaceID != strings.TrimSpace(e.PlaceID) {
			return fmt.Errorf("place_id is required and cannot have surrounding whitespace")
		}
	case "note":
		if e.Text == nil && e.Markdown == nil {
			return fmt.Errorf("note requires text or markdown")
		}
		if e.Text != nil && strings.TrimSpace(*e.Text) == "" || e.Markdown != nil && strings.TrimSpace(*e.Markdown) == "" {
			return fmt.Errorf("note text cannot be empty")
		}
	case "checklist":
		if (e.Title == nil || strings.TrimSpace(*e.Title) == "") && len(e.Items) == 0 {
			return fmt.Errorf("checklist requires a title or items")
		}
		for _, item := range e.Items {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("checklist items cannot be empty")
			}
		}
	default:
		return fmt.Errorf("type must be place, note, or checklist")
	}
	_, _, err := buildScheduleOps(map[string]any{}, nil, createEntrySchedule(e), false)
	return err
}

func createEntrySchedule(e planCreateEntry) map[string]any {
	out := map[string]any{}
	if e.Start != nil {
		out["startTime"] = *e.Start
	}
	if e.End != nil {
		out["endTime"] = *e.End
	}
	if e.Duration != nil {
		out["durationMinutes"] = *e.Duration
	}
	if e.Timezone != nil {
		out["timezone"] = *e.Timezone
	}
	return out
}
