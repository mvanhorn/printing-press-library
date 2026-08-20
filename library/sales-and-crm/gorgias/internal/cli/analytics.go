// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
	"github.com/spf13/cobra"
)

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var resourceType string
	var groupBy string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "analytics",
		Short:       "Run analytics queries on locally synced data",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Count records by type
  gorgias-pp-cli analytics --type messages

  # Group by a field
  gorgias-pp-cli analytics --type messages --group-by author_id

  # Top 10 most frequent values
  gorgias-pp-cli analytics --type messages --group-by channel_id --limit 10 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("gorgias-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gorgias-pp-cli sync' first.", err)
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
				return runGroupBy(db, resourceType, groupBy, limit, flags)
			}

			count, err := db.Count(resourceType)
			if err != nil {
				return fmt.Errorf("counting: %w", err)
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

func runGroupBy(db *store.Store, resourceType, field string, limit int, flags *rootFlags) error {
	groups, err := db.GroupByJSONField(resourceType, field, limit)
	if err != nil {
		return err
	}

	if flags.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(groups)
	}

	fmt.Printf("%s\tCount\n", field)
	fmt.Println("---\t-----")
	for _, group := range groups {
		fmt.Printf("%s\t%d\n", group.Value, group.Count)
	}
	return nil
}
