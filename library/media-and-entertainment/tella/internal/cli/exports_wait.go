// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newExportsCmd is the parent for the `exports` family of bulk commands that
// don't fit naturally under `videos exports` (which is keyed on a single
// video).
func newExportsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exports",
		Short: "Bulk export operations across videos",
		RunE:  rejectUnknownSubcommand,
	}
	cmd.AddCommand(newExportsWaitCmd(flags))
	return cmd
}

// newExportsWaitCmd kicks off an export for one or more videos and waits up
// to --timeout for each to reach a terminal status by polling its GET endpoint.
func newExportsWaitCmd(flags *rootFlags) *cobra.Command {
	var videoIDs []string
	var timeout time.Duration
	var pollInterval time.Duration
	cmd := &cobra.Command{
		Use:     "wait",
		Short:   "Kick off exports and wait for them to reach a terminal status",
		Example: "  tella-pp-cli exports wait --video vid_abc --timeout 10m --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(videoIDs) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if timeout <= 0 || pollInterval <= 0 {
				return usageErr(fmt.Errorf("--timeout and --poll-interval must be positive"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Each poll must see fresh status rather than a cached queued/running response.
			c.NoCache = true
			type exportResult struct {
				VideoID  string   `json:"video_id"`
				ExportID string   `json:"export_id,omitempty"`
				Status   string   `json:"status"`
				Progress *float64 `json:"progress"`
				URL      string   `json:"url,omitempty"`
				ElapsedS int      `json:"elapsed_s"`
				Note     string   `json:"note,omitempty"`
			}
			results := make([]exportResult, 0, len(videoIDs))
			failed := false
			for _, vid := range videoIDs {
				start := time.Now()
				path := fmt.Sprintf("/v1/videos/%s/exports", url.PathEscape(vid))
				data, _, err := c.Post(cmd.Context(), path, map[string]any{})
				r := exportResult{VideoID: vid}
				if err != nil {
					r.Status = "error"
					r.Note = truncate(err.Error(), 200)
				} else {
					r.ExportID, r.Status, r.URL, r.Progress = parseExportResponse(data)
					if r.ExportID == "" && !isExportTerminal(r.Status) {
						r.Status = "error"
						r.Note = "export response is missing exportId"
					}
				}
				deadline := start.Add(timeout)
				for !isExportTerminal(r.Status) && time.Now().Before(deadline) {
					time.Sleep(min(pollInterval, time.Until(deadline)))
					if !time.Now().Before(deadline) {
						break
					}
					data, err := c.Get(cmd.Context(), path+"/"+url.PathEscape(r.ExportID), nil)
					if err != nil {
						r.Status = "error"
						r.Note = truncate(err.Error(), 200)
						break
					}
					_, r.Status, r.URL, r.Progress = parseExportResponse(data)
				}
				r.ElapsedS = int(time.Since(start).Seconds())
				if !isExportTerminal(r.Status) {
					r.Note = "timed out waiting for export to complete"
					failed = true
				} else if strings.EqualFold(r.Status, "failed") || strings.EqualFold(r.Status, "error") {
					failed = true
				}
				results = append(results, r)
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"exports": results,
			}, flags); err != nil {
				return err
			}
			if failed {
				return apiErr(fmt.Errorf("one or more exports failed or timed out"))
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&videoIDs, "video", nil, "Video ID to export (repeatable for batch)")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum time to wait per export")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "How often to poll for status")
	cmd.Flags().Int("inbox-limit", 200, "Deprecated; export status is polled directly")
	_ = cmd.Flags().MarkDeprecated("inbox-limit", "export status is polled directly; this flag is ignored")
	_ = cmd.Flags().MarkHidden("inbox-limit")
	return cmd
}

// parseExportResponse extracts (exportId, status, downloadUrl, progress) from a Tella
// export response, accepting common field naming variants.
func parseExportResponse(data json.RawMessage) (string, string, string, *float64) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", "pending", "", nil
	}
	if export, ok := obj["export"].(map[string]any); ok {
		obj = export
	}
	id := stringField(obj, "exportId", "export_id", "id")
	status := stringField(obj, "status", "state")
	if status == "" {
		status = "pending"
	}
	url := stringField(obj, "url", "downloadUrl", "download_url")
	if progress, ok := obj["progress"].(float64); ok {
		return id, status, url, &progress
	}
	return id, status, url, nil
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func isExportTerminal(status string) bool {
	s := strings.ToLower(status)
	return s == "ready" || s == "completed" || s == "done" || s == "failed" || s == "error"
}
