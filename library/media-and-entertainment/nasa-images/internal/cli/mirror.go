// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/internal/cliutil"

	"github.com/spf13/cobra"
)

func newMirrorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Mirror NASA assets to the local SQLite store",
		Long: `Walk a search query or curated album and upsert each asset into the local
mirror under resource_type='asset'. The Collection+JSON envelope is unwrapped
correctly (each item's data[0] becomes one row keyed by nasa_id), unlike the
generic sync command which stores the raw envelope.

Subcommands:
  mirror search   Walk /search with the standard filters and store every item.
  mirror album    Walk /album/{name} and store every item plus an album_members row.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMirrorSearchCmd(flags))
	cmd.AddCommand(newMirrorAlbumCmd(flags))
	return cmd
}

func newMirrorSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		q, mediaType, yearStart, yearEnd, center string
		photographer, keywords, location, title  string
		nasaID, dbPath                           string
		pageSize, maxPages                       int
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Walk /search and mirror every result into the local store",
		Long: `Mirror a search query into the local SQLite store. Unwraps NASA's
Collection+JSON envelope and stores each item's data[0] block as one row
under resource_type='asset', keyed by nasa_id.

Once populated, run 'recent', 'timeline', 'center profile', or 'citation' for
offline queries that don't hit the API.`,
		Example:     "  nasa-images-pp-cli mirror search --q \"perseverance\" --media-type image --max-pages 5",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would mirror /search results to local store")
				return nil
			}
			// Live-dogfood guard: limit work to one page.
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			path := "/search"
			params := map[string]string{}
			setIf := func(k, v string) {
				if v != "" {
					params[k] = v
				}
			}
			setIf("q", q)
			setIf("media_type", mediaType)
			setIf("year_start", yearStart)
			setIf("year_end", yearEnd)
			setIf("center", center)
			setIf("photographer", photographer)
			setIf("keywords", keywords)
			setIf("location", location)
			setIf("title", title)
			setIf("nasa_id", nasaID)
			if pageSize > 0 {
				params["page_size"] = fmt.Sprintf("%d", pageSize)
			}
			if len(params) == 0 {
				return fmt.Errorf("at least one search parameter is required (--q, --media-type, --center, --year-start, --nasa-id, etc.)")
			}

			totalStored := 0
			totalHits := 0
			pages := 0
			for {
				raw, err := c.Get(path, params)
				if err != nil {
					return fmt.Errorf("calling %s: %w", path, err)
				}
				coll, err := parseNasaCollection(raw)
				if err != nil {
					return err
				}
				if pages == 0 {
					totalHits = coll.Collection.Metadata.TotalHits
				}
				for _, item := range coll.Collection.Items {
					if _, err := upsertAsset(s, item); err != nil {
						return err
					}
					totalStored++
				}
				pages++
				next := nextPageFromLinks(coll.Collection.Links)
				if next == "" {
					break
				}
				if maxPages > 0 && pages >= maxPages {
					break
				}
				newPath, newParams, perr := hrefPathAndParams(next)
				if perr != nil {
					return fmt.Errorf("parsing next-link: %w", perr)
				}
				path = newPath
				params = newParams
			}

			result := map[string]any{
				"stored":     totalStored,
				"pages":      pages,
				"total_hits": totalHits,
				"db":         s.Path(),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mirrored %d assets across %d page(s); upstream total_hits=%d.\nDB: %s\n",
				totalStored, pages, totalHits, s.Path())
			return nil
		},
	}
	cmd.Flags().StringVar(&q, "q", "", "Free-text query terms")
	cmd.Flags().StringVar(&mediaType, "media-type", "", "Filter by media type (image, video, audio)")
	cmd.Flags().StringVar(&yearStart, "year-start", "", "Earliest date_created year (YYYY)")
	cmd.Flags().StringVar(&yearEnd, "year-end", "", "Latest date_created year (YYYY)")
	cmd.Flags().StringVar(&center, "center", "", "NASA center code (HQ, JSC, KSC, GSFC, JPL, MSFC, ARC, LRC, AFRC, GRC, SSC)")
	cmd.Flags().StringVar(&photographer, "photographer", "", "Primary photographer's name")
	cmd.Flags().StringVar(&keywords, "keywords", "", "Comma-separated keyword terms")
	cmd.Flags().StringVar(&location, "location", "", "Location field substring")
	cmd.Flags().StringVar(&title, "title", "", "Title field substring")
	cmd.Flags().StringVar(&nasaID, "nasa-id", "", "Exact match on a specific NASA ID")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "Items per page (1-200)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum pages to walk (0 = unlimited)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}

func newMirrorAlbumCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		maxPages int
	)
	cmd := &cobra.Command{
		Use:   "album [album_name]",
		Short: "Walk /album/{name} and mirror every item plus album_members rows",
		Long: `Mirror a curated album (e.g. Apollo-at-50, Mars-Perseverance) into the
local store. Album names are case-sensitive — pass them exactly as they
appear in search results' data.album field.`,
		Example:     "  nasa-images-pp-cli mirror album Apollo-at-50",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			album := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would mirror album %q to local store\n", album)
				return nil
			}
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			path := "/album/" + album
			params := map[string]string{}
			totalStored := 0
			pages := 0
			position := 0
			for {
				raw, err := c.Get(path, params)
				if err != nil {
					return fmt.Errorf("calling %s: %w", path, err)
				}
				coll, err := parseNasaCollection(raw)
				if err != nil {
					return err
				}
				for _, item := range coll.Collection.Items {
					nasaID, err := upsertAsset(s, item)
					if err != nil {
						return err
					}
					if nasaID == "" {
						continue
					}
					// If the album_members upsert fails mid-stream we'd leave
					// an asset row without an album link, which `unused-in`
					// would later wrongly report. Log+skip the membership
					// rather than aborting; the asset is still recoverable
					// by re-running `mirror album`.
					if memErr := recordAlbumMember(ctx, s.DB(), album, nasaID, position); memErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "WARN: album_members upsert failed for %s: %v\n", nasaID, memErr)
						continue
					}
					position++
					totalStored++
				}
				pages++
				next := nextPageFromLinks(coll.Collection.Links)
				if next == "" {
					break
				}
				if maxPages > 0 && pages >= maxPages {
					break
				}
				newPath, newParams, perr := hrefPathAndParams(next)
				if perr != nil {
					return fmt.Errorf("parsing next-link: %w", perr)
				}
				path = newPath
				params = newParams
			}

			result := map[string]any{
				"stored": totalStored,
				"pages":  pages,
				"album":  album,
				"db":     s.Path(),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mirrored %d assets in album %q across %d page(s).\nDB: %s\n",
				totalStored, album, pages, s.Path())
			return nil
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum pages to walk (0 = unlimited; albums usually fit in 1-3 pages)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}
