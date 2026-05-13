// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
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
// to --timeout for each to reach a terminal status. The Tella exports
// endpoint returns the export status synchronously on POST; without a
// dedicated GET-status endpoint, this command issues the POST and polls the
// webhook inbox for an `Export ready` event.
func newExportsWaitCmd(flags *rootFlags) *cobra.Command {
	var videoIDs []string
	var timeout time.Duration
	var pollInterval time.Duration
	var inboxLimit int
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
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type exportResult struct {
				VideoID  string `json:"video_id"`
				ExportID string `json:"export_id,omitempty"`
				Status   string `json:"status"`
				URL      string `json:"url,omitempty"`
				ElapsedS int    `json:"elapsed_s"`
				Note     string `json:"note,omitempty"`
			}
			results := make([]exportResult, 0, len(videoIDs))
			for _, vid := range videoIDs {
				start := time.Now()
				data, _, err := c.Post(fmt.Sprintf("/v1/videos/%s/exports", vid), map[string]any{})
				r := exportResult{VideoID: vid}
				if err != nil {
					r.Status = "error"
					r.Note = truncate(err.Error(), 200)
					results = append(results, r)
					continue
				}
				r.ExportID, r.Status, r.URL = parseExportResponse(data)
				if isExportTerminal(r.Status) {
					r.ElapsedS = int(time.Since(start).Seconds())
					results = append(results, r)
					continue
				}
				deadline := start.Add(timeout)
				for time.Now().Before(deadline) {
					time.Sleep(pollInterval)
					if r.ExportID != "" {
						if status, url := lookupExportInWebhooks(c, r.ExportID, inboxLimit); status != "" {
							r.Status = status
							r.URL = url
							if isExportTerminal(status) {
								break
							}
						}
					}
				}
				r.ElapsedS = int(time.Since(start).Seconds())
				if !isExportTerminal(r.Status) {
					r.Note = "still processing; poll later or watch the webhook inbox"
				}
				results = append(results, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"exports": results,
			}, flags)
		},
	}
	cmd.Flags().StringSliceVar(&videoIDs, "video", nil, "Video ID to export (repeatable for batch)")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum time to wait per export")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "How often to poll for status")
	cmd.Flags().IntVar(&inboxLimit, "inbox-limit", 200, "Max webhook messages to scan per poll when looking for the Export ready event. Increase if your workspace generates many other events between the export trigger and completion")
	return cmd
}

// parseExportResponse extracts (exportId, status, downloadUrl) from a Tella
// export response, accepting common field naming variants.
func parseExportResponse(data json.RawMessage) (string, string, string) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", "pending", ""
	}
	id := stringField(obj, "exportId", "export_id", "id")
	status := stringField(obj, "status", "state")
	if status == "" {
		status = "pending"
	}
	url := stringField(obj, "url", "downloadUrl", "download_url")
	return id, status, url
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

// lookupExportInWebhooks scans the recent webhook inbox for an `Export ready`
// (or similarly-named) event matching exportID and returns (status, url) when
// found. Empty status means no matching event yet.
//
// inboxLimit caps how many recent messages are scanned per poll. The Tella
// webhook inbox is a chronological stream; a small cap can miss the
// `Export ready` event in active workspaces where many unrelated events
// arrive between the trigger and any subsequent poll. Caller supplies the
// limit so the user can tune it via --inbox-limit. Floor of 50 keeps a
// nonsense value (e.g. 0) from producing zero-scan polls.
func lookupExportInWebhooks(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, exportID string, inboxLimit int) (string, string) {
	if inboxLimit < 50 {
		inboxLimit = 50
	}
	data, err := c.Get("/v1/webhooks/messages", map[string]string{"limit": fmt.Sprintf("%d", inboxLimit)})
	if err != nil {
		return "", ""
	}
	for _, m := range extractMessageObjects(data) {
		isExport := false
		for _, k := range []string{"eventType", "event_type", "type", "event"} {
			if v, ok := m[k].(string); ok && strings.Contains(strings.ToLower(v), "export") {
				isExport = true
				break
			}
		}
		if !isExport {
			continue
		}
		candidates := []map[string]any{m}
		for _, k := range []string{"data", "payload"} {
			if n, ok := m[k].(map[string]any); ok {
				candidates = append(candidates, n)
			}
		}
		for _, cnd := range candidates {
			id := stringField(cnd, "exportId", "export_id", "id")
			if id != "" && id == exportID {
				status := stringField(cnd, "status", "state")
				if status == "" {
					status = "ready"
				}
				url := stringField(cnd, "url", "downloadUrl", "download_url")
				return status, url
			}
		}
	}
	return "", ""
}
