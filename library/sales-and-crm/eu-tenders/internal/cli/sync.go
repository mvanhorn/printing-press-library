// Copyright 2026 Mathias Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "eu-tenders-pp-cli", "notices.db")
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		query   string
		country string
		cpv     string
		since   string
		limit   int
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync TED procurement notices to the local SQLite store",
		Long: `Pull TED notices matching your filters and store them locally for offline analysis.

Examples:
  eu-tenders-pp-cli sync --country DEU --cpv 72000000 --since 2024-01-01
  eu-tenders-pp-cli sync --query "buyer-country=FRA AND notice-type=can-standard" --since 2024-06-01`,
		Annotations: map[string]string{
			"pp:endpoint": "notices.search",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// Build query string from convenience flags.
			q := buildSyncQuery(query, country, cpv, since)
			if q == "" {
				return fmt.Errorf("provide at least one filter: --query, --country, --cpv, or --since\nhint: e.g. --country DEU --cpv 72000000 --since 2024-01-01")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store at %s: %w\nhint: check write permissions", dbPath, err)
			}
			defer st.Close()

			queryKey := fmt.Sprintf("sync:%s", q)
			count, err := syncNoticesWithClient(c, st, q, limit, queryKey, flags, cmd)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"synced": count,
					"query":  q,
					"db":     dbPath,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d notices → %s\n", count, dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Expert query string (overrides --country/--cpv/--since)")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code (e.g. DEU, FRA, NLD)")
	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV division prefix (e.g. 72000000 or 45)")
	cmd.Flags().StringVar(&since, "since", "", "Sync notices published on or after this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum notices to sync (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

// buildSyncQuery constructs a TED expert query string from convenience flags.
func buildSyncQuery(query, country, cpv, since string) string {
	if query != "" {
		return query
	}
	var parts []string
	if country != "" {
		parts = append(parts, fmt.Sprintf("buyer-country=%s", strings.ToUpper(country)))
	}
	if cpv != "" {
		cpvNorm := normalizeCPV(cpv)
		parts = append(parts, fmt.Sprintf("classification-cpv IN (%s)", cpvNorm))
	}
	if since != "" {
		datePart := strings.ReplaceAll(since, "-", "")
		parts = append(parts, fmt.Sprintf("PD>=%s", datePart))
	}
	return strings.Join(parts, " AND ")
}

// normalizeCPV turns a short CPV prefix like "45" into "45000000".
func normalizeCPV(cpv string) string {
	cpv = strings.TrimSpace(cpv)
	if len(cpv) < 8 {
		return cpv + strings.Repeat("0", 8-len(cpv))
	}
	return cpv
}

type tedSearchRequest struct {
	Query              string   `json:"query"`
	Fields             []string `json:"fields"`
	Limit              int      `json:"limit"`
	PaginationMode     string   `json:"paginationMode"`
	Scope              string   `json:"scope"`
	Page               int      `json:"page,omitempty"`
	IterationNextToken string   `json:"iterationNextToken,omitempty"`
}

type tedSearchResponse struct {
	Notices            []map[string]interface{} `json:"notices"`
	Total              int                      `json:"totalNoticeCount"`
	IterationNextToken string                   `json:"iterationNextToken"`
}

var tedSyncFields = []string{
	"publication-number",
	"notice-type",
	"publication-date",
	"buyer-name",
	"buyer-country",
	"classification-cpv",
	"result-value-notice",
	"result-value-cur-notice",
	"winner-name",
	"winner-country",
	"procedure-type",
	"deadline-receipt-tender-date-lot",
	"title-lot",
	"title-proc",
	"previous-notice-id-proc",
	"place-of-performance-post-code-part",
}

// clientPoster is satisfied by client.Client.
type clientPoster interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

func syncNoticesWithClient(c clientPoster, st *store.Store, query string, limit int, queryKey string, flags *rootFlags, cmd *cobra.Command) (int, error) {
	const pageSize = 250
	var (
		count          int
		iterationToken string
	)

	for {
		remaining := 0
		if limit > 0 {
			remaining = limit - count
			if remaining <= 0 {
				break
			}
		}

		batchSize := pageSize
		if limit > 0 && remaining < batchSize {
			batchSize = remaining
		}

		// ITERATION mode uses opaque iterationId tokens, not page numbers.
		// Sending both causes the API to restart from the beginning each call.
		req := tedSearchRequest{
			Query:              query,
			Fields:             tedSyncFields,
			Limit:              batchSize,
			PaginationMode:     "ITERATION",
			Scope:              "ALL",
			IterationNextToken: iterationToken,
		}

		raw, _, err := c.Post("/v3/notices/search", req)
		if err != nil {
			return count, classifyAPIError(err, flags)
		}

		var resp tedSearchResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return count, fmt.Errorf("parse TED response: %w", err)
		}

		if len(resp.Notices) == 0 {
			break
		}

		iterationToken = resp.IterationNextToken
		now := time.Now().UTC().Format(time.RFC3339)

		for _, rawNotice := range resp.Notices {
			n := extractNotice(rawNotice, now)
			if err := st.UpsertNotice(n); err != nil {
				fmt.Fprintf(os.Stderr, "warn: upsert %s: %v\n", n.ID, err)
			}
			count++
		}

		if !flags.quiet {
			fmt.Fprintf(os.Stderr, "synced %d/%d notices...\n", count, resp.Total)
		}

		if resp.IterationNextToken == "" || len(resp.Notices) < batchSize || (limit > 0 && count >= limit) {
			break
		}
	}

	_ = st.SetLastSyncDate(queryKey, time.Now().UTC().Format("2006-01-02"))
	return count, nil
}

func extractNotice(raw map[string]interface{}, syncedAt string) store.Notice {
	n := store.Notice{
		Currency: "EUR",
		SyncedAt: syncedAt,
	}

	if v, ok := raw["publication-number"]; ok {
		n.ID = fmt.Sprintf("%v", v)
	}
	if v, ok := raw["notice-type"]; ok {
		n.NoticeType = store.ResolveMultilingual(v)
	}
	if v, ok := raw["publication-date"]; ok {
		s := fmt.Sprintf("%v", v)
		// Trim timezone offset (e.g. "2026-04-08+02:00" → "2026-04-08").
		if len(s) > 10 {
			s = s[:10]
		}
		n.PublicationDate = s
	}
	if v, ok := raw["buyer-name"]; ok {
		n.BuyerName = store.ResolveMultilingual(v)
	}
	if v, ok := raw["buyer-country"]; ok {
		n.BuyerCountry = store.ResolveMultilingual(v)
	}
	if v, ok := raw["classification-cpv"]; ok {
		n.CPVCode, n.CPVCodesJSON = store.CPVFromList(v)
	}
	if v, ok := raw["result-value-notice"]; ok {
		n.ContractValue = extractFloat(v)
		if n.EstimatedValue == 0 {
			n.EstimatedValue = n.ContractValue
		}
	}
	if v, ok := raw["result-value-cur-notice"]; ok {
		// This field is the currency string (e.g. "EUR"), not a numeric value.
		if s, ok := v.(string); ok && s != "" {
			n.Currency = s
		}
	}
	if v, ok := raw["winner-name"]; ok {
		n.WinnerName = store.ResolveMultilingual(v)
	}
	if v, ok := raw["winner-country"]; ok {
		n.WinnerCountry = store.ResolveMultilingual(v)
	}
	if v, ok := raw["procedure-type"]; ok {
		n.ProcedureType = store.ResolveMultilingual(v)
	}
	if v, ok := raw["deadline-receipt-tender-date-lot"]; ok {
		n.SubmissionDeadline = store.ResolveMultilingual(v)
	}
	// PATCH(amend-2026-06-09: prefer title-proc over title-lot) — many buyers
	// (notably BE/Wallonia/Brussels) fill title-lot with the lot reference code
	// plus lot number (e.g. "EPV/2026/03/TP - 1"), not a human-readable subject.
	// title-proc carries the descriptive procedure title in those cases, so it is
	// the more reliable first choice. Fall back to title-lot (still descriptive
	// for notices that omit title-proc), then the legacy contract-title field for
	// older eForms notices. Reference-shaped lot titles are dropped in favor of a
	// real subject when one exists.
	for _, field := range []string{"title-proc", "title-lot", "contract-title"} {
		if n.Title != "" {
			break
		}
		if v, ok := raw[field]; ok {
			n.Title = store.ResolveMultilingual(v)
		}
	}
	if v, ok := raw["previous-notice-id-proc"]; ok {
		n.PreviousNoticeID = store.ResolveMultilingual(v)
	}
	if v, ok := raw["place-of-performance-post-code-part"]; ok {
		n.PlaceOfPerformance = store.ResolveMultilingual(v)
	}

	if n.ID != "" {
		n.NoticeURL = fmt.Sprintf("https://ted.europa.eu/en/notice/-/detail/%s", n.ID)
	}

	rawBytes, _ := json.Marshal(raw)
	n.RawData = string(rawBytes)

	return n
}

func extractFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case []interface{}:
		if len(val) > 0 {
			if f, ok := val[0].(float64); ok {
				return f
			}
		}
	}
	return 0
}

// postSearch executes a single TED search request and returns the notices slice.
func postSearch(c clientPoster, query string, fields []string, limitN int) ([]map[string]interface{}, error) {
	req := tedSearchRequest{
		Query:          query,
		Fields:         fields,
		Limit:          limitN,
		PaginationMode: "ITERATION",
		Scope:          "ALL",
	}
	raw, _, err := c.Post("/v3/notices/search", req)
	if err != nil {
		return nil, err
	}
	var resp tedSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		// Try bare array fallback.
		var arr []map[string]interface{}
		if json.Unmarshal(raw, &arr) == nil {
			return arr, nil
		}
		return nil, fmt.Errorf("parse search response: %w", err)
	}
	return resp.Notices, nil
}
