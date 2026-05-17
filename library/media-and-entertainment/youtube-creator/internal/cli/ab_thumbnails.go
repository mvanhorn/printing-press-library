// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator/internal/config"
)

type abTest struct {
	VideoID     string       `json:"video_id"`
	StartedAt   time.Time    `json:"started_at"`
	RotateHours int          `json:"rotate_hours"`
	Variants    []abVariant  `json:"variants"`
	History     []abRotation `json:"history"`
}

type abVariant struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

type abRotation struct {
	Variant   string    `json:"variant_label"`
	AppliedAt time.Time `json:"applied_at"`
}

func abStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "youtube-pp-cli", "ab-thumbnails.json")
}

func abLoad() (map[string]*abTest, error) {
	data, err := os.ReadFile(abStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*abTest{}, nil
		}
		return nil, err
	}
	tests := map[string]*abTest{}
	if err := json.Unmarshal(data, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func abSave(tests map[string]*abTest) error {
	if err := os.MkdirAll(filepath.Dir(abStatePath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tests, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(abStatePath(), data, 0o600)
}

func newAbCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ab",
		Short: "DIY A/B testing (currently: thumbnails)",
		Long: `A/B test thumbnails by rotating variants via thumbnails.set on a schedule
and computing significance from CTR pulled from the Analytics API.

This is the only API-only A/B path; YouTube's native Test & Compare verdicts
are not exposed in the API.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAbThumbnailsCmd(flags))
	return cmd
}

func newAbThumbnailsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "thumbnails",
		Short:       "Manage thumbnail A/B tests",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAbThumbnailsStartCmd(flags))
	cmd.AddCommand(newAbThumbnailsRotateCmd(flags))
	cmd.AddCommand(newAbThumbnailsReportCmd(flags))
	cmd.AddCommand(newAbThumbnailsListCmd(flags))
	cmd.AddCommand(newAbThumbnailsStopCmd(flags))
	return cmd
}

func newAbThumbnailsStartCmd(flags *rootFlags) *cobra.Command {
	var video, variants string
	var rotateHours int
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start an A/B test on a video's thumbnail",
		Example: "  youtube-pp-cli ab thumbnails start --video dQw4w9WgXcQ \\\n" +
			"    --variants A:./thumb_a.png,B:./thumb_b.png --rotate 24",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if video == "" || variants == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--video and --variants are required (format: label:path,label:path)"))
			}
			var parsed []abVariant
			for _, v := range strings.Split(variants, ",") {
				parts := strings.SplitN(strings.TrimSpace(v), ":", 2)
				if len(parts) != 2 {
					return usageErr(fmt.Errorf("variant %q must be label:path", v))
				}
				label := strings.TrimSpace(parts[0])
				path := strings.TrimSpace(parts[1])
				if _, err := os.Stat(path); err != nil {
					return configErr(fmt.Errorf("variant %s: file not found: %s", label, path))
				}
				parsed = append(parsed, abVariant{Label: label, Path: path})
			}
			if len(parsed) < 2 {
				return usageErr(fmt.Errorf("at least 2 variants required"))
			}
			tests, err := abLoad()
			if err != nil {
				return err
			}
			tests[video] = &abTest{
				VideoID:     video,
				StartedAt:   time.Now(),
				RotateHours: rotateHours,
				Variants:    parsed,
			}
			if err := abSave(tests); err != nil {
				return err
			}
			// Apply first variant
			if !flags.dryRun {
				if err := uploadThumbnail(flags, video, parsed[0].Path); err != nil {
					return classifyAPIError(err, flags)
				}
				tests[video].History = append(tests[video].History, abRotation{
					Variant:   parsed[0].Label,
					AppliedAt: time.Now(),
				})
				_ = abSave(tests)
			}
			return flags.printJSON(cmd, map[string]any{
				"started":      video,
				"variants":     parsed,
				"rotate_hours": rotateHours,
				"next_step":    "Schedule `youtube-pp-cli ab thumbnails rotate " + video + "` every " + fmt.Sprintf("%d", rotateHours) + "h via cron/n8n",
			})
		},
	}
	cmd.Flags().StringVar(&video, "video", "", "Video ID (required)")
	cmd.Flags().StringVar(&variants, "variants", "", "Comma-separated label:path pairs (required)")
	cmd.Flags().IntVar(&rotateHours, "rotate", 24, "Rotation interval in hours (informational; you trigger rotate)")
	return cmd
}

func newAbThumbnailsRotateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rotate [video-id]",
		Short:       "Rotate to the next variant",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			videoID := args[0]
			tests, err := abLoad()
			if err != nil {
				return err
			}
			test, ok := tests[videoID]
			if !ok {
				return notFoundErr(fmt.Errorf("no active test for video %s", videoID))
			}
			// Find current variant (last in history) and pick next
			currentIdx := 0
			if len(test.History) > 0 {
				last := test.History[len(test.History)-1].Variant
				for i, v := range test.Variants {
					if v.Label == last {
						currentIdx = i
						break
					}
				}
			}
			nextIdx := (currentIdx + 1) % len(test.Variants)
			next := test.Variants[nextIdx]
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"would_apply": next.Label,
					"variant":     next,
				})
			}
			if err := uploadThumbnail(flags, videoID, next.Path); err != nil {
				return classifyAPIError(err, flags)
			}
			test.History = append(test.History, abRotation{Variant: next.Label, AppliedAt: time.Now()})
			tests[videoID] = test
			_ = abSave(tests)
			return flags.printJSON(cmd, map[string]any{
				"rotated_to": next.Label,
				"applied_at": time.Now(),
			})
		},
	}
	return cmd
}

func newAbThumbnailsReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report [video-id]",
		Short: "Report CTR per variant + significance test",
		Long: `Pulls daily impressions and CTR from the Analytics API, joins with the
rotation history, computes per-variant CTR and a two-proportion z-test for
significance between the top two variants.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			videoID := args[0]
			tests, err := abLoad()
			if err != nil {
				return err
			}
			test, ok := tests[videoID]
			if !ok {
				return notFoundErr(fmt.Errorf("no active test for video %s", videoID))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			startDate := test.StartedAt.Format("2006-01-02")
			endDate := time.Now().Format("2006-01-02")
			// Query daily impressions + CTR
			quotaLogCost("analytics-query", 1)
			raw, err := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "impressions,impressionClickThroughRate",
				"dimensions": "day",
				"filters":    "video==" + videoID,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var report struct {
				Rows [][]any `json:"rows"`
			}
			_ = json.Unmarshal(raw, &report)

			// Map each row's date to the variant that was active that day
			type bucket struct {
				Label       string  `json:"variant"`
				DaysActive  int     `json:"days_active"`
				Impressions float64 `json:"impressions"`
				ClickRate   float64 `json:"avg_ctr"`
			}
			buckets := map[string]*bucket{}
			for _, v := range test.Variants {
				buckets[v.Label] = &bucket{Label: v.Label}
			}
			for _, row := range report.Rows {
				if len(row) < 3 {
					continue
				}
				dayStr, _ := row[0].(string)
				impressions, _ := row[1].(float64)
				ctr, _ := row[2].(float64)
				day, err := time.Parse("2006-01-02", dayStr)
				if err != nil {
					continue
				}
				label := activeVariantOn(test, day)
				if b, ok := buckets[label]; ok {
					b.DaysActive++
					b.Impressions += impressions
					b.ClickRate += ctr
				}
			}
			var rows []bucket
			for _, b := range buckets {
				if b.DaysActive > 0 {
					b.ClickRate /= float64(b.DaysActive)
				}
				rows = append(rows, *b)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].ClickRate > rows[j].ClickRate })

			// Z-test between top two
			zReport := map[string]any{}
			if len(rows) >= 2 {
				p1 := rows[0].ClickRate / 100.0
				p2 := rows[1].ClickRate / 100.0
				n1 := rows[0].Impressions
				n2 := rows[1].Impressions
				if n1 > 0 && n2 > 0 {
					p := (p1*n1 + p2*n2) / (n1 + n2)
					se := math.Sqrt(p * (1 - p) * (1/n1 + 1/n2))
					var z float64
					if se > 0 {
						z = (p1 - p2) / se
					}
					zReport["winner"] = rows[0].Label
					zReport["loser"] = rows[1].Label
					zReport["z_score"] = z
					zReport["significant_p<0.05"] = math.Abs(z) > 1.96
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"video_id": videoID,
				"start":    startDate,
				"end":      endDate,
				"variants": rows,
				"z_test":   zReport,
			})
		},
	}
	return cmd
}

func activeVariantOn(test *abTest, day time.Time) string {
	dayEnd := day.Add(24 * time.Hour)
	var active string
	for _, r := range test.History {
		if r.AppliedAt.Before(dayEnd) {
			active = r.Variant
		} else {
			break
		}
	}
	return active
}

func newAbThumbnailsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List active thumbnail A/B tests",
		Example:     "  youtube-pp-cli ab thumbnails list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			tests, err := abLoad()
			if err != nil {
				return err
			}
			var out []*abTest
			for _, t := range tests {
				out = append(out, t)
			}
			return flags.printJSON(cmd, map[string]any{"tests": out, "count": len(out)})
		},
	}
	return cmd
}

func newAbThumbnailsStopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stop [video-id]",
		Short:       "Stop an A/B test (does not change the currently applied thumbnail)",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			tests, err := abLoad()
			if err != nil {
				return err
			}
			delete(tests, args[0])
			if err := abSave(tests); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"stopped": args[0]})
		},
	}
	return cmd
}

// uploadThumbnail uses thumbnails.set with multipart upload. Returns nil on success.
func uploadThumbnail(flags *rootFlags, videoID, thumbPath string) error {
	// We bypass the generated client's request body path here: client.do()
	// unconditionally json.Marshals its body argument, which would base64-
	// encode the binary multipart frame and corrupt the upload.
	//
	// We still need client.do()'s refresh-if-expired logic — so we make one
	// trivial GET through the client first (i18nLanguages.list, 1 quota
	// unit). That call triggers the in-flight token refresh and persists a
	// fresh access_token into Config + config.toml. We then construct the
	// upload request manually using that freshly-validated token.
	c, err := flags.newClient()
	if err != nil {
		return fmt.Errorf("building client: %w", err)
	}
	if _, gerr := c.Get("/youtube/v3/i18nLanguages", map[string]string{
		"part":       "snippet",
		"hl":         "en",
		"maxResults": "1",
	}); gerr != nil {
		// Surface auth/network failures from the refresh probe so we don't
		// silently send a stale Bearer header below.
		return classifyAPIError(gerr, flags)
	}
	// c.Config now holds the freshest access token (refresh fired inside
	// the client's do() if it was needed). Reload from config to also pick
	// up any side-effect updates a parallel CLI process may have written.
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return fmt.Errorf("loading auth config: %w", err)
	}
	// Prefer the in-memory client config (most current); fall back to
	// reloaded cfg only when client config is empty.
	accessToken := c.Config.AccessToken
	if accessToken == "" {
		accessToken = cfg.AccessToken
	}
	if accessToken == "" {
		return fmt.Errorf("no access token available; run `youtube-pp-cli auth login` first")
	}

	data, err := os.ReadFile(thumbPath)
	if err != nil {
		return fmt.Errorf("reading thumbnail: %w", err)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, perr := w.CreateFormFile("media", filepath.Base(thumbPath))
	if perr != nil {
		return fmt.Errorf("multipart writer: %w", perr)
	}
	if _, werr := part.Write(data); werr != nil {
		return fmt.Errorf("writing multipart body: %w", werr)
	}
	if cerr := w.Close(); cerr != nil {
		return fmt.Errorf("closing multipart writer: %w", cerr)
	}

	uploadURL := "https://www.googleapis.com/upload/youtube/v3/thumbnails/set?videoId=" + videoID + "&uploadType=multipart"
	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		return fmt.Errorf("building upload request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+accessToken)
	quotaLogCost("thumbnails-set", 50)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("uploading thumbnail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("thumbnails.set returned %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return nil
}
