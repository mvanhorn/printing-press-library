// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

// variantOrder ranks rendition kinds from highest to lowest quality.
// Used as the default --prefer when the caller doesn't supply one.
var variantOrder = []string{"orig", "large", "medium", "small", "thumb", "mobile", "preview", "audio_orig", "audio_128k"}

func newAssetsBestCmd(flags *rootFlags) *cobra.Command {
	var (
		prefer   string
		maxBytes int64
	)
	cmd := &cobra.Command{
		Use:   "best [nasa_id]",
		Short: "Print the URL of the best available variant for an asset (deterministic)",
		Long: `Parse an asset's rendition manifest, classify each file by variant
(orig/large/medium/small/thumb/mobile/preview/audio_orig/audio_128k), apply a
caller preference order, and print exactly one URL. Optionally apply a byte
ceiling via a HEAD probe — variants larger than --max-bytes are skipped.

Agents should reach for this when they need a deterministic one-URL answer to
"give me the best image for nasa_id X under 5 MB" — no manifest parsing, no
filename-prose interpretation.`,
		Example:     "  nasa-images-pp-cli assets best as11-40-5874 --prefer orig,large,medium --max-bytes 5000000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			nasaID := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would pick best variant for %q\n", nasaID)
				return nil
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/asset/"+nasaID, nil)
			if err != nil {
				return fmt.Errorf("calling /asset/%s: %w", nasaID, err)
			}
			coll, err := parseNasaCollection(raw)
			if err != nil {
				return err
			}

			// Collect candidate URLs by variant kind.
			byVariant := make(map[string][]string)
			for _, item := range coll.Collection.Items {
				if item.Href == "" {
					continue
				}
				kind := classifyVariant(item.Href)
				if kind == "metadata" || kind == "captions" || kind == "other" {
					continue
				}
				byVariant[kind] = append(byVariant[kind], upgradeToHTTPS(item.Href))
			}

			order := variantOrder
			if strings.TrimSpace(prefer) != "" {
				order = nil
				for _, p := range strings.Split(prefer, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						order = append(order, p)
					}
				}
			}

			var chosenURL, chosenVariant string
			var chosenBytes int64
			for _, variant := range order {
				urls, ok := byVariant[variant]
				if !ok || len(urls) == 0 {
					continue
				}
				for _, u := range urls {
					if maxBytes > 0 {
						size, ok := probeContentLength(ctx, flags, u)
						if !ok {
							// HEAD failed — skip this URL rather than accept
							// it without verifying the size; the caller asked
							// for a hard byte ceiling and a flaky HEAD must
							// not silently waive it.
							continue
						}
						if size > maxBytes {
							continue
						}
						chosenURL, chosenVariant, chosenBytes = u, variant, size
						goto chose
					}
					chosenURL, chosenVariant = u, variant
					goto chose
				}
			}

		chose:
			if chosenURL == "" {
				return fmt.Errorf("no variant for nasa_id %q matched the preference %v with max-bytes %d", nasaID, order, maxBytes)
			}
			result := map[string]any{
				"nasa_id":   nasaID,
				"variant":   chosenVariant,
				"url":       chosenURL,
				"max_bytes": maxBytes,
				"prefer":    order,
			}
			if chosenBytes > 0 {
				result["bytes"] = chosenBytes
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintln(cmd.OutOrStdout(), chosenURL)
			return nil
		},
	}
	cmd.Flags().StringVar(&prefer, "prefer", "", "Comma-separated variant preference order (default: orig,large,medium,small,thumb,mobile,preview)")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", 0, "Skip variants larger than this size (0 = no limit); uses HEAD probe to learn size")
	return cmd
}

// probeContentLength issues a HEAD request to learn the file size.
// Uses an *http.Client honoring flags.timeout so a stalled CDN HEAD respects
// --timeout instead of hanging.
// Returns (0, false) if the HEAD fails or Content-Length is missing.
func probeContentLength(ctx context.Context, flags *rootFlags, url string) (int64, bool) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, false
	}
	resp, err := httpClientForFlags(flags).Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false
	}
	return resp.ContentLength, resp.ContentLength > 0
}

// unused (kept for future use): convert the asset manifest into a JSON-friendly view.
var _ = json.Marshal
