// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// newClipsHarvestCmd combines:
//
//	GET /api/v4/projects/{projectId}/ai-generation-status  (gate)
//	GET /api/v4/projects/{projectId}/clips/exports         (list)
//	GET /api/v4/clip/{clipId}                              (refresh signed URL)
//
// Optionally downloads each clip body when --download is set.
func newClipsHarvestCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var since string
	var download bool
	var outDir string
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:         "harvest",
		Short:       "Harvest Magic Clips: gate on AI-status, list exports, refresh signed URLs, optionally download MP4s.",
		Long:        "Combines the AI-generation-status gate + clips-exports list + per-clip URL refresh into a single command.",
		Example:     `  riverside-fm-pp-cli clips harvest --project 69fcda9fba030a19ae93a526 --download --out ./clips`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if projectID == "" {
				return usageErr(fmt.Errorf("--project is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Gate on AI generation status
			if wait {
				deadline := time.Now().Add(timeout)
				for {
					if time.Now().After(deadline) {
						return &cliError{code: 2, err: fmt.Errorf("AI generation did not finish before --timeout")}
					}
					data, err := c.Get("/api/v4/projects/"+url.PathEscape(projectID)+"/ai-generation-status", nil)
					if err == nil && aiAllDone(data) {
						break
					}
					fmt.Fprintln(cmd.OutOrStdout(), "Waiting for AI generation...")
					time.Sleep(15 * time.Second)
				}
			} else {
				statusData, err := c.Get("/api/v4/projects/"+url.PathEscape(projectID)+"/ai-generation-status", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if !aiAllDone(statusData) {
					if flags.asJSON {
						fmt.Fprintln(cmd.OutOrStdout(), `{"ready":false,"reason":"ai_generation_in_progress"}`)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "AI generation still in progress. Use --wait to block until ready.")
					}
					return nil
				}
			}

			// 2. List exports for the project
			expData, err := c.Get("/api/v4/projects/"+url.PathEscape(projectID)+"/clips/exports",
				map[string]string{"offset": "0", "limit": "500"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			clipIDs := extractClipIDsFromExports(expData)

			// 3. Refresh each clip + collect URL
			type harvestRow struct {
				ClipID    string  `json:"clip_id"`
				Title     string  `json:"title,omitempty"`
				URL       string  `json:"url"`
				DurationS float64 `json:"duration_seconds,omitempty"`
				LocalPath string  `json:"local_path,omitempty"`
				Bytes     int64   `json:"bytes,omitempty"`
				Status    string  `json:"status"`
			}
			rows := []harvestRow{}
			if download && outDir == "" {
				outDir = "./clips"
			}
			if download {
				if err := os.MkdirAll(outDir, 0o755); err != nil {
					return err
				}
			}

			_ = since // since-filter could be applied if exports list returns createdAt; payload was empty in browser-sniff so left for future enrichment.

			for _, cid := range clipIDs {
				clipData, cerr := c.Get("/api/v4/clip/"+url.PathEscape(cid), nil)
				if cerr != nil {
					rows = append(rows, harvestRow{ClipID: cid, Status: "fetch-failed"})
					continue
				}
				title := extractClipTitle(clipData)
				urls := extractClipExportURLs(clipData)
				if len(urls) == 0 {
					rows = append(rows, harvestRow{ClipID: cid, Title: title, Status: "no-export-url"})
					continue
				}
				// Pick the first concrete download URL
				row := harvestRow{ClipID: cid, Title: title, URL: urls[0].URL, Status: "ready"}
				if download {
					fp := filepath.Join(outDir, sanitize(title)+"-"+cid+guessExt(row.URL))
					n, derr := streamDownload(row.URL, fp)
					if derr != nil {
						row.Status = "download-failed: " + derr.Error()
					} else {
						row.LocalPath = fp
						row.Bytes = n
						row.Status = "downloaded"
					}
				}
				rows = append(rows, row)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(j))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No clips to harvest in this project.")
				return nil
			}
			for _, r := range rows {
				if r.LocalPath != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s | %s -> %s (%d bytes)\n", r.ClipID, r.Title, r.LocalPath, r.Bytes)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s | %s | %s | %s\n", r.ClipID, r.Title, r.Status, truncate(r.URL, 80))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&since, "since", "", "Only include clips created after this date or duration (reserved; not enforced in v0.1)")
	cmd.Flags().BoolVar(&download, "download", false, "Download each clip body to --out before signed-URL TTL expires")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory when --download (default ./clips)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until ai-generation-status reports done before harvesting")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Max wait when --wait")
	return cmd
}

func extractClipTitle(data json.RawMessage) string {
	var probe struct {
		Data struct {
			Take struct {
				Name string `json:"name"`
			} `json:"take"`
			Title string `json:"title"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return ""
	}
	if probe.Data.Title != "" {
		return probe.Data.Title
	}
	return probe.Data.Take.Name
}
