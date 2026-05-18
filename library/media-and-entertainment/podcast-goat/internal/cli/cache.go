// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `cache` parent + subcommands.

package cli

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/spotify"
)

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect and manage the local transcript cache",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCacheListCmd(flags))
	cmd.AddCommand(newCacheExportCmd(flags))
	cmd.AddCommand(newCacheClearCmd(flags))
	return cmd
}

func newCacheListCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSource string
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List cached episodes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.ListEpisodes(cmd.Context(), flagLimit, flagSource)
			if err != nil {
				return err
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no episodes cached yet. Try `episode get <url>` first.")
				return nil
			}
			headers := []string{"show", "title", "source", "tier", "fetched_at"}
			var data [][]string
			for _, r := range rows {
				data = append(data, []string{r.Show, r.Title, r.Source, r.Tier, r.FetchedAt.Format("2006-01-02")})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	cmd.Flags().StringVar(&flagSource, "source", "", "Filter by source (e.g. dwarkesh, spoken)")
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 100, "Max rows")
	return cmd
}

func newCacheExportCmd(flags *rootFlags) *cobra.Command {
	var (
		flagFormat string
		flagOut    string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export cached transcripts as md, jsonl, or a zip bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.ListEpisodes(cmd.Context(), 10_000, "")
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no episodes cached yet")
				return nil
			}
			outPath := flagOut
			if outPath == "" {
				ts := time.Now().Format("20060102-150405")
				outPath = filepath.Join(podcastConfigDir(), "exports", "transcripts-"+ts+"."+flagFormat)
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			switch flagFormat {
			case "md":
				var b []byte
				for _, r := range rows {
					full, _ := ps.GetTranscript(cmd.Context(), r.URL)
					if full == nil {
						continue
					}
					b = append(b, []byte("\n\n---\n\n")...)
					b = append(b, []byte(full.ContentMD)...)
				}
				if err := os.WriteFile(outPath, b, 0o644); err != nil {
					return err
				}
			case "jsonl":
				f, err := os.Create(outPath)
				if err != nil {
					return err
				}
				defer f.Close()
				enc := json.NewEncoder(f)
				for _, r := range rows {
					_ = enc.Encode(r)
				}
			case "zip":
				f, err := os.Create(outPath)
				if err != nil {
					return err
				}
				defer f.Close()
				zw := zip.NewWriter(f)
				defer zw.Close()
				for _, r := range rows {
					full, _ := ps.GetTranscript(cmd.Context(), r.URL)
					if full == nil {
						continue
					}
					name := r.Source + "/" + r.ID[:16] + ".md"
					w, err := zw.Create(name)
					if err != nil {
						return err
					}
					_, _ = w.Write([]byte(full.ContentMD))
				}
			default:
				return fmt.Errorf("unknown --format %q (use md|jsonl|zip)", flagFormat)
			}
			fmt.Fprintln(cmd.OutOrStdout(), outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "md", "Export format: md, jsonl, or zip")
	cmd.Flags().StringVar(&flagOut, "out", "", "Destination path (default: ~/.config/podcast-goat/exports/...)")
	return cmd
}

func newCacheClearCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSource  string
		flagConfirm bool
	)
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear cached transcripts (--source restricts to one adapter)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !flagConfirm && !flags.yes {
				return fmt.Errorf("cache clear is destructive; pass --confirm (or --yes)")
			}
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			if flagSource == "" {
				return fmt.Errorf("specify --source <name> (use --source all to wipe everything)")
			}
			if flagSource == "all" {
				// Propagate ListEpisodes errors. Without this, a DB read
				// failure (corruption, schema mismatch, locked) silently
				// returns nil rows, the loop is a no-op, and we tell the
				// user "cleared 0 episodes" — false confirmation that
				// nothing was actually wiped.
				rows, listErr := ps.ListEpisodes(cmd.Context(), 100_000, "")
				if listErr != nil {
					return fmt.Errorf("list episodes for clear: %w", listErr)
				}
				// Dedupe sources before iterating — ListEpisodes returns one
				// row per episode, so without dedup we'd call ClearBySource
				// once per row instead of once per source.
				seen := map[string]bool{}
				total := int64(0)
				for _, r := range rows {
					if seen[r.Source] {
						continue
					}
					seen[r.Source] = true
					n, err := ps.ClearBySource(cmd.Context(), r.Source)
					if err != nil {
						return err
					}
					total += n
				}
				fmt.Fprintf(cmd.OutOrStdout(), "cleared %d episodes\n", total)
				return nil
			}
			n, err := ps.ClearBySource(cmd.Context(), flagSource)
			if err != nil {
				return err
			}
			// Source-specific ephemeral state: spotify's bearer cache is
			// keyed by sp_dc hash; clearing the source signals the user
			// wants a clean slate, so the cached bearer goes too.
			if flagSource == "spotify" {
				_ = spotify.ClearDiskCache()
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cleared %d episodes from source=%s\n", n, flagSource)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSource, "source", "", "Adapter slug or 'all'")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Required confirmation flag")
	return cmd
}
