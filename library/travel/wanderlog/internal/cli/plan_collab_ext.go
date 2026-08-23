// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/spf13/cobra"
)

func newNovelPlanBlockScheduleCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, applyRetries: 2}
	var start string
	var end string
	var duration int
	var timezone string
	var clear bool
	cmd := &cobra.Command{
		Use:     "schedule",
		Short:   "Set --start, --end, --duration-minutes, and --timezone on a block, or --clear them; previews unless --apply",
		Example: "  wanderlog-pp-cli plan block schedule --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 2 --block-index 0 --start 09:30 --duration-minutes 90 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear {
				if cmd.Flags().Changed("start") || cmd.Flags().Changed("end") || cmd.Flags().Changed("duration-minutes") || cmd.Flags().Changed("timezone") {
					return usageErr(errors.New("--clear cannot be combined with schedule value flags"))
				}
			} else if !cmd.Flags().Changed("start") && !cmd.Flags().Changed("end") && !cmd.Flags().Changed("duration-minutes") && !cmd.Flags().Changed("timezone") {
				return usageErr(errors.New("set at least one of --start, --end, --duration-minutes, --timezone, or pass --clear"))
			}
			if start != "" && !validClock(start) {
				return usageErr(errors.New("--start must be HH:MM"))
			}
			if end != "" && !validClock(end) {
				return usageErr(errors.New("--end must be HH:MM"))
			}
			if duration < 0 {
				return usageErr(errors.New("--duration-minutes must be >= 0"))
			}
			return runPlanEdit(cmd, flags, opts, "plan block schedule", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				base := []any{"itinerary", "sections", sec.Index, "blocks", idx}
				var ops []map[string]any
				previewBlock := cloneJSONMap(block)
				if clear {
					for _, field := range []string{"startTime", "endTime", "durationMinutes", "timezone"} {
						old, exists := block[field]
						if exists {
							ops = append(ops, objectSetOp(append(base, field), old, exists, nil, true))
							delete(previewBlock, field)
						}
					}
				} else {
					if cmd.Flags().Changed("start") {
						ops = append(ops, objectSetOp(append(base, "startTime"), block["startTime"], block["startTime"] != nil, start, false))
						previewBlock["startTime"] = start
					}
					if cmd.Flags().Changed("end") {
						ops = append(ops, objectSetOp(append(base, "endTime"), block["endTime"], block["endTime"] != nil, end, false))
						previewBlock["endTime"] = end
					}
					if cmd.Flags().Changed("duration-minutes") {
						ops = append(ops, objectSetOp(append(base, "durationMinutes"), block["durationMinutes"], block["durationMinutes"] != nil, duration, false))
						previewBlock["durationMinutes"] = duration
					}
					if cmd.Flags().Changed("timezone") {
						ops = append(ops, objectSetOp(append(base, "timezone"), block["timezone"], block["timezone"] != nil, timezone, false))
						previewBlock["timezone"] = timezone
					}
				}
				if len(ops) == 0 {
					return planEditBuildResult{}, errors.New("schedule edit produced no changes")
				}
				report := baseEditReport("plan block schedule", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(previewBlock)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "set block schedule"
				if clear {
					report.Operation = "clear block schedule"
				}
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&start, "start", "", "Start time in HH:MM")
	cmd.Flags().StringVar(&end, "end", "", "End time in HH:MM")
	cmd.Flags().IntVar(&duration, "duration-minutes", 0, "Duration in minutes")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Optional timezone label or IANA timezone")
	cmd.Flags().BoolVar(&clear, "clear", false, "Remove startTime, endTime, durationMinutes, and timezone fields")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockAttachmentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "attachment [command]", Short: "List, add, or remove block attachments", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanBlockAttachmentListCmd(flags))
	cmd.AddCommand(newNovelPlanBlockAttachmentAddCmd(flags))
	cmd.AddCommand(newNovelPlanBlockAttachmentRemoveCmd(flags))
	return cmd
}

func newNovelPlanBlockAttachmentListCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List attachments on a Wanderlog block",
		Example:     "  wanderlog-pp-cli plan block attachment list --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			key, err := resolveEditablePlanKey(opts)
			if err != nil {
				return usageErr(err)
			}
			trip, _, err := fetchPlanSnapshotViaShareDB(ctx, c, key, opts.clientSchemaVersion)
			if err != nil {
				trip, _, err = fetchPlan(ctx, c, key, opts.clientSchemaVersion)
				if err != nil {
					return err
				}
			}
			sec, block, idx, err := resolveBlock(trip, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
			if err != nil {
				return usageErr(err)
			}
			attachments, _ := block["attachments"].([]any)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan block attachment list", "target_key": key, "section": sec.Report, "block_id": intAny(block["id"]), "block_index": idx, "attachments": attachments}, flags)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockAttachmentAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, position: -1, applyRetries: 2}
	var title, url, filename, mimeType, kind, jsonValue string
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add an attachment metadata object to a Wanderlog block",
		Example: "  wanderlog-pp-cli plan block attachment add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --title Tickets --url https://example.com/tickets.pdf --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonValue == "" && strings.TrimSpace(url) == "" {
				return usageErr(errors.New("--url or --json-value is required"))
			}
			var attachment map[string]any
			if jsonValue != "" {
				if err := json.Unmarshal([]byte(jsonValue), &attachment); err != nil {
					return usageErr(fmt.Errorf("parse --json-value: %w", err))
				}
			} else {
				attachment = map[string]any{"id": randomWanderlogID(), "type": firstNonEmpty(kind, "attachment"), "url": url}
				if title != "" {
					attachment["title"] = title
				}
				if filename != "" {
					attachment["filename"] = filename
				}
				if mimeType != "" {
					attachment["mimeType"] = mimeType
				}
			}
			if attachment["id"] == nil {
				attachment["id"] = randomWanderlogID()
			}
			return runPlanEdit(cmd, flags, opts, "plan block attachment add", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, blockIdx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				attachments, _ := block["attachments"].([]any)
				pos := normalizeInsertPosition(opts.position, len(attachments))
				base := []any{"itinerary", "sections", sec.Index, "blocks", blockIdx, "attachments"}
				var ops []map[string]any
				if _, exists := block["attachments"]; !exists {
					newAttachments := append([]any{}, attachments...)
					newAttachments = append(newAttachments, attachment)
					ops = append(ops, objectSetOp(base, nil, false, newAttachments, false))
				} else {
					ops = append(ops, map[string]any{"p": append(base, pos), "li": attachment})
				}
				report := baseEditReport("plan block attachment add", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = blockIdx
				report.Operation = "add block attachment"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&title, "title", "", "Attachment title")
	cmd.Flags().StringVar(&url, "url", "", "Attachment URL")
	cmd.Flags().StringVar(&filename, "filename", "", "Attachment filename")
	cmd.Flags().StringVar(&mimeType, "mime-type", "", "Attachment MIME type")
	cmd.Flags().StringVar(&kind, "kind", "", "Attachment type/kind field; default attachment")
	cmd.Flags().StringVar(&jsonValue, "json-value", "", "Full attachment JSON object")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based attachment insertion position; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockAttachmentRemoveCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, applyRetries: 2}
	var attachmentID int
	var index int = -1
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove an attachment from a Wanderlog block",
		Example: "  wanderlog-pp-cli plan block attachment remove --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --attachment-index 0 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if attachmentID == 0 && index < 0 {
				return usageErr(errors.New("--attachment-id or --attachment-index is required"))
			}
			return runPlanEdit(cmd, flags, opts, "plan block attachment remove", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, blockIdx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				attachments, _ := block["attachments"].([]any)
				idx := index
				if attachmentID != 0 {
					idx = findObjectIndexByID(attachments, attachmentID)
				}
				if idx < 0 || idx >= len(attachments) {
					return planEditBuildResult{}, errors.New("attachment not found")
				}
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", blockIdx, "attachments", idx}, "ld": attachments[idx]}}
				report := baseEditReport("plan block attachment remove", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = blockIdx
				report.Operation = "remove block attachment"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().IntVar(&attachmentID, "attachment-id", 0, "Attachment id")
	cmd.Flags().IntVar(&index, "attachment-index", -1, "Zero-based attachment index")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanChecklistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "checklist", Short: "Add and edit checklist blocks", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanChecklistAddCmd(flags))
	cmd.AddCommand(newNovelPlanChecklistItemCmd(flags))
	return cmd
}

func newNovelPlanChecklistAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, position: -1, applyRetries: 2}
	var title string
	var items []string
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a checklist block to a day or section",
		Example: "  wanderlog-pp-cli plan checklist add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --title Packing --item Passport --item Sunscreen --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(title) == "" && len(items) == 0 {
				return usageErr(errors.New("--title or at least one --item is required"))
			}
			return runPlanEdit(cmd, flags, opts, "plan checklist add", func(target map[string]any) (planEditBuildResult, error) {
				sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
				if err != nil {
					return planEditBuildResult{}, err
				}
				block := newChecklistBlock(title, items)
				idx := normalizeInsertPosition(opts.position, len(sec.Blocks))
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "li": block}}
				report := baseEditReport("plan checklist add", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "insert checklist block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&title, "title", "", "Checklist title")
	cmd.Flags().StringArrayVar(&items, "item", nil, "Checklist item text; repeatable")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based block insertion position; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanChecklistItemCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "item", Short: "Add, check, or remove checklist items", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanChecklistItemAddCmd(flags))
	cmd.AddCommand(newNovelPlanChecklistItemCheckCmd(flags))
	cmd.AddCommand(newNovelPlanChecklistItemRemoveCmd(flags))
	return cmd
}

func newNovelPlanChecklistItemAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, position: -1, applyRetries: 2}
	var text string
	cmd := &cobra.Command{Use: "add", Short: "Add an item to a checklist block", Example: "  wanderlog-pp-cli plan checklist item add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --text Passport --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(text) == "" {
			return usageErr(errors.New("--text is required"))
		}
		return runChecklistItemEdit(cmd, flags, opts, "plan checklist item add", func(items []any) (map[string]any, []map[string]any, string, error) {
			idx := normalizeInsertPosition(opts.position, len(items))
			item := newChecklistItem(text, false)
			return item, []map[string]any{{"p": nil, "li": item, "_idx": idx}}, "add checklist item", nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&text, "text", "", "Checklist item text")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based item position; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanChecklistItemCheckCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, applyRetries: 2}
	var itemID int
	var itemIndex int = -1
	var checked bool
	cmd := &cobra.Command{Use: "check", Short: "Set checked state on a checklist item", Example: "  wanderlog-pp-cli plan checklist item check --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --item-index 0 --checked --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if itemID == 0 && itemIndex < 0 {
			return usageErr(errors.New("--item-id or --item-index is required"))
		}
		return runChecklistItemEdit(cmd, flags, opts, "plan checklist item check", func(items []any) (map[string]any, []map[string]any, string, error) {
			idx := itemIndex
			if itemID != 0 {
				idx = findObjectIndexByID(items, itemID)
			}
			if idx < 0 || idx >= len(items) {
				return nil, nil, "", errors.New("checklist item not found")
			}
			item, _ := items[idx].(map[string]any)
			return item, []map[string]any{{"p": []any{idx, "checked"}, "od": item["checked"], "oi": checked}}, "set checklist item checked", nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().IntVar(&itemID, "item-id", 0, "Checklist item id")
	cmd.Flags().IntVar(&itemIndex, "item-index", -1, "Zero-based checklist item index")
	cmd.Flags().BoolVar(&checked, "checked", true, "Checked state to set")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanChecklistItemRemoveCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, applyRetries: 2}
	var itemID int
	var itemIndex int = -1
	cmd := &cobra.Command{Use: "remove", Short: "Remove an item from a checklist block", Example: "  wanderlog-pp-cli plan checklist item remove --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --item-index 0 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if itemID == 0 && itemIndex < 0 {
			return usageErr(errors.New("--item-id or --item-index is required"))
		}
		return runChecklistItemEdit(cmd, flags, opts, "plan checklist item remove", func(items []any) (map[string]any, []map[string]any, string, error) {
			idx := itemIndex
			if itemID != 0 {
				idx = findObjectIndexByID(items, itemID)
			}
			if idx < 0 || idx >= len(items) {
				return nil, nil, "", errors.New("checklist item not found")
			}
			item, _ := items[idx].(map[string]any)
			return item, []map[string]any{{"p": []any{idx}, "ld": items[idx]}}, "remove checklist item", nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().IntVar(&itemID, "item-id", 0, "Checklist item id")
	cmd.Flags().IntVar(&itemIndex, "item-index", -1, "Zero-based checklist item index")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func runChecklistItemEdit(cmd *cobra.Command, flags *rootFlags, opts planEditOptions, commandName string, build func([]any) (map[string]any, []map[string]any, string, error)) error {
	return runPlanEdit(cmd, flags, opts, commandName, func(target map[string]any) (planEditBuildResult, error) {
		sec, block, blockIdx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
		if err != nil {
			return planEditBuildResult{}, err
		}
		if stringField(block, "type") != "checklist" {
			return planEditBuildResult{}, errors.New("selected block is not a checklist")
		}
		items, _ := block["items"].([]any)
		_, partial, operation, err := build(items)
		if err != nil {
			return planEditBuildResult{}, err
		}
		base := []any{"itinerary", "sections", sec.Index, "blocks", blockIdx, "items"}
		var ops []map[string]any
		if _, exists := block["items"]; !exists {
			ops = append(ops, objectSetOp(base, nil, false, []any{}, false))
		}
		for _, op := range partial {
			if idxAny, ok := op["_idx"]; ok {
				delete(op, "_idx")
				op["p"] = append(base, idxAny)
			} else {
				op["p"] = append(base, op["p"].([]any)...)
			}
			ops = append(ops, op)
		}
		report := baseEditReport(commandName, opts, target)
		report.Section = ptrSectionReport(sec.Report)
		report.Block = summarizeBlock(block)
		report.BlockID = intAny(block["id"])
		report.BlockIndex = blockIdx
		report.Operation = operation
		report.OpPaths = opPaths(ops)
		return planEditBuildResult{Ops: ops, Report: report}, nil
	})
}

func newNovelPlanCommentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "comments", Short: "List and edit Wanderlog plan comments", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanCommentsListCmd(flags))
	cmd.AddCommand(newNovelPlanCommentsAddCmd(flags))
	cmd.AddCommand(newNovelPlanCommentsEditCmd(flags))
	cmd.AddCommand(newNovelPlanCommentsDeleteCmd(flags))
	cmd.AddCommand(newNovelPlanCommentsVoteCmd(flags))
	return cmd
}

func newNovelPlanCommentsListCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{
		Use:         "list [key]",
		Short:       "List comments for a Wanderlog plan",
		Example:     "  wanderlog-pp-cli plan comments list --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			key, err := resolveCommentsListPlanKey(opts, args)
			if err != nil {
				return usageErr(err)
			}
			data, err := c.GetNoCache(ctx, "/api/tripPlans/"+key+"/comments", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if envErr := wanderlogAPIEnvelopeError(data); envErr != nil {
				return envErr
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan comments list", "target_key": key, "results": json.RawMessage(data)}, flags)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

// PATCH(amend-2026-08-23: comments list accepts identifier flags and a positional key)
func resolveCommentsListPlanKey(opts planEditOptions, args []string) (string, error) {
	if len(args) > 0 {
		pos := strings.TrimSpace(args[0])
		if pos != "" {
			if isAllDigits(pos) {
				return "", fmt.Errorf("%s is tripPlan.id, not a key. Use the 16-char key from trips home (field: key). Example: --target-key naertjcoixqrgrfc", pos)
			}
			if opts.targetKey == "" && opts.planURL == "" {
				opts.targetKey = pos
			}
		}
	}
	return resolveEditablePlanKey(opts)
}

func newNovelPlanCommentsAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var text string
	var parentID int
	cmd := &cobra.Command{Use: "add", Short: "Add a comment to a Wanderlog plan", Example: "  wanderlog-pp-cli plan comments add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --text 'Nice itinerary' --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(text) == "" {
			return usageErr(errors.New("--text is required"))
		}
		body := map[string]any{"text": richText(text)}
		if parentID != 0 {
			body["parentId"] = parentID
		} else {
			body["parentId"] = nil
		}
		return runPlanCommentWrite(cmd, flags, opts, "plan comments add", func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Post(ctx, "/api/tripPlans/"+key+"/comments", body)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&text, "text", "", "Comment text")
	cmd.Flags().IntVar(&parentID, "parent-id", 0, "Parent comment id for a reply")
	return cmd
}

func newNovelPlanCommentsEditCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var text string
	var commentID int
	cmd := &cobra.Command{Use: "edit", Short: "Edit a Wanderlog comment", Example: "  wanderlog-pp-cli plan comments edit --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --comment-id 1 --text Updated --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if commentID == 0 {
			return usageErr(errors.New("--comment-id is required"))
		}
		if strings.TrimSpace(text) == "" {
			return usageErr(errors.New("--text is required"))
		}
		body := map[string]any{"text": richText(text)}
		return runPlanCommentWrite(cmd, flags, opts, "plan comments edit", func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Post(ctx, "/api/comments/"+strconv.Itoa(commentID), body)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&commentID, "comment-id", 0, "Comment id")
	cmd.Flags().StringVar(&text, "text", "", "Replacement comment text")
	return cmd
}

func newNovelPlanCommentsDeleteCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var commentID int
	cmd := &cobra.Command{Use: "delete", Short: "Delete a Wanderlog comment", Example: "  wanderlog-pp-cli plan comments delete --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --comment-id 1 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if commentID == 0 {
			return usageErr(errors.New("--comment-id is required"))
		}
		return runPlanCommentWrite(cmd, flags, opts, "plan comments delete", func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Delete(ctx, "/api/comments/"+strconv.Itoa(commentID))
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&commentID, "comment-id", 0, "Comment id")
	return cmd
}

func newNovelPlanCommentsVoteCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var commentID int
	var direction string
	cmd := &cobra.Command{Use: "vote", Short: "Vote on a Wanderlog comment", Example: "  wanderlog-pp-cli plan comments vote --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --comment-id 1 --direction up --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if commentID == 0 {
			return usageErr(errors.New("--comment-id is required"))
		}
		direction = strings.ToLower(strings.TrimSpace(direction))
		if direction != "up" && direction != "down" && direction != "none" {
			return usageErr(errors.New("--direction must be up, down, or none"))
		}
		return runPlanCommentWrite(cmd, flags, opts, "plan comments vote", func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Post(ctx, "/api/comments/"+strconv.Itoa(commentID)+"/vote/"+direction, nil)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&commentID, "comment-id", 0, "Comment id")
	cmd.Flags().StringVar(&direction, "direction", "up", "Vote direction: up, down, or none")
	return cmd
}

func runPlanCommentWrite(cmd *cobra.Command, flags *rootFlags, opts planEditOptions, commandName string, call func(context.Context, *client.Client, string) (json.RawMessage, int, error)) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	if err := requireCookie(c); err != nil {
		return authErr(err)
	}
	key, err := resolveEditablePlanKey(opts)
	if err != nil {
		return usageErr(err)
	}
	if flags.dryRun {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": commandName, "target_key": key, "dry_run": true, "applied": false}, flags)
	}
	data, status, err := call(ctx, c, key)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": commandName, "target_key": key, "status": status, "applied": true, "results": json.RawMessage(data)}, flags)
}

func newNovelPlanCollaboratorsCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{Use: "collaborators", Short: "Inspect collaborator, permission, and share-key metadata on a Wanderlog plan (read-only; the add/remove/invite/share-key subcommands do the writing)", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
		if err != nil {
			return err
		}
		fields := map[string]any{}
		for _, k := range []string{"ownerId", "userId", "userIds", "users", "collaborators", "members", "permissions", "privacy", "editKey", "viewKey", "shareKey", "journalKey", "tripPlanUsers", "TripPlanUsers"} {
			if v, ok := trip[k]; ok {
				fields[k] = v
			}
		}
		warnings := []string{}
		if len(fields) == 0 {
			warnings = append(warnings, "no collaborator fields were present in the fetched tripPlan payload")
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan collaborators", "target_key": key, "fields": fields, "warnings": warnings}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	cmd.AddCommand(newNovelPlanCollaboratorsInvitesCmd(flags))
	cmd.AddCommand(newNovelPlanCollaboratorsInviteCmd(flags))
	cmd.AddCommand(newNovelPlanCollaboratorsAddCmd(flags))
	cmd.AddCommand(newNovelPlanCollaboratorsRemoveCmd(flags))
	cmd.AddCommand(newNovelPlanCollaboratorsShareKeyCmd(flags))
	return cmd
}

func newNovelPlanCollaboratorsInvitesCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	cmd := &cobra.Command{Use: "invites", Short: "List pending Wanderlog trip invites", Example: "  wanderlog-pp-cli plan collaborators invites --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		data, err := c.GetNoCache(ctx, "/api/tripPlans/"+key+"/invites", nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan collaborators invites", "target_key": key, "results": json.RawMessage(data)}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	return cmd
}

func newNovelPlanCollaboratorsInviteCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var emails []string
	var userIDs []int
	var message string
	var inviteesJSON string
	cmd := &cobra.Command{Use: "invite", Short: "Send Wanderlog trip invites by email or user id", Example: "  wanderlog-pp-cli plan collaborators invite --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --email example@example.com --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		invitees, err := collaboratorInvitees(emails, userIDs, inviteesJSON)
		if err != nil {
			return usageErr(err)
		}
		body := map[string]any{"invitees": invitees, "message": message}
		return runPlanCollaboratorWrite(cmd, flags, opts, "plan collaborators invite", body, func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Post(ctx, "/api/tripPlans/"+key+"/invite", body)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringArrayVar(&emails, "email", nil, "Email invitee; repeatable")
	cmd.Flags().IntSliceVar(&userIDs, "user-id", nil, "Wanderlog user id invitee; repeatable")
	cmd.Flags().StringVar(&message, "message", "", "Optional invite message")
	cmd.Flags().StringVar(&inviteesJSON, "invitees-json", "", "Exact invitees JSON array; overrides --email and --user-id")
	return cmd
}

func newNovelPlanCollaboratorsAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var userID int
	cmd := &cobra.Command{Use: "add", Short: "Add a Wanderlog collaborator by user id", Example: "  wanderlog-pp-cli plan collaborators add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --user-id 1 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if userID == 0 {
			return usageErr(errors.New("--user-id is required"))
		}
		body := map[string]any{"userId": userID}
		return runPlanCollaboratorWrite(cmd, flags, opts, "plan collaborators add", body, func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Post(ctx, "/api/tripPlans/"+key+"/collaborator", body)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&userID, "user-id", 0, "Wanderlog user id")
	return cmd
}

func newNovelPlanCollaboratorsRemoveCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var userID int
	cmd := &cobra.Command{Use: "remove", Short: "Remove a Wanderlog collaborator by user id", Example: "  wanderlog-pp-cli plan collaborators remove --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --user-id 1 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if userID == 0 {
			return usageErr(errors.New("--user-id is required"))
		}
		body := map[string]any{"userId": userID}
		return runPlanCollaboratorWrite(cmd, flags, opts, "plan collaborators remove", body, func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.DeleteWithBody(ctx, "/api/tripPlans/"+key+"/collaborator", body)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&userID, "user-id", 0, "Wanderlog user id")
	return cmd
}

func newNovelPlanCollaboratorsShareKeyCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	var permissions string
	var permissionsJSON string
	cmd := &cobra.Command{Use: "share-key", Short: "Create or refresh a Wanderlog share key with permissions", Example: "  wanderlog-pp-cli plan collaborators share-key --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --permissions view --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		value, err := collaboratorPermissions(permissions, permissionsJSON)
		if err != nil {
			return usageErr(err)
		}
		body := map[string]any{"permissions": value}
		return runPlanCollaboratorWrite(cmd, flags, opts, "plan collaborators share-key", body, func(ctx context.Context, c *client.Client, key string) (json.RawMessage, int, error) {
			return c.Post(ctx, "/api/tripPlans/"+key+"/shareKey", body)
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&permissions, "permissions", "", "Share permissions value observed from Wanderlog, for example view, edit, or suggest")
	cmd.Flags().StringVar(&permissionsJSON, "permissions-json", "", "Exact JSON permissions value; overrides --permissions")
	return cmd
}

func collaboratorInvitees(emails []string, userIDs []int, inviteesJSON string) ([]any, error) {
	if strings.TrimSpace(inviteesJSON) != "" {
		var parsed []any
		if err := json.Unmarshal([]byte(inviteesJSON), &parsed); err != nil {
			return nil, fmt.Errorf("parse --invitees-json: %w", err)
		}
		if len(parsed) == 0 {
			return nil, errors.New("--invitees-json must contain at least one invitee")
		}
		return parsed, nil
	}
	invitees := []any{}
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		invitees = append(invitees, map[string]any{"type": "email", "email": email})
	}
	for _, userID := range userIDs {
		if userID <= 0 {
			return nil, errors.New("--user-id must be positive")
		}
		invitees = append(invitees, map[string]any{"type": "user", "id": userID})
	}
	if len(invitees) == 0 {
		return nil, errors.New("pass at least one --email, --user-id, or --invitees-json")
	}
	return invitees, nil
}

func collaboratorPermissions(permissions string, permissionsJSON string) (any, error) {
	if strings.TrimSpace(permissionsJSON) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(permissionsJSON), &parsed); err != nil {
			return nil, fmt.Errorf("parse --permissions-json: %w", err)
		}
		return parsed, nil
	}
	permissions = strings.TrimSpace(permissions)
	if permissions == "" {
		return nil, errors.New("--permissions or --permissions-json is required")
	}
	return permissions, nil
}

func runPlanCollaboratorWrite(cmd *cobra.Command, flags *rootFlags, opts planEditOptions, commandName string, body any, call func(context.Context, *client.Client, string) (json.RawMessage, int, error)) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	if err := requireCookie(c); err != nil {
		return authErr(err)
	}
	key, err := resolveEditablePlanKey(opts)
	if err != nil {
		return usageErr(err)
	}
	if flags.dryRun {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": commandName, "target_key": key, "dry_run": true, "applied": false, "body": body}, flags)
	}
	data, status, err := call(ctx, c, key)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": commandName, "target_key": key, "status": status, "applied": true, "results": json.RawMessage(data)}, flags)
}

func newNovelPlanRouteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "route", Short: "Build or send Wanderlog route optimization requests", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanRouteDayBodyCmd(flags))
	cmd.AddCommand(newNovelPlanRouteOptimizeCmd(flags))
	return cmd
}

func newNovelPlanRouteDayBodyCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var mode string
	cmd := &cobra.Command{Use: "day-body", Short: "Build a route optimization JSON body from one itinerary day", Example: "  wanderlog-pp-cli plan route day-body --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
		if err != nil {
			return err
		}
		sec, err := resolveSection(trip, opts.day, opts.sectionIndex, opts.sectionID)
		if err != nil {
			return usageErr(err)
		}
		body := routeBodyForSection(sec, mode)
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan route day-body", "target_key": key, "section": sec.Report, "body": body}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&mode, "travel-mode", "DRIVING", "Travel mode hint for optimizeRoute")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanRouteOptimizeCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var mode, bodyJSON string
	cmd := &cobra.Command{Use: "optimize", Short: "Call Wanderlog route optimization", Example: "  wanderlog-pp-cli plan route optimize --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		if err := requireCookie(c); err != nil {
			return authErr(err)
		}
		var body any
		if bodyJSON != "" {
			var parsed any
			if err := json.Unmarshal([]byte(bodyJSON), &parsed); err != nil {
				return usageErr(fmt.Errorf("parse --body-json: %w", err))
			}
			body = parsed
		} else {
			trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
			if err != nil {
				return err
			}
			sec, err := resolveSection(trip, opts.day, opts.sectionIndex, opts.sectionID)
			if err != nil {
				return usageErr(err)
			}
			body = routeBodyForSection(sec, mode)
		}
		if flags.dryRun {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan route optimize", "target_key": key, "dry_run": true, "body": body}, flags)
		}
		data, status, err := c.Post(ctx, "/api/directions/optimizeRoute", body)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan route optimize", "target_key": key, "status": status, "results": json.RawMessage(data)}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&mode, "travel-mode", "DRIVING", "Travel mode hint when building body from a day")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Exact optimizeRoute JSON body; overrides --day body builder")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func validClock(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	return err1 == nil && err2 == nil && h >= 0 && h <= 23 && m >= 0 && m <= 59
}
func findObjectIndexByID(items []any, id int) int {
	for i, raw := range items {
		m, _ := raw.(map[string]any)
		if intAny(m["id"]) == id {
			return i
		}
	}
	return -1
}
func newChecklistItem(text string, checked bool) map[string]any {
	return map[string]any{"id": randomWanderlogID(), "text": checklistRichText(text), "checked": checked}
}
func checklistRichText(text string) map[string]any {
	return map[string]any{"ops": []any{map[string]any{"insert": text}}}
}
func newChecklistBlock(title string, items []string) map[string]any {
	out := map[string]any{"type": "checklist", "id": randomWanderlogID(), "title": title, "items": []any{}, "addedBy": map[string]any{"type": "user"}}
	arr := []any{}
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			arr = append(arr, newChecklistItem(item, false))
		}
	}
	out["items"] = arr
	return out
}
func routeBodyForSection(sec resolvedSection, mode string) map[string]any {
	places := []any{}
	for _, raw := range sec.Blocks {
		block, _ := raw.(map[string]any)
		if stringField(block, "type") != "place" {
			continue
		}
		place := mapField(block, "place")
		geometry := mapField(place, "geometry")
		loc := mapField(geometry, "location")
		id := firstNonEmpty(stringField(place, "placeId"), stringField(place, "place_id"), stringField(block, "placeId"))
		stop := map[string]any{"id": id, "blockId": intAny(block["id"])}
		if loc != nil {
			stop["latitude"] = loc["lat"]
			stop["longitude"] = loc["lng"]
		}
		places = append(places, stop)
	}
	return map[string]any{"places": places, "travelMode": firstNonEmpty(mode, "DRIVING")}
}
