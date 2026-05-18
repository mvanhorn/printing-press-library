package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Monitor FDA resources for new activity",
		Long: `Maintain a watchlist of drugs, devices, and firms to monitor.
Add items to watch, then check periodically for new records since last check.`,
	}

	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistCheckCmd(flags))

	return cmd
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "add <resource_type> <name>",
		Short: "Add an item to the watchlist",
		Example: `  openfda-pp-cli watchlist add drug-events ASPIRIN
  openfda-pp-cli watchlist add drug-recalls "PFIZER"
  openfda-pp-cli watchlist add device-events "INFUSION PUMP"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := args[0]
			name := args[1]
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openfda-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			now := time.Now().UTC()
			_, err = db.DB().Exec(
				`INSERT INTO watchlist (name, resource_type, added_at, last_checked_at)
				 VALUES (?, ?, ?, ?)
				 ON CONFLICT(name, resource_type) DO UPDATE SET added_at = excluded.added_at`,
				name, resourceType, now, now,
			)
			if err != nil {
				return fmt.Errorf("adding to watchlist: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{
					"status":        "added",
					"name":          name,
					"resource_type": resourceType,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added %s/%s to watchlist\n", resourceType, name)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "remove <resource_type> <name>",
		Short: "Remove an item from the watchlist",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := args[0]
			name := args[1]
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openfda-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			result, err := db.DB().Exec(
				`DELETE FROM watchlist WHERE name = ? AND resource_type = ?`,
				name, resourceType,
			)
			if err != nil {
				return fmt.Errorf("removing from watchlist: %w", err)
			}
			affected, _ := result.RowsAffected()

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{
					"status":        "removed",
					"name":          name,
					"resource_type": resourceType,
					"found":         affected > 0,
				})
			}

			if affected == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Item %s/%s not found in watchlist\n", resourceType, name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed %s/%s from watchlist\n", resourceType, name)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all watched items",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openfda-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT name, resource_type, added_at, last_checked_at FROM watchlist ORDER BY resource_type, name`,
			)
			if err != nil {
				return fmt.Errorf("listing watchlist: %w", err)
			}
			defer rows.Close()

			type watchItem struct {
				Name          string `json:"name"`
				ResourceType  string `json:"resource_type"`
				AddedAt       string `json:"added_at"`
				LastCheckedAt string `json:"last_checked_at"`
			}

			var items []watchItem
			for rows.Next() {
				var item watchItem
				if err := rows.Scan(&item.Name, &item.ResourceType, &item.AddedAt, &item.LastCheckedAt); err != nil {
					continue
				}
				items = append(items, item)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Use 'watchlist add <type> <name>' to start monitoring.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TYPE\tNAME\tADDED\tLAST CHECKED")
			for _, item := range items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					item.ResourceType, item.Name,
					truncate(item.AddedAt, 19), truncate(item.LastCheckedAt, 19))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchlistCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check for new activity on watched items",
		Long:  `Query each watched item for records newer than the last check time, then update the timestamp.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openfda-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT name, resource_type, last_checked_at FROM watchlist ORDER BY resource_type, name`,
			)
			if err != nil {
				return fmt.Errorf("listing watchlist: %w", err)
			}

			type watchedItem struct {
				Name          string
				ResourceType  string
				LastCheckedAt string
			}

			var items []watchedItem
			for rows.Next() {
				var item watchedItem
				if err := rows.Scan(&item.Name, &item.ResourceType, &item.LastCheckedAt); err != nil {
					continue
				}
				items = append(items, item)
			}
			rows.Close()

			type checkResult struct {
				Name         string                   `json:"name"`
				ResourceType string                   `json:"resource_type"`
				NewItems     int                      `json:"new_items"`
				Samples      []map[string]interface{} `json:"samples,omitempty"`
			}

			var results []checkResult
			now := time.Now().UTC()

			for _, item := range items {
				nameUpper := strings.ToUpper(item.Name)

				// Find records updated since last check for this resource type matching the name
				var query string
				var queryArgs []interface{}

				switch item.ResourceType {
				case "drug-events":
					// Use DISTINCT on r.id to avoid inflated counts when a report
					// lists the same drug multiple times in $.patient.drug.
					query = `
						SELECT DISTINCT r.data FROM resources r, json_each(json_extract(r.data, '$.patient.drug')) je
						WHERE r.resource_type = ?
						AND UPPER(json_extract(je.value, '$.medicinalproduct')) LIKE ?
						AND r.updated_at > ?
						ORDER BY r.updated_at DESC
					`
					queryArgs = []interface{}{item.ResourceType, "%" + nameUpper + "%", item.LastCheckedAt}
				case "device-events":
					query = `
						SELECT DISTINCT r.data FROM resources r, json_each(json_extract(r.data, '$.device')) je
						WHERE r.resource_type = ?
						AND UPPER(json_extract(je.value, '$.brand_name')) LIKE ?
						AND r.updated_at > ?
						ORDER BY r.updated_at DESC
					`
					queryArgs = []interface{}{item.ResourceType, "%" + nameUpper + "%", item.LastCheckedAt}
				default:
					// Generic search in data blob
					query = `
						SELECT r.data FROM resources r
						WHERE r.resource_type = ?
						AND UPPER(r.data) LIKE ?
						AND r.updated_at > ?
						ORDER BY r.updated_at DESC
					`
					queryArgs = []interface{}{item.ResourceType, "%" + nameUpper + "%", item.LastCheckedAt}
				}

				dataRows, err := db.Query(query, queryArgs...)
				if err != nil {
					continue
				}

				cr := checkResult{
					Name:         item.Name,
					ResourceType: item.ResourceType,
				}

				for dataRows.Next() {
					var dataStr string
					if err := dataRows.Scan(&dataStr); err != nil {
						continue
					}
					cr.NewItems++
					if len(cr.Samples) < 3 {
						var obj map[string]interface{}
						if json.Unmarshal([]byte(dataStr), &obj) == nil {
							cr.Samples = append(cr.Samples, obj)
						}
					}
				}
				dataRows.Close()

				results = append(results, cr)

				// Update last_checked_at
				db.DB().Exec(
					`UPDATE watchlist SET last_checked_at = ? WHERE name = ? AND resource_type = ?`,
					now, item.Name, item.ResourceType,
				)
			}

			totalNew := 0
			for _, r := range results {
				totalNew += r.NewItems
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{
					"checked_at": now.Format(time.RFC3339),
					"total_new":  totalNew,
					"results":    results,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Watchlist Check (%s)\n", now.Format("2006-01-02 15:04"))
			fmt.Fprintf(cmd.OutOrStdout(), "Total new items: %d\n\n", totalNew)

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TYPE\tNAME\tNEW ITEMS")
			for _, r := range results {
				status := fmt.Sprintf("%d", r.NewItems)
				if r.NewItems > 0 {
					status = fmt.Sprintf("%d NEW", r.NewItems)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.ResourceType, r.Name, status)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ensureWatchlistTable creates the watchlist table if it doesn't exist.
// Called at runtime rather than only during migration so the table is
// available even on databases created before this feature was added.
func ensureWatchlistTable(s *store.Store) error {
	_, err := s.DB().Exec(`CREATE TABLE IF NOT EXISTS watchlist (
		name TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (name, resource_type)
	)`)
	if err != nil {
		return fmt.Errorf("creating watchlist table: %w", err)
	}
	return nil
}

// Ensure sql import is used (for sql.ErrNoRows reference potential).
var _ = sql.ErrNoRows
