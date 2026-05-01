// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var resourceType string
	var groupBy string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "analytics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Run analytics queries on locally synced data",
		Long: `Analyze locally synced data with count, group-by, and summary operations.
Data must be synced first with the sync command.`,
		Example: `  # Summary of all synced resource types
  linear-pp-cli analytics

  # Count issues
  linear-pp-cli analytics --type issues

  # Group issues by state
  linear-pp-cli analytics --type issues --group-by state_name

  # Top 5 assignees by issue count
  linear-pp-cli analytics --type issues --group-by assignee_id --limit 5 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			if resourceType == "" {
				// Show summary of all resource types
				status, err := db.Status()
				if err != nil {
					return fmt.Errorf("getting status: %w", err)
				}
				if flags.asJSON {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(status)
				}
				fmt.Println("Resource Type\tCount")
				fmt.Println("-------------\t-----")
				for rt, count := range status {
					fmt.Printf("%s\t%d\n", rt, count)
				}
				return nil
			}

			if groupBy != "" {
				results, err := db.GroupBy(resourceType, groupBy, limit)
				if err != nil {
					return err
				}
				if flags.asJSON {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(results)
				}
				fmt.Printf("%s\tCount\n", groupBy)
				fmt.Println("---\t-----")
				for _, r := range results {
					fmt.Printf("%s\t%d\n", r.Value, r.Count)
				}
				return nil
			}

			count, err := db.Count(resourceType)
			if err != nil {
				return err
			}

			if flags.asJSON {
				result := map[string]any{"resource_type": resourceType, "count": count}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("%s: %d records\n", resourceType, count)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourceType, "type", "", "Resource type to analyze")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Field to group by")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max groups to show")

	return cmd
}
