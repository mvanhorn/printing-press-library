// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

type licenseKeyRow struct {
	KeyID           string `json:"key_id"`
	KeyShort        string `json:"key_short,omitempty"`
	Status          string `json:"status"`
	CustomerID      string `json:"customer_id,omitempty"`
	CustomerEmail   string `json:"customer_email,omitempty"`
	VariantID       string `json:"variant_id,omitempty"`
	VariantName     string `json:"variant_name,omitempty"`
	ActivationLimit int    `json:"activation_limit"`
	Activations     int    `json:"activations"`
	CreatedAt       string `json:"created_at,omitempty"`
}

type variantRollup struct {
	VariantID         string  `json:"variant_id"`
	VariantName       string  `json:"variant_name,omitempty"`
	KeysIssued        int     `json:"keys_issued"`
	KeysActive        int     `json:"keys_active"`
	KeysDisabled      int     `json:"keys_disabled"`
	KeysUnknown       int     `json:"keys_unknown_status,omitempty"`
	TotalActivations  int     `json:"total_activations"`
	AvgActivationsPer float64 `json:"avg_activations_per_key"`
}

type licenseRollupView struct {
	Variants         []variantRollup `json:"variants"`
	TopByActivations []licenseKeyRow `json:"top_keys_by_activations"`
	TotalKeys        int             `json:"total_keys"`
	TotalActivations int             `json:"total_activations"`
	Note             string          `json:"note,omitempty"`
}

func newNovelLicenseRollupCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var topN int
	cmd := &cobra.Command{
		Use:   "license-rollup",
		Short: "Per-variant and per-key activation stats from a 3-table join (license-keys × instances × variants)",
		Long: `Joins the local 'license-keys', 'license-key-instances', and 'variants'
resources to produce per-variant aggregates (issued, active, disabled, total
activations, avg activations per key) and a top-N list of keys by activation
count.

Use this command for seat/usage distribution across keys. To act on one
refunded order (disable keys, audit instances), use 'refund-cascade' instead.

Data source: local. Run 'sync --resources license-keys,license-key-instances,variants,customers' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources license-keys,license-key-instances,variants,customers\n  lemonsqueezy-pp-cli license-rollup --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topN <= 0 {
				topN = 10
			}
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "license-keys") {
				hintIfStale(cmd, db, "license-keys", flags.maxAge)
			}

			view, err := buildLicenseRollup(db, topN)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().IntVar(&topN, "top", 10, "Number of top keys by activation count to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func buildLicenseRollup(db *store.Store, topN int) (licenseRollupView, error) {
	view := licenseRollupView{Variants: []variantRollup{}, TopByActivations: []licenseKeyRow{}}

	variantNames := loadVariantNames(db)
	customerEmails := loadCustomerEmails(db)
	activations := loadInstanceCountsByKey(db)

	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'license-keys' LIMIT 50000`,
	)
	if err != nil {
		return view, fmt.Errorf("querying license-keys: %w", err)
	}
	defer rows.Close()

	perVariant := map[string]*variantRollup{}
	var allKeys []licenseKeyRow

	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			ID         string `json:"id"`
			Attributes struct {
				Status          string `json:"status"`
				KeyShort        string `json:"key_short"`
				CustomerID      any    `json:"customer_id"`
				ProductID       any    `json:"product_id"`
				VariantID       any    `json:"variant_id"`
				ActivationLimit any    `json:"activation_limit"`
				Activations     any    `json:"activation_usage"`
				CreatedAt       string `json:"created_at"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		variantID := toStringLS(env.Attributes.VariantID)
		customerID := toStringLS(env.Attributes.CustomerID)
		activated := int(toFloatLS(env.Attributes.Activations))
		if instanceCount, ok := activations[env.ID]; ok && instanceCount > activated {
			activated = instanceCount
		}
		row := licenseKeyRow{
			KeyID:           env.ID,
			KeyShort:        env.Attributes.KeyShort,
			Status:          env.Attributes.Status,
			CustomerID:      customerID,
			CustomerEmail:   customerEmails[customerID],
			VariantID:       variantID,
			VariantName:     variantNames[variantID],
			ActivationLimit: int(toFloatLS(env.Attributes.ActivationLimit)),
			Activations:     activated,
			CreatedAt:       env.Attributes.CreatedAt,
		}
		allKeys = append(allKeys, row)

		vKey := variantID
		if vKey == "" {
			vKey = "(unknown)"
		}
		v, ok := perVariant[vKey]
		if !ok {
			v = &variantRollup{VariantID: vKey, VariantName: variantNames[vKey]}
			perVariant[vKey] = v
		}
		v.KeysIssued++
		v.TotalActivations += activated
		switch env.Attributes.Status {
		case "active":
			v.KeysActive++
		case "disabled", "inactive", "expired":
			v.KeysDisabled++
		default:
			// Empty or unrecognised status: count as unknown rather than silently
			// inflating "active" with rows the API didn't classify.
			v.KeysUnknown++
		}
	}

	for _, v := range perVariant {
		if v.KeysIssued > 0 {
			v.AvgActivationsPer = float64(v.TotalActivations) / float64(v.KeysIssued)
		}
		view.Variants = append(view.Variants, *v)
		view.TotalKeys += v.KeysIssued
		view.TotalActivations += v.TotalActivations
	}
	sort.Slice(view.Variants, func(i, j int) bool {
		return view.Variants[i].KeysIssued > view.Variants[j].KeysIssued
	})

	sort.Slice(allKeys, func(i, j int) bool {
		return allKeys[i].Activations > allKeys[j].Activations
	})
	if len(allKeys) > topN {
		allKeys = allKeys[:topN]
	}
	view.TopByActivations = allKeys

	if view.TotalKeys == 0 {
		view.Note = "no license-keys in local mirror; run 'sync --resources license-keys,license-key-instances' first"
	}
	return view, nil
}

func loadVariantNames(db *store.Store) map[string]string {
	out := map[string]string{}
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'variants' LIMIT 10000`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var env struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		}
		if json.Unmarshal([]byte(data.String), &env) != nil {
			continue
		}
		if env.ID != "" {
			out[env.ID] = env.Attributes.Name
		}
	}
	return out
}

func loadInstanceCountsByKey(db *store.Store) map[string]int {
	out := map[string]int{}
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'license-key-instances' LIMIT 200000`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var env struct {
			Attributes struct {
				LicenseKeyID any `json:"license_key_id"`
			} `json:"attributes"`
		}
		if json.Unmarshal([]byte(data.String), &env) != nil {
			continue
		}
		keyID := toStringLS(env.Attributes.LicenseKeyID)
		if keyID != "" {
			out[keyID]++
		}
	}
	return out
}
