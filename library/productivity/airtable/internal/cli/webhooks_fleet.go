// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/store"
	"github.com/spf13/cobra"
)

func newWebhooksFleetCmd(flags *rootFlags) *cobra.Command {
	var allProfiles bool
	var dbPath string

	cmd := &cobra.Command{
		Use:         "fleet",
		Short:       "List every webhook on every known base, sorted by time-until-expiration",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Iterates every base under the active profile (or --all-profiles for
multi-tenant), reports (baseId, webhookId, expires_in_days,
last_payload_age_seconds) sorted by expiry.

This is offline-first: uses the local mirror's synced webhooks rows. Run
'airtable-pp-cli sync' first to populate.`,
		Example: strings.Trim(`
  # Show all webhooks across known bases
  airtable-pp-cli webhooks fleet

  # JSON for piping
  airtable-pp-cli webhooks fleet --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if allProfiles && cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan all profiles")
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("airtable-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", dbPath, dbPath)
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'airtable-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, parent_id, expiration_time, data, synced_at
				 FROM webhooks
				 ORDER BY expiration_time ASC`)
			if err != nil {
				return fmt.Errorf("list webhooks: %w", err)
			}
			defer rows.Close()

			type entry struct {
				BaseID                string `json:"base_id"`
				WebhookID             string `json:"webhook_id"`
				ExpirationTime        string `json:"expiration_time,omitempty"`
				ExpiresInDays         int    `json:"expires_in_days"`
				LastPayloadAgeSeconds int64  `json:"last_payload_age_seconds"`
			}

			now := time.Now()
			var fleet []entry
			for rows.Next() {
				var id, baseID, expTime, data string
				var syncedAt sql.NullTime
				if err := rows.Scan(&id, &baseID, &expTime, &data, &syncedAt); err != nil {
					return fmt.Errorf("scan webhook: %w", err)
				}
				expDays := 0
				if expTime != "" {
					if t, err := time.Parse(time.RFC3339, expTime); err == nil {
						expDays = int(t.Sub(now).Hours() / 24)
					}
				}
				lastPayloadAge := int64(-1)
				if syncedAt.Valid {
					lastPayloadAge = int64(now.Sub(syncedAt.Time).Seconds())
				}
				fleet = append(fleet, entry{
					BaseID:                baseID,
					WebhookID:             id,
					ExpirationTime:        expTime,
					ExpiresInDays:         expDays,
					LastPayloadAgeSeconds: lastPayloadAge,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate webhooks: %w", err)
			}
			sort.Slice(fleet, func(i, j int) bool {
				return fleet[i].ExpiresInDays < fleet[j].ExpiresInDays
			})
			return flags.printJSON(cmd, fleet)
		},
	}
	cmd.Flags().BoolVar(&allProfiles, "all-profiles", false, "Scan every configured profile (not just active)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/airtable-pp-cli/data.db)")
	return cmd
}

// silence unused-import warning
var _ = json.Valid
