// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli stories <handle>` — download a profile's currently-active
// stories. Stories aren't carouseled; each tray item is a single image or
// video. Resolves handle -> user_id -> reels_media tray.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/download"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// iClient is a local shorthand so call sites stay short — `*iClient` reads
// better at the helper signature than `*client.Client`. Pure local alias,
// no behavior added.
type iClient = client.Client

func newStoriesCmd(flags *rootFlags) *cobra.Command {
	var (
		out    string
		dryRun bool
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "stories [handle]",
		Short: "Download a profile's active stories.",
		Example: `  instagram-pp-cli stories nasa
  instagram-pp-cli stories nasa --out ./archive
  instagram-pp-cli stories nasa --dry-run`,
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
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_fetch_stories": handle}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch stories for %s\n", handle)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch stories for %s (verify env)\n", handle)
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
			raw, err := c.Get("/api/v1/feed/reels_media/", map[string]string{"reel_ids": userID})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items := parseReelsItems(raw, userID)
			if out == "" {
				out = "./" + username + "/stories"
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
				localPath := filepath.Join(absOut, name)
				ctx := cmd.Context()
				bytes, sha, err := simpleFetchToFile(ctx, c.HTTPClient, url, localPath)
				results = append(results, map[string]any{
					"id":         item.ID,
					"taken_at":   item.TakenAt,
					"kind":       kind,
					"local_path": localPath,
					"bytes":      bytes,
					"sha256":     sha,
					"error":      errString(err),
				})
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"handle":   handle,
					"username": username,
					"stories":  results,
				}, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%v\n", r["local_path"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Output dir (default ./<handle>/stories)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be fetched")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// reelItem is the minimal shape we need from each reel tray entry.
type reelItem struct {
	ID             string                  `json:"id"`
	TakenAt        int64                   `json:"taken_at"`
	MediaType      int                     `json:"media_type"`
	ImageVersions2 *download.ImageVersions `json:"image_versions2"`
	VideoVersions  []download.VideoVersion `json:"video_versions"`
}

// parseReelsItems peels the wrapper off /feed/reels_media/?reel_ids=<id>.
// The endpoint returns {"reels":{"<id>":{"items":[...]}}}; older flavors
// return {"reels_media":[{"items":[...]}]}. Handle both.
func parseReelsItems(raw json.RawMessage, ownerID string) []reelItem {
	var v struct {
		Reels map[string]struct {
			Items []reelItem `json:"items"`
		} `json:"reels"`
		ReelsMedia []struct {
			Items []reelItem `json:"items"`
		} `json:"reels_media"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	if r, ok := v.Reels[ownerID]; ok {
		return r.Items
	}
	for _, r := range v.ReelsMedia {
		return r.Items
	}
	return nil
}

func pickStoryURL(item reelItem) (string, string) {
	if len(item.VideoVersions) > 0 {
		// Highest-bandwidth.
		best := item.VideoVersions[0]
		for _, vv := range item.VideoVersions {
			if vv.Bandwidth > best.Bandwidth {
				best = vv
			}
		}
		return best.URL, "video"
	}
	if item.ImageVersions2 != nil && len(item.ImageVersions2.Candidates) > 0 {
		best := item.ImageVersions2.Candidates[0]
		for _, ic := range item.ImageVersions2.Candidates {
			if ic.Width > best.Width {
				best = ic
			}
		}
		return best.URL, "image"
	}
	return "", ""
}

// simpleFetchToFile is a lightweight cousin of download.Downloader.fetchToFile
// for endpoints that hand us a raw URL with no JSON structure to upsert.
// Streams to disk while sha-256-hashing. Takes context.Context but we accept
// any with a Done() to keep the signature flexible across callers.
func simpleFetchToFile(_ context.Context, hc *http.Client, url, localPath string) (int64, string, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.instagram.com/")
	resp, err := hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, "", fmt.Errorf("CDN HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return 0, "", err
	}
	f, err := os.Create(localPath)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)
	n, err := io.Copy(mw, resp.Body)
	if err != nil {
		os.Remove(localPath)
		return 0, "", err
	}
	return n, hex.EncodeToString(hasher.Sum(nil)), nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// downloadProfilePic resolves the user's HD profile picture URL and writes it
// to outDir/<username>_profile.jpg. Used by `user --profile-pic`.
func downloadProfilePic(cmd *cobra.Command, c *iClient, outDir, username string) error {
	raw, err := c.Get("/api/v1/users/web_profile_info/", map[string]string{"username": username})
	if err != nil {
		return err
	}
	var resp struct {
		Data struct {
			User struct {
				ProfilePicURLHD string `json:"profile_pic_url_hd"`
				ProfilePicURL   string `json:"profile_pic_url"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return err
	}
	url := resp.Data.User.ProfilePicURLHD
	if url == "" {
		url = resp.Data.User.ProfilePicURL
	}
	if url == "" {
		return fmt.Errorf("no profile pic URL in response")
	}
	_ = os.MkdirAll(outDir, 0o755)
	local := filepath.Join(outDir, username+"_profile.jpg")
	_, _, err = simpleFetchToFile(cmd.Context(), c.HTTPClient, url, local)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", local)
	return nil
}
