// Copyright 2026 Jan Medina and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented similar (note-profile overlap) per research manifest.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"perfumenz-pp-cli/internal/store"
)

func newNovelSimilarCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:         "similar <handle>",
		Short:       "List perfumes with overlapping note profiles to a given one.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  perfumenz-pp-cli similar \"wolf-by-rayhaan-100ml-edp\" --limit 5 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute note overlap against seed perfume")
				return nil
			}
			handle := args[0]
			dbPath := defaultDBPath("perfumenz-pp-cli")
			if dbPath == "" {
				dbPath = os.ExpandEnv("$HOME/.local/share/perfumenz-pp-cli/data.db")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w (run sync --seed first)", err)
			}
			defer db.Close()

			// find seed
			row := db.DB().QueryRowContext(cmd.Context(), `
				SELECT data FROM resources
				WHERE resource_type IN ('perfumes','products','products-json')
				  AND (id = ? OR json_extract(data,'$.handle') = ?)
				LIMIT 1
			`, handle, handle)
			var seedData []byte
			if err := row.Scan(&seedData); err != nil {
				return fmt.Errorf("seed perfume %q not found (sync first): %w", handle, err)
			}
			var seed map[string]any
			json.Unmarshal(seedData, &seed)
			sTop, sHeart, sBase := notesFromAny(seed)

			// scan others
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM resources
				WHERE resource_type IN ('perfumes','products','products-json')
			`)
			if err != nil {
				return err
			}
			defer rows.Close()

			type res struct {
				Handle string
				Title  string
				Vendor string
				Score  int
			}
			var out []res
			for rows.Next() {
				var id string
				var data []byte
				rows.Scan(&id, &data)
				var p map[string]any
				json.Unmarshal(data, &p)
				if h, _ := p["handle"].(string); h == handle || id == handle {
					continue
				}
				t, h, b := notesFromAny(p)
				sc := NotesOverlap(sTop, sHeart, sBase, t, h, b)
				if sc == 0 {
					continue
				}
				title, _ := p["title"].(string)
				vendor, _ := p["vendor"].(string)
				out = append(out, res{Handle: id, Title: title, Vendor: vendor, Score: sc})
			}
			// sort desc score
			for i := range out {
				for j := i + 1; j < len(out); j++ {
					if out[j].Score > out[i].Score {
						out[i], out[j] = out[j], out[i]
					}
				}
			}
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"seed": handle, "similar": out})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Similar to %s (note overlap):\n", handle)
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s (score %d)\n", r.Title, r.Vendor, r.Score)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 8, "Max similar results")
	return cmd
}

func notesFromAny(p map[string]any) (t, h, b []string) {
	if tn, ok := p["top_notes"].([]any); ok {
		for _, x := range tn {
			if s, ok := x.(string); ok {
				t = append(t, s)
			}
		}
	}
	if hn, ok := p["heart_notes"].([]any); ok {
		for _, x := range hn {
			if s, ok := x.(string); ok {
				h = append(h, s)
			}
		}
	}
	if bn, ok := p["base_notes"].([]any); ok {
		for _, x := range bn {
			if s, ok := x.(string); ok {
				b = append(b, s)
			}
		}
	}
	if len(t)+len(h)+len(b) == 0 {
		if bh, ok := p["body_html"].(string); ok {
			t, h, b = ParseNotes(bh)
		}
	}
	return
}
