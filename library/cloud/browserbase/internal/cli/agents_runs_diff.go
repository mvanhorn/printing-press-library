// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type agentRunDiffView struct {
	RunA            string   `json:"run_a"`
	RunB            string   `json:"run_b"`
	StatusA         string   `json:"status_a,omitempty"`
	StatusB         string   `json:"status_b,omitempty"`
	MessagesAdded   int      `json:"messages_added"`
	MessagesRemoved int      `json:"messages_removed"`
	ResultChanged   bool     `json:"result_changed"`
	DiffLines       []string `json:"diff_lines"`
}

func newNovelAgentsRunsDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <runA> <runB>",
		Short: "Compare two agent runs structurally — message sequences and final structured results — to see what changed between prompt iterations.",
		Long: `Use this command to compare two specific runs of an agent.
Do NOT use it for weekly aggregation of agent-run activity; use 'projects digest' instead.`,
		Example: "  browserbase-pp-cli agents runs diff 52f6b13d-eb27-436d-86ff-356b2fd01697 2d310606-42fa-483c-9a7b-7102a85ddb09 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "runA=52f6b13d-eb27-436d-86ff-356b2fd01697;runB=2d310606-42fa-483c-9a7b-7102a85ddb09",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument (runA runB)"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "agents runs diff")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			runA, runB := args[0], args[1]

			fetchRun := func(runID string) (status string, messages []string, result json.RawMessage, err error) {
				// Use no-cache reads so a prior runs get/list call cannot
				// serve a stale snapshot for the diff.
				path := replacePathParam("/v1/agents/runs/{runId}", "runId", runID)
				data, err := c.GetNoCache(ctx, path, nil)
				if err != nil {
					return "", nil, nil, fmt.Errorf("fetching run %s: %w", runID, err)
				}
				var run struct {
					Status string          `json:"status"`
					Result json.RawMessage `json:"result"`
				}
				if err := json.Unmarshal(data, &run); err != nil {
					return "", nil, nil, fmt.Errorf("parsing run %s: %w", runID, err)
				}
				// Fetch messages (paginated list, up to 3 pages of 100).
				msgPath := replacePathParam("/v1/agents/runs/{runId}/messages", "runId", runID)
				var allMessages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}
				for page := 1; page <= 3; page++ {
					msgData, err := c.GetNoCache(ctx, msgPath, map[string]string{"limit": "100", "offset": fmt.Sprintf("%d", (page-1)*100)})
					if err != nil {
						break // messages optional; still diff result
					}
					var pageMsgs []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}
					if json.Unmarshal(msgData, &pageMsgs) != nil {
						var wrapped struct {
							Messages []struct {
								Role    string `json:"role"`
								Content string `json:"content"`
							} `json:"messages"`
						}
						if json.Unmarshal(msgData, &wrapped) != nil {
							break
						}
						pageMsgs = wrapped.Messages
					}
					if len(pageMsgs) == 0 {
						break
					}
					allMessages = append(allMessages, pageMsgs...)
					if len(pageMsgs) < 100 {
						break
					}
				}
				messages = make([]string, 0, len(allMessages))
				for _, m := range allMessages {
					messages = append(messages, fmt.Sprintf("%s: %s", m.Role, strings.TrimSpace(m.Content)))
				}
				return run.Status, messages, run.Result, nil
			}

			statusA, msgsA, resultA, err := fetchRun(runA)
			if err != nil {
				return err
			}
			statusB, msgsB, resultB, err := fetchRun(runB)
			if err != nil {
				return err
			}

			// Simple LCS-based diff of message sequences.
			diff := lcsDiff(msgsA, msgsB)

			added, removed := 0, 0
			for _, line := range diff {
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					added++
				}
				if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
					removed++
				}
			}

			resultChanged := false
			// Canonicalize both results (key order / whitespace should not
			// count as a change); one-sided absence still counts as changed.
			var canonA, canonB any
			if len(resultA) > 0 && json.Unmarshal(resultA, &canonA) != nil {
				canonA = string(resultA)
			}
			if len(resultB) > 0 && json.Unmarshal(resultB, &canonB) != nil {
				canonB = string(resultB)
			}
			if len(resultA) > 0 || len(resultB) > 0 {
				aJSON, _ := json.Marshal(canonA)
				bJSON, _ := json.Marshal(canonB)
				resultChanged = string(aJSON) != string(bJSON)
			}

			view := agentRunDiffView{
				RunA:            runA,
				RunB:            runB,
				StatusA:         statusA,
				StatusB:         statusB,
				MessagesAdded:   added,
				MessagesRemoved: removed,
				ResultChanged:   resultChanged,
				DiffLines:       diff,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(diff) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "runs %s and %s have identical message sequences\n", runA, runB)
				return nil
			}
			for _, line := range diff {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d added, %d removed; result changed: %v\n", added, removed, resultChanged)
			return nil
		},
	}
	return cmd
}

// lcsDiff returns a unified-diff-style line list between two string slices
// using a longest-common-subsequence alignment.
func lcsDiff(a, b []string) []string {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		out := make([]string, 0, m)
		for _, l := range b {
			out = append(out, "+ "+l)
		}
		return out
	}
	if m == 0 {
		out := make([]string, 0, n)
		for _, l := range a {
			out = append(out, "- "+l)
		}
		return out
	}
	// DP table for LCS length.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = dp[i-1][j]
				if dp[i][j-1] > dp[i][j] {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}
	// Backtrack.
	out := make([]string, 0, n+m)
	i, j := n, m
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			out = append(out, "  "+a[i-1])
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			out = append(out, "- "+a[i-1])
			i--
		} else {
			out = append(out, "+ "+b[j-1])
			j--
		}
	}
	for i > 0 {
		out = append(out, "- "+a[i-1])
		i--
	}
	for j > 0 {
		out = append(out, "+ "+b[j-1])
		j--
	}
	// Reverse.
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}
