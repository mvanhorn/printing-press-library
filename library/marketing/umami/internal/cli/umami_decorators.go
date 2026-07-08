// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored decorators layered over generated endpoint commands:
//   - --period on every command that takes --start-at/--end-at (human range
//     strings instead of hand-computed epoch-ms, absorbed from boly38's
//     period API)
//   - --qualified on `sessions list` (>=2 pageviews bot-strip, absorbed from
//     the Grafana BI dashboard's qualified-sessions filter)

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// addVersionGateHints walks the command tree and wraps every RunE so that a
// 404-with-HTML-body API error (a route the server's Umami version does not
// ship) carries an explicit version hint instead of a raw HTML dump, and the
// older-v3 "reports list requires websiteId" 400 names the missing flag.
func addVersionGateHints(root *cobra.Command) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			walk(child)
		}
		if cmd.RunE == nil {
			return
		}
		orig := cmd.RunE
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			err := orig(cmd, args)
			if err == nil {
				return nil
			}
			msg := err.Error()
			if strings.Contains(msg, "returned HTTP 404: <!DOCTYPE html") {
				return notFoundErr(fmt.Errorf("%s: this endpoint is not available on your Umami server version — it exists in newer Umami v3 releases; upgrade the server to use it", cmd.CommandPath()))
			}
			if strings.Contains(msg, "websiteId") && strings.Contains(msg, "expected string, received undefined") {
				return usageErr(fmt.Errorf("your Umami server version requires --website-id for %s", cmd.CommandPath()))
			}
			return err
		}
	}
	walk(root)
}

// addPeriodFlags walks the command tree and adds a --period convenience flag
// to every command that declares both --start-at and --end-at. When set (and
// --start-at was not), the window is computed and injected before RunE.
func addPeriodFlags(root *cobra.Command) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			walk(child)
		}
		if cmd.Flags().Lookup("start-at") == nil || cmd.Flags().Lookup("end-at") == nil ||
			cmd.Flags().Lookup("period") != nil || cmd.RunE == nil {
			return
		}
		cmd.Flags().String("period", "", "Human range instead of --start-at/--end-at: 24h, 7d, 2w, today, yesterday, this-week, this-month, last-month")
		orig := cmd.RunE
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			period, _ := cmd.Flags().GetString("period")
			if period != "" && (cmd.Flags().Changed("start-at") || cmd.Flags().Changed("end-at")) {
				return usageErr(fmt.Errorf("--period cannot be combined with explicit --start-at/--end-at"))
			}
			if period != "" {
				w, err := parsePeriod(period, time.Now())
				if err != nil {
					return usageErr(err)
				}
				if err := cmd.Flags().Set("start-at", strconv.FormatInt(w.StartMs, 10)); err != nil {
					return err
				}
				if err := cmd.Flags().Set("end-at", strconv.FormatInt(w.EndMs, 10)); err != nil {
					return err
				}
			}
			return orig(cmd, args)
		}
	}
	walk(root)
}

// addQualifiedSessionsFlag adds --qualified to `sessions list`: keep only
// sessions with at least 2 pageviews (strips most bot/bounce noise).
func addQualifiedSessionsFlag(sessionsCmd *cobra.Command) {
	var listCmd *cobra.Command
	for _, child := range sessionsCmd.Commands() {
		if child.Name() == "list" {
			listCmd = child
			break
		}
	}
	if listCmd == nil || listCmd.RunE == nil {
		return
	}
	listCmd.Flags().Bool("qualified", false, "Keep only sessions with >=2 pageviews (strips bot/bounce noise)")
	orig := listCmd.RunE
	listCmd.RunE = func(cmd *cobra.Command, args []string) error {
		qualified, _ := cmd.Flags().GetBool("qualified")
		if !qualified {
			return orig(cmd, args)
		}
		var buf bytes.Buffer
		realOut := cmd.OutOrStdout()
		cmd.SetOut(&buf)
		err := orig(cmd, args)
		cmd.SetOut(realOut)
		if err != nil {
			fmt.Fprint(realOut, buf.String())
			return err
		}
		filtered, ok := filterQualifiedSessions(buf.Bytes())
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: --qualified requires JSON output; printing the unfiltered list (add --json)")
			fmt.Fprint(realOut, buf.String())
			return nil
		}
		fmt.Fprint(realOut, string(filtered))
		return nil
	}
}

// filterQualifiedSessions removes rows with views < 2 from a sessions list
// payload, handling both the bare envelope and the agent meta wrapper.
// Returns ok=false when the payload shape is unrecognized (caller prints raw).
func filterQualifiedSessions(raw []byte) ([]byte, bool) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &doc); err != nil {
		return nil, false
	}
	filterRows := func(dataRaw json.RawMessage) (json.RawMessage, json.RawMessage, bool) {
		var rows []map[string]any
		if err := json.Unmarshal(dataRaw, &rows); err != nil {
			return nil, nil, false
		}
		kept := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			views, _ := row["views"].(float64)
			if views >= 2 {
				kept = append(kept, row)
			}
		}
		newData, err := json.Marshal(kept)
		if err != nil {
			return nil, nil, false
		}
		summary := json.RawMessage(fmt.Sprintf(`{"kept":%d,"dropped":%d}`, len(kept), len(rows)-len(kept)))
		return newData, summary, true
	}
	// Shape A: {..., "results": [rows]} or {..., "results": {"data": [rows]}}
	if results, hasResults := doc["results"]; hasResults {
		trimmed := bytes.TrimSpace(results)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			newData, summary, ok := filterRows(results)
			if !ok {
				return nil, false
			}
			doc["results"] = newData
			doc["qualified_filter"] = summary
		} else {
			var inner map[string]json.RawMessage
			if err := json.Unmarshal(results, &inner); err != nil {
				return nil, false
			}
			dataRaw, hasData := inner["data"]
			if !hasData {
				return nil, false
			}
			newData, summary, ok := filterRows(dataRaw)
			if !ok {
				return nil, false
			}
			inner["data"] = newData
			inner["qualified_filter"] = summary
			innerBytes, err := json.Marshal(inner)
			if err != nil {
				return nil, false
			}
			doc["results"] = innerBytes
		}
		out, err := json.MarshalIndent(doc, "", "  ")
		return out, err == nil
	}
	// Shape B: bare paged envelope {"data": [rows], ...}
	dataRaw, hasData := doc["data"]
	if !hasData {
		return nil, false
	}
	newData, summary, ok := filterRows(dataRaw)
	if !ok {
		return nil, false
	}
	doc["data"] = newData
	doc["qualified_filter"] = summary
	out, err := json.MarshalIndent(doc, "", "  ")
	return out, err == nil
}
