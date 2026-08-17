// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// check-vectors: validate a vectors JSON file against the index schema
// (dimension, duplicate/empty IDs, sparse/dense shape) before upserting.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type checkVectorIssue struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}

type checkVectorResult struct {
	Index       string             `json:"index"`
	File        string             `json:"file"`
	Total       int                `json:"total_vectors"`
	Issues      []checkVectorIssue `json:"issues"`
	Ok          bool               `json:"ok"`
	ExpectedDim int64              `json:"expected_dimension"`
}

func newNovelCheckVectorsCmd(flags *rootFlags) *cobra.Command {
	var indexName string
	var file string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "check-vectors",
		Short: "Validate a vectors JSON file against the index schema before upserting",
		Long: `Validate a vectors payload against the index schema before writing.

Use this command to validate a vector payload against the index schema before writing.
Do NOT use this command to write vectors; use 'upsert'.`,
		Example:     `  pinecone-pp-cli check-vectors --index travel-chat-embeddings --file vectors.json --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--index=travel-chat-embeddings;--file=vectors.json"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "check-vectors")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if indexName == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--index is required"))
			}
			if file == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required"))
			}
			// #nosec G304 -- the whole purpose of this command is to read a
			// user-specified vectors file; the path is an explicit --file arg.
			raw, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading %s: %w", file, err)
			}
			var payload struct {
				Vectors []struct {
					ID     string    `json:"id"`
					Values []float64 `json:"values"`
					Sparse *struct {
						Indices []int64   `json:"indices"`
						Values  []float64 `json:"values"`
					} `json:"sparseValues"`
				} `json:"vectors"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				return fmt.Errorf("parsing %s: %w", file, err)
			}
			if len(payload.Vectors) == 0 {
				return fmt.Errorf("no vectors found in %s", file)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Fetch the index config (dimension) from the control plane.
			path := "https://api.pinecone.io/indexes/{index_name}"
			path = replacePathParam(path, "index_name", indexName)
			idxData, err := c.Get(ctx, path, map[string]string{})
			if err != nil {
				return fmt.Errorf("describing index %q: %w", indexName, err)
			}
			var idx struct {
				Dimension int64 `json:"dimension"`
			}
			if err := json.Unmarshal(idxData, &idx); err != nil {
				return fmt.Errorf("parsing index %q: %w", indexName, err)
			}

			issues := []checkVectorIssue{}
			seen := map[string]bool{}
			for _, v := range payload.Vectors {
				if v.ID == "" {
					issues = append(issues, checkVectorIssue{Type: "empty_id", Message: "vector has empty id"})
					continue
				}
				if seen[v.ID] {
					issues = append(issues, checkVectorIssue{Type: "duplicate_id", Message: "duplicate id in payload", ID: v.ID})
				}
				seen[v.ID] = true
				if v.Sparse == nil && int64(len(v.Values)) != idx.Dimension {
					issues = append(issues, checkVectorIssue{Type: "dimension", Message: fmt.Sprintf("expected %d values, got %d", idx.Dimension, len(v.Values)), ID: v.ID})
				}
				if v.Sparse != nil && len(v.Sparse.Indices) != len(v.Sparse.Values) {
					issues = append(issues, checkVectorIssue{Type: "sparse_shape", Message: "sparse indices and values length mismatch", ID: v.ID})
				}
			}
			result := checkVectorResult{
				Index:       indexName,
				File:        file,
				Total:       len(payload.Vectors),
				Issues:      issues,
				Ok:          len(issues) == 0,
				ExpectedDim: idx.Dimension,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) || jsonOut {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if result.Ok {
				fmt.Fprintf(cmd.OutOrStdout(), "OK: %d vectors valid for index %s (dimension %d)\n", result.Total, indexName, result.ExpectedDim)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%d issue(s) found in %d vectors:\n", len(issues), result.Total)
				for _, iss := range issues {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s %s\n", iss.Type, iss.ID, iss.Message)
				}
				return usageErr(fmt.Errorf("payload failed validation"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&indexName, "index", "", "Index name to validate against")
	cmd.Flags().StringVar(&file, "file", "", "Path to vectors JSON file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}
