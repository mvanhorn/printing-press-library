// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: catalog change diff. Compares the current synced image model
// catalog against the stored previous snapshot in the local store, emitting
// newly added, retired, and changed models. No live API call.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// pp:data-source local

type catalogChange struct {
	Kind    string `json:"kind"` // added | retired | changed
	ModelID string `json:"model_id"`
	Name    string `json:"name,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type modelsDiffView struct {
	Changes      []catalogChange `json:"changes"`
	SnapshotFrom string          `json:"snapshot_from,omitempty"`
	CurrentCount int             `json:"current_models"`
	PriorCount   int             `json:"prior_models"`
	Note         string          `json:"note,omitempty"`
}

func newNovelModelsDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince string
		dbPath    string
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "See newly added, retired, and price-changed image models between syncs so pinned pipelines never break silently.",
		Long: `Compare the current synced image model catalog against the stored previous
snapshot and list what changed: newly added models, retired models, and models
whose capabilities changed.

The first run stores the current catalog as the baseline snapshot. Run sync
before diffing to refresh the current catalog:

  openrouter-image-pp-cli sync --resources images --full
  openrouter-image-pp-cli models diff

Use this command to see what changed in the model catalog between syncs.
Do NOT use it to browse the current catalog; use 'models list' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli models diff
  openrouter-image-pp-cli models diff --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff the image catalog against the stored snapshot")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("openrouter-image-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: openrouter-image-pp-cli sync --resources images --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureOpenRouterImageTables(ctx); err != nil {
				return err
			}

			// Load the current catalog from the synced images table.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT id, COALESCE(name,''), COALESCE(data,'{}') FROM images ORDER BY id`)
			if err != nil {
				return fmt.Errorf("querying image catalog: %w", err)
			}
			type catRow struct {
				id   string
				name string
				data json.RawMessage
			}
			current := make([]catRow, 0)
			for rows.Next() {
				var r catRow
				var data string
				if err := rows.Scan(&r.id, &r.name, &data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan catalog row: %w", err)
				}
				r.data = json.RawMessage(data)
				current = append(current, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate catalog rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close catalog rows: %w", err)
			}

			view := modelsDiffView{Changes: make([]catalogChange, 0), CurrentCount: len(current)}

			prior, err := db.GetCatalogSnapshot(ctx)
			if err != nil {
				return fmt.Errorf("reading catalog snapshot: %w", err)
			}
			if prior == nil || len(prior.Snapshot) == 0 {
				// First run: store baseline and report.
				snap := make([]map[string]any, 0, len(current))
				for _, r := range current {
					snap = append(snap, map[string]any{"id": r.id, "name": r.name, "data": json.RawMessage(r.data)})
				}
				if err := db.PutCatalogSnapshot(ctx, snap); err != nil {
					return err
				}
				view.Note = "no prior snapshot; stored the current catalog as baseline"
				return emitModelsDiff(cmd, flags, view)
			}
			view.SnapshotFrom = prior.TakenAt.Format(time.RFC3339)
			view.PriorCount = len(prior.Snapshot)

			priorByID := make(map[string]map[string]any, len(prior.Snapshot))
			for _, p := range prior.Snapshot {
				if id, _ := p["id"].(string); id != "" {
					priorByID[id] = p
				}
			}
			currentByID := make(map[string]catRow, len(current))
			for _, c := range current {
				currentByID[c.id] = c
			}

			for id, p := range priorByID {
				if _, ok := currentByID[id]; !ok {
					name, _ := p["name"].(string)
					view.Changes = append(view.Changes, catalogChange{Kind: "retired", ModelID: id, Name: name})
				}
			}
			for id, c := range currentByID {
				if p, ok := priorByID[id]; ok {
					// Compare capability signatures for a "changed" signal.
					// Canonicalize both sides through map[string]any so key
					// order differences in the raw JSON cannot false-positive.
					oldData, _ := json.Marshal(normalizeAny(p["data"]))
					newData, _ := json.Marshal(normalizeAny(c.data))
					if string(oldData) != string(newData) {
						view.Changes = append(view.Changes, catalogChange{Kind: "changed", ModelID: id, Name: c.name, Detail: "capabilities or metadata changed"})
					}
					continue
				}
				view.Changes = append(view.Changes, catalogChange{Kind: "added", ModelID: id, Name: c.name})
			}

			// Store the current catalog as the new baseline.
			snap := make([]map[string]any, 0, len(current))
			for _, r := range current {
				snap = append(snap, map[string]any{"id": r.id, "name": r.name, "data": json.RawMessage(r.data)})
			}
			if err := db.PutCatalogSnapshot(ctx, snap); err != nil {
				return err
			}

			return emitModelsDiff(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only report changes in the given window (e.g. 7d); informational")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: platform data dir)")
	return cmd
}

func emitModelsDiff(cmd *cobra.Command, flags *rootFlags, view modelsDiffView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if view.Note != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
	}
	if len(view.Changes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no catalog changes since last snapshot")
		return nil
	}
	for _, c := range view.Changes {
		fmt.Fprintf(cmd.OutOrStdout(), "%-8s %s", c.Kind, c.ModelID)
		if c.Name != "" {
			fmt.Fprintf(cmd.OutOrStdout(), " (%s)", c.Name)
		}
		if c.Detail != "" {
			fmt.Fprintf(cmd.OutOrStdout(), " %s", c.Detail)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	return nil
}

func normalizeAny(v any) any {
	var out any
	switch t := v.(type) {
	case json.RawMessage:
		_ = json.Unmarshal(t, &out)
		return out
	case string:
		var o any
		if json.Unmarshal([]byte(t), &o) == nil {
			return o
		}
	}
	return v
}
