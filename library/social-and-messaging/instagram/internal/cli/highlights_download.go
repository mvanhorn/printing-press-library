// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli highlights download <handle>` — bulk-download all
// highlight items for a profile. Separate from `highlights tray` (which
// just lists) and `highlights diff` (the novel C6 command).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/download"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func newHighlightsDownloadCmd(flags *rootFlags) *cobra.Command {
	var (
		ids    []string
		out    string
		dryRun bool
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "download [handle]",
		Short: "Download all (or selected) highlight items for a profile",
		Example: `  instagram-pp-cli highlights download nasa
  instagram-pp-cli highlights download nasa --ids 17912345678901234,17998765432109876
  instagram-pp-cli highlights download nasa --out ./archive --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			handle := args[0]
			if dryRunOK(flags) || dryRun {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_fetch_highlights": handle, "ids": ids}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch highlights for %s (ids=%v)\n", handle, ids)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch highlights for %s (verify env)\n", handle)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			userID, username, err := resolveUserID(c, handle)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			tray, err := c.Get(fmt.Sprintf("/api/v1/highlights/%s/highlights_tray/", userID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var trayResp struct {
				Tray []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"tray"`
			}
			if err := json.Unmarshal(tray, &trayResp); err != nil {
				return fmt.Errorf("parsing tray: %w", err)
			}

			if out == "" {
				out = "./" + username + "/highlights"
			}
			absOut, _ := filepath.Abs(out)
			_ = os.MkdirAll(absOut, 0o755)

			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			st, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			_ = store.ApplyExtraMigrations(cmd.Context(), st.DB())

			results := []map[string]any{}
			for _, h := range trayResp.Tray {
				if len(ids) > 0 && !includesItem(ids, h.ID) && !includesItem(ids, strings.TrimPrefix(h.ID, "highlight:")) {
					continue
				}
				reelID := h.ID
				if !strings.HasPrefix(reelID, "highlight:") {
					reelID = "highlight:" + reelID
				}
				raw, err := c.Get("/api/v1/feed/reels_media/", map[string]string{"reel_ids": reelID})
				if err != nil {
					results = append(results, map[string]any{"id": h.ID, "error": err.Error()})
					continue
				}
				items := parseReelsItems(raw, reelID)
				highlightDir := filepath.Join(absOut, sanitize(h.Title))
				_ = os.MkdirAll(highlightDir, 0o755)
				for _, item := range items {
					url, kind := pickStoryURL(item)
					if url == "" {
						continue
					}
					ext := download.CDNExtFromURL(url)
					if ext == "" {
						if kind == "video" {
							ext = ".mp4"
						} else {
							ext = ".jpg"
						}
					}
					name := fmt.Sprintf("%d_%s%s", item.TakenAt, item.ID, ext)
					localPath := filepath.Join(highlightDir, name)
					ctx := cmd.Context()
					bytes, sha, err := simpleFetchToFile(ctx, c.HTTPClient, url, localPath)
					results = append(results, map[string]any{
						"id":         item.ID,
						"highlight":  h.Title,
						"local_path": localPath,
						"bytes":      bytes,
						"sha256":     sha,
						"error":      errString(err),
					})
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"handle":     handle,
					"username":   username,
					"highlights": results,
				}, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%v\n", r["local_path"])
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&ids, "id", nil, "Filter to specific highlight IDs")
	cmd.Flags().StringVar(&out, "out", "", "Output dir (default ./<handle>/highlights)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be fetched")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// sanitize turns a highlight title into a filesystem-safe directory name.
// We replace path separators and a handful of common Windows-unfriendly
// characters; aggressive Unicode normalization is overkill for our use case.
func sanitize(s string) string {
	if s == "" {
		return "untitled"
	}
	bad := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, b := range bad {
		s = strings.ReplaceAll(s, b, "_")
	}
	return s
}
