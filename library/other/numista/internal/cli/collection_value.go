// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

type collectionValueItem struct {
	ID             int64   `json:"id"`
	TypeID         int64   `json:"type_id"`
	IssueID        int64   `json:"issue_id"`
	Grade          string  `json:"grade"`
	Currency       string  `json:"currency"`
	EstimatedValue float64 `json:"estimated_value"`
	MissingPrice   bool    `json:"missing_price"`
}

// PATCH: quota-aware collection valuation workflow.
func newCollectionValueCmd(flags *rootFlags) *cobra.Command {
	var currency string
	var collectionID int64
	var noRefreshStale bool

	cmd := &cobra.Command{
		Use:     "value [user_id]",
		Short:   "Sum the current estimated value of every item in a Numista user's collection. Refuses to start when remaining quota is less than the number of items needing fresh prices.",
		Example: "  numista-pp-cli collection value 12345 --json\n  numista-pp-cli collection value 12345 --currency USD --collection-id 99 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			userID, err := parsePositiveInt64Arg("user_id", args[0])
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			path := fmt.Sprintf("/users/%d/collected_items", userID)
			params := map[string]string{"lang": "en"}
			if collectionID != 0 {
				params["collection"] = strconv.FormatInt(collectionID, 10)
			}
			data, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items, err := extractObjectList(data)
			if err != nil {
				return err
			}
			priceKeys := distinctPriceKeys(items)
			if !noRefreshStale {
				q, err := readQuota(cmd.Context(), s)
				if err != nil {
					return err
				}
				if len(priceKeys) > q.Remaining {
					return usageErr(fmt.Errorf("Collection has %d items needing price lookups but only %d quota remaining this month. Use --no-refresh-stale to skip live calls and use cached prices, or wait for monthly reset.", len(priceKeys), q.Remaining))
				}
			}
			outItems := make([]collectionValueItem, 0, len(items))
			total := 0.0
			priced := 0
			priceCache := map[string][]map[string]any{}
			for _, item := range items {
				id, _ := numberField(item, "id")
				typeID := nestedID(item, "type")
				issueID := nestedID(item, "issue")
				grade := stringField(item, "grade")
				row := collectionValueItem{ID: id, TypeID: typeID, IssueID: issueID, Grade: grade, Currency: currency}
				if typeID == 0 || issueID == 0 || noRefreshStale {
					row.MissingPrice = true
					outItems = append(outItems, row)
					continue
				}
				key := priceKey(typeID, issueID)
				prices, ok := priceCache[key]
				if !ok {
					pricePath := fmt.Sprintf("/types/%d/issues/%d/prices", typeID, issueID)
					priceData, err := c.Get(pricePath, map[string]string{"currency": currency, "lang": "en"})
					if err != nil {
						row.MissingPrice = true
						outItems = append(outItems, row)
						continue
					}
					prices, _ = extractObjectList(priceData)
					priceCache[key] = prices
				}
				if value, ok := priceForGrade(prices, grade); ok {
					row.EstimatedValue = value
					total += value
					priced++
				} else {
					row.MissingPrice = true
				}
				outItems = append(outItems, row)
			}
			q, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			out := map[string]any{
				"user_id":  userID,
				"currency": currency,
				"totals": map[string]any{
					"items":               len(outItems),
					"items_priced":        priced,
					"items_missing_price": len(outItems) - priced,
					"estimated_value":     total,
				},
				"items":       outItems,
				"quota_after": q,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "EUR", "3-letter ISO 4217 currency code")
	cmd.Flags().Int64Var(&collectionID, "collection-id", 0, "Collection ID")
	cmd.Flags().BoolVar(&noRefreshStale, "no-refresh-stale", false, "Skip live price calls and use only cached pricing data")
	return cmd
}

func parsePositiveInt64Arg(name, value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, usageErr(fmt.Errorf("%s must be a positive integer; got %q", name, value))
	}
	return n, nil
}

func distinctPriceKeys(items []map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		typeID := nestedID(item, "type")
		issueID := nestedID(item, "issue")
		if typeID != 0 && issueID != 0 {
			out[priceKey(typeID, issueID)] = true
		}
	}
	return out
}

func priceKey(typeID, issueID int64) string {
	return fmt.Sprintf("%d/%d", typeID, issueID)
}

func nestedID(obj map[string]any, key string) int64 {
	if n, ok := numberField(obj, key); ok {
		return n
	}
	v, ok := obj[key]
	if !ok {
		if n, ok := numberField(obj, key+"_id"); ok {
			return n
		}
		return 0
	}
	if m, ok := v.(map[string]any); ok {
		if n, ok := numberField(m, "id"); ok {
			return n
		}
	}
	return 0
}

func stringField(obj map[string]any, key string) string {
	if v, ok := obj[key]; ok {
		switch x := v.(type) {
		case string:
			return x
		case map[string]any:
			if s, ok := x["name"].(string); ok {
				return s
			}
		}
	}
	return ""
}

func priceForGrade(prices []map[string]any, grade string) (float64, bool) {
	var fallback float64
	var haveFallback bool
	for _, p := range prices {
		value, ok := firstFloatField(p, "estimated_value", "value", "price", "mean", "average")
		if !ok {
			continue
		}
		pg := stringField(p, "grade")
		if grade != "" && strings.EqualFold(pg, grade) {
			return value, true
		}
		if !haveFallback {
			fallback = value
			haveFallback = true
		}
	}
	if grade == "" && haveFallback {
		return fallback, true
	}
	return 0, false
}

func firstFloatField(obj map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		v, ok := obj[key]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case float64:
			return x, true
		case int:
			return float64(x), true
		case int64:
			return float64(x), true
		case json.Number:
			f, err := x.Float64()
			return f, err == nil
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
			return f, err == nil
		}
	}
	return 0, false
}
