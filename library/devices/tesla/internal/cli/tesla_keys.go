// tesla keys audit — list enrolled phone/NFC keys with last-seen and stale flags.
// Hand-coded; out-of-tree from generator.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/store"
)

func newKeysCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Tesla key management (audit)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newKeysAuditCmd(flags))
	return cmd
}

func newKeysAuditCmd(flags *rootFlags) *cobra.Command {
	var (
		vehicleID  string
		staleAfter string
	)
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List enrolled keys with last-seen and flag stale candidates",
		Long: `Reads vehicle_state.key_metadata via /api/1/vehicles/{vehicle_id}/data_request/vehicle_state,
upserts into tesla_keys_enrolled, and flags keys whose last-seen exceeds the
--stale-after threshold.`,
		Example:     "  tesla-pp-cli keys audit --vehicle 5YJ3E1EA6XXXXXXXX --stale-after 90d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}
			if vehicleID == "" {
				return fmt.Errorf("--vehicle is required (vehicle_id from /api/1/products)")
			}
			ctx := cmd.Context()
			s, err := store.OpenWithContext(ctx, defaultDBPath("tesla-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := store.EnsureTeslaSchema(ctx, s); err != nil {
				return err
			}

			staleDur, err := parseDurationDayAware(staleAfter)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := strings.ReplaceAll("/api/1/vehicles/{vehicle_id}/data_request/vehicle_state", "{vehicle_id}", vehicleID)
			raw, err := c.Get(path, nil)
			if err != nil {
				return fmt.Errorf("vehicle_state: %w", err)
			}
			var env struct {
				Response struct {
					VIN         string `json:"vin"`
					KeyMetadata []struct {
						PubkeyHash  string `json:"public_key"`
						Role        string `json:"role"`
						FormFactor  string `json:"form_factor"`
						DisplayName string `json:"display_name"`
						AddedAt     int64  `json:"created_at"`
						LastActive  int64  `json:"last_active"`
					} `json:"key_metadata"`
				} `json:"response"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parse vehicle_state: %w", err)
			}
			vin := env.Response.VIN
			if vin == "" {
				vin = vehicleID
			}

			now := time.Now().UTC()
			for _, k := range env.Response.KeyMetadata {
				addedAt := time.Unix(k.AddedAt/1000, 0).UTC().Format(time.RFC3339)
				lastSeen := time.Unix(k.LastActive/1000, 0).UTC().Format(time.RFC3339)
				if k.AddedAt == 0 {
					addedAt = ""
				}
				if k.LastActive == 0 {
					lastSeen = ""
				}
				_, _ = s.DB().ExecContext(ctx, `INSERT OR REPLACE INTO tesla_keys_enrolled (
                    vin, pubkey_hash, role, form_factor, display_name, added_at, last_seen
                  ) VALUES (?,?,?,?,?,?,?)`, vin, k.PubkeyHash, k.Role, k.FormFactor, k.DisplayName, addedAt, lastSeen)
			}

			// Read back the audit
			rows, err := s.DB().QueryContext(ctx, `
                SELECT pubkey_hash, role, form_factor, display_name, added_at, last_seen
                FROM tesla_keys_enrolled WHERE vin = ? ORDER BY last_seen DESC`, vin)
			if err != nil {
				return err
			}
			defer rows.Close()
			type keyOut struct {
				PubkeyHash        string `json:"pubkey_hash"`
				Role              string `json:"role,omitempty"`
				FormFactor        string `json:"form_factor,omitempty"`
				DisplayName       string `json:"display_name,omitempty"`
				AddedAt           string `json:"added_at,omitempty"`
				LastSeen          string `json:"last_seen,omitempty"`
				DaysSinceLastSeen int    `json:"days_since_last_seen,omitempty"`
				Stale             bool   `json:"stale"`
			}
			result := struct {
				VIN        string   `json:"vin"`
				TotalKeys  int      `json:"total_keys"`
				StaleCount int      `json:"stale_count"`
				Keys       []keyOut `json:"keys"`
				StaleKeys  []keyOut `json:"stale_keys"`
			}{VIN: vin, Keys: []keyOut{}, StaleKeys: []keyOut{}}
			for rows.Next() {
				var pk, role, ff, dn, addedAt, lastSeen sql.NullString
				if err := rows.Scan(&pk, &role, &ff, &dn, &addedAt, &lastSeen); err != nil {
					continue
				}
				k := keyOut{
					PubkeyHash:  pk.String,
					Role:        role.String,
					FormFactor:  ff.String,
					DisplayName: dn.String,
					AddedAt:     addedAt.String,
					LastSeen:    lastSeen.String,
				}
				if lastSeen.Valid && lastSeen.String != "" {
					if t, perr := time.Parse(time.RFC3339, lastSeen.String); perr == nil {
						k.DaysSinceLastSeen = int(now.Sub(t).Hours() / 24)
						if time.Since(t) > staleDur {
							k.Stale = true
						}
					}
				}
				result.Keys = append(result.Keys, k)
				if k.Stale {
					result.StaleKeys = append(result.StaleKeys, k)
				}
			}
			result.TotalKeys = len(result.Keys)
			result.StaleCount = len(result.StaleKeys)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&vehicleID, "vehicle", "", "Vehicle id from /api/1/products (required)")
	cmd.Flags().StringVar(&staleAfter, "stale-after", "90d", "Flag keys not seen in N (e.g. 90d, 6mo)")
	return cmd
}

// parseDurationDayAware accepts "Nd" / "Nmo" / "Ny" plus any time.ParseDuration form.
func parseDurationDayAware(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		var n int
		_, err := fmt.Sscanf(s, "%dd", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "mo") {
		var n int
		_, err := fmt.Sscanf(s, "%dmo", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "y") {
		var n int
		_, err := fmt.Sscanf(s, "%dy", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
