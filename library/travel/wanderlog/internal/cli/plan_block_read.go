// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlanBlockGetCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	var markdown, rawText bool
	var blockIDs []int
	cmd := &cobra.Command{Use: "get", Short: "Read one or several stable block ids with complete notes and visit details in one fetch", Args: cobra.NoArgs,
		Example: "  # Replace YOUR_TRIP_KEY and 123 with the key and block id from your plan outline.\n" +
			"  wanderlog-pp-cli plan block get --target-key YOUR_TRIP_KEY --block-id 123 --markdown --agent\n" +
			"  wanderlog-pp-cli plan block get --target-key YOUR_TRIP_KEY --block-id 123 --raw-text --agent\n" +
			"  wanderlog-pp-cli plan block get --target-key YOUR_TRIP_KEY --block-ids 123,456 --markdown --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			multiple := cmd.Flags().Changed("block-ids")
			if multiple && cmd.Flags().Changed("block-id") {
				return usageErr(fmt.Errorf("--block-id and --block-ids are mutually exclusive"))
			}
			ids := blockIDs
			if !multiple {
				ids = []int{opts.blockID}
			}
			if err := validateReadBlockIDs(ids); err != nil {
				return usageErr(err)
			}
			key, err := resolveEditablePlanKey(opts)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
			if err != nil {
				return err
			}
			resolved, err := resolveUniquePlanBlocks(trip, ids)
			if err != nil {
				return usageErr(err)
			}
			if !multiple {
				block := resolved[0]
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan block get", "target_key": key, "section": block.Section.Report, "block_index": block.Index, "block": readablePlanBlockWithPlanning(trip, block.Block, markdown, rawText)}, flags)
			}
			blocks := make([]map[string]any, 0, len(resolved))
			sectionRows := []planSectionReport{}
			seenSections := map[int]bool{}
			for _, item := range resolved {
				block := readablePlanBlockWithPlanning(trip, item.Block, markdown, rawText)
				block["section_index"] = item.Section.Index
				block["block_index"] = item.Index
				blocks = append(blocks, block)
				if !seenSections[item.Section.Index] {
					sectionRows = append(sectionRows, item.Section.Report)
					seenSections[item.Section.Index] = true
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan block get", "target_key": key, "blocks": blocks, "sections": sectionRows, "count": len(blocks)}, flags)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.blockID, "block-id", 0, "Stable block id; searched across all sections")
	cmd.Flags().IntSliceVar(&blockIDs, "block-ids", nil, "Comma-separated stable block ids; one fetch, input order retained; exclusive with --block-id")
	cmd.Flags().BoolVar(&markdown, "markdown", false, "Include readable Markdown for supported rich-text formatting, alongside complete plain note text")
	cmd.Flags().BoolVar(&rawText, "raw-text", false, "Include original Quill text for advanced inspection")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

// Resolve globally rather than guessing a day. Duplicate ids are unsafe even if
// both blocks happen to look identical (for example legacy copied hotel blocks).
func resolveUniquePlanBlock(trip map[string]any, id int) (resolvedSection, map[string]any, int, error) {
	resolved, err := resolveUniquePlanBlocks(trip, []int{id})
	if err != nil {
		return resolvedSection{}, nil, 0, err
	}
	return resolved[0].Section, resolved[0].Block, resolved[0].Index, nil
}

type resolvedReadBlock struct {
	Section resolvedSection
	Block   map[string]any
	Index   int
}

func validateReadBlockIDs(ids []int) error {
	if len(ids) == 0 {
		return fmt.Errorf("--block-ids must contain positive ids; use plan outline")
	}
	seen := map[int]bool{}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("--block-id or --block-ids must contain positive ids; use plan outline")
		}
		if seen[id] {
			return fmt.Errorf("duplicate requested block id %d", id)
		}
		seen[id] = true
	}
	return nil
}

// Index requested IDs in one walk and fail the whole read on ambiguity or a
// missing block. Partial output could make an agent mistake missing data for
// an empty note and overwrite content on its next edit.
func resolveUniquePlanBlocks(trip map[string]any, ids []int) ([]resolvedReadBlock, error) {
	if err := validateReadBlockIDs(ids); err != nil {
		return nil, err
	}
	wanted := map[int]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	found := map[int]resolvedReadBlock{}
	reports := sectionReports(trip)
	for si, raw := range sections(trip) {
		section, _ := raw.(map[string]any)
		sec := makeResolvedSection(si, reports[si], section)
		for bi, rawBlock := range sec.Blocks {
			candidate, _ := rawBlock.(map[string]any)
			id := intAny(candidate["id"])
			if !wanted[id] {
				continue
			}
			if _, exists := found[id]; exists {
				return nil, fmt.Errorf("block id %d is ambiguous: appears in multiple positions; repair duplicate ids before editing", id)
			}
			found[id] = resolvedReadBlock{Section: sec, Block: cloneJSONMap(candidate), Index: bi}
		}
	}
	result := make([]resolvedReadBlock, 0, len(ids))
	for _, id := range ids {
		block, ok := found[id]
		if !ok {
			return nil, fmt.Errorf("block id %d not found; refresh plan outline", id)
		}
		result = append(result, block)
	}
	return result, nil
}

func readablePlanBlock(block map[string]any, markdown, rawText bool) map[string]any {
	out := map[string]any{"id": block["id"], "type": block["type"], "text": plainBlockText(block)}
	place := mapField(block, "place")
	out["name"] = firstNonEmpty(stringField(place, "name"), stringField(block, "title"), stringField(block, "name"))
	for _, key := range []string{"startTime", "endTime", "durationMinutes", "timezone", "date", "attachments", "hotel", "depart", "arrive", "pickUp", "dropOff", "carrier", "flightInfo", "confirmationNumber", "partySize", "nameForReservation", "cruiseLine", "shipName", "voyageNumber", "title", "visited", "isVisited", "visitStatus", "warnings", "issues"} {
		if value, ok := block[key]; ok {
			out[key] = value
		}
	}
	if place != nil {
		details := map[string]any{}
		for _, key := range []string{"place_id", "placeId", "name", "formatted_address", "opening_hours", "current_opening_hours", "business_status", "utc_offset", "utc_offset_minutes", "website", "url", "formatted_phone_number", "international_phone_number", "rating", "user_ratings_total", "price_level", "types"} {
			if value, ok := place[key]; ok {
				details[key] = value
			}
		}
		if geometry := mapField(place, "geometry"); geometry != nil {
			if location := geometry["location"]; location != nil {
				details["location"] = location
			}
		}
		out["place"] = details
	}
	if stringField(block, "type") == "checklist" {
		items := []map[string]any{}
		rows, _ := block["items"].([]any)
		for _, raw := range rows {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			textBlock := map[string]any{"text": item["text"]}
			entry := map[string]any{"id": item["id"], "checked": item["checked"], "text": plainBlockText(textBlock)}
			if markdown {
				entry["markdown"] = readableBlockMarkdown(textBlock)
			}
			if rawText {
				entry["raw_text"] = item["text"]
			}
			if links := readableBlockLinks(textBlock); len(links) > 0 {
				entry["links"] = links
			}
			items = append(items, entry)
		}
		out["items"] = items
	}
	if links := readableBlockLinks(block); len(links) > 0 {
		out["links"] = links
	}
	if kind := reservationKindForBlock(block); kind != "" {
		out["reservation_kind"] = kind
	}
	if markdown {
		out["markdown"] = readableBlockMarkdown(block)
	}
	if rawText {
		out["raw_text"] = block["text"]
	}
	return out
}

// This is a readable rendering, not a lossless Quill round-trip. The complete
// plain text remains available and --raw-text exposes embedded/custom formats.
func readableBlockMarkdown(block map[string]any) string {
	text := mapField(block, "text")
	if text == nil {
		return plainBlockText(block)
	}
	ops, _ := text["ops"].([]any)
	var out, line strings.Builder
	flush := func(attrs map[string]any) {
		switch stringField(attrs, "list") {
		case "bullet":
			out.WriteString("- ")
		case "ordered":
			out.WriteString("1. ")
		}
		out.WriteString(line.String())
		out.WriteByte('\n')
		line.Reset()
	}
	for _, raw := range ops {
		op, _ := raw.(map[string]any)
		value, ok := op["insert"].(string)
		if !ok {
			line.WriteString("[embedded content]")
			continue
		}
		attrs := mapField(op, "attributes")
		chunks := strings.Split(value, "\n")
		for i, chunk := range chunks {
			if chunk != "" {
				chunk = strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`").Replace(chunk)
				if attrs["bold"] == true {
					chunk = "**" + chunk + "**"
				}
				if attrs["italic"] == true {
					chunk = "*" + chunk + "*"
				}
				if attrs["strike"] == true {
					chunk = "~~" + chunk + "~~"
				}
				if link := stringField(attrs, "link"); link != "" {
					destination := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\n", "%0A", "\r", "%0D").Replace(link)
					chunk = "[" + chunk + "](" + destination + ")"
				}
				line.WriteString(chunk)
			}
			if i < len(chunks)-1 {
				flush(attrs)
			}
		}
	}
	out.WriteString(line.String())
	return out.String()
}

// A plain-text note cannot express link targets; retain them without requiring
// models to fetch the whole Quill payload just to recover a booking URL.
func readableBlockLinks(block map[string]any) []string {
	text := mapField(block, "text")
	ops, _ := text["ops"].([]any)
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range ops {
		op, _ := raw.(map[string]any)
		link := stringField(mapField(op, "attributes"), "link")
		if link != "" && !seen[link] {
			out = append(out, link)
			seen[link] = true
		}
	}
	return out
}

func readablePlanBlockWithPlanning(trip, block map[string]any, markdown, rawText bool) map[string]any {
	out := readablePlanBlock(block, markdown, rawText)
	if place := mapField(block, "place"); place != nil {
		out["planning"] = travelPlanningFields(trip, place)
	}
	return out
}
