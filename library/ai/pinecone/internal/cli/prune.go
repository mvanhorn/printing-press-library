// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// prune: select stale vectors by local metadata timestamps and delete them in
// batches. Dry-run by default; --apply commits the deletes.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type prunePlan struct {
	Index     string   `json:"index"`
	Namespace string   `json:"namespace"`
	OlderThan string   `json:"older_than"`
	Count     int      `json:"count"`
	IDs       []string `json:"ids"`
	DryRun    bool     `json:"dry_run"`
	Applied   bool     `json:"applied,omitempty"`
	Deleted   int      `json:"deleted,omitempty"`
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
		Example: `  pinecone-pp-cli prune travel-chat-embeddings --namespace __default__ --older-than 90d
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
			s, db, err := openNovelDB(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			// Scan synced vector records from the resources table
			// (records synced under the 'vectors' resource type).
			rows, err := db.QueryContext(ctx,
				`SELECT data FROM resources WHERE resource_type = 'vectors'`)
			if err != nil {
				return fmt.Errorf("querying vectors: %w", err)
			}
			type vec struct {
				ID   string         `json:"id"`
				Meta map[string]any `json:"metadata"`
			}
			var vecs []vec
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning vector: %w", err)
				}
				var v vec
				_ = json.Unmarshal([]byte(data), &v)
				if v.ID != "" {
					vecs = append(vecs, v)
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating vectors: %w", err)
			}
			_ = rows.Close()

			var stale []string
			for _, v := range vecs {
				ts, ok := v.Meta["timestamp"].(string)
				if !ok {
					continue
				}
				t, err := time.Parse("02/01/06 3:04:05 PM", ts)
				if err != nil {
					// also try RFC3339
					t2, err2 := time.Parse(time.RFC3339, ts)
					if err2 != nil {
						continue
					}
					t = t2
				}
				if t.Before(cutoffTime) {
					stale = append(stale, v.ID)
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
				path, err := dataPlanePath(ctx, c, indexName, "/vectors/delete")
				if err != nil {
					return err
				}
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
				plan.Applied = true
				plan.Deleted = deleted
				plan.DryRun = false
				if _, err := db.ExecContext(ctx,
					`INSERT INTO pp_prune_runs (index_name, namespace, ran_at, deleted, ids) VALUES (?, ?, ?, ?, ?)`,
					indexName, namespace, time.Now().UTC().Format(time.RFC3339), deleted, mustJSON(stale),
				); err != nil {
					return fmt.Errorf("recording prune run: %w", err)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			if len(stale) == 0 {
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
