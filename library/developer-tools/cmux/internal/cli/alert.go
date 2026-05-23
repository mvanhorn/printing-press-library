// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/snapshotstore"

	"github.com/spf13/cobra"
)

// newAlertCmd is the parent for the alert subcommands.
func newAlertCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alert",
		Short: "Declare state-transition alerts that fire to external sinks",
		Long: `Alert rules watch the local snapshot store for workspace state
transitions. When a (workspace, key) flips into the rule's on_state,
the configured sink fires (stdout, file, exec, macOS notification,
Slack webhook, or generic webhook).

Each snapshot or sync evaluates every rule against the just-recorded
transitions and writes an alert_fires row with the sink's outcome.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAlertAddCmd(flags))
	cmd.AddCommand(newAlertListCmd(flags))
	cmd.AddCommand(newAlertRemoveCmd(flags))
	cmd.AddCommand(newAlertFiresCmd(flags))
	cmd.AddCommand(newAlertTestCmd(flags))
	return cmd
}

func newAlertAddCmd(flags *rootFlags) *cobra.Command {
	var workspace, key, onState, sink, label string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an alert rule",
		Example: `  cmux-pp-cli alert add --workspace Tuck --on awaiting --sink slack:https://hooks.slack.com/services/X --label tuck-stuck
  cmux-pp-cli alert add --on awaiting --sink macos:cmux`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// No flags given: show help so help-only / dogfood-happy-path
			// invocations don't fail. Real callers always pass --on + --sink.
			if onState == "" && sink == "" {
				return cmd.Help()
			}
			if onState == "" {
				return fmt.Errorf("--on is required (one of: working, awaiting, idle, stranded)")
			}
			if sink == "" {
				return fmt.Errorf("--sink is required (e.g. 'macos:' or 'slack:<webhook url>')")
			}
			if _, err := ParseSink(sink); err != nil {
				return err
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"dry_run":   true,
						"workspace": workspace,
						"key":       key,
						"on":        onState,
						"sink":      sink,
						"label":     label,
					})
				}
				return nil
			}
			ref, err := resolveWorkspaceArg(ctx, workspace)
			if err != nil {
				return err
			}
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			id, err := ss.AddAlertRule(ctx, ref, key, onState, sink, label)
			if err != nil {
				return err
			}
			out := map[string]any{
				"id":        id,
				"workspace": ref,
				"key":       key,
				"on":        onState,
				"sink":      sink,
				"label":     label,
			}
			return jsonOrHuman(cmd, flags, out, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "alert added: id=%d on=%s sink=%s\n", id, onState, sink)
			})
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "limit to one workspace (ref, index, or title substring); omit to match all")
	cmd.Flags().StringVar(&key, "key", "claude_code", "status key to match (defaults to claude_code)")
	cmd.Flags().StringVar(&onState, "on", "", "canonical state to fire on (working / awaiting / idle / stranded)")
	cmd.Flags().StringVar(&sink, "sink", "", "destination: 'stdout', 'file:/path', 'exec:/path', 'slack:<url>', 'webhook:<url>', or 'macos:<title>'")
	cmd.Flags().StringVar(&label, "label", "", "optional human label rendered with each fire")
	return cmd
}

func newAlertListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List configured alert rules",
		Example:     "  cmux-pp-cli alert list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			rules, err := ss.ListAlertRules(ctx)
			if err != nil {
				return err
			}
			return renderRows(cmd, flags, rules, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "ID\tWORKSPACE\tKEY\tON\tSINK\tLABEL")
				for _, r := range rules {
					fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.WorkspaceRef, r.Key, r.OnState, r.Sink, r.Label)
				}
			})
		},
	}
	return cmd
}

func newAlertRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove [id]",
		Short:       "Remove an alert rule by id",
		Example:     "  cmux-pp-cli alert remove 2",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"dry_run": true,
						"id":      args[0],
					})
				}
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid rule id %q: %w", args[0], err)
			}
			ctx := cmd.Context()
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			removed, err := ss.RemoveAlertRule(ctx, id)
			if err != nil {
				return err
			}
			return jsonOrHuman(cmd, flags, map[string]any{"removed": removed, "id": id}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "removed=%d id=%d\n", removed, id)
			})
		},
	}
	return cmd
}

func newAlertFiresCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "fires",
		Short:       "Show the recent alert_fires log",
		Example:     "  cmux-pp-cli alert fires --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			fires, err := ss.ListAlertFires(ctx, limit)
			if err != nil {
				return err
			}
			return renderRows(cmd, flags, fires, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "TIME\tRULE\tWORKSPACE\tPREV→NEW\tSINK\tOUTCOME")
				for _, f := range fires {
					ts := time.Unix(int64(f.FiredAtUnix), 0).Format("15:04:05")
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%s\t%s→%s\t%s\t%s\n", ts, f.RuleID, f.WorkspaceRef, f.PrevValue, f.NewValue, f.Sink, f.Outcome)
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "max rows to return")
	return cmd
}

func newAlertTestCmd(flags *rootFlags) *cobra.Command {
	var sink, title, body string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Fire a synthetic event to a sink without creating a rule",
		Long: `Use this to verify a sink works (webhook URL is valid, exec script
runs, macOS notification permission is granted). When PRINTING_PRESS_VERIFY
or PRINTING_PRESS_DOGFOOD is set, the command prints what would be sent
instead of dialing out.`,
		Example: `  cmux-pp-cli alert test --sink macos: --title cmux --body smoke
  cmux-pp-cli alert test --sink slack:https://hooks.slack.com/services/X`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if sink == "" {
				// Help-only / dogfood happy-path: show help instead of erroring.
				return cmd.Help()
			}
			s, err := ParseSink(sink)
			if err != nil {
				return err
			}
			event := map[string]any{
				"workspace_title": "test",
				"prev_value":      "",
				"new_value":       "awaiting",
				"title":           title,
				"body":            body,
			}
			if isVerifyOrDogfood() {
				fmt.Fprintf(cmd.OutOrStdout(), "would emit to sink %s: %+v\n", sink, event)
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			outcome, err := s.Emit(cmd.Context(), event)
			if err != nil {
				return err
			}
			return jsonOrHuman(cmd, flags, map[string]any{"sink": sink, "outcome": outcome}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "sink=%s outcome=%s\n", sink, outcome)
			})
		},
	}
	cmd.Flags().StringVar(&sink, "sink", "", "destination spec (e.g. 'macos:', 'slack:<url>')")
	cmd.Flags().StringVar(&title, "title", "cmux-pp-cli test", "alert title")
	cmd.Flags().StringVar(&body, "body", "smoke test", "alert body")
	return cmd
}

// evaluateAlerts is called by takeSnapshot. It scans the most recent
// transition rows (since the last alert run) and fires any matching rules.
// Idempotent guard: we look up the most-recent fire timestamp per rule and
// only fire on transitions newer than that.
func evaluateAlerts(ctx context.Context, ss *snapshotstore.Store) error {
	rules, err := ss.ListAlertRules(ctx)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}
	wss, err := cmuxclient.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	wsIdx := indexWorkspacesByRef(wss)

	// Most recent fire per rule (anchor).
	fires, err := ss.ListAlertFires(ctx, 500)
	if err != nil {
		return err
	}
	lastByRule := make(map[int64]float64)
	for _, f := range fires {
		if f.FiredAtUnix > lastByRule[f.RuleID] {
			lastByRule[f.RuleID] = f.FiredAtUnix
		}
	}

	// Recent transitions.
	transitions, err := ss.Changes(ctx, 0)
	if err != nil {
		return err
	}

	for _, t := range transitions {
		for _, r := range rules {
			if r.WorkspaceRef != "" && r.WorkspaceRef != t.WorkspaceRef {
				continue
			}
			if r.Key != "" && r.Key != t.Key {
				continue
			}
			if r.OnState != t.Canonical {
				continue
			}
			if t.ObservedAtUnix <= lastByRule[r.ID] {
				continue
			}
			sink, err := ParseSink(r.Sink)
			if err != nil {
				_ = ss.RecordAlertFire(ctx, r.ID, t.WorkspaceRef, t.Key, t.PrevValue, t.Value, r.Sink, "parse-error", err.Error())
				continue
			}
			event := map[string]any{
				"rule_id":         r.ID,
				"label":           r.Label,
				"workspace_ref":   t.WorkspaceRef,
				"workspace_title": wsIdx[t.WorkspaceRef].Title,
				"key":             t.Key,
				"prev_value":      t.PrevValue,
				"new_value":       t.Value,
				"canonical":       t.Canonical,
				"transitioned_at": t.ObservedAtUnix,
			}
			if isVerifyOrDogfood() {
				_ = ss.RecordAlertFire(ctx, r.ID, t.WorkspaceRef, t.Key, t.PrevValue, t.Value, r.Sink, "dry-run", "")
				continue
			}
			outcome, emitErr := sink.Emit(ctx, event)
			if emitErr != nil {
				_ = ss.RecordAlertFire(ctx, r.ID, t.WorkspaceRef, t.Key, t.PrevValue, t.Value, r.Sink, "error", emitErr.Error())
				continue
			}
			_ = ss.RecordAlertFire(ctx, r.ID, t.WorkspaceRef, t.Key, t.PrevValue, t.Value, r.Sink, outcome, "")
		}
	}
	return nil
}

func jsonOrHuman(cmd *cobra.Command, flags *rootFlags, v any, human func()) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	if human != nil {
		human()
	}
	return nil
}

// isVerifyOrDogfood returns true when either PRINTING_PRESS_VERIFY or
// PRINTING_PRESS_DOGFOOD is set in the environment. Mirrors cliutil but kept
// local so this file doesn't reach into the generator-reserved namespace.
func isVerifyOrDogfood() bool {
	if v := os.Getenv("PRINTING_PRESS_VERIFY"); v != "" && v != "0" {
		return true
	}
	if v := os.Getenv("PRINTING_PRESS_DOGFOOD"); v != "" && v != "0" {
		return true
	}
	return false
}
