// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newGrabCmd(flags *rootFlags) *cobra.Command {
	var pattern, platform, industry, out, rename string
	var limit int
	cmd := &cobra.Command{
		Use:     "grab",
		Short:   "Batch-download matching Mobbin screens with deterministic filenames.",
		Example: `  mobbin-pp-cli grab --pattern empty-state --platform web --industry fintech --out ./refs --rename "{app}_{pattern}_{idx}.png" --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			hits, err := searchScreensAPI(cmd.Context(), c, platform, pattern, industry, limit)
			if err != nil {
				return err
			}
			hits, errs := cacheHits(cmd.Context(), hits)
			for _, e := range errs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: image download failed: %v\n", e)
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			items := []map[string]any{}
			for i, h := range hits {
				name := renderName(rename, h, i+1)
				dst := filepath.Join(out, name)
				if h.LocalPath != "" {
					data, err := os.ReadFile(h.LocalPath)
					if err != nil {
						return err
					}
					if err := os.WriteFile(dst, data, 0o644); err != nil {
						return err
					}
				}
				items = append(items, map[string]any{"filename": name, "app": h.App, "pattern": h.Pattern, "screen_id": h.ID, "image_url": h.ImageURL, "local_path": h.LocalPath})
			}
			manifest := map[string]any{"generated_at": time.Now().UTC().Format(time.RFC3339), "query": map[string]any{"pattern": pattern, "platform": platform, "industry": industry, "limit": limit}, "count": len(items), "items": items}
			if len(items) == 0 {
				// Empty-result guidance: screen search requires a Mobbin Pro
				// session; surface the hint instead of returning a silent
				// zero-count manifest that looks like the command did nothing.
				manifest["note"] = "no screens matched; this endpoint requires a Mobbin Pro session — run `mobbin-pp-cli auth login --chrome` and try a broader --pattern or remove --industry"
			}
			b, _ := json.MarshalIndent(manifest, "", "  ")
			if err := os.WriteFile(filepath.Join(out, "manifest.json"), b, 0o644); err != nil {
				return err
			}
			return flags.printJSON(cmd, manifest)
		},
	}
	cmd.Flags().StringVar(&pattern, "pattern", "", "Screen pattern slug, e.g. empty-state")
	cmd.Flags().StringVar(&platform, "platform", "web", "Platform to search")
	cmd.Flags().StringVar(&industry, "industry", "", "App category to narrow by")
	cmd.Flags().StringVar(&out, "out", "./refs", "Output directory")
	cmd.Flags().StringVar(&rename, "rename", "{app}_{pattern}_{idx}.webp", "Filename template using {app}, {pattern}, {idx}, {screen_id}")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum screens to download")
	return cmd
}

func renderName(t string, h screenHit, idx int) string {
	repls := map[string]string{"{app}": safeFile(h.App), "{pattern}": safeFile(h.Pattern), "{idx}": strconv.Itoa(idx), "{screen_id}": safeFile(h.ID)}
	for k, v := range repls {
		t = strings.ReplaceAll(t, k, v)
	}
	// Collapse any directory portion so a --rename template like
	// "../../x_{idx}.png" (or an absolute path) cannot escape --out.
	t = filepath.Base(filepath.Clean(t))
	if t == "." || t == ".." || t == string(filepath.Separator) {
		t = strconv.Itoa(idx)
	}
	return t
}

func safeFile(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("/", "-", "\\", "-", " ", "-", "_", "-")
	return r.Replace(s)
}
