// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// cascade: run one semantic query across multiple indexes and merge ranked
// results (deduped by vector ID, best score wins).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type cascadeResult struct {
	Index    string             `json:"index"`
	Query    string             `json:"query"`
	TopK     int                `json:"top_k"`
	Matches  []textQueryMatch   `json:"matches"`
	Note     string             `json:"note,omitempty"`
	Failures []textQueryFailure `json:"fetch_failures,omitempty"`
}

func newNovelCascadeCmd(flags *rootFlags) *cobra.Command {
	var topK int
	var namespace string
	var includeMetadata bool
	var model string
	var text string
	var indexes string

	cmd := &cobra.Command{
		Use:   "cascade",
		Short: "Run one semantic query across multiple indexes and merge ranked results",
		Long: `Run the same semantic query across multiple indexes and merge ranked results.

Use this command to run the same semantic query across multiple indexes and merge ranked results.
Do NOT use this command for a single index; use 'text-query'.`,
		Example:     `  pinecone-pp-cli cascade --indexes travel-chat-embeddings,travel-chat-embeddings-v2 --text "kyoto itinerary" --top-k 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cascade")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if indexes == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--indexes is required (comma-separated)"))
			}
			if text == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--text is required"))
			}
			if topK <= 0 {
				topK = 5
			}
			names := strings.Split(indexes, ",")
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if model == "" {
				model = "multilingual-e5-large"
			}
			// Resolve every index's dimension up front. All indexes in a
			// cascade must share a dimension (the embed is performed once).
			// Unresolvable indexes are recorded as failures, not silently
			// dropped, so the output always accounts for every requested index.
			dim, err := indexDimension(ctx, c, names[0])
			if err != nil {
				return err
			}
			if err := ensureModelDimension(ctx, c, model, dim); err != nil {
				result := cascadeResult{
					Index:   strings.Join(names, ","),
					Query:   text,
					TopK:    topK,
					Matches: []textQueryMatch{},
					Note:    fmt.Sprintf("no Pinecone hosted embedding model matches the indexes' %d-dimension vectors; embed externally and use 'query' instead", dim),
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), result.Note)
				return nil
			}
			// Embed once, query N indexes.
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

			type perIndex struct {
				index   string
				matches []textQueryMatch
				err     error
			}
			ch := make(chan perIndex, len(names))
			var wg sync.WaitGroup
			for _, name := range names {
				name := strings.TrimSpace(name)
				if name == "" {
					continue
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Validate this index resolves and matches the shared
					// dimension; unresolvable or mismatched indexes are
					// accounted as per-index failures, never silently dropped.
					idim, err := indexDimension(ctx, c, name)
					if err != nil {
						ch <- perIndex{index: name, err: fmt.Errorf("resolving %q: %w", name, err)}
						return
					}
					if idim != dim {
						ch <- perIndex{index: name, err: fmt.Errorf("index %q is %d-dim; cascade requires %d-dim", name, idim, dim)}
						return
					}
					path, err := dataPlanePath(ctx, c, name, "/query")
					if err != nil {
						ch <- perIndex{index: name, err: err}
						return
					}
					queryBody := map[string]any{"vector": vector, "topK": topK}
					if namespace != "" {
						queryBody["namespace"] = namespace
					}
					if includeMetadata {
						queryBody["includeMetadata"] = true
					}
					data, _, err := c.PostQueryWithParamsAndHeaders(ctx, path, nil, queryBody, apiVersionHeaders())
					if err != nil {
						ch <- perIndex{index: name, err: fmt.Errorf("querying %q: %w", name, err)}
						return
					}
					var qr struct {
						Matches []struct {
							ID       string         `json:"id"`
							Score    float64        `json:"score"`
							Metadata map[string]any `json:"metadata"`
							Values   []float64      `json:"values"`
						} `json:"matches"`
					}
					if err := json.Unmarshal(data, &qr); err != nil {
						ch <- perIndex{index: name, err: fmt.Errorf("parsing %q: %w", name, err)}
						return
					}
					ms := make([]textQueryMatch, 0, len(qr.Matches))
					for _, m := range qr.Matches {
						tm := textQueryMatch{ID: m.ID, Score: m.Score, Metadata: m.Metadata}
						if includeMetadata {
							tm.Values = m.Values
						}
						ms = append(ms, tm)
					}
					ch <- perIndex{index: name, matches: ms}
				}()
			}
			go func() {
				wg.Wait()
				close(ch)
			}()

			best := map[string]textQueryMatch{}
			order := []string{}
			var failures []textQueryFailure
			for r := range ch {
				if r.err != nil {
					failures = append(failures, textQueryFailure{Index: r.index, Error: r.err.Error()})
					continue
				}
				for _, m := range r.matches {
					if existing, ok := best[m.ID]; !ok || m.Score > existing.Score {
						if !ok {
							order = append(order, m.ID)
						}
						best[m.ID] = m
					}
				}
			}
			merged := make([]textQueryMatch, 0, len(order))
			for _, id := range order {
				merged = append(merged, best[id])
			}
			// Deterministic ranking: merged results are ordered by score
			// descending, not by channel-arrival order.
			sort.Slice(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d index fetches failed; merged results over the remaining %d\n", len(failures), len(names), len(names)-len(failures))
			}
			result := cascadeResult{Index: strings.Join(names, ","), Query: text, TopK: topK, Matches: merged, Failures: failures}
			if len(merged) == 0 && len(failures) > 0 {
				result.Note = "all index queries failed; see fetch_failures"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(merged) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching vectors found across indexes.")
				return nil
			}
			rows := make([]map[string]any, 0, len(merged))
			for _, m := range merged {
				row := map[string]any{"id": m.ID, "score": m.Score}
				if sender, ok := m.Metadata["sender"]; ok {
					row["sender"] = sender
				}
				rows = append(rows, row)
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&indexes, "indexes", "", "Comma-separated index names to search")
	cmd.Flags().StringVar(&text, "text", "", "Natural-language query text")
	cmd.Flags().StringVar(&model, "model", "multilingual-e5-large", "Embedding model to use")
	cmd.Flags().IntVar(&topK, "top-k", 5, "Number of results per index before merging")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace to query (all indexes)")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Include metadata in results")
	return cmd
}
