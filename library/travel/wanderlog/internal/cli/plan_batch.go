// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/spf13/cobra"
)

// fillDayStop is one object in plan fill-day --stops-json.
type fillDayStop struct {
	PlaceID string   `json:"place_id"`
	Query   string   `json:"query"`
	Lat     *float64 `json:"lat"`
	Lng     *float64 `json:"lng"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
	Note    string   `json:"note"`
	NoteMD  string   `json:"note_md"`
}

func newNovelPlanFillDayCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, radius: 50000, language: "en"}
	closedPlacePolicy := "block"
	var stopsJSON string
	cmd := &cobra.Command{
		Use:     "fill-day",
		Short:   "Insert a batch of place stops into one Wanderlog day",
		Example: "  wanderlog-pp-cli plan fill-day --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --stops-json '[{\"place_id\":\"ChIJxekpmbdp5TQRSqyFdGKMUJc\",\"start\":\"09:00\",\"note\":\"Cafe\"}]' --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateClosedPlacePolicy(closedPlacePolicy); err != nil {
				return usageErr(err)
			}
			stops, err := parseFillDayStops(stopsJSON)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			places, err := resolveFillDayPlaces(ctx, c, opts, stops)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return runPlanEditWithClient(cmd, flags, c, opts, "plan fill-day", func(target map[string]any) (planEditBuildResult, error) {
				return buildFillDayOps(target, opts, stops, places, closedPlacePolicy)
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().StringVar(&stopsJSON, "stops-json", "", "JSON array of stops: place_id and/or query (+lat/lng), optional start, end, note, note_md")
	cmd.Flags().IntVar(&opts.radius, "radius", 50000, "Autocomplete radius in meters for stop query")
	cmd.Flags().StringVar(&opts.language, "language", "en", "Language code for place details")
	cmd.Flags().StringVar(&closedPlacePolicy, "closed-place-policy", "block", "How to handle places explicitly closed on the selected section date: block, warn, or ignore")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanSectionSwapDaysCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	var withDay int
	cmd := &cobra.Command{
		Use:     "swap-days",
		Short:   "Swap the blocks arrays of two day sections in one JSON0 batch",
		Example: "  wanderlog-pp-cli plan section swap-days --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --with-day 2 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.day <= 0 || withDay <= 0 {
				return usageErr(errors.New("--day and --with-day are required"))
			}
			if opts.day == withDay {
				return usageErr(errors.New("--day and --with-day must be different"))
			}
			return runPlanEdit(cmd, flags, opts, "plan section swap-days", func(target map[string]any) (planEditBuildResult, error) {
				return buildSwapDaysOps(target, opts, withDay)
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.day, "day", 0, "1-based day number to swap")
	cmd.Flags().IntVar(&withDay, "with-day", 0, "1-based day number to swap with")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanPlaceReplaceCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, radius: 50000, language: "en"}
	closedPlacePolicy := "block"
	cmd := &cobra.Command{
		Use:     "replace",
		Short:   "Replace only the nested place on an existing itinerary block",
		Example: "  wanderlog-pp-cli plan place replace --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 1 --place-id ChIJLU7jZClu5kcR4PcOOO6p3I0 --dry-run --agent",
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
			place, err := resolvePlaceDetailsForEdit(ctx, c, cmd, flags, opts)
			if err != nil {
				return err
			}
			return runPlanEditWithClient(cmd, flags, c, opts, "plan place replace", func(target map[string]any) (planEditBuildResult, error) {
				return buildPlaceReplaceOps(target, opts, place, closedPlacePolicy)
			})
		},
	}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	addPlanBlockFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.placeID, "place-id", "", "Google/Wanderlog place id to set on the block")
	cmd.Flags().StringVar(&opts.query, "query", "", "Search query to resolve to the first place autocomplete result")
	cmd.Flags().Float64Var(&opts.lat, "lat", 0, "Latitude for --query autocomplete bias")
	cmd.Flags().Float64Var(&opts.lng, "lng", 0, "Longitude for --query autocomplete bias")
	cmd.Flags().IntVar(&opts.radius, "radius", 50000, "Autocomplete radius in meters for --query")
	cmd.Flags().StringVar(&opts.language, "language", "en", "Language code for place details")
	cmd.Flags().StringVar(&closedPlacePolicy, "closed-place-policy", "block", "How to handle places explicitly closed on the selected section date: block, warn, or ignore")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBlockApplyCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	var opsFile string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Preview or apply a JSON0 operation array from a file",
		Example: "  wanderlog-pp-cli plan block apply --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --ops-file ./ops.json --agent           # preview\n" +
			"  wanderlog-pp-cli plan block apply --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --ops-file ./ops.json --apply --agent   # write\n" +
			"  # global --dry-run returns immediately, before --ops-file is read",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Dry-run guard first, unconditionally: --dry-run must return
			// before any filesystem read. --ops-file is validated inside RunE
			// (not via MarkFlagRequired, which cobra enforces before RunE ever
			// runs and so makes a --dry-run probe unreachable). Preview mode is
			// the default for real invocations -- omit --apply -- so nothing is
			// lost by making --dry-run a pure no-op here.
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"command":         "plan block apply",
					"dry_run":         true,
					"applied":         false,
					"apply_requested": false,
					"ops":             0,
					"warnings":        []string{"global --dry-run set: --ops-file was not read and no operations were applied"},
				}, flags)
			}
			ops, err := loadJSON0Ops("", opsFile)
			if err != nil {
				return usageErr(err)
			}
			return runJSON0OpsEdit(cmd, flags, opts, "plan block apply", ops)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().StringVar(&opsFile, "ops-file", "", "Path to a JSON0 operation array (required; global --dry-run returns before the file is read)")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the ops through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

// PATCH(amend-2026-08-23: swap two day section blocks arrays in one JSON0 batch)
func buildSwapDaysOps(target map[string]any, opts planEditOptions, withDay int) (planEditBuildResult, error) {
	secA, err := resolveSection(target, opts.day, -1, 0)
	if err != nil {
		return planEditBuildResult{}, err
	}
	secB, err := resolveSection(target, withDay, -1, 0)
	if err != nil {
		return planEditBuildResult{}, fmt.Errorf("resolve --with-day: %w", err)
	}
	blocksA := secA.Blocks
	if blocksA == nil {
		blocksA = []any{}
	}
	blocksB := secB.Blocks
	if blocksB == nil {
		blocksB = []any{}
	}
	ops := []map[string]any{
		objectSetOp([]any{"itinerary", "sections", secA.Index, "blocks"}, blocksA, true, blocksB, false),
		objectSetOp([]any{"itinerary", "sections", secB.Index, "blocks"}, blocksB, true, blocksA, false),
	}
	repA := secA.Report
	repB := secB.Report
	repA.BlockCount = len(blocksB)
	repB.BlockCount = len(blocksA)
	report := baseEditReport("plan section swap-days", opts, target)
	report.Section = ptrSectionReport(repA)
	report.Destination = ptrSectionReport(repB)
	report.Operation = "swap day section blocks"
	report.OpPaths = opPaths(ops)
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

// PATCH(amend-2026-08-23: fill-day builds N li ops from already-resolved places)
func buildFillDayOps(target map[string]any, opts planEditOptions, stops []fillDayStop, resolvedPlaces map[string]map[string]any, closedPlacePolicy string) (planEditBuildResult, error) {
	sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
	if err != nil {
		return planEditBuildResult{}, err
	}
	var ops []map[string]any
	var stripped []string
	var warnings []string
	base := len(sec.Blocks)
	var lastBlock map[string]any
	for i, stop := range stops {
		if err := validateFillDayStop(stop); err != nil {
			return planEditBuildResult{}, fmt.Errorf("stops[%d]: %w", i, err)
		}
		place := resolvedPlaces[fillDayStopKey(stop)]
		if place == nil {
			return planEditBuildResult{}, fmt.Errorf("stops[%d]: place is not resolved", i)
		}
		if warning, closed := placeClosedOnDateWarning(place, sec.Report.Date); closed {
			if closedPlacePolicy == "block" {
				return planEditBuildResult{}, fmt.Errorf("%s; pass --closed-place-policy=warn or --closed-place-policy=ignore only if this is intentionally kept as a backup", warning)
			}
			if closedPlacePolicy == "warn" {
				warnings = append(warnings, warning)
			}
		}
		note := strings.TrimSpace(stop.Note)
		block := newPlaceBlock(place, note)
		if md := strings.TrimSpace(stop.NoteMD); md != "" {
			delta, extra := compileMarkdownDelta(md)
			block["text"] = delta
			stripped = append(stripped, extra...)
		}
		if start := strings.TrimSpace(stop.Start); start != "" {
			block["startTime"] = start
		}
		if end := strings.TrimSpace(stop.End); end != "" {
			block["endTime"] = end
		}
		idx := base + i
		ops = append(ops, map[string]any{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "li": block})
		lastBlock = block
	}
	report := baseEditReport("plan fill-day", opts, target)
	report.Section = ptrSectionReport(sec.Report)
	if lastBlock != nil {
		report.Block = summarizeBlock(lastBlock)
		report.BlockID = intAny(lastBlock["id"])
		report.BlockIndex = base + len(stops) - 1
	}
	report.Operation = fmt.Sprintf("insert %d place blocks", len(stops))
	report.OpPaths = opPaths(ops)
	report.Warnings = warnings
	report.Stripped = stripped
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

// PATCH(amend-2026-08-23: place replace writes only nested place)
func buildPlaceReplaceOps(target map[string]any, opts planEditOptions, place map[string]any, closedPlacePolicy string) (planEditBuildResult, error) {
	sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
	if err != nil {
		return planEditBuildResult{}, err
	}
	if stringField(block, "type") != "place" && mapField(block, "place") == nil {
		return planEditBuildResult{}, fmt.Errorf("block %d is not a place block", intAny(block["id"]))
	}
	oldPlace := mapField(block, "place")
	exists := oldPlace != nil
	ops := []map[string]any{objectSetOp([]any{"itinerary", "sections", sec.Index, "blocks", idx, "place"}, oldPlace, exists, place, false)}
	preview := cloneJSONMap(block)
	preview["place"] = place
	report := baseEditReport("plan place replace", opts, target)
	report.Section = ptrSectionReport(sec.Report)
	if warning, closed := placeClosedOnDateWarning(place, sec.Report.Date); closed {
		if closedPlacePolicy == "block" {
			return planEditBuildResult{}, fmt.Errorf("%s; pass --closed-place-policy=warn or --closed-place-policy=ignore only if this is intentionally kept as a backup", warning)
		}
		if closedPlacePolicy == "warn" {
			report.Warnings = append(report.Warnings, warning)
		}
	}
	report.Block = summarizeBlock(preview)
	report.BlockID = intAny(block["id"])
	report.BlockIndex = idx
	report.Operation = "replace nested place"
	report.OpPaths = opPaths(ops)
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

func parseFillDayStops(raw string) ([]fillDayStop, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("--stops-json is required")
	}
	var stops []fillDayStop
	if err := json.Unmarshal([]byte(raw), &stops); err != nil {
		return nil, fmt.Errorf("parse --stops-json: %w", err)
	}
	if len(stops) == 0 {
		return nil, errors.New("--stops-json array is empty")
	}
	for i, stop := range stops {
		if err := validateFillDayStop(stop); err != nil {
			return nil, fmt.Errorf("stops[%d]: %w", i, err)
		}
	}
	return stops, nil
}

func validateFillDayStop(stop fillDayStop) error {
	placeID := strings.TrimSpace(stop.PlaceID)
	query := strings.TrimSpace(stop.Query)
	if placeID == "" && query == "" {
		return errors.New("place_id or query is required")
	}
	if placeID == "" {
		if stop.Lat == nil || stop.Lng == nil {
			return errors.New("query requires lat and lng")
		}
	}
	if strings.TrimSpace(stop.Note) != "" && strings.TrimSpace(stop.NoteMD) != "" {
		return errors.New("note and note_md cannot both be set")
	}
	if start := strings.TrimSpace(stop.Start); start != "" && !validClock(start) {
		return errors.New("start must be HH:MM")
	}
	if end := strings.TrimSpace(stop.End); end != "" && !validClock(end) {
		return errors.New("end must be HH:MM")
	}
	return nil
}

func fillDayStopKey(stop fillDayStop) string {
	if id := strings.TrimSpace(stop.PlaceID); id != "" {
		return "id:" + id
	}
	return "query:" + strings.TrimSpace(stop.Query)
}

func resolveFillDayPlaces(ctx context.Context, c *client.Client, opts planEditOptions, stops []fillDayStop) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(stops))
	for i, stop := range stops {
		key := fillDayStopKey(stop)
		if out[key] != nil {
			continue
		}
		placeOpts := opts
		placeOpts.placeID = strings.TrimSpace(stop.PlaceID)
		placeOpts.query = strings.TrimSpace(stop.Query)
		if stop.Lat != nil {
			placeOpts.lat = *stop.Lat
		}
		if stop.Lng != nil {
			placeOpts.lng = *stop.Lng
		}
		place, err := fetchResolvedPlaceDetails(ctx, c, placeOpts, stop.Lat != nil && stop.Lng != nil)
		if err != nil {
			return nil, fmt.Errorf("stops[%d]: %w", i, err)
		}
		out[key] = place
	}
	return out, nil
}

func resolvePlaceDetailsForEdit(ctx context.Context, c *client.Client, cmd *cobra.Command, flags *rootFlags, opts planEditOptions) (map[string]any, error) {
	hasCoords := cmd.Flags().Changed("lat") && cmd.Flags().Changed("lng")
	place, err := fetchResolvedPlaceDetails(ctx, c, opts, hasCoords)
	if err != nil {
		if errors.Is(err, errPlaceIDOrQueryRequired) || strings.Contains(err.Error(), "--query requires --lat") {
			return nil, usageErr(err)
		}
		return nil, classifyAPIError(err, flags)
	}
	return place, nil
}

var errPlaceIDOrQueryRequired = errors.New("--place-id or --query is required")

func fetchResolvedPlaceDetails(ctx context.Context, c *client.Client, opts planEditOptions, hasCoords bool) (map[string]any, error) {
	placeID := strings.TrimSpace(opts.placeID)
	if placeID == "" && strings.TrimSpace(opts.query) != "" {
		if !hasCoords {
			return nil, errors.New("--query requires --lat and --lng; use places autocomplete first if you do not have coordinates")
		}
		id, err := firstAutocompletePlaceID(ctx, c, opts)
		if err != nil {
			return nil, err
		}
		placeID = id
	}
	if placeID == "" {
		return nil, errPlaceIDOrQueryRequired
	}
	return fetchPlaceDetailsForBlock(ctx, c, placeID, opts.language)
}

func loadJSON0Ops(opJSON string, opsFile string) ([]map[string]any, error) {
	opJSON = strings.TrimSpace(opJSON)
	opsFile = strings.TrimSpace(opsFile)
	if opJSON == "" && opsFile == "" {
		return nil, errors.New("--op or --ops-file is required")
	}
	if opJSON != "" && opsFile != "" {
		return nil, errors.New("--op and --ops-file cannot both be set")
	}
	if opsFile != "" {
		return readJSON0OpsFile(opsFile)
	}
	return parseJSON0Ops([]byte(opJSON), "--op")
}

func readJSON0OpsFile(path string) ([]map[string]any, error) {
	path = filepath.Clean(path)
	// #nosec G304 -- path comes from the operator's own --ops-file flag on
	// their own machine; there is no privilege boundary to cross and no
	// server-side caller that could supply it.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --ops-file: %w", err)
	}
	return parseJSON0Ops(data, "--ops-file")
}

func parseJSON0Ops(data []byte, source string) ([]map[string]any, error) {
	var ops []map[string]any
	if err := json.Unmarshal(data, &ops); err != nil || len(ops) == 0 {
		if err == nil {
			err = errors.New("operation array is empty")
		}
		return nil, fmt.Errorf("parse %s JSON array: %w", source, err)
	}
	return ops, nil
}

func runJSON0OpsEdit(cmd *cobra.Command, flags *rootFlags, opts planEditOptions, commandName string, ops []map[string]any) error {
	return runPlanEdit(cmd, flags, opts, commandName, func(target map[string]any) (planEditBuildResult, error) {
		report := baseEditReport(commandName, opts, target)
		report.Operation = "raw JSON0 op"
		report.OpPaths = opPaths(ops)
		report.Warnings = append(report.Warnings, "raw ops can corrupt a collaborative plan; use only after inspecting dry-run output")
		return planEditBuildResult{Ops: ops, Report: report}, nil
	})
}
