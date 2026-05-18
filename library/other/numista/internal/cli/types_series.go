// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

// PATCH: hand-written full type/issue/price curve command promised by README Highlights.
func newTypesSeriesCmd(flags *rootFlags) *cobra.Command {
	var lang string
	var currency string

	cmd := &cobra.Command{
		Use:     "series <type_id> [--lang en|es|fr] [--currency EUR]",
		Short:   "For one Numista type, pull every issue + every grade's price into the local store, then print the full price/mintage curve.",
		Example: "  numista-pp-cli types series 11013 --json\n  numista-pp-cli types series 95420 --currency USD --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			typeID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || typeID <= 0 {
				return usageErr(fmt.Errorf("type_id must be a positive integer; got %q", args[0]))
			}
			if err := validateLang(lang); err != nil {
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
			typePath := "/types/" + strconv.FormatInt(typeID, 10)
			typeData, typeLive, err := quotaTrackedGet(cmd.Context(), c, s, typePath, map[string]string{"lang": lang})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			issuesPath := typePath + "/issues"
			issuesData, issuesLive, err := quotaTrackedGet(cmd.Context(), c, s, issuesPath, map[string]string{"lang": lang})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			issues, err := extractObjectList(issuesData)
			if err != nil {
				return err
			}
			liveCalls := boolToCount(typeLive) + boolToCount(issuesLive)
			cacheHits := 2 - liveCalls
			priceTotal := 0
			for i := range issues {
				issueID, ok := numberField(issues[i], "id")
				if !ok {
					issues[i]["prices"] = nil
					continue
				}
				pricePath := fmt.Sprintf("%s/issues/%d/prices", typePath, issueID)
				priceData, live, err := quotaTrackedGet(cmd.Context(), c, s, pricePath, map[string]string{"currency": currency, "lang": lang})
				if err != nil {
					issues[i]["prices"] = nil
					continue
				}
				if live {
					liveCalls++
				} else {
					cacheHits++
				}
				prices, _ := extractAnyList(priceData)
				issues[i]["prices"] = prices
				priceTotal += len(prices)
			}
			var typeObj any
			if err := json.Unmarshal(typeData, &typeObj); err != nil {
				typeObj = typeData
			}
			out := map[string]any{
				"type":   typeObj,
				"issues": issues,
				"totals": map[string]any{
					"issues":     len(issues),
					"prices":     priceTotal,
					"live_calls": liveCalls,
					"cache_hits": cacheHits,
				},
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "en", "Language (one of: en, es, fr)")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "3-letter ISO 4217 currency code")
	return cmd
}

func boolToCount(v bool) int {
	if v {
		return 1
	}
	return 0
}

func extractObjectList(raw json.RawMessage) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	for _, key := range []string{"issues", "types", "items", "results", "data"} {
		if v, ok := obj[key]; ok {
			if err := json.Unmarshal(v, &arr); err == nil {
				return arr, nil
			}
		}
	}
	return nil, fmt.Errorf("response did not contain a list")
}

func extractAnyList(raw json.RawMessage) ([]any, error) {
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	for _, key := range []string{"prices", "issues", "types", "items", "results", "data"} {
		if v, ok := obj[key]; ok {
			if err := json.Unmarshal(v, &arr); err == nil {
				return arr, nil
			}
		}
	}
	return nil, fmt.Errorf("response did not contain a list")
}

func numberField(obj map[string]any, key string) (int64, bool) {
	v, ok := obj[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return int64(x), x > 0
	case int64:
		return x, x > 0
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return n, err == nil && n > 0
	default:
		return 0, false
	}
}
