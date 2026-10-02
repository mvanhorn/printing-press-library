// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
			escapedJobID, err := sttJobPathSegment(jobID)
			if err != nil {
				return usageErr(fmt.Errorf("invalid job_id: %w", err))
			}
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
			data, err := c.GetNoCache(ctx, "/speech-to-text/job/v1/"+escapedJobID+"/status", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var status sttJobStatusPayload
			if err := json.Unmarshal(data, &status); err != nil {
				return apiErr(fmt.Errorf("parsing job status: %w", err))
			}
			filesToRetry := sttJobInputFileNames(status.JobDetails, flagFailedOnly)
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
				if flagFailedOnly && status.FailedFiles > 0 {
					return apiErr(fmt.Errorf("job reports %d failed file(s), but its status has no retryable input filenames", status.FailedFiles))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no failed files to retry")
				return nil
			}
			checkpointPath, err := sttRetryCheckpointPath(jobID, flags)
			if err != nil {
				return configErr(fmt.Errorf("preparing retry checkpoint: %w", err))
			}
			lease, err := acquireSTTRetryLease(checkpointPath)
			if err != nil {
				return configErr(err)
			}
			defer func() {
				if err := lease.Release(); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: retry lock could not be removed: %v\n", err)
				}
			}()
			// A new attempt must validate every local input before claiming a
			// checkpoint. Otherwise a missing file would leave an empty claim
			// that blocks every later retry. Existing attempts only need files
			// that their checkpoint has not recorded as uploaded.
			_, checkpointExists, err := loadSTTRetryCheckpoint(checkpointPath, jobID, filesToRetry)
			if err != nil {
				return configErr(err)
			}
			var preparedFiles []preparedSTTRetryFile
			if !checkpointExists {
				preparedFiles, err = prepareSTTRetryFiles(flagDir, filesToRetry)
				if err != nil {
					return apiErr(err)
				}
			}
			checkpoint, resumed, err := acquireSTTRetryCheckpoint(checkpointPath, jobID, filesToRetry)
			if err != nil {
				closePreparedSTTRetryFiles(preparedFiles)
				return configErr(err)
			}
			if checkpoint.Started {
				closePreparedSTTRetryFiles(preparedFiles)
				return writeSTTRetryResult(cmd, flags, jobID, checkpoint.ReplacementJobID, filesToRetry, true, "already_started")
			}
			remainingNames := pendingSTTRetryFileNames(filesToRetry, checkpoint.UploadedFiles)
			if checkpointExists || resumed {
				closePreparedSTTRetryFiles(preparedFiles)
				preparedFiles, err = prepareSTTRetryFiles(flagDir, remainingNames)
				if err != nil {
					return apiErr(err)
				}
			}
			defer closePreparedSTTRetryFiles(preparedFiles)

			// 2. Initiate a replacement only when no resumable one exists. The
			// checkpoint is claimed first, so concurrent/rerun invocations never
			// silently create another replacement after a partial failure.
			if checkpoint.ReplacementJobID == "" {
				jobParams := status.JobParameters
				if jobParams == nil {
					jobParams = map[string]any{}
				}
				initData, statusCode, err := c.Post(ctx, "/speech-to-text/job/v1", map[string]any{
					"job_parameters": jobParams,
				})
				if err != nil {
					return pendingSTTRetryError("", checkpointPath, classifyAPIError(err, flags))
				}
				if statusCode != http.StatusAccepted && statusCode != http.StatusOK {
					return pendingSTTRetryError("", checkpointPath, apiErr(fmt.Errorf("initiating retry job: HTTP %d", statusCode)))
				}
				var initResp struct {
					JobID string `json:"job_id"`
				}
				if err := json.Unmarshal(initData, &initResp); err != nil || initResp.JobID == "" {
					return pendingSTTRetryError("", checkpointPath, apiErr(fmt.Errorf("parsing job initiation response")))
				}
				checkpoint.ReplacementJobID = initResp.JobID
				if err := saveSTTRetryCheckpoint(checkpointPath, checkpoint); err != nil {
					return unsavedSTTRetryIDError(initResp.JobID, checkpointPath, err)
				}
			}
			newJobID := checkpoint.ReplacementJobID
			escapedNewJobID, err := sttJobPathSegment(newJobID)
			if err != nil {
				return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("invalid replacement job ID: %w", err)))
			}

			// 3. Get presigned URLs only for files not already recorded as
			// uploaded by a previous attempt.
			var uploadResp struct {
				UploadURLs map[string]struct {
					FileURL string `json:"file_url"`
				} `json:"upload_urls"`
			}
			if len(preparedFiles) > 0 {
				uploadData, _, err := c.Post(ctx, "/speech-to-text/job/v1/upload-files", map[string]any{
					"job_id": newJobID,
					"files":  remainingNames,
				})
				if err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, classifyAPIError(err, flags))
				}
				if err := json.Unmarshal(uploadData, &uploadResp); err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("parsing upload URLs: %w", err)))
				}
			}

			// 4. Upload each remaining file and checkpoint progress. Open file
			// descriptors were validated before the replacement was created.
			for _, prepared := range preparedFiles {
				fname := prepared.Name
				info, ok := uploadResp.UploadURLs[fname]
				if !ok || info.FileURL == "" {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("no upload URL for %q", fname)))
				}
				if err := validatePresignedUploadURL(info.FileURL); err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("upload URL for %q: %w", fname, err)))
				}
				if _, err := prepared.File.Seek(0, io.SeekStart); err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("rewinding %s: %w", prepared.Path, err)))
				}
				if err := validatePreparedSTTRetryFileSize(prepared); err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(err))
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, info.FileURL, prepared.File)
				if err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("building presigned upload request for %s failed", fname)))
				}
				req.ContentLength = prepared.Size
				req.Header.Set("Content-Type", "application/octet-stream")
				uploadClient := presignedUploadHTTPClient(c.HTTPClient, flags.timeout)
				resp, err := uploadClient.Do(req)
				if err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("presigned upload request for %s failed", fname)))
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(fmt.Errorf("uploading %s: HTTP %d", fname, resp.StatusCode)))
				}
				if err := validatePreparedSTTRetryFileSize(prepared); err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, apiErr(err))
				}
				checkpoint.UploadedFiles = append(checkpoint.UploadedFiles, fname)
				if err := saveSTTRetryCheckpoint(checkpointPath, checkpoint); err != nil {
					return pendingSTTRetryError(newJobID, checkpointPath, configErr(fmt.Errorf("saving upload progress: %w", err)))
				}
			}

			// 5. Record the attempt before calling the provider. If the request
			// succeeds remotely but its response is lost, rerunning must not send
			// another /start with an unknown outcome.
			checkpoint.StartAttempted = true
			if err := saveSTTRetryCheckpoint(checkpointPath, checkpoint); err != nil {
				return configErr(fmt.Errorf("recording retry start attempt: %w", err))
			}
			startData, _, err := c.Post(ctx, "/speech-to-text/job/v1/"+escapedNewJobID+"/start", map[string]any{})
			if err != nil {
				return fmt.Errorf("%w; start outcome for replacement job %s is unknown; inspect provider status before removing checkpoint %s", classifyAPIError(err, flags), newJobID, checkpointPath)
			}
			_ = startData
			checkpoint.Started = true
			if err := saveSTTRetryCheckpoint(checkpointPath, checkpoint); err != nil {
				return configErr(fmt.Errorf("retry job %s started, but recording completion failed: %w; remove checkpoint %s after confirming the provider status", newJobID, err, checkpointPath))
			}
			return writeSTTRetryResult(cmd, flags, jobID, newJobID, filesToRetry, resumed, "started")
		},
	}
	cmd.Flags().BoolVar(&flagFailedOnly, "failed-only", true, "Retry only files that failed in the original job")
	cmd.Flags().StringVar(&flagDir, "dir", "", "Local directory containing the audio files to re-upload")
	return cmd
}

func writeSTTRetryResult(cmd *cobra.Command, flags *rootFlags, oldJobID, newJobID string, files []string, resumed bool, status string) error {
	result := map[string]any{
		"old_job_id":          oldJobID,
		"new_job_id":          newJobID,
		"retried_files":       files,
		"resumed_replacement": resumed,
		"status":              status,
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if status == "already_started" {
		fmt.Fprintf(cmd.OutOrStdout(), "retry job %s was already started from %s; no new job was created\n", newJobID, oldJobID)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "retry job %s created from %s with %d file(s)\n", newJobID, oldJobID, len(files))
	return nil
}
