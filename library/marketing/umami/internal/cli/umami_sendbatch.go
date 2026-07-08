// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: /api/batch takes a top-level JSON array, which the generated
// body schema cannot express, so batch ingestion is a hand-built subcommand.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newSendBatchCmd(flags *rootFlags) *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Send a batch of tracking payloads from a JSON array file or stdin",
		Long: `POST a JSON array of /api/send-shaped payloads to /api/batch in one call.
Each element has the same shape as the single send command's body:
{"type":"event","payload":{"website":"<uuid>","url":"/page","name":"my_event"}}.

Read from --file, or pipe the array on stdin with --file -.`,
		Example:     "  umami-pp-cli send batch --file events.json\n  cat events.json | umami-pp-cli send batch --file -",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would POST the JSON array to /api/batch")
				return nil
			}
			if filePath == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required (path to a JSON array, or - for stdin)"))
			}
			var raw []byte
			var err error
			if filePath == "-" {
				raw, err = io.ReadAll(cmd.InOrStdin())
			} else {
				raw, err = os.ReadFile(filePath) // #nosec G304 -- filePath is the user's explicit --file flag; reading it is this command's purpose
			}
			if err != nil {
				return fmt.Errorf("reading payload: %w", err)
			}
			var batch []json.RawMessage
			if err := json.Unmarshal(raw, &batch); err != nil {
				return usageErr(fmt.Errorf("payload must be a JSON array of send-shaped objects: %w", err))
			}
			if len(batch) == 0 {
				return usageErr(fmt.Errorf("payload array is empty"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(ctx, "/api/batch", batch)
			if err != nil {
				return apiErr(fmt.Errorf("batch send failed (status %d): %w", status, err))
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to the JSON array file, or - for stdin")
	return cmd
}
