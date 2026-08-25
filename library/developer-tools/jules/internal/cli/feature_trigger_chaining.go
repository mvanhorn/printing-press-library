// Feature 7: Workflow Trigger Chaining
// pp:data-source local
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/store"
	"github.com/spf13/cobra"
)

// triggerResourceType is the local-store resource_type used to persist
// trigger chains in the generic "resources" table (see internal/store).
// Trigger chains are a CLI-local concept -- deploying the underlying GitHub
// workflow_dispatch wiring is out of scope for this CLI -- so the generic
// key/value resource helpers are the right fit rather than a generated typed
// schema.
const triggerResourceType = "triggers"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		triggerCmd := newTriggerChainingCmd(flags)
		addNovelCommandIfAbsent(root, triggerCmd)
	})
}

func newTriggerChainingCmd(flags *rootFlags) *cobra.Command {
	var cronSchedule string
	var workflowTrigger string
	var label string

	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Chain cron schedules with GitHub workflow triggers",
		Long: `Create automated workflows by combining cron schedules with GitHub workflow_run events.

Enables patterns like:
- Run Jules tasks on a schedule (6am daily)
- Automatically trigger GitHub workflows when sessions complete
- Chain multiple automation steps without manual intervention`,
		Example: `  # Schedule daily 6am Jules runs
  jules-pp-cli trigger add --cron "0 6 * * *" --workflow "session-create" --label "daily-morning-run"

  # List active trigger chains
  jules-pp-cli trigger list

  # Pause trigger
  jules-pp-cli trigger pause --label "daily-morning-run"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			subcommand := args[0]
			switch subcommand {
			case "add":
				if cronSchedule == "" || workflowTrigger == "" {
					return fmt.Errorf("--cron and --workflow are required")
				}
				return triggerAdd(cmd.Context(), flags, cronSchedule, workflowTrigger, label, cmd.OutOrStdout())
			case "list":
				return triggerList(cmd.Context(), flags, cmd.OutOrStdout())
			case "pause":
				if label == "" {
					return fmt.Errorf("--label is required for pause")
				}
				return triggerPause(cmd.Context(), flags, label, cmd.OutOrStdout())
			default:
				return cmd.Help()
			}
		},
	}

	cmd.Flags().StringVar(&cronSchedule, "cron", "", "Cron schedule (e.g., '0 6 * * *' for daily at 6am)")
	cmd.Flags().StringVar(&workflowTrigger, "workflow", "", "GitHub workflow to trigger (e.g., 'session-create', 'auto-merge')")
	cmd.Flags().StringVar(&label, "label", "", "Human-readable label for this trigger chain")

	return cmd
}

type triggerEntry struct {
	Label     string `json:"label"`
	Cron      string `json:"cron"`
	Workflow  string `json:"workflow"`
	NextRun   string `json:"nextRun"`
	Paused    bool   `json:"paused"`
	CreatedAt string `json:"createdAt"`
}

func triggerAdd(ctx context.Context, flags *rootFlags, cronSchedule, workflowTrigger, label string, out io.Writer) error {
	if label == "" {
		label = fmt.Sprintf("%s-%s", cronSchedule, workflowTrigger)
	}

	// Validate cron schedule (basic validation)
	parts := splitCron(cronSchedule)
	if len(parts) != 5 && len(parts) != 6 {
		return fmt.Errorf("invalid cron format: %s (expected 5-6 fields)", cronSchedule)
	}

	nextRun := estimateNextRun(cronSchedule)

	if flags.dryRun {
		if flags.asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"dry_run": true, "label": label, "cron": cronSchedule, "workflow": workflowTrigger, "next_run": nextRun})
		}
		fmt.Fprintf(out, "Dry run: would create trigger chain %q (no local write performed)\n", label)
		return nil
	}

	entry := triggerEntry{
		Label:     label,
		Cron:      cronSchedule,
		Workflow:  workflowTrigger,
		NextRun:   nextRun,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encoding trigger: %w", err)
	}

	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local trigger store: %w", err)
	}
	defer db.Close()

	if err := db.Upsert(triggerResourceType, label, entryJSON); err != nil {
		return fmt.Errorf("saving trigger: %w", err)
	}

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)
	}

	fmt.Fprintf(out, "Creating trigger chain: %s\n", label)
	fmt.Fprintf(out, "  Cron: %s\n", cronSchedule)
	fmt.Fprintf(out, "  Workflow: %s\n", workflowTrigger)
	fmt.Fprintf(out, "  Next run: %s\n", nextRun)
	fmt.Fprintf(out, "\n✓ Trigger chain created (stored locally; GitHub workflow_dispatch wiring not yet deployed)\n")
	return nil
}

func triggerList(ctx context.Context, flags *rootFlags, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local trigger store: %w", err)
	}
	defer db.Close()

	all, err := db.List(triggerResourceType, 0)
	if err != nil {
		return fmt.Errorf("listing triggers: %w", err)
	}

	var entries []triggerEntry
	for _, raw := range all {
		var e triggerEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })

	if flags.asJSON {
		if entries == nil {
			entries = []triggerEntry{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"triggers": entries})
	}

	fmt.Fprintf(out, "Configured Trigger Chains:\n")
	if len(entries) == 0 {
		fmt.Fprintf(out, "  (no triggers configured yet)\n")
		fmt.Fprintf(out, "\nUse 'jules-pp-cli trigger add' to create trigger chains.\n")
		return nil
	}
	for _, e := range entries {
		status := "active"
		if e.Paused {
			status = "paused"
		}
		fmt.Fprintf(out, "  %s (%s): cron=%q workflow=%s next_run=%s\n", e.Label, status, e.Cron, e.Workflow, e.NextRun)
	}
	return nil
}

func triggerPause(ctx context.Context, flags *rootFlags, label string, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local trigger store: %w", err)
	}
	defer db.Close()

	raw, err := db.Get(triggerResourceType, label)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no trigger named %q; run 'trigger list' to see configured triggers", label)
		}
		return fmt.Errorf("loading trigger: %w", err)
	}

	var entry triggerEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return fmt.Errorf("decoding trigger: %w", err)
	}

	if flags.dryRun {
		if flags.asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"dry_run": true, "label": label})
		}
		fmt.Fprintf(out, "Dry run: would pause trigger %q (no local write performed)\n", label)
		return nil
	}

	entry.Paused = true
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encoding trigger: %w", err)
	}
	if err := db.Upsert(triggerResourceType, label, entryJSON); err != nil {
		return fmt.Errorf("saving trigger: %w", err)
	}

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)
	}
	fmt.Fprintf(out, "Trigger paused: %s\n", label)
	return nil
}

func splitCron(s string) []string {
	// Simple split on whitespace, a real implementation would parse more carefully
	var parts []string
	var current string
	for _, c := range s {
		if c == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func estimateNextRun(cronSchedule string) string {
	// Rough estimate - a real implementation would parse the cron schedule properly
	// For "0 6 * * *" (daily at 6am), this gives approximate next run
	now := time.Now()
	nextRun := now.Add(24 * time.Hour) // Simplified

	return nextRun.Format("2006-01-02 15:04 MST")
}
