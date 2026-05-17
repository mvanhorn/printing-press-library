// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

// PATCH: hydrate one OAuth-visible user collection into the local store.
func newUsersCollectionsHydrateCmd(flags *rootFlags) *cobra.Command {
	var collectionID int64
	var withPrices bool
	var currency string
	cmd := &cobra.Command{
		Use:     "hydrate <user_id> [--collection-id <int>] [--with-prices] [--currency EUR]",
		Short:   "Sync one user collection-folder into the local store, optionally fanning out prices for every item.",
		Example: "  numista-pp-cli users collections hydrate 12345 --collection-id 99 --with-prices --json",
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
			params := map[string]string{"lang": "en"}
			if collectionID != 0 {
				params["collection"] = strconv.FormatInt(collectionID, 10)
			}
			data, err := c.Get(fmt.Sprintf("/users/%d/collected_items", userID), params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items, err := extractObjectList(data)
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if withPrices {
				q, err := readQuota(cmd.Context(), s)
				if err != nil {
					return err
				}
				needed := len(distinctPriceKeys(items))
				if needed > q.Remaining {
					return usageErr(fmt.Errorf("collection has %d items needing price lookups but only %d quota remaining this month", needed, q.Remaining))
				}
			}
			synced := 0
			for _, item := range items {
				raw, _ := json.Marshal(item)
				if err := s.UpsertCollectedItems(json.RawMessage(raw)); err == nil {
					synced++
				}
			}
			pricesFetched := 0
			if withPrices {
				seen := map[string]bool{}
				for _, item := range items {
					typeID := nestedID(item, "type")
					issueID := nestedID(item, "issue")
					if typeID == 0 || issueID == 0 {
						continue
					}
					key := priceKey(typeID, issueID)
					if seen[key] {
						continue
					}
					seen[key] = true
					priceData, err := c.Get(fmt.Sprintf("/types/%d/issues/%d/prices", typeID, issueID), map[string]string{"currency": currency, "lang": "en"})
					if err != nil {
						return classifyAPIError(err, flags)
					}
					if err := s.Upsert("issue_prices", key+"."+currency, priceData); err != nil {
						// Cache write failed but the API response is still in hand;
						// surface the miss instead of silently lying about the count.
						fmt.Fprintf(os.Stderr, "warning: cache write for %s failed: %v\n", key, err)
						continue
					}
					pricesFetched++
				}
			}
			q, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"user_id":        userID,
				"collection_id":  collectionID,
				"items_synced":   synced,
				"prices_fetched": pricesFetched,
				"quota_after":    q,
			}, flags)
		},
	}
	cmd.Flags().Int64Var(&collectionID, "collection-id", 0, "Collection ID")
	cmd.Flags().BoolVar(&withPrices, "with-prices", false, "Fetch prices for every distinct type/issue in the collection")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "3-letter ISO 4217 currency code")
	return cmd
}
