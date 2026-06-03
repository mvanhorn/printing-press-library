// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/ahrefs/internal/client"
	"github.com/spf13/cobra"
)

type organicKeywordCompositeRow struct {
	Keyword           string `json:"keyword"`
	KeywordCountry    string `json:"keyword_country,omitempty"`
	Volume            int    `json:"volume"`
	KeywordDifficulty int    `json:"keyword_difficulty"`
	BestPosition      *int   `json:"best_position,omitempty"`
	BestPositionURL   string `json:"best_position_url,omitempty"`
	SumTraffic        int    `json:"sum_traffic"`
	CPC               int    `json:"cpc,omitempty"`
}

type backlinkCompositeRow struct {
	URLFrom            string   `json:"url_from"`
	DomainRatingSource *float64 `json:"domain_rating_source,omitempty"`
	FirstSeen          string   `json:"first_seen,omitempty"`
	TrafficDomain      int      `json:"traffic_domain,omitempty"`
}

func todayUTCDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func validateCompositeMode(cmd *cobra.Command, mode string) error {
	if !cmd.Flags().Changed("mode") {
		return nil
	}
	allowed := []string{"exact", "prefix", "domain", "subdomains"}
	for _, v := range allowed {
		if mode == v {
			return nil
		}
	}
	return fmt.Errorf("invalid --mode %q: must be one of %s", mode, strings.Join(allowed, ", "))
}

func compositeWhere(parts ...string) string {
	kept := make([]json.RawMessage, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, json.RawMessage(part))
		}
	}
	if len(kept) == 0 {
		return ""
	}
	if len(kept) == 1 {
		return string(kept[0])
	}
	out, err := json.Marshal(map[string][]json.RawMessage{"and": kept})
	if err != nil {
		return ""
	}
	return string(out)
}

func compositeNumberWhere(field string, op string, value int) string {
	out, err := json.Marshal(map[string]any{
		"field": field,
		"is":    []any{op, value},
	})
	if err != nil {
		return ""
	}
	return string(out)
}

func fetchCompositeRows[T any](cmd *cobra.Command, c *client.Client, flags *rootFlags, path string, params map[string]string) ([]T, DataProvenance, error) {
	data, prov, err := resolveRead(cmd.Context(), c, flags, "site_explorer", false, path, params, nil)
	if err != nil {
		return nil, prov, err
	}
	if flags.dryRun {
		return nil, prov, nil
	}
	data = extractCompositeArray(data)
	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, prov, fmt.Errorf("decoding %s response: %w", path, err)
	}
	return rows, prov, nil
}

func fetchCompositeObject(cmd *cobra.Command, c *client.Client, flags *rootFlags, path string, params map[string]string) (map[string]any, DataProvenance, error) {
	data, prov, err := resolveRead(cmd.Context(), c, flags, "site_explorer", false, path, params, nil)
	if err != nil {
		return nil, prov, err
	}
	if flags.dryRun {
		return nil, prov, nil
	}
	data = extractResponseData(data)
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, prov, fmt.Errorf("decoding %s response: %w", path, err)
	}
	if len(obj) == 1 {
		for _, value := range obj {
			nestedData, err := json.Marshal(value)
			if err != nil {
				continue
			}
			var nested map[string]any
			if err := json.Unmarshal(nestedData, &nested); err == nil {
				return nested, prov, nil
			}
		}
	}
	return obj, prov, nil
}

func extractCompositeArray(data json.RawMessage) json.RawMessage {
	data = extractResponseData(data)
	var rows []json.RawMessage
	if err := json.Unmarshal(data, &rows); err == nil {
		return data
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return data
	}
	for _, key := range []string{"results", "data", "items", "keywords", "backlinks"} {
		if raw, ok := envelope[key]; ok {
			var nested []json.RawMessage
			if err := json.Unmarshal(raw, &nested); err == nil {
				return raw
			}
		}
	}
	return data
}

func mergeCompositeProvenance(provs ...DataProvenance) DataProvenance {
	out := DataProvenance{Source: "live", ResourceType: "site_explorer"}
	filtered := make([]DataProvenance, 0, len(provs))
	for _, prov := range provs {
		if prov.Source != "" {
			filtered = append(filtered, prov)
		}
	}
	if len(filtered) == 0 {
		return out
	}
	out = filtered[0]
	out.ResourceType = "site_explorer"
	for _, prov := range filtered[1:] {
		if prov.Source != out.Source {
			out.Source = "mixed"
		}
		if out.SyncedAt == nil || (prov.SyncedAt != nil && prov.SyncedAt.After(*out.SyncedAt)) {
			out.SyncedAt = prov.SyncedAt
		}
		if out.Reason == "" {
			out.Reason = prov.Reason
		} else if prov.Reason != "" && prov.Reason != out.Reason {
			out.Reason = "multiple"
		}
		if out.Freshness == nil {
			out.Freshness = prov.Freshness
		}
	}
	return out
}

func limitCompositeRows[T any](rows []T, limit int) []T {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func printCompositeOutput(cmd *cobra.Command, payload any, count int, prov DataProvenance, flags *rootFlags) error {
	return printCompositeOutputWithCompact(cmd, payload, nil, count, prov, flags)
}

func printCompositeOutputWithCompact(cmd *cobra.Command, payload any, compactPayload any, count int, prov DataProvenance, flags *rootFlags) error {
	if flags.compact && flags.selectFields == "" && compactPayload != nil {
		payload = compactPayload
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	printProvenance(cmd, count, prov)
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		filtered := json.RawMessage(data)
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		wrapped, wrapErr := wrapWithProvenance(filtered, prov)
		if wrapErr != nil {
			return wrapErr
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var items []map[string]any
		if json.Unmarshal(data, &items) == nil && len(items) > 0 {
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				return err
			}
			if len(items) >= 25 {
				fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
			}
			return nil
		}
	}
	return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
}
