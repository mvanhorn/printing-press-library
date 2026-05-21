// Copyright 2026 error. Licensed under Apache-2.0.
// Transcendence command: poll ListTasks per agent and print state transitions
// live. "Is ori still working?" answered without paying for a Claude Code turn.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/cliutil"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var agentFilter string
	var interval time.Duration
	var iterations int
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll ListTasks and print state transitions live",
		Long: `Polls /a2a/{agent}/tasks for every configured agent every --interval and
prints lines like:

  23:01:17  ori   task abc12345  running → input_required
  23:01:22  adam  task def78901  running → completed

By default polls forever; pass --iterations N to bound. Under PRINTING_PRESS_VERIFY
the watcher prints a single-shot snapshot and exits.`,
		Example: `  ori-pp-cli watch
  ori-pp-cli watch --agent ori --interval 3s
  ori-pp-cli watch --iterations 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve agent list.
			var agents []string
			if agentFilter != "" {
				agents = []string{agentFilter}
			} else {
				agents, err = discoverAgentNames(c)
				if err != nil {
					return apiErr(err)
				}
			}

			// Under verify env OR non-TTY stdout with no explicit iterations:
			// print a single snapshot and exit. This makes watch script-safe and
			// dogfood-friendly. Interactive TTY users get the loop they expect.
			if cliutil.IsVerifyEnv() || (!isTerminal(cmd.OutOrStdout()) && iterations == 0) {
				snap, _ := pollTasksSnapshot(c, agents)
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}

			ctx := cmd.Context()
			prev := map[string]string{} // key: agent|task_id -> last state
			w := cmd.OutOrStdout()
			iter := 0

			tick := time.NewTicker(interval)
			defer tick.Stop()
			// First read immediately.
			report := pollAndPrintTransitions(c, agents, prev, w, flags)
			iter++
			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
					return err
				}
			}
			if iterations > 0 && iter >= iterations {
				return nil
			}

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-tick.C:
					report := pollAndPrintTransitions(c, agents, prev, w, flags)
					iter++
					if flags.asJSON {
						if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
							return err
						}
					}
					if iterations > 0 && iter >= iterations {
						return nil
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&agentFilter, "agent", "", "Limit to one agent (default: all configured)")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Poll interval")
	cmd.Flags().IntVar(&iterations, "iterations", 0, "Stop after N polls (0 = forever)")
	return cmd
}

type taskSnap struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
	Agent  string `json:"agent"`
}

func pollTasksSnapshot(c *client.Client, agents []string) (map[string]any, error) {
	out := map[string]any{}
	all := []taskSnap{}
	for _, name := range agents {
		body, err := c.Get("/a2a/"+name+"/tasks", map[string]string{"page_size": "50"})
		if err != nil {
			out["error_"+name] = err.Error()
			continue
		}
		var resp struct {
			Tasks []struct {
				ID     string `json:"id"`
				Status struct {
					State string `json:"state"`
				} `json:"status"`
			} `json:"tasks"`
		}
		if jerr := json.Unmarshal(body, &resp); jerr != nil {
			continue
		}
		for _, t := range resp.Tasks {
			all = append(all, taskSnap{TaskID: t.ID, State: shortenState(t.Status.State), Agent: name})
		}
	}
	out["tasks"] = all
	out["agents"] = agents
	out["at"] = time.Now().UTC().Format(time.RFC3339)
	return out, nil
}

func pollAndPrintTransitions(c *client.Client, agents []string, prev map[string]string, w interface{ Write([]byte) (int, error) }, flags *rootFlags) map[string]any {
	current := map[string]string{}
	transitions := []map[string]string{}
	for _, name := range agents {
		body, err := c.Get("/a2a/"+name+"/tasks", map[string]string{"page_size": "50"})
		if err != nil {
			continue
		}
		var resp struct {
			Tasks []struct {
				ID     string `json:"id"`
				Status struct {
					State string `json:"state"`
				} `json:"status"`
			} `json:"tasks"`
		}
		if jerr := json.Unmarshal(body, &resp); jerr != nil {
			continue
		}
		for _, t := range resp.Tasks {
			state := shortenState(t.Status.State)
			key := name + "|" + t.ID
			current[key] = state
			old, had := prev[key]
			if !had {
				transitions = append(transitions, map[string]string{"agent": name, "task_id": t.ID, "from": "(new)", "to": state})
			} else if old != state {
				transitions = append(transitions, map[string]string{"agent": name, "task_id": t.ID, "from": old, "to": state})
			}
		}
	}
	// detect disappeared tasks
	for key, oldState := range prev {
		if _, still := current[key]; !still {
			parts := strings.SplitN(key, "|", 2)
			if len(parts) == 2 {
				transitions = append(transitions, map[string]string{"agent": parts[0], "task_id": parts[1], "from": oldState, "to": "(gone)"})
			}
		}
	}
	// stable order by agent, then task_id
	sort.Slice(transitions, func(i, j int) bool {
		if transitions[i]["agent"] != transitions[j]["agent"] {
			return transitions[i]["agent"] < transitions[j]["agent"]
		}
		return transitions[i]["task_id"] < transitions[j]["task_id"]
	})

	if !flags.asJSON {
		now := time.Now().Format("15:04:05")
		for _, t := range transitions {
			short := t["task_id"]
			if len(short) > 12 {
				short = short[:8]
			}
			fmt.Fprintf(w, "%s  %-4s  task %s  %s → %s\n", now, t["agent"], short, t["from"], t["to"])
		}
	}

	// swap prev := current
	for k := range prev {
		delete(prev, k)
	}
	for k, v := range current {
		prev[k] = v
	}

	return map[string]any{
		"transitions": transitions,
		"at":          time.Now().UTC().Format(time.RFC3339),
		"tasks_seen":  len(current),
	}
}

// shortenState collapses A2A protobuf enum names ("TASK_STATE_RUNNING") to
// human-readable lowercase ("running"). Unknown values are returned as-is.
func shortenState(s string) string {
	const prefix = "TASK_STATE_"
	if strings.HasPrefix(s, prefix) {
		return strings.ToLower(strings.TrimPrefix(s, prefix))
	}
	return s
}

// discoverAgentNames probes /.well-known/agents.json to learn the configured
// agent list. Used by every cross-agent command.
func discoverAgentNames(c *client.Client) ([]string, error) {
	body, err := c.Get("/.well-known/agents.json", nil)
	if err != nil {
		return nil, fmt.Errorf("agent discovery: %w", err)
	}
	var payload struct {
		Agents []string `json:"agents"`
	}
	if jerr := json.Unmarshal(body, &payload); jerr != nil {
		return nil, fmt.Errorf("agent discovery: parse: %w", jerr)
	}
	return payload.Agents, nil
}

// _ = ctx usage placeholder to keep context import live for callers that pass it.
var _ = context.Background
