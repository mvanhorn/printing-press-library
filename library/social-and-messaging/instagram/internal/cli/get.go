// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli get <url-or-shortcode>` — single-post downloader. Wraps
// internal/download with the user-facing flag surface.

package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/download"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/igutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func newGetCmd(flags *rootFlags) *cobra.Command {
	var (
		noVideos       bool
		noPictures     bool
		noThumbnails   bool
		captions       bool
		captionsFormat string
		geotags        bool
		comments       bool
		commentsLimit  int
		slide          string
		slideKind      string
		out            string
		dryRun         bool
		maxBytes       int64
		dbPath         string
	)

	cmd := &cobra.Command{
		Use:   "get [url-or-shortcode]",
		Short: "Download a single Instagram post, reel, or IGTV by URL or shortcode",
		Long: `Download one Instagram media item to the local filesystem and record it
in the local catalog.

Accepts a full URL (https://www.instagram.com/p/<sc>/, /reel/<sc>/, /tv/<sc>/)
or a bare shortcode. Carousel posts download all children unless --slide
narrows the range. --slide-kind picks just images or just videos.

Side-effects: writes <shortcode>_<idx>.<ext> files into --out, plus optional
<shortcode>.txt|.json caption file, .location.json geotag, and
.comments.json comments.`,
		Example: `  instagram-pp-cli get https://www.instagram.com/p/B0Lz_4QH/
  instagram-pp-cli get B0Lz_4QH --captions --geotags
  instagram-pp-cli get B0Lz_4QH --slide 1-3 --slide-kind image
  instagram-pp-cli get B0Lz_4QH --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			shortcode, err := igutil.ParseShortcode(args[0])
			if err != nil {
				return usageErr(err)
			}

			// dryRunOK runs BEFORE any IO so `PRINTING_PRESS_VERIFY=1 ... --dry-run`
			// exits 0 without dialing out. The verify pipeline depends on this.
			if dryRunOK(flags) || dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch shortcode=%s out=%s slide=%q slide_kind=%q\n",
					shortcode, out, slide, slideKind)
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"would_fetch": true,
						"shortcode":   shortcode,
						"out":         out,
						"slide":       slide,
						"slide_kind":  slideKind,
					}, flags)
				}
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch shortcode=%s (verify env)\n", shortcode)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			st, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer st.Close()
			if err := store.ApplyExtraMigrations(cmd.Context(), st.DB()); err != nil {
				return err
			}

			if out == "" {
				out = "."
			}
			absOut, _ := filepath.Abs(out)

			d := &download.Downloader{
				OutputDir:  absOut,
				HTTPClient: c.HTTPClient,
				DB:         st.DB(),
			}

			// Translate the simpler boolean flags into the package's typed
			// SlideKind. Explicit slide-kind wins over the no-foo flags.
			effSlideKind := slideKind
			if effSlideKind == "" {
				switch {
				case noVideos && !noPictures:
					effSlideKind = "image"
				case noPictures && !noVideos:
					effSlideKind = "video"
				}
			}
			_ = noThumbnails // accepted for parity with instaloader; full media is always served, not thumbs

			opts := download.DownloadOptions{
				Shortcode:       shortcode,
				SlideKind:       effSlideKind,
				IncludeCaptions: captions,
				CaptionsFormat:  captionsFormat,
				IncludeGeotag:   geotags,
				IncludeComments: comments,
				Slide:           slide,
				MaxBytes:        maxBytes,
			}
			res, err := d.DownloadPost(cmd.Context(), c, opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			for _, f := range res.Files {
				if f.Skipped {
					fmt.Fprintf(cmd.OutOrStdout(), "skip %s: %s\n", f.CDNURL, f.SkipReason)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%dB)\n", f.LocalPath, f.Bytes)
			}
			_ = commentsLimit // wired through opts; comments writer pulls a single page today
			return nil
		},
	}

	cmd.Flags().BoolVar(&noVideos, "no-videos", false, "Skip video carousel children")
	cmd.Flags().BoolVar(&noPictures, "no-pictures", false, "Skip image carousel children")
	cmd.Flags().BoolVar(&noThumbnails, "no-thumbnails", false, "Accepted for instaloader parity; ignored (we always pick best-resolution candidates)")
	cmd.Flags().BoolVar(&captions, "captions", false, "Write <shortcode>.txt (or .json with --captions-format json)")
	cmd.Flags().StringVar(&captionsFormat, "captions-format", "txt", "Caption file format: txt or json")
	cmd.Flags().BoolVar(&geotags, "geotags", false, "Write <shortcode>.location.json when the post has location metadata")
	cmd.Flags().BoolVar(&comments, "comments", false, "Write <shortcode>.comments.json (first page only)")
	cmd.Flags().IntVar(&commentsLimit, "comments-limit", 50, "Maximum comments to fetch (per IG default page size)")
	cmd.Flags().StringVar(&slide, "slide", "", "Carousel slide range (1-indexed, inclusive), e.g. 1-3 or 2")
	cmd.Flags().StringVar(&slideKind, "slide-kind", "", "Filter carousel children by kind: image | video")
	cmd.Flags().StringVar(&out, "out", "./", "Output directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be fetched without downloading")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", 0, "Skip files larger than this many bytes (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")

	return cmd
}
