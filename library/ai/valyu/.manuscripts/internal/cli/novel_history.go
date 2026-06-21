// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history",
		Short:       "View and replay past search queries.",
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newHistoryListCmd(flags))
	cmd.AddCommand(newHistoryReplayCmd(flags))
	cmd.AddCommand(newHistoryExportCmd(flags))
	return cmd
}

func newHistoryListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var since string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List recent search queries.",
		Example:     "  valyu-pp-cli history list\n  valyu-pp-cli history list --limit 50 --since 7d",
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var sinceTS string
			if since != "" {
				d, parseErr := parseSinceDur(since)
				if parseErr != nil {
					return fmt.Errorf("invalid --since %q: expected format like 7d, 24h, 1w", since)
				}
				sinceTS = time.Now().Add(-d).UTC().Format(time.RFC3339)
			}

			query := `SELECT id, query, cost, created_at FROM search_history`
			args2 := []any{}
			if sinceTS != "" {
				query += ` WHERE created_at >= ?`
				args2 = append(args2, sinceTS)
			}
			query += ` ORDER BY created_at DESC LIMIT ?`
			args2 = append(args2, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying search history: %w", err)
			}
			defer rows.Close()

			type row struct {
				ID        int64
				Query     string
				Cost      float64
				CreatedAt string
			}
			var items []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.ID, &r.Query, &r.Cost, &r.CreatedAt); err != nil {
					return err
				}
				items = append(items, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				out, _ := json.Marshal(items)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tQuery\tCost\tCreated At")
			fmt.Fprintln(tw, "--\t-----\t----\t----------")
			for _, r := range items {
				fmt.Fprintf(tw, "%d\t%s\t$%.4f\t%s\n", r.ID, truncateStr(r.Query, 60), r.Cost, r.CreatedAt)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of history entries to show")
	cmd.Flags().StringVar(&since, "since", "", "Filter to entries since this duration ago (e.g. 7d, 24h, 1w)")
	return cmd
}

func newHistoryReplayCmd(flags *rootFlags) *cobra.Command {
	var diff bool

	cmd := &cobra.Command{
		Use:   "replay <id>",
		Short: "Re-run a stored search and compare results.",
		Annotations: map[string]string{"pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("usage: valyu-pp-cli history replay <id>")
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var id int64
			if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
				return fmt.Errorf("invalid id %q: must be a number", args[0])
			}

			var query string
			var paramsJSON string
			var storedURLsJSON string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT query, params, result_urls FROM search_history WHERE id = ?`, id,
			).Scan(&query, &paramsJSON, &storedURLsJSON)
			if err != nil {
				return fmt.Errorf("history entry %d not found: %w", id, err)
			}

			var params map[string]any
			json.Unmarshal([]byte(paramsJSON), &params)
			var storedURLs []string
			json.Unmarshal([]byte(storedURLsJSON), &storedURLs)

			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			body := map[string]any{"query": query}
			for k, v := range params {
				body[k] = v
			}

			data, _, apiErr := c.PostQueryWithParams(cmd.Context(), "/v1/search", map[string]string{}, body)
			if apiErr != nil {
				return classifyAPIError(apiErr, flags)
			}

			newURLs := extractURLsFromSearchResponse(data)
			cost := extractCostFromResponse(data)
			paramsAny := map[string]any{}
			for k, v := range params {
				paramsAny[k] = v
			}
			recordSearchHistory(db, query, paramsAny, newURLs, cost)
			recordCost(db, "history replay", cost)

			if diff || true {
				oldSet := make(map[string]bool, len(storedURLs))
				for _, u := range storedURLs {
					oldSet[u] = true
				}
				newSet := make(map[string]bool, len(newURLs))
				for _, u := range newURLs {
					newSet[u] = true
				}
				var added, dropped, unchanged []string
				for _, u := range newURLs {
					if oldSet[u] {
						unchanged = append(unchanged, u)
					} else {
						added = append(added, u)
					}
				}
				for _, u := range storedURLs {
					if !newSet[u] {
						dropped = append(dropped, u)
					}
				}

				if flags.asJSON {
					out, _ := json.Marshal(map[string]any{
						"query":     query,
						"added":     added,
						"dropped":   dropped,
						"unchanged": unchanged,
						"cost":      cost,
					})
					return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				fmt.Fprintf(tw, "Query:\t%s\n", query)
				fmt.Fprintf(tw, "New URLs:\t%d added, %d dropped, %d unchanged\n", len(added), len(dropped), len(unchanged))
				fmt.Fprintf(tw, "Cost:\t$%.4f\n", cost)
				for _, u := range added {
					fmt.Fprintf(tw, "  + %s\n", u)
				}
				for _, u := range dropped {
					fmt.Fprintf(tw, "  - %s\n", u)
				}
				return tw.Flush()
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().BoolVar(&diff, "diff", true, "Show diff of added/dropped URLs vs stored result")
	return cmd
}

func newHistoryExportCmd(flags *rootFlags) *cobra.Command {
	var format string
	var limit int

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export search history to CSV or JSON.",
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, query, params, result_urls, cost, created_at FROM search_history ORDER BY created_at DESC LIMIT ?`,
				limit,
			)
			if err != nil {
				return fmt.Errorf("querying search history: %w", err)
			}
			defer rows.Close()

			type historyRow struct {
				ID         int64   `json:"id"`
				Query      string  `json:"query"`
				Params     string  `json:"params"`
				ResultURLs string  `json:"result_urls"`
				Cost       float64 `json:"cost"`
				CreatedAt  string  `json:"created_at"`
			}
			var items []historyRow
			for rows.Next() {
				var r historyRow
				if err := rows.Scan(&r.ID, &r.Query, &r.Params, &r.ResultURLs, &r.Cost, &r.CreatedAt); err != nil {
					return err
				}
				items = append(items, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "csv":
				w := csv.NewWriter(cmd.OutOrStdout())
				w.Write([]string{"id", "query", "params", "result_urls", "cost", "created_at"})
				for _, r := range items {
					w.Write([]string{
						fmt.Sprintf("%d", r.ID),
						r.Query,
						r.Params,
						r.ResultURLs,
						fmt.Sprintf("%.6f", r.Cost),
						r.CreatedAt,
					})
				}
				w.Flush()
				return w.Error()
			default:
				out, _ := json.Marshal(items)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum number of entries to export")
	return cmd
}

// recordSearchHistory saves a search execution to the local history table.
// Non-fatal: errors are silently swallowed.
func recordSearchHistory(db *store.Store, query string, params map[string]any, urls []string, cost float64) {
	if db == nil {
		return
	}
	paramsJSON, _ := json.Marshal(params)
	urlsJSON, _ := json.Marshal(urls)
	db.DB().Exec(
		`INSERT INTO search_history (query, params, result_urls, cost) VALUES (?, ?, ?, ?)`,
		query, string(paramsJSON), string(urlsJSON), cost,
	)
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
