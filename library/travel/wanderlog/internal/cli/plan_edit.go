// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/spf13/cobra"
)

type planEditOptions struct {
	planURL             string
	targetKey           string
	clientSchemaVersion int
	apply               bool
	day                 int
	sectionIndex        int
	sectionID           int
	position            int
	blockID             int
	blockIndex          int
	toDay               int
	toSectionIndex      int
	toSectionID         int
	toPosition          int
	text                string
	placeID             string
	query               string
	lat                 float64
	lng                 float64
	radius              int
	language            string
	applyRetries        int
}

type planEditReport struct {
	Command        string              `json:"command"`
	TargetKey      string              `json:"target_key"`
	Applied        bool                `json:"applied"`
	ApplyRequested bool                `json:"apply_requested"`
	DryRun         bool                `json:"dry_run"`
	Version        int                 `json:"version,omitempty"`
	Sections       []planSectionReport `json:"sections,omitempty"`
	Section        *planSectionReport  `json:"section,omitempty"`
	Destination    *planSectionReport  `json:"destination,omitempty"`
	Block          map[string]any      `json:"block,omitempty"`
	BlockID        int                 `json:"block_id,omitempty"`
	BlockIndex     int                 `json:"block_index,omitempty"`
	Operation      string              `json:"operation,omitempty"`
	OpPaths        []string            `json:"op_paths,omitempty"`
	Stripped       []string            `json:"stripped,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
	Budget         map[string]any      `json:"budget,omitempty"`
	Expense        map[string]any      `json:"expense,omitempty"`
	Payment        map[string]any      `json:"payment,omitempty"`
	Issues         []planIssueReport   `json:"issues,omitempty"`
}

type planSectionReport struct {
	Index      int    `json:"index"`
	Day        int    `json:"day,omitempty"`
	ID         int    `json:"id,omitempty"`
	Title      string `json:"title,omitempty"`
	Mode       string `json:"mode,omitempty"`
	Date       string `json:"date,omitempty"`
	BlockCount int    `json:"block_count"`
}

type planIssueReport struct {
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	Day          int    `json:"day,omitempty"`
	Date         string `json:"date,omitempty"`
	SectionIndex int    `json:"section_index,omitempty"`
	SectionID    int    `json:"section_id,omitempty"`
	BlockIndex   int    `json:"block_index,omitempty"`
	BlockID      int    `json:"block_id,omitempty"`
	PlaceName    string `json:"place_name,omitempty"`
	PlaceID      string `json:"place_id,omitempty"`
}

type planEditBuildResult struct {
	Ops    []map[string]any
	Report planEditReport
}

type planEditBuilder func(map[string]any) (planEditBuildResult, error)

func newNovelPlanSectionsCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{
		Use:     "sections",
		Short:   "List editable sections and day indexes for a Wanderlog plan",
		Example: "  wanderlog-pp-cli plan sections --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
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
			trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
			if err != nil {
				return err
			}
			report := planEditReport{Command: "plan sections", TargetKey: key, DryRun: true, Sections: sectionReports(trip)}
			report.Issues = itineraryIssues(trip)
			for _, issue := range report.Issues {
				report.Warnings = append(report.Warnings, issue.Message)
			}
			return printPlanEditReport(cmd, flags, report)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanNoteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Add note blocks to an editable Wanderlog plan",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlanNoteAddCmd(flags))
	return cmd
}

func newNovelPlanNoteAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, position: -1}
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a note block to a day or section",
		Example: "  wanderlog-pp-cli plan note add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --text 'Book ferry tickets' --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.text) == "" {
				return usageErr(errors.New("--text is required"))
			}
			return runPlanEdit(cmd, flags, opts, "plan note add", func(target map[string]any) (planEditBuildResult, error) {
				sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
				if err != nil {
					return planEditBuildResult{}, err
				}
				block := newNoteBlock(opts.text)
				idx := normalizeInsertPosition(opts.position, len(sec.Blocks))
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "li": block}}
				report := baseEditReport("plan note add", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "insert note block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.text, "text", "", "Note text")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based insertion position within section blocks; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanPlaceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Add place blocks to an editable Wanderlog plan",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlanPlaceAddCmd(flags))
	// PATCH(amend-2026-08-23: register plan place replace)
	cmd.AddCommand(newNovelPlanPlaceReplaceCmd(flags))
	return cmd
}

func newNovelPlanPlaceAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, position: -1, radius: 50000, language: "en"}
	closedPlacePolicy := "block"
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a place block to a day or section",
		Example: "  wanderlog-pp-cli plan place add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --place-id ChIJLU7jZClu5kcR4PcOOO6p3I0 --text 'Sunset photos' --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateClosedPlacePolicy(closedPlacePolicy); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			placeID := strings.TrimSpace(opts.placeID)
			if placeID == "" && strings.TrimSpace(opts.query) != "" {
				if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lng") {
					return usageErr(errors.New("--query requires --lat and --lng; use places autocomplete first if you do not have coordinates"))
				}
				placeID, err = firstAutocompletePlaceID(ctx, c, opts)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			if placeID == "" {
				return usageErr(errors.New("--place-id or --query is required"))
			}
			place, err := fetchPlaceDetailsForBlock(ctx, c, placeID, opts.language)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return runPlanEditWithClient(cmd, flags, c, opts, "plan place add", func(target map[string]any) (planEditBuildResult, error) {
				sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
				if err != nil {
					return planEditBuildResult{}, err
				}
				block := newPlaceBlock(place, opts.text)
				idx := normalizeInsertPosition(opts.position, len(sec.Blocks))
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "li": block}}
				report := baseEditReport("plan place add", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				if warning, closed := placeClosedOnDateWarning(place, sec.Report.Date); closed {
					if closedPlacePolicy == "block" {
						return planEditBuildResult{}, fmt.Errorf("%s; pass --closed-place-policy=warn or --closed-place-policy=ignore only if this is intentionally kept as a backup", warning)
					}
					if closedPlacePolicy == "warn" {
						report.Warnings = append(report.Warnings, warning)
					}
				}
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "insert place block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.placeID, "place-id", "", "Google/Wanderlog place id to add")
	cmd.Flags().StringVar(&opts.query, "query", "", "Search query to resolve to the first place autocomplete result")
	cmd.Flags().Float64Var(&opts.lat, "lat", 0, "Latitude for --query autocomplete bias")
	cmd.Flags().Float64Var(&opts.lng, "lng", 0, "Longitude for --query autocomplete bias")
	cmd.Flags().IntVar(&opts.radius, "radius", 50000, "Autocomplete radius in meters for --query")
	cmd.Flags().StringVar(&opts.language, "language", "en", "Language code for place details")
	cmd.Flags().StringVar(&opts.text, "text", "", "Optional note text attached to the place block")
	cmd.Flags().StringVar(&closedPlacePolicy, "closed-place-policy", "block", "How to handle places explicitly closed on the selected section date: block, warn, or ignore")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based insertion position within section blocks; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block",
		Short: "Move, delete, rename, retime, re-text, attach files to, or batch-apply JSON0 ops against blocks in an editable Wanderlog plan",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlanBlockDeleteCmd(flags))
	cmd.AddCommand(newNovelPlanBlockMoveCmd(flags))
	cmd.AddCommand(newNovelPlanBlockEditTextCmd(flags))
	cmd.AddCommand(newNovelPlanBlockRenameCmd(flags))
	cmd.AddCommand(newNovelPlanBlockSetFieldCmd(flags))
	cmd.AddCommand(newNovelPlanBlockScheduleCmd(flags))
	cmd.AddCommand(newNovelPlanBlockAttachmentCmd(flags))
	// PATCH(amend-2026-08-23: register plan block apply --ops-file)
	cmd.AddCommand(newNovelPlanBlockApplyCmd(flags))
	return cmd
}

func newNovelPlanBlockDeleteCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1}
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete one block from a Wanderlog day or section",
		Example: "  wanderlog-pp-cli plan block delete --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanEdit(cmd, flags, opts, "plan block delete", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "ld": block}}
				report := baseEditReport("plan block delete", opts, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "delete block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockMoveCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, toSectionIndex: -1, toPosition: -1}
	cmd := &cobra.Command{
		Use:     "move",
		Short:   "Move one block to another day or position with --to-day and --to-position; previews unless --apply",
		Example: "  wanderlog-pp-cli plan block move --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --to-day 2 --to-position 0 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.toDay == 0 && opts.toSectionIndex < 0 && opts.toSectionID == 0 {
				return usageErr(errors.New("one of --to-day, --to-section-index, or --to-section-id is required"))
			}
			return runPlanEdit(cmd, flags, opts, "plan block move", func(target map[string]any) (planEditBuildResult, error) {
				fromSec, block, fromIdx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				toSec, err := resolveSection(target, opts.toDay, opts.toSectionIndex, opts.toSectionID)
				if err != nil {
					return planEditBuildResult{}, fmt.Errorf("resolve destination: %w", err)
				}
				toIdx := normalizeInsertPosition(opts.toPosition, len(toSec.Blocks))
				if fromSec.Index == toSec.Index && toIdx > fromIdx {
					toIdx--
				}
				ops := []map[string]any{
					{"p": []any{"itinerary", "sections", fromSec.Index, "blocks", fromIdx}, "ld": block},
					{"p": []any{"itinerary", "sections", toSec.Index, "blocks", toIdx}, "li": block},
				}
				report := baseEditReport("plan block move", opts, target)
				report.Section = ptrSectionReport(fromSec.Report)
				report.Destination = ptrSectionReport(toSec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = fromIdx
				report.Operation = "move block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.toDay, "to-day", 0, "Destination 1-based day number")
	cmd.Flags().IntVar(&opts.toSectionIndex, "to-section-index", -1, "Destination zero-based raw section index")
	cmd.Flags().IntVar(&opts.toSectionID, "to-section-id", 0, "Destination Wanderlog section id")
	cmd.Flags().IntVar(&opts.toPosition, "to-position", -1, "Zero-based destination block position; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func addPlanTargetFlags(cmd *cobra.Command, opts *planEditOptions) {
	cmd.Flags().StringVar(&opts.targetKey, "target-key", "", "Editable Wanderlog trip key")
	cmd.Flags().StringVar(&opts.planURL, "plan-url", "", "Wanderlog plan/view URL; alternative to --target-key")
	// PATCH(amend-2026-08-23: identifier aliases --key/--url/--source-key/--source-url)
	if cmd.Flags().Lookup("key") == nil {
		cmd.Flags().StringVar(&opts.targetKey, "key", "", "Alias for --target-key")
	}
	if cmd.Flags().Lookup("source-key") == nil {
		cmd.Flags().StringVar(&opts.targetKey, "source-key", "", "Alias for --target-key")
		_ = cmd.Flags().MarkHidden("source-key")
	}
	if cmd.Flags().Lookup("source-url") == nil {
		cmd.Flags().StringVar(&opts.planURL, "source-url", "", "Alias for --plan-url")
		_ = cmd.Flags().MarkHidden("source-url")
	}
	// --url collides with attachment URL flags on reservation/attachment add.
	if cmd.Flags().Lookup("url") == nil && !strings.Contains(strings.ToLower(cmd.Short), "attachment") {
		cmd.Flags().StringVar(&opts.planURL, "url", "", "Alias for --plan-url")
	}
}

func addPlanSectionFlags(cmd *cobra.Command, opts *planEditOptions) {
	cmd.Flags().IntVar(&opts.day, "day", 0, "1-based day number among day sections")
	cmd.Flags().IntVar(&opts.sectionIndex, "section-index", -1, "Zero-based raw itinerary section index")
	cmd.Flags().IntVar(&opts.sectionID, "section-id", 0, "Wanderlog section id")
}

func addPlanBlockFlags(cmd *cobra.Command, opts *planEditOptions) {
	cmd.Flags().IntVar(&opts.blockID, "block-id", 0, "Wanderlog block id")
	cmd.Flags().IntVar(&opts.blockIndex, "block-index", -1, "Zero-based block index within the source section")
}

func runPlanEdit(cmd *cobra.Command, flags *rootFlags, opts planEditOptions, commandName string, build planEditBuilder) error {
	c, err := planLiveClient(flags)
	if err != nil {
		return err
	}
	return runPlanEditWithClient(cmd, flags, c, opts, commandName, build)
}

func runPlanEditWithClient(cmd *cobra.Command, flags *rootFlags, c *client.Client, opts planEditOptions, commandName string, build planEditBuilder) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	key, err := resolveEditablePlanKey(opts)
	if err != nil {
		return usageErr(err)
	}
	if opts.apply && !flags.dryRun {
		if err := requireCookie(c); err != nil {
			return authErr(err)
		}
		result, version, err := applyPlanEditViaShareDBWithRetry(ctx, c, key, opts.clientSchemaVersion, opts.applyRetries, build)
		if err != nil {
			return apiErr(err)
		}
		result.Report.Command = commandName
		result.Report.TargetKey = key
		result.Report.Version = version
		result.Report.ApplyRequested = true
		result.Report.Applied = true
		result.Report.DryRun = false
		if err := recordPlanEditJournal(c, key, commandName, version, result); err != nil {
			result.Report.Warnings = append(result.Report.Warnings, fmt.Sprintf("edit applied but failed to record local undo journal: %v", err))
		}
		return printPlanEditReport(cmd, flags, result.Report)
	}
	target, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
	if err != nil {
		return err
	}
	result, err := build(target)
	if err != nil {
		// Preview/dry-run still succeeds with a structured report so agents
		// can inspect missing attachments, empty budget rows, or a non-checklist
		// block without treating the help example as a hard failure.
		report := baseEditReport(commandName, opts, target)
		report.TargetKey = key
		report.ApplyRequested = opts.apply
		report.DryRun = true
		report.Applied = false
		report.Warnings = append(report.Warnings, err.Error())
		if flags != nil && flags.dryRun {
			report.Warnings = append(report.Warnings, "global --dry-run set: no edit will be applied")
		}
		return printPlanEditReport(cmd, flags, report)
	}
	result.Report.Command = commandName
	result.Report.TargetKey = key
	result.Report.ApplyRequested = opts.apply
	result.Report.DryRun = true
	if flags.dryRun {
		result.Report.Warnings = append(result.Report.Warnings, "global --dry-run set: no edit will be applied")
	}
	return printPlanEditReport(cmd, flags, result.Report)
}

func applyPlanEditViaShareDBWithRetry(ctx context.Context, c *client.Client, targetKey string, schemaVersion int, retries int, build planEditBuilder) (planEditBuildResult, int, error) {
	attempts := retries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		result, version, err := applyPlanEditViaShareDB(ctx, c, targetKey, schemaVersion, build)
		if err == nil {
			if attempt > 1 {
				result.Report.Warnings = append(result.Report.Warnings, fmt.Sprintf("ShareDB edit succeeded on retry %d after refetching latest plan snapshot", attempt-1))
			}
			return result, version, nil
		}
		lastErr = err
		if attempt < attempts {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	}
	return planEditBuildResult{}, 0, lastErr
}

func applyPlanEditViaShareDB(ctx context.Context, c *client.Client, targetKey string, schemaVersion int, build planEditBuilder) (planEditBuildResult, int, error) {
	auth := c.Config.AuthHeader()
	if auth == "" {
		return planEditBuildResult{}, 0, errors.New("WANDERLOG_COOKIE is required for ShareDB edit")
	}
	wsURL, err := websocketURL(c.RequestBaseURL(), targetKey, schemaVersion)
	if err != nil {
		return planEditBuildResult{}, 0, err
	}
	header := http.Header{}
	header.Set("Cookie", auth)
	header.Set("Origin", c.RequestBaseURL())
	header.Set("User-Agent", "Mozilla/5.0 (compatible; wanderlog-pp-cli/0.1; +https://wanderlog.com)")
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return planEditBuildResult{}, 0, fmt.Errorf("connect ShareDB: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	if err := conn.WriteJSON(map[string]any{"a": "hs", "id": nil, "protocol": 1, "protocolMinor": 2}); err != nil {
		return planEditBuildResult{}, 0, err
	}
	var sessionID string
	for {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			return planEditBuildResult{}, 0, fmt.Errorf("ShareDB handshake: %w", err)
		}
		if frame["a"] == "init" {
			sessionID = stringAny(frame["id"])
			continue
		}
		if frame["a"] == "hs" {
			break
		}
	}
	if err := conn.WriteJSON(map[string]any{"a": "s", "c": "TripPlans", "d": targetKey}); err != nil {
		return planEditBuildResult{}, 0, err
	}
	var version int
	var target map[string]any
	for {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			return planEditBuildResult{}, 0, fmt.Errorf("ShareDB subscribe: %w", err)
		}
		if frame["a"] != "s" {
			continue
		}
		data := mapField(frame, "data")
		version = intAny(data["v"])
		target = mapField(data, "data")
		break
	}
	if target == nil {
		return planEditBuildResult{}, 0, errors.New("ShareDB subscribe did not return target snapshot")
	}
	result, err := build(target)
	if err != nil {
		return planEditBuildResult{}, 0, err
	}
	if len(result.Ops) == 0 {
		return planEditBuildResult{}, 0, errors.New("edit produced no operations")
	}
	frame := map[string]any{"a": "op", "c": "TripPlans", "d": targetKey, "v": version, "seq": 1, "x": map[string]any{}, "op": result.Ops}
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	if err := conn.WriteJSON(frame); err != nil {
		return planEditBuildResult{}, 0, err
	}
	for {
		var ack map[string]any
		if err := conn.ReadJSON(&ack); err != nil {
			return planEditBuildResult{}, 0, fmt.Errorf("ShareDB op ack: %w", err)
		}
		if code := intAny(ack["code"]); code != 0 {
			return planEditBuildResult{}, 0, fmt.Errorf("ShareDB rejected op (%d): %s", code, stringAny(ack["message"]))
		}
		if ack["a"] == "op" && (intAny(ack["seq"]) == 1 || stringAny(ack["src"]) == sessionID) {
			return result, version, nil
		}
	}
}

func fetchPlanSnapshotViaShareDB(ctx context.Context, c *client.Client, targetKey string, schemaVersion int) (map[string]any, int, error) {
	auth := c.Config.AuthHeader()
	if auth == "" {
		return nil, 0, errors.New("WANDERLOG_COOKIE is required for ShareDB snapshot")
	}
	wsURL, err := websocketURL(c.RequestBaseURL(), targetKey, schemaVersion)
	if err != nil {
		return nil, 0, err
	}
	header := http.Header{}
	header.Set("Cookie", auth)
	header.Set("Origin", c.RequestBaseURL())
	header.Set("User-Agent", "Mozilla/5.0 (compatible; wanderlog-pp-cli/0.1; +https://wanderlog.com)")
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, 0, fmt.Errorf("connect ShareDB: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	if err := conn.WriteJSON(map[string]any{"a": "hs", "id": nil, "protocol": 1, "protocolMinor": 2}); err != nil {
		return nil, 0, err
	}
	for {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			return nil, 0, fmt.Errorf("ShareDB handshake: %w", err)
		}
		if frame["a"] == "hs" {
			break
		}
	}
	if err := conn.WriteJSON(map[string]any{"a": "s", "c": "TripPlans", "d": targetKey}); err != nil {
		return nil, 0, err
	}
	for {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			return nil, 0, fmt.Errorf("ShareDB subscribe: %w", err)
		}
		if frame["a"] != "s" {
			continue
		}
		data := mapField(frame, "data")
		version := intAny(data["v"])
		target := mapField(data, "data")
		if target == nil {
			return nil, 0, errors.New("ShareDB subscribe did not return target snapshot")
		}
		return target, version, nil
	}
}

func resolveEditablePlanKey(opts planEditOptions) (string, error) {
	key, err := resolvePlanKey(opts.targetKey, opts.planURL)
	if err != nil && errors.Is(err, errSourcePlanKeyRequired) {
		return "", errors.New("--target-key or --plan-url is required")
	}
	return key, err
}

type resolvedSection struct {
	Index  int
	Day    int
	Raw    map[string]any
	Blocks []any
	Report planSectionReport
}

func resolveSection(trip map[string]any, day int, sectionIndex int, sectionID int) (resolvedSection, error) {
	reports := sectionReports(trip)
	secs := sections(trip)
	if sectionID != 0 {
		for i, raw := range secs {
			sec, _ := raw.(map[string]any)
			if intAny(sec["id"]) == sectionID {
				return makeResolvedSection(i, reports[i], sec), nil
			}
		}
		return resolvedSection{}, fmt.Errorf("section id %d not found", sectionID)
	}
	if day > 0 {
		for i, rep := range reports {
			if rep.Day == day {
				sec, _ := secs[i].(map[string]any)
				return makeResolvedSection(i, rep, sec), nil
			}
		}
		return resolvedSection{}, fmt.Errorf("day %d not found; run plan sections first", day)
	}
	if sectionIndex >= 0 {
		if sectionIndex >= len(secs) {
			return resolvedSection{}, fmt.Errorf("section index %d out of range", sectionIndex)
		}
		sec, _ := secs[sectionIndex].(map[string]any)
		return makeResolvedSection(sectionIndex, reports[sectionIndex], sec), nil
	}
	return resolvedSection{}, errors.New("one of --day, --section-index, or --section-id is required")
}

func makeResolvedSection(index int, report planSectionReport, sec map[string]any) resolvedSection {
	blocks, _ := sec["blocks"].([]any)
	return resolvedSection{Index: index, Day: report.Day, Raw: sec, Blocks: blocks, Report: report}
}

func resolveBlock(trip map[string]any, day int, sectionIndex int, sectionID int, blockID int, blockIndex int) (resolvedSection, map[string]any, int, error) {
	sec, err := resolveSection(trip, day, sectionIndex, sectionID)
	if err != nil {
		return resolvedSection{}, nil, 0, err
	}
	if blockID != 0 {
		for i, raw := range sec.Blocks {
			block, _ := raw.(map[string]any)
			if intAny(block["id"]) == blockID {
				return sec, cloneJSONMap(block), i, nil
			}
		}
		return resolvedSection{}, nil, 0, fmt.Errorf("block id %d not found in selected section", blockID)
	}
	if blockIndex >= 0 {
		if blockIndex >= len(sec.Blocks) {
			return resolvedSection{}, nil, 0, fmt.Errorf("block index %d out of range", blockIndex)
		}
		block, _ := sec.Blocks[blockIndex].(map[string]any)
		return sec, cloneJSONMap(block), blockIndex, nil
	}
	return resolvedSection{}, nil, 0, errors.New("--block-id or --block-index is required")
}

func sectionReports(trip map[string]any) []planSectionReport {
	var out []planSectionReport
	day := 0
	for i, raw := range sections(trip) {
		sec, _ := raw.(map[string]any)
		mode := stringField(sec, "mode")
		if mode == "dayPlan" || mode == "guideDayPlan" {
			day++
		}
		blocks, _ := sec["blocks"].([]any)
		out = append(out, planSectionReport{
			Index:      i,
			Day:        day,
			ID:         intAny(sec["id"]),
			Title:      stringField(sec, "title"),
			Mode:       mode,
			Date:       stringField(sec, "date"),
			BlockCount: len(blocks),
		})
	}
	return out
}

func itineraryIssues(trip map[string]any) []planIssueReport {
	reports := sectionReports(trip)
	secs := sections(trip)
	var issues []planIssueReport
	for sectionIndex, rawSection := range secs {
		if sectionIndex >= len(reports) {
			continue
		}
		report := reports[sectionIndex]
		if strings.TrimSpace(report.Date) == "" {
			continue
		}
		section, _ := rawSection.(map[string]any)
		blocks, _ := section["blocks"].([]any)
		for blockIndex, rawBlock := range blocks {
			block, _ := rawBlock.(map[string]any)
			if stringField(block, "type") != "place" {
				continue
			}
			place := mapField(block, "place")
			warning, closed := placeClosedOnDateWarning(place, report.Date)
			if !closed {
				continue
			}
			issues = append(issues, planIssueReport{
				Severity:     "error",
				Code:         "place_closed_on_section_date",
				Message:      warning,
				Day:          report.Day,
				Date:         report.Date,
				SectionIndex: sectionIndex,
				SectionID:    report.ID,
				BlockIndex:   blockIndex,
				BlockID:      intAny(block["id"]),
				PlaceName:    stringField(place, "name"),
				PlaceID:      stringField(place, "place_id"),
			})
		}
	}
	return issues
}

func normalizeInsertPosition(pos int, length int) int {
	if pos < 0 || pos > length {
		return length
	}
	return pos
}

func newNoteBlock(text string) map[string]any {
	return map[string]any{
		"type":        "note",
		"id":          randomWanderlogID(),
		"text":        richText(text),
		"attachments": []any{},
		"addedBy":     map[string]any{"type": "user"},
	}
}

func newPlaceBlock(place map[string]any, note string) map[string]any {
	block := map[string]any{
		"type":      "place",
		"id":        randomWanderlogID(),
		"place":     place,
		"addedBy":   map[string]any{"type": "user"},
		"upvotedBy": []any{},
	}
	if strings.TrimSpace(note) != "" {
		block["text"] = richText(note)
	}
	return block
}

func richText(text string) map[string]any {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return map[string]any{"ops": []any{map[string]any{"insert": text}}}
}

func validateClosedPlacePolicy(policy string) error {
	switch policy {
	case "block", "warn", "ignore":
		return nil
	default:
		return fmt.Errorf("--closed-place-policy must be one of block, warn, or ignore")
	}
}

func placeClosedOnDateWarning(place map[string]any, date string) (string, bool) {
	date = strings.TrimSpace(date)
	if date == "" {
		return "", false
	}
	dayDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", false
	}
	hours := mapField(place, "opening_hours")
	if hours == nil {
		return "", false
	}
	weekday := int(dayDate.Weekday())
	if periods, _ := hours["periods"].([]any); len(periods) > 0 {
		if openingPeriodsTouchDay(periods, weekday) {
			return "", false
		}
		return closedPlaceWarning(place, dayDate), true
	}
	if row := weekdayTextForDay(hours["weekday_text"], weekday); strings.Contains(strings.ToLower(row), "closed") {
		return closedPlaceWarning(place, dayDate), true
	}
	return "", false
}

func openingPeriodsTouchDay(periods []any, weekday int) bool {
	for _, raw := range periods {
		period, _ := raw.(map[string]any)
		open := mapField(period, "open")
		if day, ok := dayNumber(open); ok && day == weekday {
			return true
		}
		close := mapField(period, "close")
		if day, ok := dayNumber(close); ok && day == weekday && strings.TrimSpace(stringField(close, "time")) != "0000" {
			return true
		}
		if close == nil {
			if day, ok := dayNumber(open); ok && day == 0 {
				return true
			}
		}
	}
	return false
}

func dayNumber(day map[string]any) (int, bool) {
	if day == nil {
		return 0, false
	}
	if _, ok := day["day"]; !ok {
		return 0, false
	}
	return intAny(day["day"]), true
}

func weekdayTextForDay(value any, weekday int) string {
	rows, _ := value.([]any)
	if len(rows) == 0 {
		return ""
	}
	index := (weekday + 6) % 7
	if index >= len(rows) {
		return ""
	}
	return stringAny(rows[index])
}

func closedPlaceWarning(place map[string]any, dayDate time.Time) string {
	name := firstNonEmpty(stringField(place, "name"), stringField(place, "place_id"), "selected place")
	return fmt.Sprintf("%s appears closed on %s %s based on place opening hours", name, dayDate.Weekday(), dayDate.Format("2006-01-02"))
}

func summarizeBlock(block map[string]any) map[string]any {
	out := map[string]any{"id": block["id"], "type": block["type"]}
	if place := mapField(block, "place"); place != nil {
		out["place_id"] = stringField(place, "place_id")
		out["place_name"] = stringField(place, "name")
		out["formatted_address"] = stringField(place, "formatted_address")
	}
	if text := mapField(block, "text"); text != nil {
		out["text"] = plainRichText(text)
	}
	if stringField(block, "type") == "checklist" {
		items, _ := block["items"].([]any)
		out["item_count"] = len(items)
		var textTypes []string
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			if item == nil {
				continue
			}
			switch item["text"].(type) {
			case map[string]any:
				textTypes = append(textTypes, "richText")
			case string:
				textTypes = append(textTypes, "string")
			default:
				textTypes = append(textTypes, fmt.Sprintf("%T", item["text"]))
			}
		}
		if len(textTypes) > 0 {
			out["item_text_types"] = textTypes
		}
	}
	if kind := reservationKindForBlock(block); kind != "" {
		out["reservation_kind"] = kind
	}
	if hotel := mapField(block, "hotel"); hotel != nil {
		out["hotel"] = hotel
	}
	if lodgingOffer := mapField(block, "lodgingOffer"); lodgingOffer != nil {
		out["lodging_offer"] = lodgingOffer
	}
	for _, field := range []string{"flightInfo", "depart", "arrive", "pickUp", "dropOff", "carrier", "cruiseLine", "shipName", "voyageNumber", "title", "date", "startTime", "endTime", "durationMinutes", "timezone", "partySize", "nameForReservation", "confirmationNumber"} {
		if value, ok := block[field]; ok {
			out[field] = value
		}
	}
	return out
}

func plainRichText(text map[string]any) string {
	ops, _ := text["ops"].([]any)
	var b strings.Builder
	for _, raw := range ops {
		op, _ := raw.(map[string]any)
		b.WriteString(stringAny(op["insert"]))
	}
	return strings.TrimSpace(b.String())
}

func baseEditReport(command string, opts planEditOptions, target map[string]any) planEditReport {
	return planEditReport{Command: command, ApplyRequested: opts.apply, DryRun: !opts.apply, Sections: sectionReports(target)}
}

func ptrSectionReport(in planSectionReport) *planSectionReport { return &in }

func opPaths(ops []map[string]any) []string {
	paths := make([]string, 0, len(ops))
	for _, op := range ops {
		parts, _ := op["p"].([]any)
		var s []string
		for _, p := range parts {
			s = append(s, fmt.Sprint(p))
		}
		paths = append(paths, strings.Join(s, "."))
	}
	return paths
}

func printPlanEditReport(cmd *cobra.Command, flags *rootFlags, report planEditReport) error {
	// PATCH(amend-2026-08-23: terse mutation JSON omits op_paths and sections unless --verbose)
	if flags == nil || !flags.verbose {
		report.OpPaths = nil
		report.Sections = nil
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func fetchPlaceDetailsForBlock(ctx context.Context, c *client.Client, placeID string, language string) (map[string]any, error) {
	data, err := c.GetNoCache(ctx, "/api/placesAPI/getPlaceDetails/v2", map[string]string{"placeId": placeID, "language": firstNonEmpty(language, "en")})
	if err != nil {
		return nil, err
	}
	var env struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
		Error   any            `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if !env.Success || env.Data == nil {
		return nil, fmt.Errorf("place details not returned successfully for %s", placeID)
	}
	return env.Data, nil
}

func firstAutocompletePlaceID(ctx context.Context, c *client.Client, opts planEditOptions) (string, error) {
	request := map[string]any{
		"input":        opts.query,
		"sessiontoken": fmt.Sprintf("wanderlog-pp-cli-%d", time.Now().UnixNano()),
		"location":     map[string]any{"lat": opts.lat, "lng": opts.lng},
		"radius":       opts.radius,
		"language":     firstNonEmpty(opts.language, "en"),
	}
	body, _ := json.Marshal(request)
	data, err := c.GetNoCache(ctx, "/api/placesAPI/autocomplete/v2", map[string]string{"request": string(body)})
	if err != nil {
		return "", err
	}
	var env struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
		Error   any              `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", err
	}
	if !env.Success {
		return "", fmt.Errorf("place autocomplete did not succeed")
	}
	for _, item := range env.Data {
		if id := stringField(item, "place_id"); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("place autocomplete returned no place_id for %q", opts.query)
}

// PATCH(amend-2026-08-23: edit-text compiles markdown to Quill Delta; never emit header attributes)
func blockNoteText(text string, markdown bool) (map[string]any, []string, error) {
	if markdown {
		delta, stripped := compileMarkdownDelta(text)
		return delta, stripped, nil
	}
	if markdownLooksFormatted(text) {
		return nil, nil, errors.New("text contains markdown formatting; pass --markdown to compile bold, bullets, or headings (headings become bold labels, not header attributes)")
	}
	return richText(text), nil, nil
}

func markdownLooksFormatted(text string) bool {
	if strings.Contains(text, "**") {
		return true
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	for _, line := range strings.Split(text, "\n") {
		body := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(body, "- ") || strings.HasPrefix(body, "* ") || atxHeadingPrefixLen(body) > 0 {
			return true
		}
	}
	return false
}

func atxHeadingPrefixLen(s string) int {
	n := 0
	for n < len(s) && n < 6 && s[n] == '#' {
		n++
	}
	if n == 0 || n >= len(s) || s[n] != ' ' {
		return 0
	}
	return n + 1
}

func compileMarkdownDelta(text string) (map[string]any, []string) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return richText(""), nil
	}
	var ops []any
	for _, line := range lines {
		body := strings.TrimLeft(line, " \t")
		if n := atxHeadingPrefixLen(body); n > 0 {
			label := strings.TrimSpace(body[n:])
			if label != "" {
				ops = append(ops, map[string]any{"insert": label, "attributes": map[string]any{"bold": true}})
			}
			ops = append(ops, map[string]any{"insert": "\n"})
			continue
		}
		if strings.HasPrefix(body, "- ") || strings.HasPrefix(body, "* ") {
			appendInlineMarkdown(&ops, body[2:])
			ops = append(ops, map[string]any{"insert": "\n", "attributes": map[string]any{"list": "bullet"}})
			continue
		}
		appendInlineMarkdown(&ops, line)
		ops = append(ops, map[string]any{"insert": "\n"})
	}
	ops, stripped := stripDeltaHeaderAttributes(ops)
	return map[string]any{"ops": ops}, stripped
}

func appendInlineMarkdown(ops *[]any, s string) {
	for s != "" {
		start := strings.Index(s, "**")
		if start < 0 {
			*ops = append(*ops, map[string]any{"insert": s})
			return
		}
		rest := s[start+2:]
		end := strings.Index(rest, "**")
		if end < 0 {
			*ops = append(*ops, map[string]any{"insert": s})
			return
		}
		if start > 0 {
			*ops = append(*ops, map[string]any{"insert": s[:start]})
		}
		if inner := rest[:end]; inner != "" {
			*ops = append(*ops, map[string]any{"insert": inner, "attributes": map[string]any{"bold": true}})
		}
		s = rest[end+2:]
	}
}

func stripDeltaHeaderAttributes(ops []any) ([]any, []string) {
	var stripped []string
	out := make([]any, 0, len(ops))
	for _, raw := range ops {
		op, ok := raw.(map[string]any)
		if !ok {
			out = append(out, raw)
			continue
		}
		cloned := cloneJSONMap(op)
		attrs := mapField(cloned, "attributes")
		if _, ok := attrs["header"]; ok {
			delete(attrs, "header")
			if !sliceContainsString(stripped, "header") {
				stripped = append(stripped, "header")
			}
			if len(attrs) == 0 {
				delete(cloned, "attributes")
			} else {
				cloned["attributes"] = attrs
			}
		}
		out = append(out, cloned)
	}
	return out, stripped
}

func sliceContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
