// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/store"
	"github.com/spf13/cobra"
)

func newBroadcastsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "broadcasts",
		Short: "Broadcast planning helpers not provided by the API",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBroadcastsWhatifCmd(flags))
	return cmd
}

func newBroadcastsWhatifCmd(flags *rootFlags) *cobra.Command {
	var segment string
	var openRate float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "whatif",
		Short: "Estimate audience size, predicted opens, and hygiene risk for a draft",
		Example: strings.Trim(`
  # Project against a tag-based segment
  bento-pp-cli broadcasts whatif --segment customers

  # Override the historical open rate
  bento-pp-cli broadcasts whatif --segment vip --open-rate 0.42
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would project broadcast against segment", segment)
				return nil
			}
			if segment == "" {
				return cmd.Help()
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			subs, err := loadLocalSubscribers(db)
			if err != nil {
				return fmt.Errorf("loading subscribers: %w", err)
			}
			if len(subs) == 0 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store' to populate"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("no subscribers in local store; run 'bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store' first (Enterprise accounts can use 'bento-pp-cli sync --resources subscribers')"))
			}

			audience := 0
			hygieneRisk := 0
			for _, s := range subs {
				match := false
				for _, t := range subscriberTags(s) {
					if strings.EqualFold(t, segment) {
						match = true
						break
					}
				}
				if !match {
					continue
				}
				audience++
				// hygiene risk: unconfirmed, hard-bounced, or marked invalid.
				if v, ok := s["bounced_at"].(string); ok && v != "" {
					hygieneRisk++
					continue
				}
				if v, ok := s["unsubscribed_at"].(string); ok && v != "" {
					hygieneRisk++
					continue
				}
				if v, ok := s["valid"].(bool); ok && !v {
					hygieneRisk++
				}
			}

			rate := openRate
			if rate <= 0 {
				rate = inferOpenRate(db)
			}
			projection := map[string]any{
				"segment":          segment,
				"audience":         audience,
				"hygiene_risk":     hygieneRisk,
				"predicted_opens":  int(float64(audience-hygieneRisk) * rate),
				"open_rate_used":   rate,
				"open_rate_source": "historical_broadcasts",
			}
			if openRate > 0 {
				projection["open_rate_source"] = "user_override"
			}
			return printJSONFiltered(cmd.OutOrStdout(), projection, flags)
		},
	}
	cmd.Flags().StringVar(&segment, "segment", "", "Tag name (or segment id) defining the audience")
	cmd.Flags().Float64Var(&openRate, "open-rate", 0, "Override the historical open rate (0-1)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

// inferOpenRate averages opens/total across synced broadcasts. Falls back to
// 0.25 (the published Bento ecommerce average) when no broadcast data is
// in the store yet.
func inferOpenRate(db *store.Store) float64 {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'broadcasts'`)
	if err != nil {
		return 0.25
	}
	defer rows.Close()
	var num, denom float64
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		opens, _ := obj["opens"].(float64)
		total, _ := obj["total_recipients"].(float64)
		if total > 0 {
			num += opens
			denom += total
		}
	}
	if denom == 0 {
		return 0.25
	}
	return num / denom
}
