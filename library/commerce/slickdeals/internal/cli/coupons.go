// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// coupons.go implements `slickdeals-pp-cli coupons`. v0.2 reaches the live
// coupon list via Slickdeals' Nuxt JSON endpoint /web-api/frontpage/featured-coupons/
// — the RSS coupon filter (newsearch.php?filter=f2=1) silently returns the
// frontpage (verified 2026-05-11), so the RSS surface cannot honestly serve
// coupons. The Nuxt endpoint was captured in v0.1 and returns the real data.
//
// Optional --store filter trims results client-side by merchant name.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// featuredCouponsPath is the live Nuxt JSON endpoint v0.1 captured.
const featuredCouponsPath = "/web-api/frontpage/featured-coupons/"

func newCouponsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var storeFilter string

	cmd := &cobra.Command{
		Use:         "coupons",
		Short:       "List Slickdeals featured coupons (live Nuxt JSON)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetch the live Slickdeals featured-coupons list via the Nuxt JSON endpoint.

The RSS coupon-filter pattern (newsearch.php?filter=f2=1&rss=1) does not
honor the filter — Slickdeals returns the frontpage feed regardless — so
this command uses /web-api/frontpage/featured-coupons/ instead, which
returns the actual featured-coupon set.`,
		Example: strings.Trim(`
  # Top 5 featured coupons as JSON
  slickdeals-pp-cli coupons --limit 5 --json

  # Filter by merchant
  slickdeals-pp-cli coupons --store amazon --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(featuredCouponsPath, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			items := extractSearchResults(data)

			if storeFilter != "" {
				needle := strings.ToLower(storeFilter)
				kept := items[:0]
				for _, raw := range items {
					if matchesStoreFilter(raw, needle) {
						kept = append(kept, raw)
					}
				}
				items = kept
			}

			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			body, err := json.Marshal(items)
			if err != nil {
				return fmt.Errorf("marshaling coupons: %w", err)
			}
			prov := DataProvenance{Source: "live", ResourceType: "coupons"}
			wrapped, err := wrapWithProvenance(body, prov)
			if err != nil {
				return err
			}
			printProvenance(cmd, len(items), prov)
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(wrapped), flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum coupons to return")
	cmd.Flags().StringVar(&storeFilter, "store", "", "Filter by merchant/store name (case-insensitive substring match)")

	return cmd
}

// matchesStoreFilter checks an opaque coupon JSON row for a store/merchant
// substring match. Looks at common field names; Slickdeals' featured-coupons
// payload uses "store", "storeName", "merchant", and sometimes a nested
// "store" object with "name". Empty needle returns true.
func matchesStoreFilter(raw json.RawMessage, needle string) bool {
	if needle == "" {
		return true
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	for _, key := range []string{"store", "storeName", "merchant", "store_slug", "advertiserName", "title", "name"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.Contains(strings.ToLower(val), needle) {
				return true
			}
		case map[string]any:
			for _, inner := range []string{"name", "slug", "title"} {
				if s, ok := val[inner].(string); ok && strings.Contains(strings.ToLower(s), needle) {
					return true
				}
			}
		}
	}
	return false
}
