// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: tag analytics over the locally synced mirror —
// the most common tags, or models matching ALL of several tags (intersection).

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local

type taggedDesign struct {
	ID        string
	Title     string
	Creator   string
	Downloads int
	Tags      []string
}

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type tagMatch struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Creator   string   `json:"creator"`
	Downloads int      `json:"downloads"`
	Tags      []string `json:"tags"`
	URL       string   `json:"url"`
}

type taggedEnvelope struct {
	ID            json.Number `json:"id"`
	Title         string      `json:"title"`
	DownloadCount int         `json:"downloadCount"`
	Tags          []string    `json:"tags"`
	DesignCreator struct {
		Name string `json:"name"`
	} `json:"designCreator"`
}

func loadTaggedDesigns(ctx context.Context, sqlDB *sql.DB) ([]taggedDesign, error) {
	rows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'designs'`)
	if err != nil {
		return nil, fmt.Errorf("reading designs mirror: %w", err)
	}
	defer rows.Close()
	var out []taggedDesign
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scanning tagged design row: %w", err)
		}
		if !raw.Valid {
			continue // NULL payload — skip, not an error
		}
		var e taggedEnvelope
		if json.Unmarshal([]byte(raw.String), &e) != nil {
			continue
		}
		id := e.ID.String()
		if id == "" || id == "0" {
			continue
		}
		out = append(out, taggedDesign{
			ID:        id,
			Title:     e.Title,
			Creator:   e.DesignCreator.Name,
			Downloads: e.DownloadCount,
			Tags:      e.Tags,
		})
	}
	return out, rows.Err()
}

// aggregateTags counts distinct tags across designs (case-insensitive merge).
func aggregateTags(designs []taggedDesign, minCount, limit int) []tagCount {
	counts := make(map[string]int)
	for _, d := range designs {
		seen := make(map[string]bool)
		for _, t := range d.Tags {
			lt := strings.ToLower(strings.TrimSpace(t))
			if lt == "" || seen[lt] {
				continue
			}
			seen[lt] = true
			counts[lt]++
		}
	}
	out := make([]tagCount, 0, len(counts))
	for t, c := range counts {
		if c >= minCount {
			out = append(out, tagCount{Tag: t, Count: c})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// matchAllTags returns designs whose tag set contains every wanted tag
// (case-insensitive), ranked by downloads.
func matchAllTags(designs []taggedDesign, want []string, limit int) []tagMatch {
	lwant := make([]string, 0, len(want))
	for _, w := range want {
		lw := strings.ToLower(strings.TrimSpace(w))
		if lw != "" {
			lwant = append(lwant, lw)
		}
	}
	out := make([]tagMatch, 0)
	// No usable tags (e.g. all-whitespace args) must not vacuously match every
	// design; return no matches so the caller surfaces its empty-result hint.
	if len(lwant) == 0 {
		return out
	}
	for _, d := range designs {
		set := make(map[string]bool, len(d.Tags))
		for _, t := range d.Tags {
			set[strings.ToLower(strings.TrimSpace(t))] = true
		}
		all := true
		for _, w := range lwant {
			if !set[w] {
				all = false
				break
			}
		}
		if !all {
			continue
		}
		out = append(out, tagMatch{
			ID:        d.ID,
			Title:     d.Title,
			Creator:   d.Creator,
			Downloads: d.Downloads,
			Tags:      d.Tags,
			URL:       "https://makerworld.com/en/models/" + d.ID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Downloads > out[j].Downloads
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func newNovelTagsCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagMinCount int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "tags [tag...]",
		Short: "Tag analytics: list the most common tags, or find models matching ALL given tags",
		Long: "With no arguments, lists the most common tags across your locally synced designs. " +
			"With one or more tag arguments, returns designs whose tags include ALL of them " +
			"(a case-insensitive intersection) ranked by downloads — something the MakerWorld " +
			"web UI cannot do, since it filters by a single tag at a time. Run 'sync' first.",
		Example: strings.Trim(`
  makerworld-pp-cli tags --limit 25 --agent
  makerworld-pp-cli tags toy fidget --agent
  makerworld-pp-cli tags "no ams" keychain --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute tag analytics over the local mirror")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("tags reads your local mirror and has no live mode; run 'sync' then 'tags'"))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be >= 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("makerworld-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: makerworld-pp-cli sync --resources designs --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "designs") {
				hintIfStale(cmd, db, "designs", flags.maxAge)
			}

			designs, err := loadTaggedDesigns(ctx, db.DB())
			if err != nil {
				return err
			}

			machine := flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain)

			if len(args) == 0 {
				cloud := aggregateTags(designs, flagMinCount, flagLimit)
				if len(cloud) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no tags found in a local mirror of %d designs; run 'sync' first\n", len(designs))
				}
				if machine {
					return printJSONFiltered(cmd.OutOrStdout(), cloud, flags)
				}
				items := make([]map[string]any, 0, len(cloud))
				for _, t := range cloud {
					items = append(items, map[string]any{"tag": t.Tag, "count": t.Count})
				}
				if len(items) > 0 {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
				return nil
			}

			matches := matchAllTags(designs, args, flagLimit)
			if len(matches) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no synced designs match all tags %v; try fewer tags or 'sync' more (--max-pages)\n", args)
			}
			if machine {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}
			items := make([]map[string]any, 0, len(matches))
			for _, m := range matches {
				items = append(items, map[string]any{
					"id": m.ID, "title": m.Title, "creator": m.Creator, "downloads": m.Downloads,
				})
			}
			if len(items) > 0 {
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum tags or models to return")
	cmd.Flags().IntVar(&flagMinCount, "min-count", 1, "Minimum occurrences for a tag to appear in the tag list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
