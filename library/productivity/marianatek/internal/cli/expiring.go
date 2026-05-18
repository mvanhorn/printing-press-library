// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/store"
	"github.com/spf13/cobra"
)

func newExpiringCmd(flags *rootFlags) *cobra.Command {
	var within time.Duration
	var dbPath string

	cmd := &cobra.Command{
		Use:   "expiring",
		Short: "Surface credit packs and memberships about to lapse",
		Long: `expiring scans your synced credits + memberships and reports any expiring
within the window. The API exposes expires_at per pack but never aggregates
"use it or lose it" — this command does.

Run 'marianatek-pp-cli sync --resources me_credits,me_memberships' first to
populate the local store.`,
		Example: `  # Anything expiring in the next 30 days
	  marianatek-pp-cli expiring --within 720h

	  # JSON for an agent / cron job
	  marianatek-pp-cli expiring --within 720h --json`,
		Annotations: map[string]string{
			"pp:novel":      "expiring",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("marianatek-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			deadline := now.Add(within)
			out := []expiringRow{}

			for _, kind := range []string{"me_credits", "me_memberships"} {
				rows, err := db.List(kind, 5000)
				if err != nil {
					continue
				}
				for _, raw := range rows {
					var rec map[string]any
					if err := json.Unmarshal(raw, &rec); err != nil {
						continue
					}
					attrs := pickAttrs(rec)
					if attrs == nil {
						continue
					}
					exp := stringAttr(attrs, "expires_at", "expiration_date", "end_date")
					if exp == "" {
						continue
					}
					t, err := time.Parse(time.RFC3339, exp)
					if err != nil {
						// Try date-only
						t, err = time.Parse("2006-01-02", exp)
						if err != nil {
							continue
						}
					}
					if t.Before(now) || t.After(deadline) {
						continue
					}
					id := expiringRecordID(rec)
					out = append(out, expiringRow{
						Kind:      kind,
						ID:        id,
						Name:      stringAttr(attrs, "name", "title", "product_name"),
						ExpiresAt: exp,
						DaysLeft:  int(t.Sub(now).Hours() / 24),
						Remaining: intAttr(attrs, "remaining", "balance", "amount_remaining"),
					})
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt < out[j].ExpiresAt })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().DurationVar(&within, "within", 30*24*time.Hour, "expiry window (default 30 days)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/marianatek-pp-cli/data.db)")
	return cmd
}

type expiringRow struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	ExpiresAt string `json:"expires_at"`
	DaysLeft  int    `json:"days_left"`
	Remaining int    `json:"remaining,omitempty"`
}

func expiringRecordID(rec map[string]any) string {
	if data, ok := rec["data"].(map[string]any); ok {
		if id, ok := data["id"].(string); ok {
			return id
		}
	}
	if id, ok := rec["id"].(string); ok {
		return id
	}
	return ""
}

func intAttr(attrs map[string]any, keys ...string) int {
	for _, key := range keys {
		if v, ok := attrs[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
	}
	return 0
}
