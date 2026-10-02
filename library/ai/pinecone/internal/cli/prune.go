// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// prune: select stale vectors by local metadata timestamps and delete them in
// batches. Dry-run by default; --apply commits the deletes.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/store"
	"github.com/spf13/cobra"
)

type prunePlan struct {
	Index     string   `json:"index"`
	Namespace string   `json:"namespace"`
	OlderThan string   `json:"older_than"`
	Count     int      `json:"count"`
	IDs       []string `json:"ids"`
	Missing   []string `json:"already_absent,omitempty"`
	DryRun    bool     `json:"dry_run"`
	Applied   bool     `json:"applied,omitempty"`
	Deleted   int      `json:"deleted,omitempty"`
}

type localPruneVector struct {
	ID        string
	StorageID string
	RawData   string
	Meta      map[string]any
}

func explicitStringField(obj map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if ok {
			return text, true
		}
	}
	return "", false
}

// loadScopedPruneVectors fails closed: a locally mirrored vector is eligible
// only when its stored payload explicitly identifies the requested index and
// namespace. Older/unscoped mirror rows are deliberately ignored because
// sending their IDs to another namespace can delete an unrelated live vector.
func loadScopedPruneVectors(ctx context.Context, db *sql.DB, indexName, namespace string) ([]localPruneVector, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = 'vectors'`)
	if err != nil {
		return nil, fmt.Errorf("querying vectors: %w", err)
	}
	defer rows.Close()

	var vectors []localPruneVector
	for rows.Next() {
		var storageID, data string
		if err := rows.Scan(&storageID, &data); err != nil {
			return nil, fmt.Errorf("scanning vector: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data), &obj); err != nil {
			continue
		}
		storedIndex, hasIndex := explicitStringField(obj, "index_name", "indexName", "index")
		storedNamespace, hasNamespace := explicitStringField(obj, "namespace")
		if !hasIndex || !hasNamespace || storedIndex != indexName || storedNamespace != namespace {
			continue
		}
		id, _ := explicitStringField(obj, "id")
		if id == "" {
			continue
		}
		metadata, _ := obj["metadata"].(map[string]any)
		vectors = append(vectors, localPruneVector{ID: id, StorageID: storageID, RawData: data, Meta: metadata})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating vectors: %w", err)
	}
	return vectors, nil
}

func newNovelPruneCmd(flags *rootFlags) *cobra.Command {
	var namespace string
	var olderThan string
	var apply bool
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "prune <index>",
		Short: "Find and delete stale vectors by local metadata timestamps (dry-run by default)",
		Long: `Find vectors whose metadata timestamp is older than a threshold and delete them in batches.

Use this command to delete stale vectors identified from local metadata timestamps.
Do NOT use this command for arbitrary filter/ID deletion; use 'delete'.`,
		Example: `  pinecone-pp-cli prune travel-chat-embeddings --older-than 90d
  pinecone-pp-cli prune travel-chat-embeddings --older-than 90d --apply`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true", "pp:happy-args": "index=travel-chat-embeddings", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "prune")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required"))
			}
			indexName := args[0]
			if olderThan == "" {
				olderThan = "90d"
			}
			cutoff, err := parseDurationLoose(olderThan)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --older-than %q (use Go duration like 90d, 24h): %w", olderThan, err))
			}
			cutoffTime := time.Now().Add(-cutoff)
			resolvedDB, err := defaultNovelDB(dbPath)
			if err != nil {
				return err
			}
			if missingMirrorHint(cmd.ErrOrStderr(), resolvedDB) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), prunePlan{Index: indexName, Namespace: namespace, OlderThan: olderThan, Count: 0, IDs: []string{}, DryRun: true}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No local snapshot data; run 'pinecone-pp-cli sync' or 'snapshot' first.")
				return nil
			}
			s, db, err := openNovelDB(ctx, resolvedDB)
			if err != nil {
				return err
			}
			defer s.Close()

			vecs, err := loadScopedPruneVectors(ctx, db, indexName, namespace)
			if err != nil {
				return err
			}

			var stale []string
			seenStale := make(map[string]bool)
			for _, v := range vecs {
				t, ok := pruneTimestamp(v.Meta)
				if !ok {
					continue
				}
				if t.Before(cutoffTime) && !seenStale[v.ID] {
					stale = append(stale, v.ID)
					seenStale[v.ID] = true
				}
			}
			if limit > 0 && len(stale) > limit {
				stale = stale[:limit]
			}
			plan := prunePlan{
				Index:     indexName,
				Namespace: namespace,
				OlderThan: olderThan,
				Count:     len(stale),
				IDs:       stale,
				DryRun:    !apply,
			}
			if apply && len(stale) > 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				host, err := verifiedIndexHost(ctx, c, indexName)
				if err != nil {
					return err
				}
				base := "https://" + host
				var missing []string
				stale, missing, err = verifyPruneCandidates(ctx, c, base+"/vectors/fetch", indexName, namespace, stale, cutoffTime)
				if err != nil {
					return err
				}
				plan.IDs = stale
				plan.Count = len(stale)
				plan.Missing = missing
				path := base + "/vectors/delete"
				// batch in chunks of 100
				deleted := 0
				for i := 0; i < len(stale); i += 100 {
					end := i + 100
					if end > len(stale) {
						end = len(stale)
					}
					body := map[string]any{
						"ids":       stale[i:end],
						"namespace": namespace,
					}
					_, _, err := c.PostWithParamsAndHeaders(ctx, path, nil, body, apiVersionHeaders())
					if err != nil {
						return fmt.Errorf("deleting batch %d-%d: %w", i, end, err)
					}
					deleted += end - i
				}
				plan.Deleted = deleted
				if len(missing) > 0 {
					missingSet := make(map[string]bool, len(missing))
					for _, id := range missing {
						missingSet[id] = true
					}
					var rows []store.ScopedVectorRow
					for _, vector := range vecs {
						if missingSet[vector.ID] {
							rows = append(rows, store.ScopedVectorRow{StorageID: vector.StorageID, BareID: vector.ID, ExpectedData: vector.RawData})
						}
					}
					if _, err := s.DeleteScopedVectorRows(ctx, indexName, namespace, rows); err != nil {
						return fmt.Errorf("removing confirmed-absent vectors from local mirror: %w", err)
					}
				}
				if _, err := db.ExecContext(ctx,
					`INSERT INTO pp_prune_runs (index_name, namespace, ran_at, deleted, ids) VALUES (?, ?, ?, ?, ?)`,
					indexName, namespace, time.Now().UTC().Format(time.RFC3339), deleted, mustJSON(stale),
				); err != nil {
					return fmt.Errorf("recording prune run: %w", err)
				}
			}
			if apply {
				plan.Applied = true
				plan.DryRun = false
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			if len(stale) == 0 {
				if len(plan.Missing) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipped %d vector(s) already absent from the index.\n", len(plan.Missing))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No stale vectors found.")
				return nil
			}
			verb := "would delete"
			if apply {
				verb = "deleted"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d stale vector(s) in %s\n", verb, len(stale), indexName)
			for _, id := range stale {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", id)
			}
			if len(plan.Missing) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipped %d vector(s) already absent from the index.\n", len(plan.Missing))
			}
			if !apply {
				fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --apply to delete.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace to prune (default: index default)")
	cmd.Flags().StringVar(&olderThan, "older-than", "90d", "Delete vectors with metadata timestamp older than this (Go duration: 24h, 90d)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Commit the deletes (default is dry-run)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum vectors to prune in this run (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: platform data dir)")
	return cmd
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
