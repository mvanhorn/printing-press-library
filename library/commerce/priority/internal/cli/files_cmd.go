// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written attachment surface: EXTFILES_SUBFORM upload (local file →
// base64 data URI + SUFFIX) and download (data URI → file on disk).

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newFilesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "files",
		Short:       "Attachments (EXTFILES_SUBFORM): attach local files, download attached ones",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFilesAttachCmd(flags))
	cmd.AddCommand(newFilesGetCmd(flags))
	return cmd
}

func newFilesAttachCmd(flags *rootFlags) *cobra.Command {
	var file string
	var description string
	cmd := &cobra.Command{
		Use:   "attach <form> <keyspec>",
		Short: "Attach a local file to a record (uploads as base64 data URI with MIME suffix)",
		Example: strings.Trim(`
  priority-pp-cli files attach ORDERS "'SO21000113'" --file ./quote.pdf --description "customer quote" --dry-run
  priority-pp-cli files attach CUSTOMERS "'1011'" --file ./contract.docx`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would upload the file as a base64 data URI to EXTFILES_SUBFORM")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form and keyspec are required"))
			}
			if file == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required"))
			}
			raw, err := os.ReadFile(file) // #nosec G304 -- path is the user's own --file flag; uploading a user-named local file is this command's purpose
			if err != nil {
				return usageErr(fmt.Errorf("reading --file: %w", err))
			}
			ext := filepath.Ext(file)
			mimeType := mime.TypeByExtension(ext)
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			if description == "" {
				description = strings.TrimSuffix(filepath.Base(file), ext)
			}
			dataURI := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw)
			body := map[string]string{
				"EXTFILEDES":  description,
				"EXTFILENAME": dataURI,
			}
			if ext != "" {
				body["SUFFIX"] = ext // leading period included, per docs
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Post(ctx, fmt.Sprintf("/%s(%s)/EXTFILES_SUBFORM", args[0], args[1]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "local file to attach")
	cmd.Flags().StringVar(&description, "description", "", "attachment description (EXTFILEDES; defaults to the file name)")
	return cmd
}

func newFilesGetCmd(flags *rootFlags) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "get <form> <keyspec> <line>",
		Short: "Download one attachment (decodes the base64 data URI to a file)",
		Example: strings.Trim(`
  priority-pp-cli files get ORDERS "'SO18000002'" 1 --out ./attachment.bin
  priority-pp-cli files get ORDERS "'SO18000002'" 1 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would download and decode the attachment")
				return nil
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form, keyspec, and line are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, fmt.Sprintf("/%s(%s)/EXTFILES_SUBFORM(%s)", args[0], args[1], args[2]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var rec map[string]json.RawMessage
			if err := json.Unmarshal(data, &rec); err != nil {
				return fmt.Errorf("parsing attachment record: %w", err)
			}
			name := jsonStrField(rec, "EXTFILENAME")
			desc := jsonStrField(rec, "EXTFILEDES")
			suffix := jsonStrField(rec, "SUFFIX")
			if out == "" {
				// No output path: emit the record metadata (JSON) without the blob.
				meta, _ := json.Marshal(map[string]any{
					"EXTFILEDES": desc,
					"SUFFIX":     suffix,
					"note":       "pass --out <path> to decode the file to disk",
				})
				return printOutput(cmd.OutOrStdout(), meta, true)
			}
			idx := strings.Index(name, ";base64,")
			if idx < 0 {
				return apiErr(fmt.Errorf("EXTFILENAME is not a base64 data URI (tenant older than Priority 21.0?): %s", truncate(name, 60)))
			}
			blob, err := base64.StdEncoding.DecodeString(name[idx+len(";base64,"):])
			if err != nil {
				return fmt.Errorf("decoding attachment: %w", err)
			}
			// 0600: ERP attachments (invoices, contracts) may be sensitive; keep them owner-only.
			if err := os.WriteFile(out, blob, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", out, err)
			}
			result, _ := json.Marshal(map[string]any{
				"written":    out,
				"bytes":      len(blob),
				"EXTFILEDES": desc,
				"SUFFIX":     suffix,
			})
			return printOutput(cmd.OutOrStdout(), result, true)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "write the decoded file to this path (omit to print metadata only)")
	return cmd
}
