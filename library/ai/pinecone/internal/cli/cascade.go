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

func ensureCascadeCompatible(baseName string, base pineconeIndexShape, indexName string, candidate pineconeIndexShape) error {
	if base.Dimension != candidate.Dimension {
		return fmt.Errorf("index %q is %d-dim; cascade index %q is %d-dim", indexName, candidate.Dimension, baseName, base.Dimension)
	}
	if base.Metric == "" || candidate.Metric == "" {
		return fmt.Errorf("cannot verify scoring metric compatibility between indexes %q and %q", baseName, indexName)
	}
	if !strings.EqualFold(base.Metric, candidate.Metric) {
		return fmt.Errorf("index %q uses %s metric; cascade index %q uses %s metric", indexName, candidate.Metric, baseName, base.Metric)
	}
	return nil
}

func selectCascadeIndexes(names []string, describe func(string) (pineconeIndexShape, error)) ([]string, pineconeIndexShape, []textQueryFailure, error) {
	type shapeGroup struct {
		shape pineconeIndexShape
		names []string
	}
	groups := make(map[string]*shapeGroup)
	ordered := make([]*shapeGroup, 0)
	seenNames := make(map[string]bool, len(names))
	var failures []textQueryFailure
	for _, name := range names {
		if seenNames[name] {
			continue
		}
		seenNames[name] = true
		shape, err := describe(name)
		if err == nil && (shape.Dimension <= 0 || shape.Metric == "") {
			err = fmt.Errorf("index %q did not report a comparable dimension and scoring metric", name)
		}
		if err != nil {
			failures = append(failures, textQueryFailure{Index: name, Error: err.Error()})
			continue
		}
		key := fmt.Sprintf("%d:%s", shape.Dimension, strings.ToLower(shape.Metric))
		group := groups[key]
		if group == nil {
			group = &shapeGroup{shape: shape}
			groups[key] = group
			ordered = append(ordered, group)
		}
		group.names = append(group.names, name)
	}
	if len(ordered) == 0 {
		return nil, pineconeIndexShape{}, failures, nil
	}
	best := ordered[0]
	tied := false
	for _, group := range ordered[1:] {
		if len(group.names) > len(best.names) {
			best, tied = group, false
		} else if len(group.names) == len(best.names) {
			tied = true
		}
	}
	if tied {
		return nil, pineconeIndexShape{}, failures, fmt.Errorf("indexes have incompatible dimensions or scoring metrics with no unique largest compatible group")
	}
	for _, group := range ordered {
		if group == best {
			continue
		}
		for _, name := range group.names {
			err := ensureCascadeCompatible(best.names[0], best.shape, name, group.shape)
			failures = append(failures, textQueryFailure{Index: name, Error: err.Error()})
		}
	}
	return best.names, best.shape, failures, nil
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
			names := make([]string, 0)
			for _, name := range strings.Split(indexes, ",") {
				if name = strings.TrimSpace(name); name != "" {
					names = append(names, name)
				}
			}
			if len(names) == 0 {
				return usageErr(fmt.Errorf("--indexes must include at least one index name"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if model == "" {
				model = "multilingual-e5-large"
			}
			// Keep reachable, compatible indexes. A stale name is a per-index
			// failure, not a reason to discard results from healthy indexes.
			validNames, baseShape, failures, err := selectCascadeIndexes(names, func(name string) (pineconeIndexShape, error) {
				return describeIndexShape(ctx, c, name)
			})
			if err != nil {
				return err
			}
			if len(validNames) == 0 {
				return fmt.Errorf("no compatible indexes available for cascade: %s", failures[0].Error)
			}
			dim := baseShape.Dimension
			if err := ensureModelDimension(ctx, c, model, dim); err != nil {
				result := cascadeResult{
					Index:    strings.Join(names, ","),
					Query:    text,
					TopK:     topK,
					Matches:  []textQueryMatch{},
					Note:     fmt.Sprintf("no Pinecone hosted embedding model matches the indexes' %d-dimension vectors; embed externally and use 'query' instead", dim),
					Failures: failures,
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
			ch := make(chan perIndex, len(validNames))
			var wg sync.WaitGroup
			for _, name := range validNames {
				wg.Add(1)
				go func() {
					defer wg.Done()
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
