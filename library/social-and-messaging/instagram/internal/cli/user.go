// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli user <handle>` — bulk profile archive. Resolves the
// handle to a user_id via accounts/web_profile_info, then walks
// /api/v1/feed/user/<user_id>/ and downloads each post.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/download"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// resolveUserID looks up a username via the public web_profile_info endpoint
// and returns the IG numeric user_id. Centralized here because user, stories,
// and highlights all need the same resolve.
func resolveUserID(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, handle string) (string, string, error) {
	raw, err := c.Get("/api/v1/users/web_profile_info/", map[string]string{"username": strings.TrimPrefix(handle, "@")})
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Data struct {
			User struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Pk       string `json:"pk"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", "", fmt.Errorf("parsing web_profile_info: %w", err)
	}
	id := resp.Data.User.ID
	if id == "" {
		id = resp.Data.User.Pk
	}
	if id == "" {
		return "", "", fmt.Errorf("could not resolve user_id for %q", handle)
	}
	return id, resp.Data.User.Username, nil
}

func newUserCmd(flags *rootFlags) *cobra.Command {
	var (
		include        []string
		limit          int
		since          string
		until          string
		fastUpdate     bool
		noResume       bool
		infoOnly       bool
		profilePicOnly bool
		out            string
		dryRun         bool
		dbPath         string
		captions       bool
		captionsFormat string
		geotags        bool
		comments       bool
		slide          string
		slideKind      string
		maxBytes       int64
		rate           int
	)

	cmd := &cobra.Command{
		Use:   "user [handle]",
		Short: "Archive a profile's posts (and optionally reels, IGTV, profile pic).",
		Long: `Walk a profile's feed cursor and download each post into --out.

The default --include is "posts"; pass a comma list to add more. "reels" /
"igtv" / "tagged" fall through to the same feed walker today since IG
returns them mixed into the user feed.

Use --dry-run to estimate request count, wall-clock under --rate-limit, and
total bytes without dialing the CDN.`,
		Example: `  instagram-pp-cli user nasa
  instagram-pp-cli user nasa --include posts,profile-pic
  instagram-pp-cli user nasa --since 7d --fast-update
  instagram-pp-cli user nasa --dry-run --json`,
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
				return runUserDryRun(cmd, flags, handle, limit, out, rate)
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would archive %s (verify env)\n", handle)
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
				out = "./" + username
			}
			absOut, _ := filepath.Abs(out)

			d := &download.Downloader{
				OutputDir:  absOut,
				HTTPClient: c.HTTPClient,
				DB:         st.DB(),
			}

			if includesItem(include, "profile-pic") || profilePicOnly {
				if err := downloadProfilePic(cmd, c, absOut, username); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: profile pic download: %v\n", err)
				}
				if profilePicOnly {
					return nil
				}
			}
			if infoOnly {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"profile":  handle,
						"user_id":  userID,
						"username": username,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", username, userID)
				return nil
			}

			var sinceTS, untilTS int64
			if since != "" {
				if t, err := parseSinceDuration(since); err == nil {
					sinceTS = t.Unix()
				}
			}
			if until != "" {
				if t, err := parseSinceDuration(until); err == nil {
					untilTS = t.Unix()
				}
			}

			downloaded := 0
			skipped := 0
			cursor := ""
			pages := 0

			for {
				if !noResume {
					// Note: a richer resume would key on user_id; for now the
					// fast-update path stops on a known shortcode, which is
					// what users actually want for daily syncs.
				}
				params := map[string]string{}
				if cursor != "" {
					params["max_id"] = cursor
				}
				raw, err := c.Get(fmt.Sprintf("/api/v1/feed/user/%s/", userID), params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Items         []json.RawMessage `json:"items"`
					MoreAvailable bool              `json:"more_available"`
					NextMaxID     string            `json:"next_max_id"`
				}
				if err := json.Unmarshal(raw, &page); err != nil {
					return fmt.Errorf("parsing feed page: %w", err)
				}
				pages++
				stop := false
				for _, item := range page.Items {
					var meta struct {
						Code    string `json:"code"`
						TakenAt int64  `json:"taken_at"`
					}
					_ = json.Unmarshal(item, &meta)
					if meta.Code == "" {
						continue
					}
					if sinceTS > 0 && meta.TakenAt > 0 && meta.TakenAt < sinceTS {
						stop = true
						break
					}
					if untilTS > 0 && meta.TakenAt > 0 && meta.TakenAt > untilTS {
						continue
					}
					if fastUpdate && hasDownloadHistory(st.DB(), meta.Code) {
						stop = true
						break
					}
					_, err := d.DownloadPost(cmd.Context(), c, download.DownloadOptions{
						Shortcode:       meta.Code,
						SlideKind:       slideKind,
						IncludeCaptions: captions,
						CaptionsFormat:  captionsFormat,
						IncludeGeotag:   geotags,
						IncludeComments: comments,
						Slide:           slide,
						MaxBytes:        maxBytes,
					})
					if err != nil {
						skipped++
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", meta.Code, err)
						continue
					}
					downloaded++
					if limit > 0 && downloaded >= limit {
						stop = true
						break
					}
				}
				if stop || !page.MoreAvailable || page.NextMaxID == "" {
					break
				}
				cursor = page.NextMaxID
			}

			summary := map[string]any{
				"profile":    username,
				"downloaded": downloaded,
				"skipped":    skipped,
				"pages":      pages,
				"out":        absOut,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d posts downloaded, %d skipped, %d pages walked\n",
				username, downloaded, skipped, pages)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&include, "include", []string{"posts"}, "What to fetch: posts,reels,igtv,tagged,stories,highlights,profile-pic")
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after N posts (0 = no limit)")
	cmd.Flags().StringVar(&since, "since", "", "Skip posts older than this (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&until, "until", "", "Skip posts newer than this")
	cmd.Flags().BoolVar(&fastUpdate, "fast-update", true, "Stop the walk on first known shortcode (default: true)")
	cmd.Flags().BoolVar(&noResume, "no-resume", false, "Force walk from top, ignore last-known cursor")
	cmd.Flags().BoolVar(&infoOnly, "info-only", false, "Just sync the profile row, no downloads")
	cmd.Flags().BoolVar(&profilePicOnly, "profile-pic-only", false, "Download only the profile picture")
	cmd.Flags().StringVar(&out, "out", "", "Output directory (default: ./<handle>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate counts and bytes without downloading")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	cmd.Flags().BoolVar(&captions, "captions", false, "Write per-post caption files")
	cmd.Flags().StringVar(&captionsFormat, "captions-format", "txt", "txt or json")
	cmd.Flags().BoolVar(&geotags, "geotags", false, "Write per-post .location.json")
	cmd.Flags().BoolVar(&comments, "comments", false, "Write per-post .comments.json")
	cmd.Flags().StringVar(&slide, "slide", "", "Carousel slide range, e.g. 1-3")
	cmd.Flags().StringVar(&slideKind, "slide-kind", "", "Filter children: image | video")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", 0, "Skip files larger than this many bytes")
	cmd.Flags().IntVar(&rate, "rate", 0, "Requests per minute for dry-run wall-clock estimate (0 = unlimited; 30 is a sensible cap)")

	return cmd
}

// runUserDryRun walks the cursor without downloading and emits a summary
// envelope shaped for agents. rate is requests-per-minute; when > 0 the
// envelope carries wall_clock_seconds = ceil(requests / (rate/60)).
func runUserDryRun(cmd *cobra.Command, flags *rootFlags, handle string, limit int, out string, rate int) error {
	c, err := flags.newClient()
	if err == nil {
		// Best-effort dry probe: hit web_profile_info to count posts and
		// estimate bytes. If the network is unreachable (or PRINTING_PRESS_VERIFY
		// is set), fall through to a static envelope.
		if !cliutil.IsVerifyEnv() {
			userID, _, rerr := resolveUserID(c, handle)
			if rerr == nil {
				params := map[string]string{}
				raw, gerr := c.Get(fmt.Sprintf("/api/v1/feed/user/%s/", userID), params)
				if gerr == nil {
					var page struct {
						Items []json.RawMessage `json:"items"`
					}
					_ = json.Unmarshal(raw, &page)
					posts := len(page.Items)
					bytes := int64(posts) * 500 * 1024 // 500KB heuristic
					requests := posts + 1
					summary := map[string]any{
						"profile":  handle,
						"posts":    posts,
						"bytes":    bytes,
						"requests": requests,
					}
					if rate > 0 {
						// rate is req/min. seconds = ceil(requests / (rate/60))
						seconds := int(math.Ceil(float64(requests) / (float64(rate) / 60.0)))
						summary["wall_clock_seconds"] = seconds
					}
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
					}
					if rate > 0 {
						seconds := summary["wall_clock_seconds"].(int)
						fmt.Fprintf(cmd.OutOrStdout(),
							"would fetch %d posts (%d bytes, ~%ds at %d req/min)\n",
							posts, bytes, seconds, rate)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(),
							"would fetch %d posts (%d bytes, no rate cap)\n",
							posts, bytes)
					}
					return nil
				}
			}
		}
	}
	// Verify-env / offline fallback: just print intent without dialing out.
	envelope := map[string]any{
		"profile":  handle,
		"posts":    0,
		"bytes":    0,
		"requests": 0,
		"limit":    limit,
		"out":      out,
		"note":     "dry-run without network probe",
	}
	if rate > 0 {
		envelope["wall_clock_seconds"] = 0
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "would archive %s (dry-run, no network probe)\n", handle)
	return nil
}

func includesItem(list []string, item string) bool {
	for _, s := range list {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// hasDownloadHistory reports whether the local catalog already has a record
// for shortcode. Used by --fast-update to stop the cursor walk on first hit.
func hasDownloadHistory(db *sql.DB, shortcode string) bool {
	if db == nil {
		return false
	}
	var n int
	err := db.QueryRow(`SELECT 1 FROM download_history WHERE shortcode = ? LIMIT 1`, shortcode).Scan(&n)
	return err == nil
}
