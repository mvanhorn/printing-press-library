// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

// Approval triage.
//
// A Rightbrain agent run that reaches a gated tool stops and parks at
// waiting_for_human. The API returns the pending approval records with raw
// timestamps and no ordering, and an approval whose window has elapsed still
// comes back in the list. This command computes what the API does not return —
// how long each request has been parked, how long until it expires, and
// whether it is still actionable at all — and sorts the queue by urgency.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/cliutil"
)

// approvalRecord is the subset of ProjectTaskAgentApprovalRequestRead this
// command consumes.
type approvalRecord struct {
	ID            string `json:"id"`
	TaskAgentID   string `json:"task_agent_id"`
	TaskAgentName string `json:"task_agent_name"`
	RunID         string `json:"task_agent_run_id"`
	SessionID     string `json:"session_id"`
	ToolName      string `json:"tool_name"`
	ToolType      string `json:"tool_type"`
	Status        string `json:"status"`
	Created       string `json:"created"`
	ExpiresAt     string `json:"expires_at"`
}

// approvalView is one row of triage output.
type approvalView struct {
	ID            string  `json:"id"`
	AgentName     string  `json:"agent_name"`
	AgentID       string  `json:"agent_id"`
	RunID         string  `json:"run_id"`
	SessionID     string  `json:"session_id"`
	ToolName      string  `json:"tool_name"`
	ToolType      string  `json:"tool_type"`
	Status        string  `json:"status"`
	Created       string  `json:"created"`
	ParkedSeconds float64 `json:"parked_seconds"`
	ParkedUnknown bool    `json:"parked_unknown,omitempty"`
	ParkedHuman   string  `json:"parked"`
	ExpiresAt     string  `json:"expires_at,omitempty"`
	ExpiresInSecs float64 `json:"expires_in_seconds,omitempty"`
	Expired       bool    `json:"expired"`
	Actionable    bool    `json:"actionable"`
}

type approvalsReport struct {
	Pending    []approvalView `json:"pending"`
	Expired    []approvalView `json:"expired"`
	Counts     map[string]int `json:"counts"`
	OldestSecs float64        `json:"oldest_parked_seconds"`
	Note       string         `json:"note,omitempty"`
}

// parseAPITime accepts the timestamps the Rightbrain API returns and reports
// whether parsing succeeded, so a malformed or absent value degrades to
// "unknown" rather than a zero time that reads as 1970.
func parseAPITime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// apiTimeAfter reports whether timestamp a is strictly later than b, parsing
// both rather than comparing them as bytes.
//
// The API emits two shapes for the same field — "2026-04-27T13:53:00.997704Z"
// and "2026-04-27T13:53:00Z", the second whenever the microsecond component
// happens to be zero. A real mirror sampled 235 of the first against 67 of the
// second. Comparing those as strings inverts the order, because '.' (0x2E)
// sorts below 'Z' (0x5A): the whole-second value reads as the later one when it
// is in fact up to a second earlier. Records that share a second are exactly
// the ones an ordering has to separate — eval runs triggered together, audit
// events emitted by a single action — so the comparison has to parse.
//
// Unparseable values sort last. Two unparseable values fall back to a byte
// comparison, which is arbitrary but keeps the ordering deterministic.
func apiTimeAfter(a, b string) bool {
	ta, aOK := parseAPITime(a)
	tb, bOK := parseAPITime(b)
	if aOK && bOK {
		return ta.After(tb)
	}
	if aOK != bOK {
		return aOK
	}
	return a > b
}

// humanizeDuration renders a coarse, readable age ("3d 4h", "12m").
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// buildApprovalsReport turns raw approval records into the triage view. Split
// out from RunE so it is unit-testable without a live API.
func buildApprovalsReport(records []approvalRecord, now time.Time, olderThan time.Duration, limit int, olderThanRaw string) approvalsReport {
	report := approvalsReport{
		Pending: make([]approvalView, 0),
		Expired: make([]approvalView, 0),
		Counts:  map[string]int{},
	}

	for _, rec := range records {
		view := approvalView{
			ID:        rec.ID,
			AgentName: rec.TaskAgentName,
			AgentID:   rec.TaskAgentID,
			RunID:     rec.RunID,
			SessionID: rec.SessionID,
			ToolName:  rec.ToolName,
			ToolType:  rec.ToolType,
			Status:    rec.Status,
			Created:   rec.Created,
			ExpiresAt: rec.ExpiresAt,
		}
		created, hasCreated := parseAPITime(rec.Created)
		if hasCreated {
			parked := now.Sub(created)
			view.ParkedSeconds = parked.Seconds()
			view.ParkedHuman = humanizeDuration(parked)
		} else {
			view.ParkedHuman = "unknown"
			view.ParkedUnknown = true
		}
		if exp, ok := parseAPITime(rec.ExpiresAt); ok {
			view.ExpiresInSecs = exp.Sub(now).Seconds()
			if exp.Before(now) {
				view.Expired = true
			}
		}
		if rec.Status == "expired" {
			view.Expired = true
		}
		view.Actionable = rec.Status == "pending" && !view.Expired

		report.Counts[rec.Status]++
		// --older-than never filters a record whose created stamp would not
		// parse: its parked age is 0 only because it is unknown, and an
		// unknown age cannot be proven recent. Dropping it would hide a
		// genuinely parked approval from both sections while still counting it
		// in Counts, so the JSON's counts would not match its own rows.
		if olderThan > 0 && hasCreated && view.ParkedSeconds < olderThan.Seconds() {
			continue
		}
		if view.Expired {
			report.Expired = append(report.Expired, view)
		} else {
			report.Pending = append(report.Pending, view)
		}
	}

	// Oldest first: the whole point of the queue is urgency order.
	sort.SliceStable(report.Pending, func(i, j int) bool {
		return report.Pending[i].ParkedSeconds > report.Pending[j].ParkedSeconds
	})
	sort.SliceStable(report.Expired, func(i, j int) bool {
		return report.Expired[i].ParkedSeconds > report.Expired[j].ParkedSeconds
	})
	if limit > 0 {
		if len(report.Pending) > limit {
			report.Pending = report.Pending[:limit]
		}
		if len(report.Expired) > limit {
			report.Expired = report.Expired[:limit]
		}
	}
	if len(report.Pending) > 0 {
		report.OldestSecs = report.Pending[0].ParkedSeconds
	}
	if len(report.Pending) == 0 && len(report.Expired) == 0 {
		if olderThan > 0 {
			report.Note = fmt.Sprintf("no approval requests parked longer than %s; drop or lower --older-than to widen the sweep", olderThanRaw)
		} else {
			report.Note = "no approval requests found; nothing is waiting on a human"
		}
	}
	return report
}

func newNovelApprovalsCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string
	var flagAgentID string
	var flagStatus string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "approvals",
		Short: "Triage every pending agent approval request by how long it has been parked and how soon it expires",
		Long: "Triage agent runs parked awaiting human approval.\n\n" +
			"Computes parked age and time-to-expiry from the raw timestamps the API returns, " +
			"separates requests whose approval window has already lapsed from the ones still " +
			"worth acting on, and sorts by how long each has been waiting.\n\n" +
			"Use this command to sweep every agent for runs parked awaiting human approval. " +
			"Do NOT use it to inspect what a single run did step by step; use 'agent-trace' instead.",
		Example: "  rightbrain-pp-cli approvals --older-than 1h --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list agent approval requests for the configured project")
				return nil
			}

			var olderThan time.Duration
			if flagOlderThan != "" {
				d, err := cliutil.ParseDurationLoose(flagOlderThan)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --older-than %q: %w", flagOlderThan, err))
				}
				olderThan = d
			}

			orgID, projectID, err := requireScope(flags)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/org/%s/project/%s/task-agent/approval-request", orgID, projectID)
			params := map[string]string{"page_limit": "100"}
			if flagStatus != "" && flagStatus != "all" {
				params["status"] = flagStatus
			}
			if flagAgentID != "" {
				params["task_agent_id"] = flagAgentID
			}

			// GetNoCache, not Get: this is a triage queue over volatile state.
			// Approving or rejecting a request and immediately re-running the
			// sweep has to show the queue shrink. Through the response cache it
			// does not — a request decided seconds ago still reads as pending,
			// so an operator either double-handles it or concludes the decision
			// silently failed.
			data, err := c.GetNoCache(ctx, path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope struct {
				Results []approvalRecord `json:"results"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing approval requests: %w", err)
			}

			report := buildApprovalsReport(envelope.Results, time.Now().UTC(), olderThan, flagLimit, flagOlderThan)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				if report.Note != "" {
					fmt.Fprintln(out, report.Note)
					return nil
				}
				if len(report.Pending) > 0 {
					fmt.Fprintf(out, "ACTIONABLE (%d)\n", len(report.Pending))
					fmt.Fprintln(out, "PARKED\tAGENT\tTOOL\tRUN")
					for _, v := range report.Pending {
						fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", v.ParkedHuman, orDash(v.AgentName), orDash(v.ToolName), v.RunID)
					}
				}
				if len(report.Expired) > 0 {
					fmt.Fprintf(out, "\nEXPIRED (%d) — the approval window lapsed; these need a re-run, not a decision\n", len(report.Expired))
					fmt.Fprintln(out, "PARKED\tAGENT\tTOOL\tRUN")
					for _, v := range report.Expired {
						fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", v.ParkedHuman, orDash(v.AgentName), orDash(v.ToolName), v.RunID)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&flagOlderThan, "older-than", "",
		"Only show requests parked longer than this (e.g. 30m, 1h, 2d)")
	cmd.Flags().StringVar(&flagAgentID, "agent-id", "",
		"Limit the sweep to a single task agent by ID")
	cmd.Flags().StringVar(&flagStatus, "status", "pending",
		"Approval status to list: pending, approved, rejected, expired, cancelled, or all")
	cmd.Flags().IntVar(&flagLimit, "limit", 0,
		"Maximum rows to show per section (0 means no limit)")
	return cmd
}
