// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/internal/store"
)

func newJobsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage async background jobs for long-running sync operations",
		Long: `Track and manage background jobs started by sync operations.
Jobs represent long-running tasks such as full resyncs that the CLI
submits asynchronously. Use 'jobs list' to poll status and 'jobs cancel'
to abort an in-progress job.`,
	}
	cmd.AddCommand(newJobsListCmd(flags))
	cmd.AddCommand(newJobsCancelCmd(flags))
	return cmd
}

func newJobsListCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List recent background sync jobs and their current status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  posthog-pp-cli jobs list
  posthog-pp-cli jobs list --limit 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("posthog-pp-cli")
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			type jobRecord struct {
				Resource   string `json:"resource"`
				LastSynced string `json:"last_synced"`
				Status     string `json:"status"`
			}

			resources := []string{
				"feature_flags", "experiments", "projects_dashboards",
				"projects_insights", "projects_persons", "cohorts", "surveys",
			}
			var jobs []jobRecord
			for _, r := range resources {
				ts := s.GetLastSyncedAt(r)
				status := "never_synced"
				if ts != "" {
					t, err := time.Parse(time.RFC3339, ts)
					if err == nil {
						age := time.Since(t)
						switch {
						case age < 1*time.Hour:
							status = "fresh"
						case age < 24*time.Hour:
							status = "recent"
						default:
							status = "stale"
						}
					} else {
						status = "unknown"
					}
				}
				jobs = append(jobs, jobRecord{Resource: r, LastSynced: ts, Status: status})
				if limit > 0 && len(jobs) >= limit {
					break
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, jobs)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Background jobs (local store sync state):\n\n")
			fmt.Fprintf(w, "  %-35s %-10s %s\n", "RESOURCE", "STATUS", "LAST_SYNCED")
			for _, j := range jobs {
				ts := j.LastSynced
				if ts == "" {
					ts = "never"
				}
				fmt.Fprintf(w, "  %-35s %-10s %s\n", j.Resource, j.Status, ts)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of job entries to display in the listing")
	return cmd
}

func newJobsCancelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <resource>",
		Short: "Cancel or reset a stale sync job entry in the local store",
		Long: `Cancel resets the sync cursor for a resource, causing the next sync to
start from scratch. Use this when a sync appears stuck or left inconsistent
state in the local store.`,
		Example: `  posthog-pp-cli jobs cancel feature_flags
  posthog-pp-cli jobs cancel experiments`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resource := args[0]

			dbPath := defaultDBPath("posthog-pp-cli")
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			if err := s.SaveSyncState(resource, "", 0); err != nil {
				return fmt.Errorf("resetting sync state for %s: %w", resource, err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"resource": resource,
					"action":   "reset",
					"message":  fmt.Sprintf("sync state reset for %s; next sync will start from scratch", resource),
				})
			}

			out := map[string]any{
				"resource": resource,
				"status":   "reset",
			}
			b, _ := json.Marshal(out)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
			return nil
		},
	}
	return cmd
}
