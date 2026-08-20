// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/aic"       // register AIC source
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/apod"      // register APOD source
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/cleveland" // register Cleveland source
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/harvard"   // register Harvard source
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/met"       // register Met source
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/npmtw"     // register NPM Taiwan source
	// rijks and tepapa removed 2026-05-22 — both adapters were broken
	// against their live APIs:
	//   - rijks: signup URL data.rijksmuseum.nl/object-metadata/api/ 404s;
	//     the keyless paths either need an XML rewrite (OAI-PMH) or 100×
	//     the request count (search API returns LOD identifiers only).
	//   - tepapa: post-q-filter mapping path drops 100% of records, sending
	//     the curated sync into a runaway pagination loop. Root cause not
	//     yet diagnosed past the q=hasRepresentation:* fix.
	// Source code preserved nowhere — restore from git if reviving either.
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source/smithsonian" // register Smithsonian source
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List configured art-goat sources and sync state",
		Long: `art-goat aggregates from multiple sources into the unified works table.
Subcommands:

  sources list   — show configured sources + per-source counts and last sync
  sources sync   — pull from configured sources into the local works table`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newSourcesListCmd(flags))
	cmd.AddCommand(newSourcesSyncCmd(flags))
	return cmd
}

func newSourcesListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show configured art-goat sources",
		Example: `  # Show all configured sources and how many works each has synced
  art-goat-pp-cli sources list

  # Machine-readable
  art-goat-pp-cli sources list --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}
			counts, _ := db.WorkCounts(cmd.Context())

			out := make([]sourceInfo, 0)
			for _, s := range source.All() {
				out = append(out, sourceInfo{
					Name:         s.Name(),
					Description:  s.Description(),
					AuthRequired: s.AuthRequired(),
					WorkCount:    counts[s.Name()],
				})
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"sources": out, "total_works": sumCounts(counts)}, flags)
			}
			renderSourcesList(cmd, out, counts)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newSourcesSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sourceFilter string
	var limit int
	var full bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull from configured sources into the local works table",
		Long: `Fetch works from configured sources and upsert them into the local works
table. The works table powers the contemplative spine (sit, today, presence)
and the cross-source surface.

Curated default = highlights per source (~5k AIC works, ~500 APOD entries),
which is plenty for daily practice. Pass --full to pull more (much slower).`,
		Example: `  # Sync all configured sources (curated default)
  art-goat-pp-cli sources sync

  # Sync only one source
  art-goat-pp-cli sources sync --source aic

  # Pull more than the curated default
  art-goat-pp-cli sources sync --full

  # Cap at a small number for a quick smoke test
  art-goat-pp-cli sources sync --limit 20`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			var sources []source.Source
			if sourceFilter != "" {
				s, err := source.Lookup(sourceFilter)
				if err != nil {
					return err
				}
				sources = []source.Source{s}
			} else {
				sources = source.All()
			}

			results := make([]map[string]any, 0, len(sources))
			ctx := cmd.Context()
			for _, s := range sources {
				start := time.Now()
				opts := source.SyncOpts{Limit: limit, Full: full}
				works, err := s.Sync(ctx, opts)
				result := map[string]any{
					"source":   s.Name(),
					"duration": time.Since(start).String(),
				}
				if err != nil {
					result["error"] = err.Error()
					result["count"] = len(works)
				}
				if len(works) > 0 {
					inserted, ierr := upsertSourceWorks(ctx, db, works)
					result["count"] = inserted
					if ierr != nil {
						result["error"] = ierr.Error()
					}
				} else if err == nil {
					result["count"] = 0
				}
				results = append(results, result)
				if !flags.asJSON {
					if errMsg, ok := result["error"].(string); ok {
						fmt.Fprintf(cmd.OutOrStdout(), "%-6s  ERROR  %s\n", s.Name(), errMsg)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%-6s  %d works  (%s)\n", s.Name(), result["count"], result["duration"])
					}
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"results": results}, flags)
			}
			counts, _ := db.WorkCounts(ctx)
			fmt.Fprintf(cmd.OutOrStdout(), "\nLocal corpus: %d works\n", sumCounts(counts))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Constrain to one source (e.g. aic, apod)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap works per source; 0 = source-default (curated)")
	cmd.Flags().BoolVar(&full, "full", false, "Pull the full archive rather than the curated highlights")
	return cmd
}

func upsertSourceWorks(ctx context.Context, db *store.Store, works []source.Work) (int, error) {
	storeWorks := make([]store.Work, 0, len(works))
	for _, w := range works {
		storeWorks = append(storeWorks, store.Work{
			ID:               w.ID,
			Source:           w.Source,
			SourceID:         w.SourceID,
			Title:            w.Title,
			Creator:          w.Creator,
			CreatorCanonical: w.CreatorCanonical,
			DateText:         w.DateText,
			DateStart:        w.DateStart,
			DateEnd:          w.DateEnd,
			Medium:           w.Medium,
			Classification:   w.Classification,
			Period:           w.Period,
			CultureRegion:    w.CultureRegion,
			Description:      w.Description,
			ImageURL:         w.ImageURL,
			ThumbnailURL:     w.ThumbnailURL,
			License:          w.License,
			SourceURL:        w.SourceURL,
			RawJSON:          w.RawJSON,
			SyncedAt:         w.SyncedAt,
		})
	}
	return db.UpsertWorksBatch(ctx, storeWorks)
}

type sourceInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	AuthRequired bool   `json:"auth_required"`
	WorkCount    int    `json:"work_count"`
}

func renderSourcesList(cmd *cobra.Command, sources []sourceInfo, counts map[string]int) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Configured sources:")
	if len(sources) == 0 {
		fmt.Fprintln(out, "  (none registered)")
		return
	}
	for _, s := range sources {
		authNote := ""
		if s.AuthRequired {
			authNote = " [auth required]"
		}
		fmt.Fprintf(out, "  %s — %s%s\n    %d works synced\n",
			s.Name, s.Description, authNote, s.WorkCount)
	}
	fmt.Fprintf(out, "\nLocal corpus: %d works\n", sumCounts(counts))
	fmt.Fprintln(out, "\nRun `art-goat-pp-cli sources sync` to populate / refresh.")
}

func sumCounts(m map[string]int) int {
	total := 0
	for _, n := range m {
		total += n
	}
	return total
}
