// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const reportingBaseURL = "https://youtubereporting.googleapis.com"

func newReportingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reporting",
		Short: "Async bulk reporting: create jobs, poll, download CSVs",
		Long: `The YouTube Reporting API is async: you create a Job for a reportType,
the system computes daily reports for that job, and you download them. This
command wraps create-poll-download into one idempotent invocation suitable
for daily cron / n8n.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReportingSyncCmd(flags))
	return cmd
}

func newReportingSyncCmd(flags *rootFlags) *cobra.Command {
	var typesCSV, outDir, since string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Ensure jobs exist for the requested report types, then download recent reports",
		Long: `Idempotent: lists existing jobs, creates missing ones, polls for completed
reports in the --since window, downloads CSVs to --out.

Use 'youtube-creator-pp-cli report-types' (generated) to list available report type
identifiers (e.g. channel_basic_a2, content_owner_a1).`,
		Example:     "  youtube-creator-pp-cli reporting sync --types channel_basic_a2,channel_combined_a2 --since 30d --out ./reports/",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if typesCSV == "" || outDir == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--types and --out are required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("creating out dir: %w", err)
			}
			wantedTypes := strings.Split(typesCSV, ",")
			for i := range wantedTypes {
				wantedTypes[i] = strings.TrimSpace(wantedTypes[i])
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1) List existing jobs
			quotaLogCost("reporting-jobs-list", 1)
			rawJobs, err := c.Get(reportingBaseURL+"/v1/jobs", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var jobsResp struct {
				Jobs []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					ReportType string `json:"reportTypeId"`
				} `json:"jobs"`
			}
			_ = json.Unmarshal(rawJobs, &jobsResp)
			existingByType := map[string]string{}
			for _, j := range jobsResp.Jobs {
				existingByType[j.ReportType] = j.ID
			}

			// 2) Create missing jobs
			created := []map[string]string{}
			for _, t := range wantedTypes {
				if _, ok := existingByType[t]; ok {
					continue
				}
				body := map[string]any{
					"reportTypeId": t,
					"name":         "pp-cli-" + t,
				}
				quotaLogCost("reporting-jobs-create", 1)
				rawCreated, _, err := c.Post(reportingBaseURL+"/v1/jobs", body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var nj struct {
					ID         string `json:"id"`
					ReportType string `json:"reportTypeId"`
				}
				_ = json.Unmarshal(rawCreated, &nj)
				existingByType[t] = nj.ID
				created = append(created, map[string]string{"type": t, "job_id": nj.ID})
			}

			// 3) For each job, list reports in --since window, download
			cutoff := time.Time{}
			if since != "" {
				if t, err := parseSince(since); err == nil {
					cutoff = t
				}
			}
			type downloaded struct {
				Type      string `json:"type"`
				JobID     string `json:"job_id"`
				ReportID  string `json:"report_id"`
				StartTime string `json:"start_time"`
				FilePath  string `json:"file_path"`
				Bytes     int64  `json:"bytes"`
			}
			var dls []downloaded

			for t, jobID := range existingByType {
				if !contains(wantedTypes, t) {
					continue
				}
				params := map[string]string{}
				if !cutoff.IsZero() {
					params["startTimeAtOrAfter"] = cutoff.UTC().Format(time.RFC3339)
				}
				quotaLogCost("reporting-jobs-reports-list", 1)
				rawReports, err := c.Get(reportingBaseURL+"/v1/jobs/"+jobID+"/reports", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var rep struct {
					Reports []struct {
						ID          string `json:"id"`
						StartTime   string `json:"startTime"`
						EndTime     string `json:"endTime"`
						DownloadURL string `json:"downloadUrl"`
					} `json:"reports"`
				}
				_ = json.Unmarshal(rawReports, &rep)
				for _, r := range rep.Reports {
					// Skip if already downloaded
					filename := fmt.Sprintf("%s_%s.csv", t, strings.ReplaceAll(strings.Split(r.StartTime, "T")[0], "-", ""))
					path := filepath.Join(outDir, filename)
					if _, err := os.Stat(path); err == nil {
						continue
					}
					// Download
					req, _ := http.NewRequest("GET", r.DownloadURL, nil)
					// Use http.DefaultClient since download URLs are signed
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						continue
					}
					f, err := os.Create(path)
					if err != nil {
						resp.Body.Close()
						return fmt.Errorf("creating file: %w", err)
					}
					n, _ := io.Copy(f, resp.Body)
					f.Close()
					resp.Body.Close()
					dls = append(dls, downloaded{
						Type:      t,
						JobID:     jobID,
						ReportID:  r.ID,
						StartTime: r.StartTime,
						FilePath:  path,
						Bytes:     n,
					})
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"types":         wantedTypes,
				"jobs_created":  created,
				"reports_saved": dls,
				"out_dir":       outDir,
			})
		},
	}
	cmd.Flags().StringVar(&typesCSV, "types", "", "Comma-separated reportType IDs (required)")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory (required)")
	cmd.Flags().StringVar(&since, "since", "30d", "Only fetch reports with startTime newer than this duration")
	return cmd
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
