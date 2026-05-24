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

			type entry struct {
				Profile               string `json:"profile,omitempty"`
				DBPath                string `json:"db_path,omitempty"`
				BaseID                string `json:"base_id"`
				WebhookID             string `json:"webhook_id"`
				ExpirationTime        string `json:"expiration_time,omitempty"`
				ExpiresInDays         int    `json:"expires_in_days"`
				LastPayloadAgeSeconds int64  `json:"last_payload_age_seconds"`
			}

			// PATCH(airtable-webhooks-fleet-all-profiles): build the list of
			// (profile-name, db-path) pairs to scan. Without --all-profiles
			// this is one entry — the active profile's db (or the user's
			// --db override). With --all-profiles, also include every saved
			// profile's --db value from `profiles.json`, deduplicated by
			// db-path so two profiles pointing at the same mirror are
			// scanned once. Profiles without a saved --db value reuse the
			// default mirror, so they collapse into the default entry.
			type target struct {
				profile string
				dbPath  string
			}
			var targets []target
			seen := map[string]bool{}
			addTarget := func(name, p string) {
				if p == "" {
					p = defaultDBPath("airtable-pp-cli")
				}
				if seen[p] {
					return
				}
				seen[p] = true
				targets = append(targets, target{profile: name, dbPath: p})
			}
			addTarget("", dbPath)
			if allProfiles {
				if store, err := loadProfileStore(); err == nil && store != nil {
					for name, prof := range store.Profiles {
						addTarget(name, prof.Values["db"])
					}
				}
			}

			now := time.Now()
			var fleet []entry
			scanned := 0
			for _, t := range targets {
				if _, statErr := os.Stat(t.dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s (profile %q)\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", t.dbPath, t.profile, t.dbPath)
					continue
				}
				db, err := store.OpenReadOnly(t.dbPath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "open %s (profile %q): %v\n", t.dbPath, t.profile, err)
					continue
				}
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT id, parent_id, expiration_time, data, synced_at
					 FROM webhooks
					 ORDER BY expiration_time ASC`)
				if err != nil {
					db.Close()
					return fmt.Errorf("list webhooks (profile %q): %w", t.profile, err)
				}
				for rows.Next() {
					var id, baseID, expTime, data string
					var syncedAt sql.NullTime
					if err := rows.Scan(&id, &baseID, &expTime, &data, &syncedAt); err != nil {
						rows.Close()
						db.Close()
						return fmt.Errorf("scan webhook (profile %q): %w", t.profile, err)
					}
					_ = data
					expDays := 0
					if expTime != "" {
						if ts, err := time.Parse(time.RFC3339, expTime); err == nil {
							expDays = int(ts.Sub(now).Hours() / 24)
						}
					}
					lastPayloadAge := int64(-1)
					if syncedAt.Valid {
						lastPayloadAge = int64(now.Sub(syncedAt.Time).Seconds())
					}
					fleet = append(fleet, entry{
						Profile:               t.profile,
						DBPath:                t.dbPath,
						BaseID:                baseID,
						WebhookID:             id,
						ExpirationTime:        expTime,
						ExpiresInDays:         expDays,
						LastPayloadAgeSeconds: lastPayloadAge,
					})
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					db.Close()
					return fmt.Errorf("iterate webhooks (profile %q): %w", t.profile, err)
				}
				rows.Close()
				db.Close()
				scanned++
			}

			if scanned == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
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
