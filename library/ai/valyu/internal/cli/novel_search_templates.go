// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newSearchSaveCmd(flags *rootFlags) *cobra.Command {
	var query string
	var searchType string
	var maxResults int

	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a search configuration as a named template.",
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("usage: valyu-pp-cli semantic-search save <name>")
			}
			if query == "" {
				return fmt.Errorf("required flag \"template-query\" not set")
			}
			name := args[0]

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			params := map[string]any{}
			if searchType != "" {
				params["search_type"] = searchType
			}
			if maxResults != 0 {
				params["max_num_results"] = maxResults
			}
			paramsJSON, _ := json.Marshal(params)

			_, err = db.DB().ExecContext(cmd.Context(),
				`INSERT OR REPLACE INTO search_templates (name, query, params) VALUES (?, ?, ?)`,
				name, query, string(paramsJSON),
			)
			if err != nil {
				return fmt.Errorf("saving template: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved template %q\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "template-query", "", "The search query to save")
	cmd.Flags().StringVar(&searchType, "template-type", "", "Search type (web, paper, finance, all, etc.)")
	cmd.Flags().IntVar(&maxResults, "max", 0, "Max results (0 = use API default)")
	return cmd
}

func newSearchRunTemplateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Run a saved search template.",
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("usage: valyu-pp-cli semantic-search run <name>")
			}
			name := args[0]

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var query string
			var paramsJSON string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT query, params FROM search_templates WHERE name = ?`, name,
			).Scan(&query, &paramsJSON)
			if err != nil {
				return fmt.Errorf("template %q not found", name)
			}

			var params map[string]any
			json.Unmarshal([]byte(paramsJSON), &params)

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

			urls := extractURLsFromSearchResponse(data)
			cost := extractCostFromResponse(data)
			recordSearchHistory(db, query, params, urls, cost)
			recordCost(db, "semantic-search run", cost)

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func newSearchListSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list-saved",
		Short:       "List saved search templates.",
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT name, query, params, created_at FROM search_templates ORDER BY name`,
			)
			if err != nil {
				return fmt.Errorf("listing templates: %w", err)
			}
			defer rows.Close()

			type tmpl struct {
				Name      string `json:"name"`
				Query     string `json:"query"`
				Params    string `json:"params"`
				CreatedAt string `json:"created_at"`
			}
			var items []tmpl
			for rows.Next() {
				var t tmpl
				if err := rows.Scan(&t.Name, &t.Query, &t.Params, &t.CreatedAt); err != nil {
					return err
				}
				items = append(items, t)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				out, _ := json.Marshal(items)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "Name\tQuery\tParams\tCreated At")
			fmt.Fprintln(tw, "----\t-----\t------\t----------")
			for _, t := range items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.Name, truncateStr(t.Query, 50), t.Params, t.CreatedAt)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newSearchDeleteSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "delete-saved <name>",
		Short:       "Delete a saved search template.",
		Example:     "  valyu-pp-cli semantic-search delete-saved earnings-scan",
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("usage: valyu-pp-cli semantic-search delete-saved <name>")
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			res, err := db.DB().ExecContext(cmd.Context(),
				`DELETE FROM search_templates WHERE name = ?`, args[0],
			)
			if err != nil {
				return fmt.Errorf("deleting template: %w", err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return fmt.Errorf("template %q not found", args[0])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted template %q\n", args[0])
			return nil
		},
	}
	return cmd
}
