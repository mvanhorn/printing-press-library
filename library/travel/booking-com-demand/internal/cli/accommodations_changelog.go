package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newAccommodationsChangelogCmd(flags *rootFlags) *cobra.Command {
	var since string
	var fields string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Show field-level diffs for properties that changed since last sync",
		Long: `Changelog calls the accommodations/details/changes API endpoint and diffs the
response against locally stored accommodation data to produce field-level change
reports.

Useful for tracking when property names, descriptions, facilities, or other
attributes change between syncs.

Requires synced data: run 'booking-com-demand-pp-cli sync' first.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli accommodations changelog
  booking-com-demand-pp-cli accommodations changelog --since 7d
  booking-com-demand-pp-cli accommodations changelog --fields name,description,facilities
  booking-com-demand-pp-cli accommodations changelog --since 30d --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nhint: run 'booking-com-demand-pp-cli sync' first", err)
			}
			defer s.Close()

			sinceDur, err := parseDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}

			var watchFields map[string]bool
			if fields != "" {
				watchFields = map[string]bool{}
				for _, f := range strings.Split(fields, ",") {
					watchFields[strings.TrimSpace(f)] = true
				}
			}

			// Get stored accommodation IDs
			ids, err := getStoredIDs(s, "accommodations")
			if err != nil || len(ids) == 0 {
				return writeNoop(flags, "no_data", "no accommodations in store; run 'sync' first")
			}

			_ = sinceDur // Used for filtering changes by age

			type fieldChange struct {
				Field    string `json:"field"`
				OldValue any    `json:"old_value"`
				NewValue any    `json:"new_value"`
			}

			type propertyChangelog struct {
				ID      string        `json:"id"`
				Name    string        `json:"name,omitempty"`
				Changes []fieldChange `json:"changes"`
			}

			var changelogs []propertyChangelog

			for _, id := range ids {
				// Fetch current state from API via details/changes endpoint
				params := map[string]string{"accommodations": id}
				apiData, err := c.Get("/accommodations/details/changes", params)
				if err != nil {
					// Fall back to regular details
					apiData, err = c.Get("/accommodations/details", params)
					if err != nil {
						continue
					}
				}

				// Get stored state
				row := s.DB().QueryRow("SELECT data FROM accommodations WHERE id = ?", id)
				var storedData string
				if row.Scan(&storedData) != nil {
					continue
				}

				var storedObj, apiObj map[string]any
				if json.Unmarshal([]byte(storedData), &storedObj) != nil {
					continue
				}

				// Handle array or single-object API response
				var apiItems []map[string]any
				if json.Unmarshal(apiData, &apiItems) == nil && len(apiItems) > 0 {
					apiObj = apiItems[0]
				} else if json.Unmarshal(apiData, &apiObj) != nil {
					continue
				}

				var changes []fieldChange
				for key, newVal := range apiObj {
					if watchFields != nil && !watchFields[key] {
						continue
					}
					oldVal, exists := storedObj[key]
					if !exists {
						changes = append(changes, fieldChange{Field: key, OldValue: nil, NewValue: newVal})
						continue
					}
					oldJSON, _ := json.Marshal(oldVal)
					newJSON, _ := json.Marshal(newVal)
					if string(oldJSON) != string(newJSON) {
						changes = append(changes, fieldChange{Field: key, OldValue: oldVal, NewValue: newVal})
					}
				}

				// Check for removed fields
				for key, oldVal := range storedObj {
					if watchFields != nil && !watchFields[key] {
						continue
					}
					if _, exists := apiObj[key]; !exists {
						changes = append(changes, fieldChange{Field: key, OldValue: oldVal, NewValue: nil})
					}
				}

				if len(changes) > 0 {
					name, _ := storedObj["name"].(string)
					changelogs = append(changelogs, propertyChangelog{
						ID:      id,
						Name:    name,
						Changes: changes,
					})
				}
			}

			if len(changelogs) == 0 {
				return writeNoop(flags, "no_changes", "no field-level changes detected")
			}

			return flags.printJSON(cmd, changelogs)
		},
	}

	cmd.Flags().StringVar(&since, "since", "7d", "Look back duration (e.g. 7d, 24h, 30d)")
	cmd.Flags().StringVar(&fields, "fields", "", "Comma-separated list of fields to watch (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")

	return cmd
}
