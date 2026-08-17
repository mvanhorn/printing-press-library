// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelSttJobRetryCmd(flags *rootFlags) *cobra.Command {
	var flagFailedOnly bool
	var flagDir string

	cmd := &cobra.Command{
		Use:         "retry [job_id]",
		Short:       "Re-run only the failed files of a batch STT job with one command",
		Example:     "  sarvam-pp-cli stt-job retry 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --failed-only --dir ./audio/",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "job=20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5;--dir=./audio/", "pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stt-job retry")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: job_id"))
			}
			jobID := args[0]
			if flagDir == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dir is required (local directory containing the audio files to re-upload)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Fetch the old job status to find failed file names.
			data, err := c.GetNoCache(ctx, "/speech-to-text/job/v1/"+jobID+"/status", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var status struct {
				JobState string `json:"job_state"`
				JobDetails []struct {
					FileName string `json:"file_name"`
					State    string `json:"state"`
				} `json:"job_details"`
			}
			if err := json.Unmarshal(data, &status); err != nil {
				return apiErr(fmt.Errorf("parsing job status: %w", err))
			}
			var filesToRetry []string
			for _, d := range status.JobDetails {
				if !flagFailedOnly || d.State == "API Error" || d.State == "Internal Server Error" {
					filesToRetry = append(filesToRetry, d.FileName)
				}
			}
			// Fall back to all files when no job_details are available.
			if len(filesToRetry) == 0 && !flagFailedOnly {
				if entries, err := os.ReadDir(flagDir); err == nil {
					for _, e := range entries {
						if !e.IsDir() {
							filesToRetry = append(filesToRetry, e.Name())
						}
					}
				}
			}
			if len(filesToRetry) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no failed files to retry")
				return nil
			}

			// 2. Initiate a new job with the same parameters.
			var jobParams map[string]any
			var oldParams struct {
				JobParameters map[string]any `json:"job_parameters"`
			}
			_ = json.Unmarshal(data, &oldParams)
			jobParams = oldParams.JobParameters
			if jobParams == nil {
				jobParams = map[string]any{}
			}
			initData, statusCode, err := c.Post(ctx, "/speech-to-text/job/v1", map[string]any{
				"job_parameters": jobParams,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if statusCode != http.StatusAccepted && statusCode != http.StatusOK {
				return apiErr(fmt.Errorf("initiating retry job: HTTP %d", statusCode))
			}
			var initResp struct {
				JobID string `json:"job_id"`
			}
			if err := json.Unmarshal(initData, &initResp); err != nil || initResp.JobID == "" {
				return apiErr(fmt.Errorf("parsing job initiation response"))
			}
			newJobID := initResp.JobID

			// 3. Get presigned upload URLs for the failed files.
			uploadData, _, err := c.Post(ctx, "/speech-to-text/job/v1/upload-files", map[string]any{
				"job_id": newJobID,
				"files":  filesToRetry,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var uploadResp struct {
				UploadURLs map[string]struct {
					FileURL string `json:"file_url"`
				} `json:"upload_urls"`
			}
			if err := json.Unmarshal(uploadData, &uploadResp); err != nil {
				return apiErr(fmt.Errorf("parsing upload URLs: %w", err))
			}

			// 4. Upload each failed file to its presigned URL.
			for _, fname := range filesToRetry {
				info, ok := uploadResp.UploadURLs[fname]
				if !ok || info.FileURL == "" {
					return apiErr(fmt.Errorf("no upload URL for %q", fname))
				}
				localPath := filepath.Join(flagDir, fname)
				// #nosec G304 -- localPath derives from the job's reported file names joined onto the user-supplied --dir; the user owns these files.
				fileBytes, err := os.ReadFile(localPath)
				if err != nil {
					return apiErr(fmt.Errorf("reading %s: %w", localPath, err))
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, info.FileURL, strings.NewReader(string(fileBytes)))
				if err != nil {
					return fmt.Errorf("building upload request for %s: %w", fname, err)
				}
				req.Header.Set("Content-Type", "application/octet-stream")
				uploadClient := c.HTTPClient
				if uploadClient == nil {
					uploadClient = http.DefaultClient
				}
				resp, err := uploadClient.Do(req)
				if err != nil {
					return fmt.Errorf("uploading %s: %w", fname, err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode >= 400 {
					return apiErr(fmt.Errorf("uploading %s: HTTP %d", fname, resp.StatusCode))
				}
			}

			// 5. Start the new job.
			startData, _, err := c.Post(ctx, "/speech-to-text/job/v1/"+newJobID+"/start", map[string]any{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_ = startData
			result := map[string]any{
				"old_job_id":   jobID,
				"new_job_id":   newJobID,
				"retried_files": filesToRetry,
				"status":       "started",
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "retry job %s created from %s with %d file(s)\n", newJobID, jobID, len(filesToRetry))
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagFailedOnly, "failed-only", true, "Retry only files that failed in the original job")
	cmd.Flags().StringVar(&flagDir, "dir", "", "Local directory containing the audio files to re-upload")
	return cmd
}
