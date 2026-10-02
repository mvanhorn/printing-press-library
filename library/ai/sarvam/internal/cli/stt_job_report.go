// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type sttJobFileDetail struct {
	FileName     string `json:"file_name,omitempty"`
	FileID       string `json:"file_id,omitempty"`
	State        string `json:"state,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type sttJobFileReference struct {
	FileName string `json:"file_name"`
	FileID   string `json:"file_id"`
}

// sttJobAPIDetail mirrors Sarvam's actual batch-status shape. Inputs and
// outputs are nested arrays; the legacy flat fields are retained only so a
// locally captured pre-release fixture remains readable.
type sttJobAPIDetail struct {
	Inputs       []sttJobFileReference `json:"inputs"`
	Outputs      []sttJobFileReference `json:"outputs"`
	State        string                `json:"state"`
	ErrorMessage string                `json:"error_message"`
	FileName     string                `json:"file_name,omitempty"`
	FileID       string                `json:"file_id,omitempty"`
}

type sttJobStatusPayload struct {
	JobState        string            `json:"job_state"`
	JobParameters   map[string]any    `json:"job_parameters"`
	TotalFiles      int               `json:"total_files"`
	SuccessfulFiles int               `json:"successful_files_count"`
	FailedFiles     int               `json:"failed_files_count"`
	JobDetails      []sttJobAPIDetail `json:"job_details"`
}

type sttJobReportView struct {
	JobID           string             `json:"job_id"`
	JobState        string             `json:"job_state"`
	TotalFiles      int                `json:"total_files"`
	SuccessfulFiles int                `json:"successful_files_count"`
	FailedFiles     int                `json:"failed_files_count"`
	FileDetails     []sttJobFileDetail `json:"file_details"`
	FailedFileNames []string           `json:"failed_file_names"`
}

func newNovelSttJobReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "report [job_id]",
		Short:       "Per-file digest of a batch STT job with typed exit codes for cron alerting",
		Example:     "  sarvam-pp-cli stt-job report 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "job=20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5", "pp:typed-exit-codes": "0,5,6"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stt-job report")
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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.GetNoCache(ctx, "/speech-to-text/job/v1/"+escapedJobID+"/status", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var status sttJobStatusPayload
			if err := json.Unmarshal(data, &status); err != nil {
				return apiErr(fmt.Errorf("parsing job status: %w", err))
			}

			view := buildSTTJobReportView(jobID, status)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "job %s [%s]\n", view.JobID, view.JobState)
				fmt.Fprintf(cmd.OutOrStdout(), "  total: %d  ok: %d  failed: %d\n", view.TotalFiles, view.SuccessfulFiles, view.FailedFiles)
				for _, d := range view.FileDetails {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", sttJobStatusMarker(d.State), d.FileName)
				}
			}

			// Typed exit: non-zero when any file failed, so cron can alert.
			if view.FailedFiles > 0 || len(view.FailedFileNames) > 0 {
				return partialFailureErr(fmt.Errorf("%d file(s) failed in job %s", view.FailedFiles, jobID))
			}
			return nil
		},
	}
	return cmd
}

func buildSTTJobReportView(jobID string, status sttJobStatusPayload) sttJobReportView {
	view := sttJobReportView{
		JobID:           jobID,
		JobState:        status.JobState,
		TotalFiles:      status.TotalFiles,
		SuccessfulFiles: status.SuccessfulFiles,
		FailedFiles:     status.FailedFiles,
		FileDetails:     sttJobReportFileDetails(status.JobDetails),
		FailedFileNames: sttJobInputFileNames(status.JobDetails, true),
	}
	if view.TotalFiles == 0 {
		view.TotalFiles = len(view.FileDetails)
	}
	if view.FailedFiles == 0 && len(view.FailedFileNames) > 0 {
		view.FailedFiles = len(view.FailedFileNames)
	}
	state := strings.ToLower(strings.TrimSpace(view.JobState))
	if view.SuccessfulFiles == 0 && view.TotalFiles >= view.FailedFiles && (state == "completed" || state == "partially_completed") {
		view.SuccessfulFiles = view.TotalFiles - view.FailedFiles
	}
	return view
}

func sttJobDetailInputs(detail sttJobAPIDetail) []sttJobFileReference {
	if len(detail.Inputs) > 0 {
		return detail.Inputs
	}
	if strings.TrimSpace(detail.FileName) != "" {
		return []sttJobFileReference{{FileName: detail.FileName, FileID: detail.FileID}}
	}
	return nil
}

func sttJobReportFileDetails(details []sttJobAPIDetail) []sttJobFileDetail {
	files := make([]sttJobFileDetail, 0)
	for _, detail := range details {
		for _, input := range sttJobDetailInputs(detail) {
			files = append(files, sttJobFileDetail{
				FileName:     input.FileName,
				FileID:       input.FileID,
				State:        detail.State,
				ErrorMessage: detail.ErrorMessage,
			})
		}
	}
	return files
}

func sttJobInputFileNames(details []sttJobAPIDetail, failedOnly bool) []string {
	seen := make(map[string]struct{})
	files := make([]string, 0)
	for _, detail := range details {
		if failedOnly && !isFailedSTTJobDetailState(detail.State) {
			continue
		}
		for _, input := range sttJobDetailInputs(detail) {
			name := strings.TrimSpace(input.FileName)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			files = append(files, name)
		}
	}
	return files
}

func isFailedSTTJobDetailState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "api error", "internal server error", "failed", "failure", "error":
		return true
	default:
		return false
	}
}

func sttJobStatusMarker(state string) string {
	if isFailedSTTJobDetailState(state) {
		return "FAIL"
	}
	return "ok"
}
