// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/powerbi/internal/config"

	"github.com/spf13/cobra"
)

type exportToFileBody struct {
	Format string `json:"format"`
}

type exportJob struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	PercentComplete  int    `json:"percentComplete"`
	ReportID         string `json:"reportId"`
	ReportName       string `json:"reportName"`
	ResourceLocation string `json:"resourceLocation"`
	ResourceFileExt  string `json:"resourceFileExtension"`
}

func newReportExportCmd(flags *rootFlags) *cobra.Command {
	var group, format, out string
	var wait bool
	var pollEvery time.Duration
	var timeoutD time.Duration
	cmd := &cobra.Command{
		Use:   "report-export <report-id>",
		Short: "Export a Power BI or paginated report to a file (POST → poll → download in one command)",
		Long: `One-shot wrapper around Power BI's three-step async export-to-file flow.

Without --wait this prints the export job ID and exits; you can poll later via
'reports export-status-in-group --exportId ID'. With --wait (default), the
command polls until the job is Succeeded, downloads the file, writes it to
--output, and exits cleanly.

Supported formats:
  Power BI reports:    PDF, PPTX, PNG
  Paginated reports:   PDF, XLSX, CSV, DOCX, IMAGE, MHTML, XML, ACCESSIBLEPDF`,
		Example: `  # Power BI report → PDF, with wait
  powerbi-pp-cli report-export 37ae3f5d-665b-4c6b-affe-37ebd176d9e5 \
    --group 804c5edc-6653-4149-8d08-a11279824b7a --format PDF --output report.pdf

  # Async — get the export ID, poll later
  powerbi-pp-cli report-export REPORT_ID --group W --format PNG --no-wait`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			reportID := args[0]
			if format == "" {
				format = "PDF"
			}
			if group == "" {
				return usageErr(fmt.Errorf("--group is required (export-to-file is not supported on My workspace via this command)"))
			}
			path := fmt.Sprintf("/groups/%s/reports/%s/ExportTo", group, reportID)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, status, err := c.Post(path, exportToFileBody{Format: format})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("ExportTo returned HTTP %d: %s", status, string(raw)))
			}
			var job exportJob
			if err := json.Unmarshal(raw, &job); err != nil {
				return apiErr(fmt.Errorf("decoding ExportTo response: %w", err))
			}
			if !wait {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), job, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Export job %s queued (status %s)\nPoll: powerbi-pp-cli reports export-status-in-group --groupId %s --reportId %s --exportId %s\n", job.ID, job.Status, group, reportID, job.ID)
				return nil
			}
			// Polling loop.
			pollPath := fmt.Sprintf("/groups/%s/reports/%s/exports/%s", group, reportID, job.ID)
			deadline := time.Now().Add(timeoutD)
			for time.Now().Before(deadline) {
				time.Sleep(pollEvery)
				st, err := c.Get(pollPath, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if err := json.Unmarshal(st, &job); err != nil {
					return apiErr(fmt.Errorf("decoding export status: %w", err))
				}
				switch strings.ToLower(job.Status) {
				case "succeeded":
					return downloadExport(cmd, flags, group, reportID, job, out)
				case "failed":
					return apiErr(fmt.Errorf("export job %s failed (status: %s)", job.ID, job.Status))
				}
			}
			return apiErr(fmt.Errorf("export job %s did not finish within --timeout (%s); last status: %s, %d%%", job.ID, timeoutD, job.Status, job.PercentComplete))
		},
	}
	cmd.Flags().StringVarP(&group, "group", "g", "", "Workspace (group) ID (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "PDF", "Export format: PDF, PPTX, PNG (Power BI); PDF, XLSX, CSV, DOCX, MHTML, XML, IMAGE, ACCESSIBLEPDF (paginated)")
	cmd.Flags().StringVarP(&out, "output", "o", "", "Path to write the exported file. Required when --wait is set.")
	cmd.Flags().BoolVar(&wait, "wait", true, "Wait for the job to finish and download the file (default true; pass --wait=false for async)")
	cmd.Flags().DurationVar(&pollEvery, "poll-interval", 5*time.Second, "How often to poll job status")
	cmd.Flags().DurationVar(&timeoutD, "timeout-wait", 10*time.Minute, "Maximum time to wait for the export job to succeed")
	return cmd
}

func downloadExport(cmd *cobra.Command, flags *rootFlags, group, reportID string, job exportJob, out string) error {
	if out == "" {
		ext := strings.TrimPrefix(job.ResourceFileExt, ".")
		if ext == "" {
			ext = "bin"
		}
		out = fmt.Sprintf("%s%s", strings.ReplaceAll(job.ReportName, " ", "_"), job.ResourceFileExt)
		if out == job.ResourceFileExt || out == "" {
			out = fmt.Sprintf("report-%s.%s", job.ID, ext)
		}
	}
	// Use a raw HTTP client so we can stream the binary response body directly
	// to disk instead of buffering the whole file in memory.
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	url := fmt.Sprintf("%s/groups/%s/reports/%s/exports/%s/file", cfg.BaseURL, group, reportID, job.ID)
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
	if err != nil {
		return apiErr(err)
	}
	if h := cfg.AuthHeader(); h != "" {
		req.Header.Set("Authorization", h)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return apiErr(fmt.Errorf("downloading export: %w", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return apiErr(fmt.Errorf("download returned HTTP %d: %s", resp.StatusCode, string(body)))
	}
	f, err := os.Create(out)
	if err != nil {
		return configErr(fmt.Errorf("creating output file: %w", err))
	}
	defer f.Close()
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return apiErr(fmt.Errorf("writing output file: %w", err))
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"export_id": job.ID,
			"output":    out,
			"bytes":     n,
			"format":    job.ResourceFileExt,
		}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d bytes to %s\n", n, out)
	return nil
}
