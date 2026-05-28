// Copyright 2026 Dave Fano and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Local Printing Press patch: export rendered Midjourney images through the
// logged-in Chrome session. Direct CDN fetches are Cloudflare/CORS protected.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type downloadFlags struct {
	index      int
	out        string
	browserCDP string
	wait       time.Duration
}

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var dl downloadFlags
	cmd := &cobra.Command{
		Use:   "download <job-id>",
		Short: "Download a rendered Midjourney image through Chrome CDP",
		Long: "Download a rendered Midjourney image through the logged-in Chrome CDP session.\n\n" +
			"Midjourney's CDN blocks plain server-side HTTP clients and disallows browser fetch() from www.midjourney.com via CORS. " +
			"This command opens the job page in Chrome, finds the rendered image element, and saves a cropped PNG screenshot.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := strings.TrimSpace(args[0])
			if jobID == "" {
				return usageErr(fmt.Errorf("job id is required"))
			}
			if dl.index < 0 || dl.index > 3 {
				return usageErr(fmt.Errorf("--index must be between 0 and 3"))
			}
			out := strings.TrimSpace(dl.out)
			if out == "" {
				out = fmt.Sprintf("%s-%d.png", jobID, dl.index)
			}
			if err := browserCaptureJobImage(cmd.Context(), dl.browserCDP, jobID, dl.index, dl.wait, out); err != nil {
				return classifyAPIError(err, flags)
			}
			data, err := json.Marshal(map[string]any{
				"job_id": jobID,
				"index":  dl.index,
				"path":   out,
				"method": "chrome_cdp_rendered_png",
			})
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&dl.index, "index", 0, "Image index to export from the job grid, 0-3")
	cmd.Flags().StringVarP(&dl.out, "out", "o", "", "Output PNG path")
	cmd.Flags().StringVar(&dl.browserCDP, "browser-cdp", "http://127.0.0.1:18800", "Chrome DevTools Protocol URL")
	cmd.Flags().DurationVar(&dl.wait, "wait", 5*time.Second, "Time to wait after opening the job page before capture")
	return cmd
}

type renderedImageRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Src    string
}

func browserCaptureJobImage(ctx context.Context, cdpBase, jobID string, index int, wait time.Duration, out string) error {
	if wait <= 0 {
		wait = 5 * time.Second
	}
	pageURL := fmt.Sprintf("https://www.midjourney.com/jobs/%s?index=%d", urlPathEscape(jobID), index)
	if err := browserNavigate(ctx, cdpBase, pageURL); err != nil {
		return err
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return ctx.Err()
	}
	rect, err := browserFindRenderedImage(ctx, cdpBase, jobID, index)
	if err != nil {
		return err
	}
	if rect.Width < 10 || rect.Height < 10 {
		return fmt.Errorf("rendered image for job %s index %d is too small to capture", jobID, index)
	}
	return browserCaptureClip(ctx, cdpBase, rect, out)
}

func browserNavigate(ctx context.Context, cdpBase, pageURL string) error {
	target, err := findMidjourneyCDPTarget(ctx, cdpBase)
	if err != nil {
		return err
	}
	_, err = cdpRoundTrip(ctx, target.WebSocketDebuggerURL, map[string]any{
		"id":     1,
		"method": "Page.navigate",
		"params": map[string]any{"url": pageURL},
	})
	return err
}

func browserFindRenderedImage(ctx context.Context, cdpBase, jobID string, index int) (renderedImageRect, error) {
	target, err := findMidjourneyCDPTarget(ctx, cdpBase)
	if err != nil {
		return renderedImageRect{}, err
	}
	needle := fmt.Sprintf("/%s/0_%d", jobID, index)
	expression := "(() => {" +
		"const needle = " + jsStringLiteral(needle) + ";" +
		"const visible = Array.from(document.images)" +
		".filter(img => (img.currentSrc || img.src || '').includes(needle))" +
		".map(img => { const r = img.getBoundingClientRect(); return {src: img.currentSrc || img.src || '', x: r.x, y: r.y, width: r.width, height: r.height}; })" +
		".filter(r => r.width > 20 && r.height > 20);" +
		"visible.sort((a, b) => (b.width * b.height) - (a.width * a.height));" +
		"if (!visible.length) return null;" +
		"return visible[0];" +
		"})()"
	response, err := cdpRoundTrip(ctx, target.WebSocketDebuggerURL, map[string]any{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expression,
			"awaitPromise":  true,
			"returnByValue": true,
		},
	})
	if err != nil {
		return renderedImageRect{}, err
	}
	var envelope struct {
		Result struct {
			Result struct {
				Subtype string
				Value   renderedImageRect
			}
		}
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return renderedImageRect{}, err
	}
	if envelope.Result.Result.Subtype == "null" || envelope.Result.Result.Value.Src == "" {
		return renderedImageRect{}, fmt.Errorf("rendered image for job %s index %d was not found in Chrome", jobID, index)
	}
	return envelope.Result.Result.Value, nil
}

func browserCaptureClip(ctx context.Context, cdpBase string, rect renderedImageRect, out string) error {
	target, err := findMidjourneyCDPTarget(ctx, cdpBase)
	if err != nil {
		return err
	}
	x := math.Max(0, math.Floor(rect.X))
	y := math.Max(0, math.Floor(rect.Y))
	width := math.Ceil(rect.Width)
	height := math.Ceil(rect.Height)
	response, err := cdpRoundTrip(ctx, target.WebSocketDebuggerURL, map[string]any{
		"id":     1,
		"method": "Page.captureScreenshot",
		"params": map[string]any{
			"format": "png",
			"clip": map[string]any{
				"x":      x,
				"y":      y,
				"width":  width,
				"height": height,
				"scale":  1,
			},
		},
	})
	if err != nil {
		return err
	}
	var envelope struct {
		Result struct {
			Data string
		}
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return err
	}
	if envelope.Result.Data == "" {
		return fmt.Errorf("Chrome returned an empty screenshot")
	}
	data, err := base64.StdEncoding.DecodeString(envelope.Result.Data)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(out); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return os.WriteFile(out, data, 0600)
}

func urlPathEscape(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "&", "%26")
}
