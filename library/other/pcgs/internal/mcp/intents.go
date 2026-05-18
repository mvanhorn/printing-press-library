// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

// PATCH mcp-intent-tools: hand-authored MCP intent layer registering three
// composed-workflow tools (pcgs_verify_and_extract, pcgs_batch_with_quota_guard,
// pcgs_pop_scarcity_report) above the generator-emitted endpoint-mirror surface.
// Registered from cmd/pcgs-pp-mcp/main.go via mcptools.RegisterIntents(s).

// Package mcp — intent handlers for PCGS.
//
// Intents compose multiple PCGS endpoint calls into named agent-facing
// workflows. They are higher-level than the per-endpoint mirror in tools.go:
// instead of asking an agent to chain `coin facts-cert` + `coin images` +
// `coin apr-cert` to "verify and extract everything about this cert," the
// intent does the orchestration and returns one merged result.
//
// Three intents are registered, each derived from a top workflow in
// research.json:
//
//   - pcgs_verify_and_extract       — verify cert legitimacy + extract every
//                                     CoinFacts field + image URLs + recent
//                                     auction realized prices (1 input, 3 calls)
//   - pcgs_batch_with_quota_guard   — forecast a batch from a fixture file,
//                                     gate on remaining quota, run with
//                                     resumable checkpointing (1 input, 1
//                                     forecast call + N batch calls)
//   - pcgs_pop_scarcity_report      — full grade-curve fanout for a PCGSNo
//                                     plus a scarcity-bucket ranking
//                                     (1 input, ~70 calls; cache reuse)
//
// Each intent normalizes cert input the same way coin batch does — accepts
// bare cert numbers and full slab IDs (PCGSNo.Grade/CertNo).

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterIntents adds PCGS intent tools to the MCP server. Called from
// RegisterTools after the endpoint-mirror surface is registered.
func RegisterIntents(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("pcgs_verify_and_extract",
			mcplib.WithDescription(
				"Verify a PCGS cert is legitimate and extract everything PCGS knows about it: full CoinFacts metadata, image URLs, and recent Auction Prices Realized. "+
					"Returns a merged object {facts, images, apr}. Accepts bare cert numbers (e.g. 64674260) or full slab IDs (e.g. 7258.58/64674260). "+
					"IsValidRequest=true on facts confirms the cert is real; empty Images is normal for very recent slabs that haven't been photographed yet.",
			),
			mcplib.WithString("cert", mcplib.Required(), mcplib.Description("Bare cert number (64674260) or full slab ID (7258.58/64674260).")),
			mcplib.WithBoolean("with_apr", mcplib.Description("Also fetch Auction Prices Realized for the cert (default: true).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleVerifyAndExtract,
	)

	s.AddTool(
		mcplib.NewTool("pcgs_batch_with_quota_guard",
			mcplib.WithDescription(
				"Plan a multi-cert batch lookup against PCGS, respecting the 1,000-call daily quota. Runs the dry-run forecast first (no API calls), confirms the batch fits remaining quota, then "+
					"executes with resumable checkpointing if approved by --execute. Returns either {forecast: {...}} (plan only) or {forecast, results: [...]} (executed). "+
					"Accepts CSV / JSON wrappers / JSONL / plain-text fixtures with full slab IDs.",
			),
			mcplib.WithString("file", mcplib.Required(), mcplib.Description("Path to a cert list fixture (CSV / JSON / JSONL / plain text). Slab IDs like 7258.58/64674260 are normalized.")),
			mcplib.WithBoolean("execute", mcplib.Description("Set true to execute after forecasting (default: false — forecast only, no API calls).")),
			mcplib.WithBoolean("resumable", mcplib.Description("When executing, write a checkpoint file so a partial run can resume tomorrow (default: true).")),
			mcplib.WithString("checkpoint", mcplib.Description("Override the checkpoint path (default: ./pcgs-batch.ckpt).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleBatchWithQuotaGuard,
	)

	s.AddTool(
		mcplib.NewTool("pcgs_pop_scarcity_report",
			mcplib.WithDescription(
				"Build a scarcity report for one PCGSNo by fanning Population/PopHigher across grades 1-70 (and PlusGrade variants, plus the 82-98 Details codes when include_details=true). "+
					"Returns {pcgs_no, curve: [{grade, plus, population, pop_higher}], scarcity_tiers: {pop1: [...], pop2_5: [...], pop6_25: [...], pop26_plus: [...]}}. "+
					"Up to ~70 API calls per invocation; cache reuse is automatic. Pre-flight quota check refuses to start if remaining budget can't cover the fanout.",
			),
			mcplib.WithString("pcgs_no", mcplib.Required(), mcplib.Description("PCGS spec number (e.g. 7356 for 1921 Peace Dollar).")),
			mcplib.WithBoolean("include_plus", mcplib.Description("Also fan PlusGrade=true for each grade (default: true).")),
			mcplib.WithBoolean("include_details", mcplib.Description("Also iterate grades 82-98 (PCGS Details codes; PlusGrade is always false for these). Default: false.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handlePopScarcityReport,
	)
}

// normalizeMCPCert returns the bare cert number from either a bare digit string
// or a full slab ID like "7258.58/64674260". Returns an MCP-formatted error
// when input is empty or non-digit after normalization.
func normalizeMCPCert(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty cert value")
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty cert value after normalization")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("invalid cert %q (must be digits after normalization)", s)
		}
	}
	return s, nil
}

func handleVerifyAndExtract(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	rawCert, _ := args["cert"].(string)
	cert, err := normalizeMCPCert(rawCert)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	withAPR := true
	if v, ok := args["with_apr"].(bool); ok {
		withAPR = v
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	out := map[string]any{}

	// Step 1: CoinFacts
	factsPath := strings.ReplaceAll("/coindetail/GetCoinFactsByCertNo/{certNo}", "{certNo}", cert)
	factsRaw, factsErr := c.Get(factsPath, map[string]string{"retrieveAllData": "true"})
	if factsErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("verify_and_extract: facts call failed: %v", factsErr)), nil
	}
	var facts map[string]any
	_ = json.Unmarshal(factsRaw, &facts)
	out["facts"] = facts

	// Step 2: Images
	imagesRaw, imgErr := c.Get("/coindetail/GetImagesByCertNo", map[string]string{"certNo": cert})
	if imgErr != nil {
		// Don't fail the whole intent on image-fetch failure; surface the error per-section.
		out["images_error"] = imgErr.Error()
	} else {
		var images map[string]any
		_ = json.Unmarshal(imagesRaw, &images)
		out["images"] = images
	}

	// Step 3: APR (optional)
	if withAPR {
		aprPath := strings.ReplaceAll("/coindetail/GetAPRByCertNo/{CertNo}", "{CertNo}", cert)
		aprRaw, aprErr := c.Get(aprPath, nil)
		if aprErr != nil {
			out["apr_error"] = aprErr.Error()
		} else {
			var apr any
			_ = json.Unmarshal(aprRaw, &apr)
			out["apr"] = apr
		}
	}

	out["cert"] = cert
	data, err := json.Marshal(out)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("verify_and_extract: encode result: %v", err)), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}

func handleBatchWithQuotaGuard(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	file, _ := args["file"].(string)
	if strings.TrimSpace(file) == "" {
		return mcplib.NewToolResultError("file is required"), nil
	}
	execute := false
	if v, ok := args["execute"].(bool); ok {
		execute = v
	}
	resumable := true
	if v, ok := args["resumable"].(bool); ok {
		resumable = v
	}
	checkpoint, _ := args["checkpoint"].(string)
	if strings.TrimSpace(checkpoint) == "" {
		checkpoint = "./pcgs-batch.ckpt"
	}

	// Forecast first via the CLI subprocess (uses existing dry-run code path).
	forecastArgs := []string{"coin", "batch", "--file", file, "--dry-run", "--json"}
	forecastOut, err := runCLISubprocess(ctx, forecastArgs)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("batch_with_quota_guard: forecast failed: %v\n%s", err, forecastOut)), nil
	}

	result := map[string]any{}
	var forecast map[string]any
	_ = json.Unmarshal([]byte(forecastOut), &forecast)
	result["forecast"] = forecast

	if !execute {
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}

	// Execute with resumable checkpointing.
	batchArgs := []string{"coin", "batch", "--file", file, "--json"}
	if resumable {
		batchArgs = append(batchArgs, "--resumable", "--checkpoint", checkpoint)
	}
	batchOut, batchErr := runCLISubprocess(ctx, batchArgs)
	if batchErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("batch_with_quota_guard: execute failed: %v\n%s", batchErr, batchOut)), nil
	}
	// batchOut is newline-delimited JSON; split into lines and parse each.
	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(batchOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row map[string]any
		if json.Unmarshal([]byte(line), &row) == nil {
			rows = append(rows, row)
		}
	}
	result["results"] = rows
	data, _ := json.Marshal(result)
	return mcplib.NewToolResultText(string(data)), nil
}

func handlePopScarcityReport(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	pcgsNo, _ := args["pcgs_no"].(string)
	if strings.TrimSpace(pcgsNo) == "" {
		return mcplib.NewToolResultError("pcgs_no is required"), nil
	}
	includePlus := true
	if v, ok := args["include_plus"].(bool); ok {
		includePlus = v
	}
	includeDetails := false
	if v, ok := args["include_details"].(bool); ok {
		includeDetails = v
	}

	cliArgs := []string{"coin", "pop-curve", pcgsNo, "--json"}
	if includePlus {
		cliArgs = append(cliArgs, "--plus")
	}
	if includeDetails {
		cliArgs = append(cliArgs, "--include-details")
	}
	out, err := runCLISubprocess(ctx, cliArgs)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("pop_scarcity_report: pop-curve failed: %v\n%s", err, out)), nil
	}

	// Parse the curve output and bucket by population tier.
	var curveRaw any
	_ = json.Unmarshal([]byte(out), &curveRaw)

	report := map[string]any{
		"pcgs_no": pcgsNo,
		"curve":   curveRaw,
	}

	// Try to extract rows for scarcity-tier bucketing.
	type curveRow = map[string]any
	rows, _ := extractCurveRows(curveRaw)
	tiers := map[string][]curveRow{
		"pop1":       {},
		"pop2_5":     {},
		"pop6_25":    {},
		"pop26_plus": {},
	}
	for _, row := range rows {
		pop := popFromRow(row)
		switch {
		case pop == 1:
			tiers["pop1"] = append(tiers["pop1"], row)
		case pop >= 2 && pop <= 5:
			tiers["pop2_5"] = append(tiers["pop2_5"], row)
		case pop >= 6 && pop <= 25:
			tiers["pop6_25"] = append(tiers["pop6_25"], row)
		case pop > 25:
			tiers["pop26_plus"] = append(tiers["pop26_plus"], row)
		}
	}
	report["scarcity_tiers"] = tiers

	data, _ := json.Marshal(report)
	return mcplib.NewToolResultText(string(data)), nil
}

// extractCurveRows pulls [{grade, plus, population, pop_higher}] rows from
// a pop-curve JSON payload, handling both the {meta, results: [...]} wrapper
// and a bare array. Returns the rows and a bool indicating successful extraction.
func extractCurveRows(raw any) ([]map[string]any, bool) {
	if arr, ok := raw.([]any); ok {
		return toMapSlice(arr), true
	}
	if obj, ok := raw.(map[string]any); ok {
		if results, ok := obj["results"]; ok {
			if arr, ok := results.([]any); ok {
				return toMapSlice(arr), true
			}
		}
	}
	return nil, false
}

func toMapSlice(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, v := range arr {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func popFromRow(row map[string]any) int {
	for _, key := range []string{"Population", "population"} {
		if v, ok := row[key]; ok {
			switch tv := v.(type) {
			case float64:
				return int(tv)
			case string:
				n, _ := strconv.Atoi(strings.TrimSpace(tv))
				return n
			}
		}
	}
	return 0
}
