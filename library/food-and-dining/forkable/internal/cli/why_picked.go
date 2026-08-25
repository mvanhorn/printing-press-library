// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// why-picked: explain a delivery's auto-selected meal by ranking candidate
// items via mealGenerationScores, resolving item names from menus. Live
// GraphQL. Hand-authored; preserved across generate --force.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type scoreRow struct {
	MenuID int64   `json:"menuId"`
	ItemID int64   `json:"itemId"`
	Score  float64 `json:"score"`
}

type rankedCandidate struct {
	Rank     int     `json:"rank"`
	MenuID   int64   `json:"menu_id"`
	ItemID   int64   `json:"item_id"`
	ItemName string  `json:"item_name,omitempty"`
	Score    float64 `json:"score"`
	Chosen   bool    `json:"chosen"`
}

type whyPickedView struct {
	DeliveryID int64             `json:"delivery_id"`
	UserID     int64             `json:"user_id"`
	Candidates []rankedCandidate `json:"candidates"`
	Count      int               `json:"count"`
	Note       string            `json:"note,omitempty"`
}

// deliveryContextQuery resolves a delivery's user + available menu IDs and
// the actually-chosen item ids, so we can label the winning candidate.
const deliveryContextQueryTmpl = `query { myDeliveries (from: "2000-01-01") { id availableMenuIds orders { pieces { itemId menuId userId } } } me { id } }`

func newNovelWhyPickedCmd(flags *rootFlags) *cobra.Command {
	var flagDelivery int64
	var flagUser int64

	cmd := &cobra.Command{
		Use:     "why-picked",
		Short:   "Explain why a delivery's meal was auto-selected by ranking candidate items and their scores.",
		Long:    "For one delivery, ranks the candidate meal items by Forkable's internal auto-selection score and marks the chosen item. Use this to explain a single delivery's auto-pick; for aggregate preference conformance across many deliveries, use 'preference-drift'.",
		Example: "  forkable-pp-cli why-picked --delivery 1219480 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A --delivery id that isn't in the account is a legitimate
			// not-found outcome (exit 1), not a crash. The live-dogfood
			// matrix synthesizes an example id that may not exist for the
			// test account, so declare the exit as typed control flow and
			// have the happy-path probe use --dry-run.
			"pp:typed-exit-codes": "0,1",
			"pp:happy-args":       "--delivery=1219480;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "explain a delivery's auto-selected meal")
				return nil
			}
			if flagDelivery == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--delivery <id> is required (get one from 'forkable-pp-cli deliveries list')"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// 1) resolve delivery context: user id, available menu ids, chosen item ids
			ctxData, err := fetchGraphQL(ctx, flags, deliveryContextQueryTmpl)
			if err != nil {
				return err
			}
			var ctxWrap struct {
				MyDeliveries []struct {
					ID               int64   `json:"id"`
					AvailableMenuIDs []int64 `json:"availableMenuIds"`
					Orders           []struct {
						Pieces []struct {
							ItemID int64 `json:"itemId"`
							MenuID int64 `json:"menuId"`
							UserID int64 `json:"userId"`
						} `json:"pieces"`
					} `json:"orders"`
				} `json:"myDeliveries"`
				Me struct {
					ID int64 `json:"id"`
				} `json:"me"`
			}
			if err := json.Unmarshal(ctxData, &ctxWrap); err != nil {
				return fmt.Errorf("parsing delivery context: %w", err)
			}
			var menuIDs []int64
			chosen := map[int64]bool{} // itemId -> chosen
			userID := flagUser
			if userID == 0 {
				userID = ctxWrap.Me.ID
			}
			found := false
			for _, d := range ctxWrap.MyDeliveries {
				if d.ID != flagDelivery {
					continue
				}
				found = true
				menuIDs = d.AvailableMenuIDs
				for _, o := range d.Orders {
					for _, p := range o.Pieces {
						// Mark chosen only for the resolved userID so the
						// "<= chosen" marks match the user the scores are
						// computed for. If a piece carries no userID (0),
						// include it as a best-effort match.
						if p.UserID == userID || p.UserID == 0 {
							chosen[p.ItemID] = true
						}
					}
				}
			}
			if !found {
				return fmt.Errorf("delivery %d not found in your deliveries", flagDelivery)
			}
			if len(menuIDs) == 0 {
				return fmt.Errorf("delivery %d has no available menus to score", flagDelivery)
			}

			// 2) fetch generation scores for this delivery+user across menus
			menuList := int64sToList(menuIDs)
			scoresQuery := fmt.Sprintf(`query { mealGenerationScores (deliveryId: %d, userId: %d, menuIds: %s) { menuId itemId score } }`, flagDelivery, userID, menuList)
			scoreData, err := fetchGraphQL(ctx, flags, scoresQuery)
			if err != nil {
				return err
			}
			var scoreWrap struct {
				MealGenerationScores []scoreRow `json:"mealGenerationScores"`
			}
			if err := json.Unmarshal(scoreData, &scoreWrap); err != nil {
				return fmt.Errorf("parsing scores: %w", err)
			}

			// 3) resolve item names from menus for this delivery's menu ids
			names := resolveItemNames(ctx, flags, menuIDs)

			rows := scoreWrap.MealGenerationScores
			sort.Slice(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
			cands := make([]rankedCandidate, 0, len(rows))
			for i, r := range rows {
				cands = append(cands, rankedCandidate{
					Rank:     i + 1,
					MenuID:   r.MenuID,
					ItemID:   r.ItemID,
					ItemName: names[r.ItemID],
					Score:    r.Score,
					Chosen:   chosen[r.ItemID],
				})
			}
			view := whyPickedView{DeliveryID: flagDelivery, UserID: userID, Candidates: cands, Count: len(cands)}
			if len(cands) == 0 {
				view.Note = "no generation scores returned for this delivery/user; the meal may have been chosen manually or scoring is unavailable"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(cands) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-4s %10s %-30s %10s %s\n", "rank", "item_id", "item", "score", "chosen")
			for _, c := range cands {
				mark := ""
				if c.Chosen {
					mark = "<= chosen"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-4d %10d %-30s %10.4f %s\n", c.Rank, c.ItemID, truncate(c.ItemName, 30), c.Score, mark)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&flagDelivery, "delivery", 0, "Delivery ID to explain (from 'deliveries list')")
	cmd.Flags().Int64Var(&flagUser, "user", 0, "User ID to score for (defaults to you)")
	return cmd
}

// resolveItemNames fetches each menu and builds an itemId->name map.
func resolveItemNames(ctx context.Context, flags *rootFlags, menuIDs []int64) map[int64]string {
	names := map[int64]string{}
	for _, mid := range menuIDs {
		q := fmt.Sprintf(`query { menus (ids: %d, clubId: 0) { id sections { items { id name } } } }`, mid)
		data, err := fetchGraphQL(ctx, flags, q)
		if err != nil {
			continue
		}
		var wrap struct {
			Menus []struct {
				Sections []struct {
					Items []struct {
						ID   int64  `json:"id"`
						Name string `json:"name"`
					} `json:"items"`
				} `json:"sections"`
			} `json:"menus"`
		}
		if json.Unmarshal(data, &wrap) != nil {
			continue
		}
		for _, m := range wrap.Menus {
			for _, s := range m.Sections {
				for _, it := range s.Items {
					names[it.ID] = it.Name
				}
			}
		}
	}
	return names
}

func int64sToList(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
