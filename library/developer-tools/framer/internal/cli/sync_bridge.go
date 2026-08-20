package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"

	"github.com/spf13/cobra"
)

func newSyncBridgeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync all Framer project data via the Server API",
		Long: `Connect to the Framer Server API via the Node.js bridge and fetch all
project resources: CMS collections, items, code files, styles, fonts,
pages, redirects, and locales. Stores everything in the local SQLite
database for offline search, snapshots, and diffing.

Requires FRAMER_PROJECT_URL and FRAMER_API_KEY environment variables.`,
		Example: `  # Sync all data from your Framer project
  framer-pp-cli sync

  # Sync with JSON progress output
  framer-pp-cli sync --json`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would connect to Framer API and sync all resources")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			fmt.Fprintln(cmd.ErrOrStderr(), "Connecting to Framer API...")

			raw, err := bc.Call("sync-all")
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			var data struct {
				Project     []json.RawMessage `json:"project"`
				Collections []json.RawMessage `json:"collections"`
				Items       []json.RawMessage `json:"items"`
				CodeFiles   []json.RawMessage `json:"codeFiles"`
				ColorStyles []json.RawMessage `json:"colorStyles"`
				TextStyles  []json.RawMessage `json:"textStyles"`
				Redirects   json.RawMessage   `json:"redirects"`
				Locales     []json.RawMessage `json:"locales"`
				Pages       []json.RawMessage `json:"pages"`
			}
			if err := json.Unmarshal(raw, &data); err != nil {
				return fmt.Errorf("parsing sync response: %w", err)
			}

			counts := make(map[string]int)

			// Capture pre-sync state for diff reporting
			type resourceKey struct {
				ID           string
				ResourceType string
			}
			preSyncMap := make(map[resourceKey]bool)
			{
				rows, qerr := db.DB().QueryContext(context.Background(),
					`SELECT id, resource_type FROM resources`)
				if qerr == nil {
					defer rows.Close()
					for rows.Next() {
						var k resourceKey
						if rows.Scan(&k.ID, &k.ResourceType) == nil {
							preSyncMap[k] = true
						}
					}
					rows.Close() // close early so upserts don't conflict
				}
			}

			// Per-type diff counters
			type diffCounts struct {
				New     int `json:"new"`
				Updated int `json:"updated"`
			}
			typeDiffs := make(map[string]*diffCounts)
			upsertedKeys := make(map[resourceKey]bool)
			syncedTypes := make(map[string]bool)

			// Helper to upsert a batch of resources
			upsert := func(resourceType string, items []json.RawMessage) error {
				syncedTypes[resourceType] = true
				if typeDiffs[resourceType] == nil {
					typeDiffs[resourceType] = &diffCounts{}
				}
				for _, item := range items {
					var obj map[string]interface{}
					if err := json.Unmarshal(item, &obj); err != nil {
						continue
					}
					id := ""
					if v, ok := obj["id"].(string); ok {
						id = v
					} else if v, ok := obj["name"].(string); ok {
						id = v
					} else {
						id = fmt.Sprintf("%s-%d", resourceType, counts[resourceType])
					}
					// Track new vs updated
					key := resourceKey{ID: id, ResourceType: resourceType}
					upsertedKeys[key] = true
					if preSyncMap[key] {
						typeDiffs[resourceType].Updated++
					} else {
						typeDiffs[resourceType].New++
					}

					dataBytes, _ := json.Marshal(obj)
					if err := db.Upsert(resourceType, id, json.RawMessage(dataBytes)); err != nil {
						return fmt.Errorf("upserting %s %s: %w", resourceType, id, err)
					}
					counts[resourceType]++
				}
				return nil
			}

			// Upsert each resource type
			if err := upsert("project", data.Project); err != nil {
				return err
			}
			if err := upsert("cms-collections", data.Collections); err != nil {
				return err
			}
			if err := upsert("cms-items", data.Items); err != nil {
				return err
			}
			if err := upsert("code", data.CodeFiles); err != nil {
				return err
			}
			if err := upsert("styles-colors", data.ColorStyles); err != nil {
				return err
			}
			if err := upsert("styles-text", data.TextStyles); err != nil {
				return err
			}
			if err := upsert("locales", data.Locales); err != nil {
				return err
			}
			if err := upsert("pages", data.Pages); err != nil {
				return err
			}

			// Redirects come as a single array, not individual items
			if len(data.Redirects) > 0 {
				var redirects []json.RawMessage
				if err := json.Unmarshal(data.Redirects, &redirects); err == nil {
					if err := upsert("redirects", redirects); err != nil {
						return err
					}
				}
			}

			// Compute removed counts: pre-sync keys whose type was synced
			// but whose ID was not seen in the response.
			removedCounts := make(map[string]int)
			for key := range preSyncMap {
				if syncedTypes[key.ResourceType] && !upsertedKeys[key] {
					if err := db.Delete(key.ResourceType, key.ID); err != nil {
						return fmt.Errorf("deleting removed %s %s: %w", key.ResourceType, key.ID, err)
					}
					removedCounts[key.ResourceType]++
				}
			}

			// Output results
			type resourceDiff struct {
				Count   int `json:"count"`
				New     int `json:"new"`
				Updated int `json:"updated"`
				Removed int `json:"removed"`
			}
			type syncResult struct {
				Status    string                  `json:"status"`
				Resources map[string]resourceDiff `json:"resources"`
				Total     int                     `json:"total"`
				DB        string                  `json:"db"`
			}

			total := 0
			for _, c := range counts {
				total += c
			}

			resources := make(map[string]resourceDiff)
			for rt, c := range counts {
				d := typeDiffs[rt]
				rd := resourceDiff{
					Count:   c,
					Removed: removedCounts[rt],
				}
				if d != nil {
					rd.New = d.New
					rd.Updated = d.Updated
				}
				resources[rt] = rd
			}

			result := syncResult{
				Status:    "ok",
				Resources: resources,
				Total:     total,
				DB:        dbPath,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d resources:\n", total)
			for rt, rd := range resources {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d (%d new, %d updated", rt, rd.Count, rd.New, rd.Updated)
				if rd.Removed > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), ", %d removed", rd.Removed)
				}
				fmt.Fprintln(cmd.OutOrStdout(), ")")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nStored in %s\n", dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
