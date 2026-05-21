// Copyright 2026 joltsconsulting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

type driftDevice struct {
	Name             string `json:"name"`
	AbrClientVersion string `json:"abrClientVersion"`
	InventoryDate    string `json:"inventoryDate"`
	Behind           string `json:"behind"`
}

func newInventoryDriftCmd(flags *rootFlags) *cobra.Command {
	var targetVersion string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:     "drift",
		Short:   "Endpoints whose AbR client version is older than the target (offline)",
		Long:    "Read the locally-synced inventory snapshot and list endpoints whose abrClientVersion is below the target version.",
		Example: "  adminbyrequest-pp-cli inventory drift --client-version 8.7.2 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if targetVersion == "" {
				return fmt.Errorf("missing required --client-version (e.g. 8.7.2)")
			}
			target := parseSemverLike(targetVersion)
			if dbPath == "" {
				dbPath = defaultDBPath("adminbyrequest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store at %s: %w (run sync first)", dbPath, err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(name,''), COALESCE(abr_client_version,''), COALESCE(inventory_date,'')
				 FROM inventory
				 WHERE inventory_available = 1`)
			if err != nil {
				return fmt.Errorf("querying inventory: %w", err)
			}
			defer rows.Close()

			var out []driftDevice
			var hitLimit bool
			for rows.Next() {
				var name, ver, invDate string
				if err := rows.Scan(&name, &ver, &invDate); err != nil {
					return err
				}
				if ver == "" {
					continue
				}
				cur := parseSemverLike(ver)
				if compareSemverLike(cur, target) < 0 {
					out = append(out, driftDevice{
						Name:             name,
						AbrClientVersion: ver,
						InventoryDate:    invDate,
						Behind:           fmt.Sprintf("%s < %s", ver, targetVersion),
					})
				}
				if limit > 0 && len(out) >= limit {
					hitLimit = true
					break
				}
			}
			// Only check rows.Err() when we drained the cursor; an intentional
			// break-for-limit leaves it open and Err() would report nil anyway.
			if !hitLimit {
				if err := rows.Err(); err != nil {
					return fmt.Errorf("iterating inventory: %w", err)
				}
			}
			if out == nil {
				out = []driftDevice{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d device(s) below %s\n", len(out), targetVersion)
			for _, d := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s (last seen %s)\n", d.Name, d.AbrClientVersion, d.InventoryDate)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&targetVersion, "client-version", "", "Target AbR client version (e.g. 8.7.2)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap results (0 = unlimited)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}

// parseSemverLike turns "8.7.2" or "8.7.2.1" into a slice of ints; missing pieces are zero.
func parseSemverLike(s string) []int {
	parts := strings.Split(strings.TrimSpace(s), ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		// Drop suffixes like "-rc1"
		if idx := strings.IndexAny(p, "-+_"); idx >= 0 {
			p = p[:idx]
		}
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}

// compareSemverLike returns -1 if a < b, 0 if equal, +1 if a > b.
func compareSemverLike(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}
