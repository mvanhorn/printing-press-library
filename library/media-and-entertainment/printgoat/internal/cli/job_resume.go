// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: `job resume` continues a previously-created `job download`
// job from wherever it stopped, reusing the same downloadFileResumable /
// processJobFiles engine defined in job_download.go.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

func newNovelJobResumeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "resume <job-id>",
		Short:       "Continue a job download from where it stopped.",
		Example:     "  printgoat-pp-cli job resume job-20260721-01 --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing required argument <job-id>\nUsage: %s <job-id>", cmd.CommandPath()))
			}
			jobID := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			var status string
			scanErr := s.DB().QueryRowContext(ctx, `SELECT status FROM printgoat_print_jobs WHERE id = ?`, jobID).Scan(&status)
			if scanErr == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("no job with id %q", jobID))
			}
			if scanErr != nil {
				return fmt.Errorf("reading job: %w", scanErr)
			}

			results, perr := processJobFiles(ctx, c, c.HTTPClient, s.DB(), jobID)
			if perr != nil {
				return fmt.Errorf("processing job files: %w", perr)
			}
			if err := finalizeJobStatus(ctx, s.DB(), jobID); err != nil {
				return fmt.Errorf("finalizing job status: %w", err)
			}

			out := map[string]any{"job_id": jobID, "files": results}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
