// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type sttJobFileDetail struct {
	FileName    string `json:"file_name,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	State       string `json:"state,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type sttJobReportView struct {
	JobID                string            `json:"job_id"`
	JobState             string            `json:"job_state"`
	TotalFiles           int               `json:"total_files"`
	SuccessfulFiles      int               `json:"successful_files_count"`
	FailedFiles          int               `json:"failed_files_count"`
	FileDetails          []sttJobFileDetail `json:"file_details"`
	FailedFileNames      []string          `json:"failed_file_names"`
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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.GetNoCache(ctx, "/speech-to-text/job/v1/"+jobID+"/status", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var status struct {
				JobState           string `json:"job_state"`
				TotalFiles         int    `json:"total_files"`
				SuccessfulFiles    int    `json:"successful_files_count"`
				FailedFiles        int    `json:"failed_files_count"`
				JobDetails         []struct {
					FileName    string `json:"file_name"`
					FileID      string `json:"file_id"`
					State       string `json:"state"`
					ErrorMessage string `json:"error_message"`
				} `json:"job_details"`
			}
			if err := json.Unmarshal(data, &status); err != nil {
				return apiErr(fmt.Errorf("parsing job status: %w", err))
			}

			view := sttJobReportView{
				JobID:           jobID,
				JobState:        status.JobState,
				TotalFiles:      status.TotalFiles,
				SuccessfulFiles: status.SuccessfulFiles,
				FailedFiles:     status.FailedFiles,
				FileDetails:     make([]sttJobFileDetail, 0),
				FailedFileNames: make([]string, 0),
			}
			for _, d := range status.JobDetails {
				detail := sttJobFileDetail{
					FileName:     d.FileName,
					FileID:       d.FileID,
					State:        d.State,
					ErrorMessage: d.ErrorMessage,
				}
				view.FileDetails = append(view.FileDetails, detail)
				if d.State == "API Error" || d.State == "Internal Server Error" {
					view.FailedFileNames = append(view.FailedFileNames, d.FileName)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "job %s [%s]\n", view.JobID, view.JobState)
				fmt.Fprintf(cmd.OutOrStdout(), "  total: %d  ok: %d  failed: %d\n", view.TotalFiles, view.SuccessfulFiles, view.FailedFiles)
				for _, d := range view.FileDetails {
					mark := "ok"
					if d.State == "API Error" || d.State == "Internal Server Error" {
						mark = "FAIL"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", mark, d.FileName)
				}
			}

			// Typed exit: non-zero when any file failed, so cron can alert.
			if len(view.FailedFileNames) > 0 {
				return partialFailureErr(fmt.Errorf("%d file(s) failed in job %s", len(view.FailedFileNames), jobID))
			}
			return nil
		},
	}
	return cmd
}
