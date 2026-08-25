// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type marketsDiffField struct {
	Field  string `json:"field"`
	PM     any    `json:"polymarket"`
	Kalshi any    `json:"kalshi"`
}

type marketsDiffResult struct {
	PMID     string             `json:"pmId"`
	KalshiID string             `json:"kalshiId"`
	Fields   []marketsDiffField `json:"fields"`
}

func addMarketsDiffCmd(rootCmd *cobra.Command, flags *rootFlags) {
	for _, child := range rootCmd.Commands() {
		if child.Name() == "markets" {
			child.AddCommand(newMarketsDiffCmd(flags))
			return
		}
	}
}

func newMarketsDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "diff <pm-slug-or-id> <kalshi-ticker>",
		Short:       "Diff one Polymarket market against one Kalshi market",
		Example:     `  prediction-goat-pp-cli markets diff <pm-slug> <kalshi-ticker> --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("markets diff requires two positional args: <pm-slug-or-id> <kalshi-ticker>"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			result, err := runMarketsDiff(cmd, dbPath, args[0], args[1])
			if err != nil {
				return fmt.Errorf("markets diff: %w", err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printSimpleTable(cmd.OutOrStdout(), []string{"Field", "Polymarket", "Kalshi"}, marketsDiffRows(result.Fields))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func runMarketsDiff(cmd *cobra.Command, dbPath, pmKey, kalshiKey string) (marketsDiffResult, error) {
	db, err := store.Open(dbPath)
	if err != nil {
		return marketsDiffResult{}, err
	}
	defer db.Close()
	pm, err := localMarketByKey(cmd, db, "markets", `json_extract(data,'$.slug')`, pmKey)
	if err != nil {
		return marketsDiffResult{}, err
	}
	if pm == nil {
		pm, err = livePolymarketBySlug(cmd, pmKey)
		if err != nil {
			return marketsDiffResult{}, err
		}
	}
	ks, err := localMarketByKey(cmd, db, "kalshi_markets", `json_extract(data,'$.ticker')`, kalshiKey)
	if err != nil {
		return marketsDiffResult{}, err
	}
	if ks == nil {
		ks, err = liveKalshiMarketByTicker(cmd, kalshiKey)
		if err != nil {
			return marketsDiffResult{}, err
		}
	}
	return buildMarketsDiff(pmKey, kalshiKey, pm, ks), nil
}

func localMarketByKey(cmd *cobra.Command, db *store.Store, resourceType, jsonField, key string) (map[string]any, error) {
	row := db.DB().QueryRowContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type=? AND (`+jsonField+`=? OR id=?) LIMIT 1`, resourceType, key, key)
	var data sql.NullString
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if !data.Valid {
		return nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data.String), &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func livePolymarketBySlug(cmd *cobra.Command, slug string) (map[string]any, error) {
	u := "https://gamma-api.polymarket.com/markets/slug/" + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "prediction-goat-pp-cli/1.0")
	// PATCH(markets-diff-pm-fallback-timeout): http.DefaultClient has no
	// timeout, so a slow or unreachable Gamma API would block `markets
	// diff` indefinitely. Other live HTTP paths in this CLI use 15-60s
	// timeouts (freshness.go and kalshi.Client). Greptile P1 on PR #780.
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("polymarket GET %s: HTTP %d: %s", slug, resp.StatusCode, truncate(string(body), 200))
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func liveKalshiMarketByTicker(cmd *cobra.Command, ticker string) (map[string]any, error) {
	body, err := kalshi.New().Get(cmd.Context(), "/markets/"+url.PathEscape(ticker), url.Values{})
	if err != nil {
		return nil, err
	}
	return kalshiEnvelopeObject(body, "market")
}

func buildMarketsDiff(pmKey, kalshiKey string, pm, ks map[string]any) marketsDiffResult {
	fields := make([]marketsDiffField, 0)
	fields = append(fields, marketsDiffField{Field: "title", PM: firstNonEmpty(jsonString(pm, "question"), jsonString(pm, "title")), Kalshi: jsonString(ks, "title")})
	fields = append(fields, marketsDiffField{Field: "impliedProbability", PM: jsonFloat(pm, "lastTradePrice"), Kalshi: jsonFloat(ks, "last_price_dollars")})
	fields = append(fields, marketsDiffField{Field: "volume24h", PM: firstFloat(pm, "volume24hr", "volume24h"), Kalshi: jsonFloat(ks, "volume_24h_fp")})
	fields = append(fields, marketsDiffField{Field: "liquidity", PM: jsonFloat(pm, "liquidityNum"), Kalshi: jsonFloat(ks, "liquidity_dollars")})
	fields = append(fields, marketsDiffField{Field: "endDate", PM: jsonString(pm, "endDate"), Kalshi: jsonString(ks, "expiration_time")})
	fields = append(fields, marketsDiffField{Field: "status", PM: pmStatus(pm), Kalshi: jsonString(ks, "status")})
	return marketsDiffResult{PMID: firstNonEmpty(jsonString(pm, "id"), jsonString(pm, "slug"), pmKey), KalshiID: firstNonEmpty(jsonString(ks, "ticker"), jsonString(ks, "id"), kalshiKey), Fields: fields}
}

func marketsDiffRows(fields []marketsDiffField) [][]string {
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Field, truncate(strings.TrimSpace(fmt.Sprint(field.PM)), 60), truncate(strings.TrimSpace(fmt.Sprint(field.Kalshi)), 60)})
	}
	return rows
}
