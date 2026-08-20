// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// text-query: embed a natural-language query via Pinecone Inference, then
// query a dense index with the resulting vector.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type textQueryResult struct {
	Index     string             `json:"index"`
	Query     string             `json:"query"`
	Namespace string             `json:"namespace,omitempty"`
	Model     string             `json:"model"`
	Matches   []textQueryMatch   `json:"matches"`
	Usage     *textQueryUsage    `json:"usage,omitempty"`
	Note      string             `json:"note,omitempty"`
	Failures  []textQueryFailure `json:"fetch_failures,omitempty"`
}

type textQueryMatch struct {
	ID       string         `json:"id"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Values   []float64      `json:"values,omitempty"`
}

type textQueryUsage struct {
	ReadUnits *int `json:"read_units,omitempty"`
}

type textQueryFailure struct {
	Index string `json:"index"`
	Error string `json:"error"`
}

func newNovelTextQueryCmd(flags *rootFlags) *cobra.Command {
	var topK int
	var namespace string
	var includeMetadata bool
	var model string
	var filter string
	var text string

	cmd := &cobra.Command{
		Use:   "text-query <index>",
		Short: "Search a dense index with a natural-language query (embeds the text, then queries)",
		Long: `Search a dense index with a natural-language query.

Use this command for natural-language text search against a single dense index (embeds the text, then queries).
Do NOT use this command for raw vector input; use 'query'. Do NOT use it to search multiple indexes at once; use 'cascade'.`,
		Example: `  pinecone-pp-cli text-query travel-chat-embeddings --text "visa on arrival for thailand" --top-k 5 --json
  pinecone-pp-cli text-query travel-chat-embeddings --text "kyoto itinerary" --namespace __default__ --include-metadata`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "index=travel-chat-embeddings", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "text-query")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required"))
			}
			text, _ := cmd.Flags().GetString("text")
			if text == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--text is required"))
			}
			if topK <= 0 {
				topK = 10
			}
			indexName := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve the index dimension and pick a matching embedding model.
			dim, err := indexDimension(ctx, c, indexName)
			if err != nil {
				return err
			}
			if model == "" {
				model = "multilingual-e5-large"
			}
			if err := ensureModelDimension(ctx, c, model, dim); err != nil {
				// No hosted model matches the index dimension. This is a
				// capability mismatch, not a usage error: return an honest
				// empty result with an explanatory note so agents can decide.
				result := textQueryResult{
					Index:   indexName,
					Query:   text,
					Model:   model,
					Matches: []textQueryMatch{},
					Note:    fmt.Sprintf("no Pinecone hosted embedding model matches this index's %d-dimension vectors; embed externally and use 'query' instead", dim),
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), result.Note)
				return nil
			}

			// Step 1: embed the text via the inference endpoint.
			embedBody := map[string]any{
				"model":  model,
				"inputs": []map[string]string{{"text": text}},
				"parameters": map[string]any{
					"input_type": "query",
					"truncate":   "END",
				},
			}
			embedData, _, err := c.PostWithParamsAndHeaders(ctx, "https://api.pinecone.io/embed", nil, embedBody, apiVersionHeaders())
			if err != nil {
				return fmt.Errorf("embedding query: %w", err)
			}
			var embedResp struct {
				Data []struct {
					Values []float64 `json:"values"`
				} `json:"data"`
			}
			if err := json.Unmarshal(embedData, &embedResp); err != nil {
				return fmt.Errorf("parsing embed response: %w", err)
			}
			if len(embedResp.Data) == 0 || len(embedResp.Data[0].Values) == 0 {
				return fmt.Errorf("embedding returned no vectors for %q", model)
			}
			vector := embedResp.Data[0].Values

			// Step 2: query the index data plane.
			path, err := dataPlanePath(ctx, c, indexName, "/query")
			if err != nil {
				return err
			}
			queryBody := map[string]any{
				"vector": vector,
				"topK":   topK,
			}
			if namespace != "" {
				queryBody["namespace"] = namespace
			}
			if includeMetadata {
				queryBody["includeMetadata"] = true
			}
			if filter != "" {
				var parsed any
				if err := json.Unmarshal([]byte(filter), &parsed); err != nil {
					return fmt.Errorf("parsing --filter JSON: %w", err)
				}
				queryBody["filter"] = parsed
			}
			data, _, err := c.PostQueryWithParamsAndHeaders(ctx, path, nil, queryBody, apiVersionHeaders())
			if err != nil {
				return fmt.Errorf("querying index %q: %w", indexName, err)
			}
			var qr struct {
				Matches []struct {
					ID       string         `json:"id"`
					Score    float64        `json:"score"`
					Metadata map[string]any `json:"metadata"`
					Values   []float64      `json:"values"`
				} `json:"matches"`
				Usage *textQueryUsage `json:"usage"`
			}
			if err := json.Unmarshal(data, &qr); err != nil {
				return fmt.Errorf("parsing query response: %w", err)
			}
			matches := make([]textQueryMatch, 0, len(qr.Matches))
			for _, m := range qr.Matches {
				tm := textQueryMatch{ID: m.ID, Score: m.Score, Metadata: m.Metadata}
				if includeMetadata {
					tm.Values = m.Values
				}
				matches = append(matches, tm)
			}
			result := textQueryResult{
				Index:     indexName,
				Query:     text,
				Namespace: namespace,
				Model:     model,
				Matches:   matches,
				Usage:     qr.Usage,
			}
			if len(matches) == 0 {
				result.Note = "no matches returned; try lowering --top-k or widening --filter"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching vectors found.")
				return nil
			}
			rows := make([]map[string]any, 0, len(matches))
			for _, m := range matches {
				row := map[string]any{"id": m.ID, "score": m.Score}
				if sender, ok := m.Metadata["sender"]; ok {
					row["sender"] = sender
				}
				if txt, ok := m.Metadata["text"]; ok {
					row["text"] = txt
				}
				rows = append(rows, row)
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&model, "model", "multilingual-e5-large", "Embedding model to use (see 'models')")
	cmd.Flags().StringVar(&text, "text", "", "Natural-language query text")
	cmd.Flags().IntVar(&topK, "top-k", 10, "Number of results to return")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace to query (default: the index default namespace)")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Include metadata and vector values in results")
	cmd.Flags().StringVar(&filter, "filter", "", "Metadata filter JSON, e.g. {\"year\": {\"$gte\": 2020}}")
	return cmd
}

var _ = strings.TrimSpace
var _ = context.Background
