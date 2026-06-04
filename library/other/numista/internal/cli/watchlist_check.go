// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
	"github.com/spf13/cobra"
)

type watchlistDiff struct {
	TypeID          int64   `json:"type_id"`
	IssueID         int64   `json:"issue_id"`
	Grade           string  `json:"grade"`
	Currency        string  `json:"currency"`
	PrevPrice       float64 `json:"prev_price"`
	CurrentPrice    float64 `json:"current_price"`
	Delta           float64 `json:"delta"`
	DeltaPct        float64 `json:"delta_pct"`
	PrevCaptured    string  `json:"prev_captured"`
	CurrentCaptured string  `json:"current_captured"`
}

// PATCH: live watchlist refresh and local price-snapshot diffing.
func newWatchlistCheckCmd(flags *rootFlags) *cobra.Command {
	var currency string
	var lang string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "For every watched type, fetch current issues + prices, persist a fresh snapshot, and print the diff since the last snapshot.",
		Example: "  numista-pp-cli watchlist check --json\n" +
			"  numista-pp-cli watchlist check --currency USD --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLang(lang); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.WatchlistList(cmd.Context())
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			liveCalls := 0
			cacheHits := 0
			for _, entry := range entries {
				issuesPath := "/types/" + strconv.FormatInt(entry.TypeID, 10) + "/issues"
				issuesData, live, err := quotaTrackedGet(cmd.Context(), c, s, issuesPath, map[string]string{"lang": lang})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if live {
					liveCalls++
				} else {
					cacheHits++
				}
				issues, err := extractObjectList(issuesData)
				if err != nil {
					return err
				}
				for _, issue := range issues {
					issueID, ok := numberField(issue, "id")
					if !ok {
						continue
					}
					pricePath := fmt.Sprintf("/types/%d/issues/%d/prices", entry.TypeID, issueID)
					priceData, live, err := quotaTrackedGet(cmd.Context(), c, s, pricePath, map[string]string{"currency": currency, "lang": lang})
					if err != nil {
						return classifyAPIError(err, flags)
					}
					if live {
						liveCalls++
					} else {
						cacheHits++
					}
					prices, _ := extractObjectList(priceData)
					for _, p := range prices {
						grade := stringField(p, "grade")
						if grade == "" {
							continue
						}
						price, ok := firstFloatField(p, "estimated_value", "value", "price", "mean", "average")
						if !ok {
							continue
						}
						if err := s.InsertPriceSnapshot(cmd.Context(), entry.TypeID, issueID, grade, price, currency); err != nil {
							return err
						}
					}
				}
			}
			var diffs []watchlistDiff
			for _, entry := range entries {
				snapshots, err := s.LatestTwoPriceSnapshots(cmd.Context(), entry.TypeID)
				if err != nil {
					return err
				}
				byPair := map[string][]store.PriceSnapshot{}
				for _, snapshot := range snapshots {
					key := fmt.Sprintf("%d/%s", snapshot.IssueID, snapshot.Grade)
					byPair[key] = append(byPair[key], snapshot)
				}
				for _, pair := range byPair {
					if len(pair) < 2 {
						continue
					}
					cur := pair[0]
					prev := pair[1]
					if cur.Price == prev.Price {
						continue
					}
					delta := cur.Price - prev.Price
					pct := 0.0
					if prev.Price != 0 {
						pct = delta / prev.Price * 100
					}
					diffs = append(diffs, watchlistDiff{
						TypeID:          cur.TypeID,
						IssueID:         cur.IssueID,
						Grade:           cur.Grade,
						Currency:        cur.Currency,
						PrevPrice:       prev.Price,
						CurrentPrice:    cur.Price,
						Delta:           delta,
						DeltaPct:        pct,
						PrevCaptured:    prev.CapturedAt.Format("2006-01-02T15:04:05Z07:00"),
						CurrentCaptured: cur.CapturedAt.Format("2006-01-02T15:04:05Z07:00"),
					})
				}
			}
			q, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"checked":     len(entries),
				"diffs":       diffs,
				"live_calls":  liveCalls,
				"cache_hits":  cacheHits,
				"quota_after": q,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "EUR", "3-letter ISO 4217 currency code")
	cmd.Flags().StringVar(&lang, "lang", "en", "Language (one of: en, es, fr)")
	return cmd
}
