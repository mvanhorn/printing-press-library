// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// messages_getattachment.go implements `superhuman-pp-cli messages
// get-attachment <message-id> <attachment-id>`. Mirrors the Superhuman
// MCP's get_attachment tool: download a single attachment by id.

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/gmail"
)

// newMessagesGetAttachmentCmd registers `messages get-attachment`.
func newMessagesGetAttachmentCmd(flags *rootFlags) *cobra.Command {
	var outputPath string
	var force bool
	cmd := &cobra.Command{
		Use:   "get-attachment <message-id> <attachment-id>",
		Short: "Download an attachment by id (Gmail passthrough)",
		Long: `Download a single attachment by id via Gmail's
users.messages.attachments.get. Both ids come from 'messages get' output
(the attachments list).

Use --output - to stream bytes to stdout (binary-safe; the human-readable
"saved N bytes" trailer is suppressed in this mode).`,
		Example: "  superhuman-pp-cli messages get-attachment 195e7c2d4f3e8a1b ANGjdJ_R --output ./logo.png\n  superhuman-pp-cli messages get-attachment <m> <a> --output - > ./logo.png",
		Annotations: map[string]string{
			"pp:endpoint":   "messages.get-attachment",
			"pp:method":     "GET",
			"pp:path":       "/users/me/messages/<m>/attachments/<a>",
			"mcp:read-only": "true",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return usageErr(fmt.Errorf("messages get-attachment: requires <message-id> <attachment-id>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return usageErr(fmt.Errorf("messages get-attachment: --output <path> is required (use --output - to stream to stdout)"))
			}
			return runMessagesGetAttachment(cmd, flags, args[0], args[1], outputPath, force)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (or '-' for stdout)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing output file")
	return cmd
}

func runMessagesGetAttachment(cmd *cobra.Command, flags *rootFlags, messageID, attachmentID, outputPath string, force bool) error {
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would call gmail.googleapis.com/.../messages/%s/attachments/%s -> %s\n", messageID, attachmentID, outputPath)
		return nil
	}

	// Output-path safety BEFORE the network round-trip: if the user pointed
	// at an existing file without --force, fail fast so a slow download
	// doesn't waste their time.
	if outputPath != "-" && !force {
		if _, statErr := os.Stat(outputPath); statErr == nil {
			return apiErr(fmt.Errorf("messages get-attachment: %s already exists (pass --force to overwrite)", outputPath))
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return apiErr(fmt.Errorf("messages get-attachment: stat %s: %w", outputPath, statErr))
		}
	}

	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("messages get-attachment: %w", err))
	}
	gc := gmail.New(acct.Store, acct.Email, acct.GoogleID, acct.AccessToken)
	gc.Stderr = cmd.ErrOrStderr()

	att, err := gc.GetAttachment(cmd.Context(), messageID, attachmentID)
	if err != nil {
		if gmail.IsAuth(err) {
			return authErr(fmt.Errorf("messages get-attachment: %w", err))
		}
		if ok, status := gmail.IsAPI(err); ok && status == 404 {
			return notFoundErr(fmt.Errorf("messages get-attachment: %s/%s not found", messageID, attachmentID))
		}
		return apiErr(fmt.Errorf("messages get-attachment: %w", err))
	}

	if outputPath == "-" {
		if _, werr := cmd.OutOrStdout().Write(att.Data); werr != nil {
			return apiErr(fmt.Errorf("messages get-attachment: write stdout: %w", werr))
		}
		return nil
	}

	// Mkdir-parent + atomic write (tmp + rename).
	parent := filepath.Dir(outputPath)
	if parent != "" && parent != "." {
		if mkerr := os.MkdirAll(parent, 0o755); mkerr != nil {
			return apiErr(fmt.Errorf("messages get-attachment: mkdir %s: %w", parent, mkerr))
		}
	}
	tmp, terr := os.CreateTemp(parent, ".attachment-*.tmp")
	if terr != nil {
		return apiErr(fmt.Errorf("messages get-attachment: create tmp: %w", terr))
	}
	tmpPath := tmp.Name()
	if _, werr := tmp.Write(att.Data); werr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return apiErr(fmt.Errorf("messages get-attachment: write tmp: %w", werr))
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpPath)
		return apiErr(fmt.Errorf("messages get-attachment: close tmp: %w", cerr))
	}
	if renErr := os.Rename(tmpPath, outputPath); renErr != nil {
		os.Remove(tmpPath)
		return apiErr(fmt.Errorf("messages get-attachment: rename to %s: %w", outputPath, renErr))
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Saved %d bytes to %s\n", att.Size, outputPath)
	return nil
}
