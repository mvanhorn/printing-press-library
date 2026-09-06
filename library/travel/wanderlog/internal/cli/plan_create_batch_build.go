// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strings"
)

func buildPlanCreateBatch(target map[string]any, opts planEditOptions, entries []planCreateEntry, places map[string]map[string]any, policy string, receiptOnly bool) (planEditBuildResult, error) {
	if len(entries) == 0 || len(entries) > 500 {
		return planEditBuildResult{}, fmt.Errorf("batch must contain 1 to 500 blocks")
	}
	if err := validateClosedPlacePolicy(policy); err != nil {
		return planEditBuildResult{}, err
	}
	used := map[int]bool{}
	collectCreateBatchIDs(target, used)
	offsets := map[int]int{}
	report := baseEditReport("plan block add-batch", opts, target)
	ops := []map[string]any{}
	for i, entry := range entries {
		if err := validatePlanCreateEntry(entry); err != nil {
			return planEditBuildResult{}, fmt.Errorf("block %d: %w", i+1, err)
		}
		if entry.SectionID > 0 {
			matches := 0
			for _, raw := range sections(target) {
				s, _ := raw.(map[string]any)
				if intAny(s["id"]) == entry.SectionID {
					matches++
				}
			}
			if matches != 1 {
				return planEditBuildResult{}, fmt.Errorf("block %d: section_id %d matches %d sections; refresh plan sections", i+1, entry.SectionID, matches)
			}
		}
		sec, err := resolveSection(target, entry.Day, -1, entry.SectionID)
		if err != nil {
			return planEditBuildResult{}, fmt.Errorf("block %d: %w", i+1, err)
		}
		if raw, ok := sec.Raw["blocks"]; !ok || raw == nil {
			return planEditBuildResult{}, fmt.Errorf("block %d: section has no blocks array", i+1)
		}
		if _, ok := sec.Raw["blocks"].([]any); !ok {
			return planEditBuildResult{}, fmt.Errorf("block %d: invalid section blocks array", i+1)
		}
		var block map[string]any
		switch entry.Type {
		case "place":
			place := cloneJSONMap(places[fillDayStopKey(fillDayStop{PlaceID: entry.PlaceID})])
			if len(place) == 0 {
				return planEditBuildResult{}, fmt.Errorf("block %d: place %q was not resolved", i+1, entry.PlaceID)
			}
			if warning, closed := placeClosedOnDateWarning(place, sec.Report.Date); closed {
				if policy == "block" {
					return planEditBuildResult{}, fmt.Errorf("block %d: %s", i+1, warning)
				}
				if policy == "warn" {
					report.Warnings = append(report.Warnings, warning)
				}
			}
			if entry.Name != nil {
				place["name"] = strings.TrimSpace(*entry.Name)
			}
			block = newPlaceBlock(place, "")
		case "note":
			block = newNoteBlock("")
		case "checklist":
			title := ""
			if entry.Title != nil {
				title = *entry.Title
			}
			block = newChecklistBlock(title, entry.Items)
		}
		if entry.Text != nil || entry.Markdown != nil {
			text := ""
			markdown := entry.Markdown != nil
			if markdown {
				text = *entry.Markdown
			} else {
				text = *entry.Text
			}
			delta, stripped, err := blockNoteText(text, markdown)
			if err != nil {
				return planEditBuildResult{}, fmt.Errorf("block %d: %w", i+1, err)
			}
			block["text"] = delta
			report.Stripped = append(report.Stripped, stripped...)
		}
		_, scheduled, err := buildScheduleOps(block, nil, createEntrySchedule(entry), false)
		if err != nil {
			return planEditBuildResult{}, fmt.Errorf("block %d: %w", i+1, err)
		}
		block = scheduled
		id, err := newCreateBatchID(used)
		if err != nil {
			return planEditBuildResult{}, err
		}
		block["id"] = id
		itemIDs := []int{}
		if items, ok := block["items"].([]any); ok {
			for _, raw := range items {
				item, _ := raw.(map[string]any)
				itemID, err := newCreateBatchID(used)
				if err != nil {
					return planEditBuildResult{}, err
				}
				item["id"] = itemID
				itemIDs = append(itemIDs, itemID)
			}
		}
		index := len(sec.Blocks) + offsets[sec.Index]
		offsets[sec.Index]++
		ops = append(ops, map[string]any{"p": []any{"itinerary", "sections", sec.Index, "blocks", index}, "li": block})
		change := map[string]any{"input_index": i, "block_id": id, "type": entry.Type, "section_id": sec.Report.ID, "section_index": sec.Index, "block_index": index}
		if entry.Ref != "" {
			change["ref"] = entry.Ref
		}
		if len(itemIDs) > 0 {
			change["item_ids"] = itemIDs
		}
		if !receiptOnly {
			preview := readablePlanBlock(block, true, false)
			delete(preview, "place")
			delete(preview, "text")
			if entry.PlaceID != "" {
				preview["place_id"] = entry.PlaceID
			}
			change["after"] = preview
		}
		report.Changes = append(report.Changes, change)
	}
	report.Operation = fmt.Sprintf("insert %d blocks atomically", len(entries))
	report.OpPaths = opPaths(ops)
	if !receiptOnly {
		report.Warnings = append(report.Warnings, "preview block IDs are provisional; use the stable IDs in the applied receipt for later edits")
	}
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

func collectCreateBatchIDs(value any, used map[int]bool) {
	switch x := value.(type) {
	case map[string]any:
		for k, v := range x {
			if k == "id" {
				if id := intAny(v); id > 0 {
					used[id] = true
				}
			}
			collectCreateBatchIDs(v, used)
		}
	case []any:
		for _, v := range x {
			collectCreateBatchIDs(v, used)
		}
	}
}

func newCreateBatchID(used map[int]bool) (int, error) {
	for tries := 0; tries < 16; tries++ {
		id := randomWanderlogID()
		if !used[id] {
			used[id] = true
			return id, nil
		}
	}
	return 0, fmt.Errorf("could not allocate a unique block ID; no edit has been submitted")
}
