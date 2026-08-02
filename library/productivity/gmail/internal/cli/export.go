// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: export messages as RFC 2822 .eml files (format=raw).
// Absorbs GYB's per-message archival niche.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newExportCmd(flags))
	})
}

type exportResult struct {
	MessageID string `json:"message_id"`
	Path      string `json:"path"`
	Bytes     int    `json:"bytes"`
}

func newExportCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	var query string
	var outDir string
	var limit int

	cmd := &cobra.Command{
		Use:   "export [message-id]",
		Short: "Export messages as standard .eml files",
		Long: `Fetches messages in raw RFC 2822 form and writes .eml files any mail
client or archival tool can open. Export one message by id, or a batch
with --query.`,
		Example: strings.Trim(`
  gmail-pp-cli export 18c2f5a9b3d4e6f7 --out message.eml
  gmail-pp-cli export --query "from:legal@example.com" --dir ./archive`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "message-id=example-message-id",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export messages as .eml files")
			}
			if len(args) == 0 && strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a message id argument or --query is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var ids []string
			if len(args) > 0 {
				ids = []string{args[0]}
			} else {
				// Resolve the messages list endpoint through the emitted
				// resource path map so batch export and the generated
				// resource commands share one source of truth for the path.
				listPath, err := resourceReadPath("messages")
				if err != nil {
					return err
				}
				ids, err = gmailListIDsAt(cmd.Context(), c, listPath, query, limit)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			var results []exportResult
			for _, id := range ids {
				msg, err := gmailGetMessage(cmd.Context(), c, id, "raw")
				if err != nil {
					return classifyAPIError(err, flags)
				}
				data, err := gmailmail.DecodeB64URL(msg.Raw)
				if err != nil {
					return fmt.Errorf("decoding raw message %s: %w", id, err)
				}
				path := outPath
				if path == "" || len(ids) > 1 {
					if err := os.MkdirAll(outDir, 0o755); err != nil {
						return fmt.Errorf("creating output dir: %w", err)
					}
					path = filepath.Join(outDir, id+".eml")
				}
				if err := os.WriteFile(path, data, 0o600); err != nil {
					return fmt.Errorf("writing %s: %w", path, err)
				}
				results = append(results, exportResult{MessageID: id, Path: path, Bytes: len(data)})
			}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "exported %s (%d bytes)\n", r.Path, r.Bytes)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path (single message; default <id>.eml)")
	cmd.Flags().StringVar(&outDir, "dir", ".", "Output directory for batch export")
	cmd.Flags().StringVar(&query, "query", "", "Export every message matching this Gmail query")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum messages to export (with --query)")
	return cmd
}
