package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// jobRecord is a lightweight record of a background operation persisted to disk.
type jobRecord struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"` // pending | running | done | failed
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Detail    string    `json:"detail,omitempty"`
}

func jobsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "anylist-pp-cli", "jobs")
}

func newJobsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage background operations (batch sync, bulk imports)",
		Long: `Track and manage background operations started by long-running commands.

Batch imports and multi-URL recipe imports can be submitted as jobs and
polled to completion. Use 'jobs list' to see pending work, 'jobs status'
to inspect a specific job, and 'jobs wait' to block until it finishes.`,
		Example: `  anylist-pp-cli jobs list
  anylist-pp-cli jobs status <id>
  anylist-pp-cli jobs wait <id> --timeout 60s`,
	}
	cmd.AddCommand(newJobsListCmd(flags))
	cmd.AddCommand(newJobsStatusCmd(flags))
	cmd.AddCommand(newJobsWaitCmd(flags))
	return cmd
}

func newJobsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List all tracked background jobs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := jobsDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), []jobRecord{}, flags)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "No jobs found")
					return nil
				}
				return fmt.Errorf("reading jobs dir: %w", err)
			}

			var jobs []jobRecord
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					continue
				}
				var j jobRecord
				if json.Unmarshal(data, &j) == nil {
					jobs = append(jobs, j)
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), jobs, flags)
			}

			if len(jobs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No jobs found")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tKIND\tSTATUS\tSTARTED")
			for _, j := range jobs {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", j.ID, j.Kind, j.Status, j.StartedAt.Format("2006-01-02 15:04:05"))
			}
			return tw.Flush()
		},
	}
}

func newJobsStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status <id>",
		Short:       "Show status of a specific job",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			path := filepath.Join(jobsDir(), id+".json")
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return &cliError{code: 3, err: fmt.Errorf("job %q not found", id)}
				}
				return fmt.Errorf("reading job: %w", err)
			}
			var j jobRecord
			if err := json.Unmarshal(data, &j); err != nil {
				return fmt.Errorf("parsing job: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), j, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ID:      %s\nKind:    %s\nStatus:  %s\nStarted: %s\n",
				j.ID, j.Kind, j.Status, j.StartedAt.Format(time.RFC3339))
			if j.Detail != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Detail:  %s\n", j.Detail)
			}
			return nil
		},
	}
}

func newJobsWaitCmd(flags *rootFlags) *cobra.Command {
	var timeout time.Duration
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "wait <id>",
		Short: "Block until a job reaches done or failed status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			path := filepath.Join(jobsDir(), id+".json")
			deadline := time.Now().Add(timeout)

			for {
				data, err := os.ReadFile(path)
				if err != nil {
					if os.IsNotExist(err) {
						return &cliError{code: 3, err: fmt.Errorf("job %q not found", id)}
					}
					return fmt.Errorf("reading job: %w", err)
				}
				var j jobRecord
				if err := json.Unmarshal(data, &j); err != nil {
					return fmt.Errorf("parsing job: %w", err)
				}
				if j.Status == "done" || j.Status == "failed" {
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), j, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Job %s: %s\n", id, j.Status)
					if j.Status == "failed" {
						return &cliError{code: 5, err: fmt.Errorf("job %s failed: %s", id, j.Detail)}
					}
					return nil
				}
				if time.Now().After(deadline) {
					return &cliError{code: 5, err: fmt.Errorf("timed out waiting for job %s (status: %s)", id, j.Status)}
				}
				time.Sleep(interval)
			}
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Poll interval")
	return cmd
}
