// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// snapshot: capture point-in-time index state (per-namespace vector counts,
// config) into local SQLite, then diff against prior snapshots.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type snapshotEnvelope struct {
	Index      string           `json:"index"`
	Captured   string           `json:"captured_at"`
	Note       string           `json:"note,omitempty"`
	TotalVec   int64            `json:"total_vectors"`
	Dimension  int64            `json:"dimension"`
	Metric     string           `json:"metric"`
	Host       string           `json:"host"`
	Namespaces map[string]int64 `json:"namespaces"`
}

type snapshotDiffRow struct {
	Index        string `json:"index"`
	CapturedAt   string `json:"captured_at"`
	Note         string `json:"note"`
	TotalVectors int64  `json:"total_vectors"`
}

type snapshotDiffEnvelope struct {
	Index      string            `json:"index"`
	Since      string            `json:"since"`
	Snapshots  []snapshotDiffRow `json:"snapshots"`
	TotalDelta int64             `json:"total_delta"`
	Notes      []string          `json:"notes,omitempty"`
}

func newNovelSnapshotCmd(flags *rootFlags) *cobra.Command {
	var note string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "snapshot <index>",
		Short: "Capture a point-in-time index state (per-namespace counts, config) into local SQLite",
		Long: `Capture a point-in-time index state and diff it against earlier snapshots.

Use this command to capture a point-in-time index state and diff it against earlier snapshots.
Do NOT use this command for growth/cost projection; use 'usage'. Do NOT use it for one-off grouping of synced entities; use 'analytics'.`,
		Example: `  pinecone-pp-cli snapshot travel-chat-embeddings --note weekly
  pinecone-pp-cli snapshot diff --index travel-chat-embeddings --since 7d`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "index=travel-chat-embeddings", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required"))
			}
			indexName := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Live describe-index (control plane)
			path := "https://api.pinecone.io/indexes/{index_name}"
			path = replacePathParam(path, "index_name", indexName)
			idxData, err := c.Get(ctx, path, map[string]string{})
			if err != nil {
				return fmt.Errorf("describing index %q: %w", indexName, err)
			}
			var idx struct {
				Host      string `json:"host"`
				Dimension int64  `json:"dimension"`
				Metric    string `json:"metric"`
			}
			if err := json.Unmarshal(idxData, &idx); err != nil {
				return fmt.Errorf("parsing index %q: %w", indexName, err)
			}

			// Live describe_index_stats (data plane)
			host, err := resolveIndexHost(ctx, c, indexName)
			if err != nil {
				return err
			}
			statsPath := host + "/describe_index_stats"
			statsData, _, err := c.PostQueryWithParamsAndHeaders(ctx, statsPath, nil, map[string]any{}, apiVersionHeaders())
			if err != nil {
				return fmt.Errorf("fetching stats for %q: %w", indexName, err)
			}
			var stats struct {
				TotalVectorCount int64 `json:"totalVectorCount"`
				Namespaces       map[string]struct {
					VectorCount int64 `json:"vectorCount"`
				} `json:"namespaces"`
			}
			if err := json.Unmarshal(statsData, &stats); err != nil {
				return fmt.Errorf("parsing stats for %q: %w", indexName, err)
			}
			ns := make(map[string]int64, len(stats.Namespaces))
			for name, n := range stats.Namespaces {
				ns[name] = n.VectorCount
			}
			now := time.Now().UTC().Format(time.RFC3339)
			env := snapshotEnvelope{
				Index:      indexName,
				Captured:   now,
				Note:       note,
				TotalVec:   stats.TotalVectorCount,
				Dimension:  idx.Dimension,
				Metric:     idx.Metric,
				Host:       idx.Host,
				Namespaces: ns,
			}
			dataJSON, _ := json.Marshal(env)

			s, db, err := openNovelDB(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			if _, err := db.ExecContext(ctx,
				`INSERT INTO pp_snapshots (index_name, captured_at, note, total_vectors, dimension, metric, host, data)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				indexName, now, note, env.TotalVec, env.Dimension, env.Metric, env.Host, string(dataJSON),
			); err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot captured for %s at %s\n", indexName, now)
			fmt.Fprintf(cmd.OutOrStdout(), "  total vectors: %d\n", env.TotalVec)
			for name, count := range ns {
				fmt.Fprintf(cmd.OutOrStdout(), "  namespace %-24s %d\n", name, count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional note for this snapshot")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: platform data dir)")
	cmd.AddCommand(newSnapshotDiffCmd(flags))
	return cmd
}

func newSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var indexName string
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Diff snapshots of an index over a time window",
		Example:     `  pinecone-pp-cli snapshot diff --index travel-chat-embeddings --since 7d`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot diff")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if indexName == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--index is required for snapshot diff"))
			}
			resolvedDB, err := defaultNovelDB(dbPath)
			if err != nil {
				return err
			}
			return runSnapshotDiff(ctx, cmd, flags, resolvedDB, indexName, since)
		},
	}
	cmd.Flags().StringVar(&indexName, "index", "", "Index name (for 'snapshot diff')")
	cmd.Flags().StringVar(&since, "since", "7d", "Window for 'snapshot diff' (Go duration: 7d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: platform data dir)")
	return cmd
}

func runSnapshotDiff(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, indexName, since string) error {
	if missingMirrorHint(cmd.ErrOrStderr(), dbPath) {
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFiltered(cmd.OutOrStdout(), snapshotDiffEnvelope{Index: indexName, Snapshots: []snapshotDiffRow{}}, flags)
		}
		return nil
	}
	s, db, err := openNovelDB(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	sinceTime := time.Now().Add(-7 * 24 * time.Hour)
	if since != "" {
		if d, err := time.ParseDuration(since); err == nil {
			sinceTime = time.Now().Add(-d)
		}
	}
	rows, err := db.QueryContext(ctx,
		`SELECT captured_at, note, total_vectors FROM pp_snapshots
		 WHERE index_name = ? AND captured_at >= ? ORDER BY captured_at ASC`,
		indexName, sinceTime.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("querying snapshots: %w", err)
	}
	type row struct {
		CapturedAt   string
		Note         string
		TotalVectors int64
	}
	var snapshots []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.CapturedAt, &r.Note, &r.TotalVectors); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scanning snapshot: %w", err)
		}
		snapshots = append(snapshots, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterating snapshots: %w", err)
	}
	_ = rows.Close()

	outRows := make([]snapshotDiffRow, 0, len(snapshots))
	var delta int64
	notes := []string{}
	for i, r := range snapshots {
		outRows = append(outRows, snapshotDiffRow{
			Index:        indexName,
			CapturedAt:   r.CapturedAt,
			Note:         r.Note,
			TotalVectors: r.TotalVectors,
		})
		if i > 0 {
			delta += r.TotalVectors - snapshots[i-1].TotalVectors
		}
		if r.Note != "" {
			notes = append(notes, r.Note)
		}
	}
	env := snapshotDiffEnvelope{Index: indexName, Since: sinceTime.UTC().Format(time.RFC3339), Snapshots: outRows, TotalDelta: delta, Notes: notes}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), env, flags)
	}
	if len(outRows) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No snapshots for %s since %s. Run: pinecone-pp-cli snapshot %s\n", indexName, env.Since, indexName)
		return nil
	}
	for _, r := range outRows {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  total=%d  note=%q\n", r.CapturedAt, r.TotalVectors, r.Note)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "total delta over window: %d vectors\n", delta)
	return nil
}
