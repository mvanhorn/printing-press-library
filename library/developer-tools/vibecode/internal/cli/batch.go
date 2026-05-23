// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

func newBatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Batch operations across multiple projects",
		Long:  `Perform operations across multiple projects matching a pattern.`,
	}

	cmd.AddCommand(newBatchDeployCmd(flags))
	return cmd
}

func newBatchDeployCmd(flags *rootFlags) *cobra.Command {
	var pattern string
	var parallel int
	var confirm bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy multiple projects matching a glob pattern",
		Long: `Orchestrate deployments across multiple projects with parallelism control.

Uses glob patterns to match project names from the local cache. Supports
parallel deployments with configurable concurrency.`,
		Example: `  vibecode-pp-cli batch deploy --pattern "frontend-*"
  vibecode-pp-cli batch deploy --pattern "*-prod" --parallel 3
  vibecode-pp-cli batch deploy --pattern "team-*" --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}

			if pattern == "" {
				return fmt.Errorf("--pattern is required")
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Find projects matching the pattern
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM resources
				WHERE resource_type = 'projects'
			`)
			if err != nil {
				return fmt.Errorf("querying projects: %w", err)
			}
			defer rows.Close()

			type project struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}

			var matchingProjects []project
			for rows.Next() {
				var id, dataStr string
				if err := rows.Scan(&id, &dataStr); err != nil {
					continue
				}

				var data map[string]any
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					continue
				}

				name, _ := data["name"].(string)
				if name == "" {
					name = id
				}

				// Match against pattern
				matched, err := filepath.Match(pattern, name)
				if err != nil {
					return fmt.Errorf("invalid pattern %q: %w", pattern, err)
				}
				if matched {
					matchingProjects = append(matchingProjects, project{ID: id, Name: name})
				}
			}

			if len(matchingProjects) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No projects match pattern %q\n", pattern)
				return nil
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Would deploy %d projects matching %q:\n", len(matchingProjects), pattern)
				for _, p := range matchingProjects {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", p.Name, p.ID)
				}
				return nil
			}

			// Confirmation if not --yes
			if !confirm && !flags.yes {
				fmt.Fprintf(cmd.OutOrStdout(), "About to deploy %d projects matching %q:\n", len(matchingProjects), pattern)
				for _, p := range matchingProjects {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", p.Name, p.ID)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nUse --yes to confirm or --dry-run to preview")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type deployResult struct {
				ProjectID   string `json:"project_id"`
				ProjectName string `json:"project_name"`
				Status      string `json:"status"`
				Error       string `json:"error,omitempty"`
				Duration    string `json:"duration,omitempty"`
			}

			results := make([]deployResult, len(matchingProjects))
			var mu sync.Mutex
			var wg sync.WaitGroup

			// Semaphore for parallelism control
			sem := make(chan struct{}, parallel)

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			for i, p := range matchingProjects {
				wg.Add(1)
				go func(idx int, proj project) {
					defer wg.Done()

					sem <- struct{}{}
					defer func() { <-sem }()

					start := time.Now()
					result := deployResult{
						ProjectID:   proj.ID,
						ProjectName: proj.Name,
					}

					_, _, err := c.Post(ctx, fmt.Sprintf("/deployments/%s/deploy", proj.ID), nil)
					if err != nil {
						result.Status = "failed"
						result.Error = err.Error()
					} else {
						result.Status = "success"
					}
					result.Duration = time.Since(start).Round(time.Millisecond).String()

					mu.Lock()
					results[idx] = result
					mu.Unlock()
				}(i, p)
			}

			wg.Wait()

			// Count successes and failures
			var successCount, failCount int
			for _, r := range results {
				if r.Status == "success" {
					successCount++
				} else {
					failCount++
				}
			}

			type batchResult struct {
				Pattern      string         `json:"pattern"`
				TotalCount   int            `json:"total_count"`
				SuccessCount int            `json:"success_count"`
				FailCount    int            `json:"fail_count"`
				Results      []deployResult `json:"results"`
			}

			output := batchResult{
				Pattern:      pattern,
				TotalCount:   len(matchingProjects),
				SuccessCount: successCount,
				FailCount:    failCount,
				Results:      results,
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, output)
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Batch Deploy Results\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Pattern: %s\n", pattern)
			fmt.Fprintf(cmd.OutOrStdout(), "Success: %d/%d\n\n", successCount, len(matchingProjects))

			headers := []string{"Project", "ID", "Status", "Duration", "Error"}
			var tableRows [][]string
			for _, r := range results {
				errStr := r.Error
				if len(errStr) > 40 {
					errStr = errStr[:37] + "..."
				}
				tableRows = append(tableRows, []string{
					r.ProjectName,
					r.ProjectID,
					r.Status,
					r.Duration,
					errStr,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "", "Glob pattern to match project names (required)")
	cmd.Flags().IntVar(&parallel, "parallel", 2, "Number of parallel deployments")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm batch operation (alias for --yes)")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Overall timeout for batch operation")
	return cmd
}

// getProjectName extracts name from project data, falling back to ID
func getProjectName(data map[string]any, id string) string {
	if name, ok := data["name"].(string); ok && name != "" {
		return name
	}
	return id
}

// matchPattern is a simple glob pattern matcher
func matchPattern(pattern, name string) bool {
	// Simple glob matching - supports * and ?
	matched, _ := filepath.Match(pattern, name)
	return matched
}

// Alternative: support more complex patterns
func matchPatternComplex(pattern, name string) bool {
	// Support ** for any path segment and comma-separated alternatives
	if strings.Contains(pattern, ",") {
		for _, p := range strings.Split(pattern, ",") {
			if matchPattern(strings.TrimSpace(p), name) {
				return true
			}
		}
		return false
	}
	return matchPattern(pattern, name)
}
