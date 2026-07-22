// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through fetchCPSC in cpsc_novel_support.go

package cli

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelInventoryCheckCmd(flags *rootFlags) *cobra.Command {
	var inventory string

	cmd := &cobra.Command{
		Use:         "inventory-check",
		Short:       "Screen a bounded CSV inventory against CPSC product fields and expose exact identifiers and token-overlap evidence for candidates.",
		Example:     "  cpsc-recalls-pp-cli inventory-check --inventory examples/products.csv --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitCPSCDryRun(cmd, flags, "inventory-check", map[string]any{"inventory": inventory, "maximum_rows": 50, "query_fields": []string{"ProductName from name", "Manufacturer from brand", "ProductName from model"}, "upc_note": "UPC is evaluated as exact evidence when returned but the CPSC endpoint has no UPC query parameter."})
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			items, err := readInventory(inventory)
			if err != nil {
				return err
			}
			if len(items) > 50 {
				return fmt.Errorf("inventory has %d rows; maximum is 50", len(items))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var results []map[string]any
			fetcher := newCPSCFetcher(flags)
			for _, item := range items {
				queries := []url.Values{{"ProductName": {item["name"]}}}
				if item["brand"] != "" {
					queries = append(queries, url.Values{"Manufacturer": {item["brand"]}})
				}
				if item["model"] != "" && !strings.EqualFold(item["model"], item["name"]) {
					queries = append(queries, url.Values{"ProductName": {item["model"]}})
				}
				byID := map[string]map[string]any{}
				for _, query := range queries {
					rows, fetchErr := fetcher.fetch(ctx, query)
					if fetchErr != nil {
						return fetchErr
					}
					for _, recall := range rows {
						id := strings.TrimSpace(fmt.Sprint(recall["RecallID"]))
						if id != "" && id != "<nil>" {
							byID[id] = recall
						}
					}
				}
				var candidates []map[string]any
				for _, recall := range byID {
					candidateText := fmt.Sprint(recall["Title"]) + " " + fmt.Sprint(recall["Description"])
					if products, ok := recall["Products"].([]any); ok {
						for _, product := range products {
							candidateText += " " + fmt.Sprint(product)
						}
					}
					shared, score := overlap(item["name"]+" "+item["brand"]+" "+item["model"]+" "+item["upc"], candidateText)
					evidence := exactInventoryEvidence(item, recall)
					exact := hasExactInventoryEvidence(evidence)
					if exact || score >= 0.34 {
						candidates = append(candidates, map[string]any{"recall_id": recall["RecallID"], "title": recall["Title"], "official_url": recall["URL"], "exact_match": exact, "exact_field_evidence": evidence, "shared_tokens": shared, "token_overlap": score, "products": recall["Products"]})
					}
				}
				sortInventoryCandidates(candidates)
				if len(candidates) > 25 {
					candidates = candidates[:25]
				}
				results = append(results, map[string]any{"inventory_item": item, "queries_sent": len(queries), "unique_recall_rows_examined": len(byID), "candidate_matches": candidates})
			}
			return emitCPSC(cmd, flags, "live", map[string]any{"inventory": inventory, "retrieval_coverage": "Queries name and model through ProductName and brand through Manufacturer; UPC cannot be queried upstream and is checked only in returned rows.", "results": results, "interpretation": "Candidates require manual identifier and description verification; token overlap is not a confidence probability.", "caveats": cpscCaveats()})
		},
	}
	cmd.Flags().StringVar(&inventory, "inventory", "", "CSV with name and optional brand, model, upc columns")
	return cmd
}

func sortInventoryCandidates(candidates []map[string]any) {
	sort.SliceStable(candidates, func(i, j int) bool {
		iExact, _ := candidates[i]["exact_match"].(bool)
		jExact, _ := candidates[j]["exact_match"].(bool)
		if iExact != jExact {
			return iExact
		}
		iScore, _ := candidates[i]["token_overlap"].(float64)
		jScore, _ := candidates[j]["token_overlap"].(float64)
		if iScore != jScore {
			return iScore > jScore
		}
		return fmt.Sprint(candidates[i]["recall_id"]) < fmt.Sprint(candidates[j]["recall_id"])
	})
}

func readInventory(path string) ([]map[string]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("--inventory is required")
	}
	// #nosec G304 -- path is an explicit operator-supplied CLI input.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	indexes := map[string]int{}
	for i, name := range header {
		indexes[strings.ToLower(strings.TrimSpace(name))] = i
	}
	if _, ok := indexes["name"]; !ok {
		return nil, errors.New("inventory CSV needs a name column")
	}
	var out []map[string]string
	for {
		row, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		item := map[string]string{}
		for _, field := range []string{"name", "brand", "model", "upc"} {
			if index, ok := indexes[field]; ok && index < len(row) {
				item[field] = strings.TrimSpace(row[index])
			}
		}
		if item["name"] != "" {
			out = append(out, item)
		}
	}
	return out, nil
}
