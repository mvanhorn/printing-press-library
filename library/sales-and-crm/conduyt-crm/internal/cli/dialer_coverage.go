// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type dialerCoveragePriority struct {
	Position    int    `json:"position"`
	SmartViewID string `json:"smart_view_id"`
	Name        string `json:"name"`
	QueueDepth  int    `json:"queue_depth"`
	Capped      bool   `json:"capped"`
	Error       string `json:"error,omitempty"`
}
type dialerCoverageView struct {
	AvailableAgents int                      `json:"available_agents"`
	Priorities      []dialerCoveragePriority `json:"priorities"`
	EmptyPriorities []string                 `json:"empty_priorities"`
	Partial         bool                     `json:"partial"`
	Checked         int                      `json:"checked"`
	Total           int                      `json:"total"`
	FetchFailures   int                      `json:"fetch_failures"`
	Failures        []string                 `json:"failures,omitempty"`
	Warning         string                   `json:"warning,omitempty"`
}

func newNovelDialerCoverageCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use: "coverage", Short: "Priorities in dial order with queue depth per priority and the count of available agents",
		Long:        "Use this command for the account-wide picture of priorities, queue depth and available agents. Do NOT use it to fetch one agent's next leads; use 'dialer queue list' instead.",
		Example:     "  conduyt-crm-pp-cli dialer coverage --limit 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dialer coverage")
			}
			if len(args) == 0 && !novelInvocationHasFlags(cmd, flags) {
				return cmd.Help()
			}
			if limit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(ctx, "/smart-views/dial-order", map[string]string{})
			if err != nil {
				wrapped := fmt.Errorf("fetching dial order: %w", err)
				if strings.Contains(err.Error(), "HTTP 403") {
					return authErr(fmt.Errorf("%w; dialer.view/admin permission is required", wrapped))
				}
				return classifyAPIErrorOnly(wrapped)
			}
			view := dialerCoverageView{Priorities: []dialerCoveragePriority{}, EmptyPriorities: []string{}, Failures: []string{}}
			order, err := decodeDialOrder(raw)
			if err != nil {
				view.FetchFailures = 1
				view.Failures = append(view.Failures, fmt.Sprintf("decoding dial order: %v", err))
				return outputDialerCoverage(cmd, flags, view, nil)
			}
			view.Priorities = make([]dialerCoveragePriority, len(order))
			view.Total = len(order)
			fetchErrors := make([]error, len(order))
			jobs := make(chan int)
			var wg sync.WaitGroup
			workers := 4
			if len(order) < workers {
				workers = len(order)
			}
			for n := 0; n < workers; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range jobs {
						item := order[i]
						id := item.SmartViewID
						if id == "" {
							id = item.ID
						}
						row := dialerCoveragePriority{Position: item.Position, SmartViewID: id, Name: item.Name}
						queue, qerr := c.Get(ctx, "/dialer/queue", map[string]string{"smartViewId": id, "limit": strconv.Itoa(limit)})
						if qerr != nil {
							fetchErrors[i] = qerr
							row.Error = fmt.Sprintf("fetching queue: %v", qerr)
						} else {
							var entries []json.RawMessage
							if e := unmarshalDataList(queue, &entries); e != nil {
								row.Error = fmt.Sprintf("decoding queue: %v", e)
							} else {
								if total, ok := responseTotal(queue); ok {
									row.QueueDepth = max(total, len(entries))
									if total < 0 || total < len(entries) || total == 0 && len(entries) > 0 {
										row.Error = fmt.Sprintf("invalid queue total %d for %d returned rows", total, len(entries))
									} else {
										row.Capped = false
									}
								} else {
									row.QueueDepth = len(entries)
									row.Capped = len(entries) == limit
								}
							}
						}
						view.Priorities[i] = row
					}
				}()
			}
			for i := range order {
				jobs <- i
			}
			close(jobs)
			wg.Wait()
			agents, err := c.Get(ctx, "/dialer/agents-status", map[string]string{})
			if err != nil {
				view.FetchFailures++
				view.Failures = append(view.Failures, fmt.Sprintf("fetching dialer agent status: %v", err))
				return outputDialerCoverage(cmd, flags, view, err)
			}
			var statuses []struct {
				Status string `json:"status"`
			}
			if err := unmarshalDataList(agents, &statuses); err != nil {
				view.FetchFailures++
				view.Failures = append(view.Failures, fmt.Sprintf("decoding dialer agent status: %v", err))
				return outputDialerCoverage(cmd, flags, view, nil)
			}
			for _, agent := range statuses {
				if strings.EqualFold(agent.Status, "Available") {
					view.AvailableAgents++
				}
			}
			for _, fetchErr := range fetchErrors {
				if fetchErr != nil {
					return outputDialerCoverage(cmd, flags, view, fetchErr)
				}
			}
			return outputDialerCoverage(cmd, flags, view, nil)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum queue rows fetched per priority")
	return cmd
}

func outputDialerCoverage(cmd *cobra.Command, flags *rootFlags, view dialerCoverageView, fetchErr error) error {
	sort.SliceStable(view.Priorities, func(i, j int) bool { return view.Priorities[i].Position < view.Priorities[j].Position })
	for _, row := range view.Priorities {
		if row.Error != "" {
			view.FetchFailures++
			view.Failures = append(view.Failures, row.Name+": "+row.Error)
		} else if row.Capped {
			view.Failures = append(view.Failures, row.Name+": --limit may have capped the queue")
		} else if row.QueueDepth == 0 {
			view.EmptyPriorities = append(view.EmptyPriorities, row.Name)
		}
		if row.Error == "" {
			view.Checked++
		}
	}
	view.Partial = len(view.Failures) > 0
	if len(view.EmptyPriorities) > 0 {
		view.Warning = fmt.Sprintf("%d priorities have an empty queue", len(view.EmptyPriorities))
	}
	if view.FetchFailures > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed\n", view.FetchFailures, len(view.Priorities))
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
	} else {
		if view.Partial {
			fmt.Fprintln(cmd.OutOrStdout(), "WARNING: coverage is partial; one or more queues failed or reached --limit.")
		}
		for _, row := range view.Priorities {
			depth := strconv.Itoa(row.QueueDepth)
			if row.Capped {
				depth += "+"
			}
			if row.Error != "" {
				depth = "error"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%d  %s  depth %s\n", row.Position, row.Name, depth)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Available agents: %d\n", view.AvailableAgents)
	}
	if view.Partial {
		if fetchErr != nil {
			return classifyAPIErrorOnly(fetchErr)
		}
		return apiErr(fmt.Errorf("dialer coverage is incomplete: %s", strings.Join(view.Failures, "; ")))
	}
	return nil
}

func novelInvocationHasFlags(cmd *cobra.Command, flags *rootFlags) bool {
	return hasChangedLocalFlags(cmd) || flags.asJSON || flags.agent || flags.compact || flags.csv || flags.plain || flags.quiet || flags.selectFields != ""
}

type dialOrderEntry struct {
	ID           string `json:"id"`
	SmartViewID  string `json:"smartViewId"`
	Name         string `json:"name"`
	Position     int    `json:"position"`
	DialPriority int    `json:"dialPriority"`
}

// decodeDialOrder accepts the live route's shape ({data:{priorities:[...], candidates:[...], total}}) as well as a bare
// list ({data:[...]} or [...]). Positions come from position, then dialPriority, then list order.
func decodeDialOrder(raw json.RawMessage) ([]dialOrderEntry, error) {
	var entries []dialOrderEntry
	if err := unmarshalDataList(raw, &entries); err != nil {
		var envelope map[string]json.RawMessage
		if err2 := json.Unmarshal(raw, &envelope); err2 != nil {
			return nil, err
		}
		if envelope["error"] != nil || envelope["message"] != nil {
			return nil, fmt.Errorf("unexpected dial order error envelope")
		}
		dataRaw, ok := envelope["data"]
		if !ok || string(dataRaw) == "null" {
			return nil, fmt.Errorf("dial order response missing data.priorities array")
		}
		var data map[string]json.RawMessage
		if err2 := json.Unmarshal(dataRaw, &data); err2 != nil {
			return nil, fmt.Errorf("decoding dial order data: %w", err2)
		}
		if data["error"] != nil || data["message"] != nil {
			return nil, fmt.Errorf("unexpected dial order error envelope")
		}
		prioritiesRaw, ok := data["priorities"]
		if !ok || string(prioritiesRaw) == "null" {
			return nil, fmt.Errorf("dial order response missing data.priorities array")
		}
		if err2 := json.Unmarshal(prioritiesRaw, &entries); err2 != nil {
			return nil, fmt.Errorf("decoding data.priorities: %w", err2)
		}
	}
	for i := range entries {
		if entries[i].Position == 0 {
			if entries[i].DialPriority != 0 {
				entries[i].Position = entries[i].DialPriority
			} else {
				entries[i].Position = i + 1
			}
		}
	}
	if entries == nil {
		entries = []dialOrderEntry{}
	}
	return entries, nil
}

func unmarshalDataList(raw json.RawMessage, dst any) error {
	if err := rejectResponseErrorEnvelope(raw); err != nil {
		return err
	}
	if hoisted, ok := hoistPaginatedEnvelope(raw); ok {
		raw = hoisted
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		raw = envelope.Data
	}
	return json.Unmarshal(raw, dst)
}

func rejectResponseErrorEnvelope(raw json.RawMessage) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	// Any non-null error value marks an error envelope, regardless of its JSON
	// type. A non-empty string or object message does the same at the response
	// root and in data envelopes; null and empty-string messages remain valid.
	for depth := 0; envelope != nil; depth++ {
		if value, ok := envelope["error"]; ok && string(bytes.TrimSpace(value)) != "null" {
			return fmt.Errorf("unexpected error envelope")
		}
		if value, ok := envelope["message"]; ok {
			var text string
			if json.Unmarshal(value, &text) == nil && text != "" {
				return fmt.Errorf("unexpected message envelope: %s", text)
			}
			var detail map[string]json.RawMessage
			if json.Unmarshal(value, &detail) == nil && detail != nil {
				return fmt.Errorf("unexpected message envelope")
			}
		}
		data, ok := envelope["data"]
		var nested map[string]json.RawMessage
		if !ok || json.Unmarshal(data, &nested) != nil {
			break
		}
		envelope = nested
	}
	return nil
}
