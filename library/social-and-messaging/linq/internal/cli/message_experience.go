// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	directAttachmentLimitBytes = int64(10 * 1024 * 1024)
	uploadAttachmentLimitBytes = int64(100 * 1024 * 1024)
	maxTypingDwell             = 5 * time.Second
)

var linkTitleRE = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
var linkMetaTagRE = regexp.MustCompile(`(?is)<meta\b([^>]*)>`)
var linkMetaAttrRE = regexp.MustCompile(`(?is)\b(property|name|content)=["']([^"']*)["']`)
var linkImageRE = regexp.MustCompile(`(?is)<img[^>]+src=["']([^"']+)["']`)

func newComposeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Build, validate, and send rich Linq messages",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newComposePreviewCmd(flags))
	cmd.AddCommand(newComposeSendCmd(flags))
	return cmd
}

func newComposePreviewCmd(flags *rootFlags) *cobra.Command {
	var opts linqMessageBuildOptions
	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Build and validate a rich message without sending",
		Example:     `  linq-pp-cli compose preview --text "Congrats!" --effect screen:confetti --decorate 0:8:bold --preferred-service iMessage --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			result := buildLinqMessageBody(opts)
			return printJSONValue(cmd, result)
		},
	}
	addMessageBuilderFlags(cmd, &opts)
	return cmd
}

func newComposeSendCmd(flags *rootFlags) *cobra.Command {
	var opts linqMessageBuildOptions
	var chatID string
	var typing bool
	var typingDwell time.Duration
	cmd := &cobra.Command{
		Use:     "send --chat-id CHAT",
		Short:   "Send a generic rich Linq message to an existing chat",
		Example: `  linq-pp-cli compose send --chat-id ch_123 --text "Congrats!" --effect screen:confetti --idempotency-key req_123 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(chatID) == "" {
				return usageErr(fmt.Errorf("--chat-id is required"))
			}
			result := buildLinqMessageBody(opts)
			if err := requireSendableMessage(result); err != nil {
				return err
			}
			path := replacePathParam("/v3/chats/{chatId}/messages", "chatId", chatID)
			typingPlan := map[string]any(nil)
			if typing {
				dwell, err := boundedTypingDwell(typingDwell)
				if err != nil {
					return err
				}
				typingPlan = map[string]any{
					"start_path": replacePathParam("/v3/chats/{chatId}/typing", "chatId", chatID),
					"dwell":      dwell.String(),
					"warning":    typingProtocolWarning(),
				}
				if !flags.dryRun {
					if err := sendTypingStart(cmd.Context(), flags, chatID); err != nil {
						return err
					}
					if err := sleepForContext(cmd.Context(), dwell); err != nil {
						return err
					}
				}
			}
			return runJSONMutation(cmd, flags, http.MethodPost, path, result.Body, "messages", result.ProtocolWarnings, typingPlan)
		},
	}
	addMessageBuilderFlags(cmd, &opts)
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing chat ID")
	cmd.Flags().BoolVar(&typing, "typing", false, "Start a bounded typing indicator before sending")
	cmd.Flags().DurationVar(&typingDwell, "typing-dwell", 800*time.Millisecond, "How long to dwell after typing starts; capped at 5s")
	return cmd
}

func addMessageBuilderFlags(cmd *cobra.Command, opts *linqMessageBuildOptions) {
	cmd.Flags().StringArrayVar(&opts.Texts, "text", nil, "Text message part; repeatable")
	cmd.Flags().StringArrayVar(&opts.MediaURLs, "media-url", nil, "Public HTTPS media URL part; repeatable")
	cmd.Flags().StringArrayVar(&opts.AttachmentIDs, "attachment-id", nil, "Pre-uploaded attachment ID media part; repeatable")
	cmd.Flags().StringVar(&opts.Link, "link", "", "HTTPS rich link preview URL; must be the only part")
	cmd.Flags().StringVar(&opts.Effect, "effect", "", "iMessage effect as TYPE:NAME, e.g. screen:confetti or bubble:slam")
	cmd.Flags().StringArrayVar(&opts.Decorations, "decorate", nil, "Text decoration as START:END:STYLE_OR_ANIMATION using UTF-16 code units; repeatable")
	cmd.Flags().StringVar(&opts.PreferredService, "preferred-service", "", "Preferred protocol: iMessage, RCS, SMS, or omitted")
	cmd.Flags().StringVar(&opts.ReplyToMessageID, "reply-to-message-id", "", "Message ID to reply to")
	cmd.Flags().IntVar(&opts.ReplyToPartIndex, "reply-to-part-index", 0, "0-based part index for replies")
	cmd.Flags().StringVar(&opts.IdempotencyKey, "idempotency-key", "", "Message idempotency key, placed inside message.idempotency_key")
	existingPreRunE := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if existingPreRunE != nil {
			if err := existingPreRunE(cmd, args); err != nil {
				return err
			}
		}
		opts.HasReplyToPartIndex = cmd.Flags().Changed("reply-to-part-index")
		return nil
	}
}

func newEffectsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "effects",
		Short: "List and preview iMessage effects and text decorations",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       "List supported effects, styles, and animations",
		Example:     `  linq-pp-cli effects list --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSONValue(cmd, map[string]any{
				"screen_effects":   sortedMapKeys(screenEffects),
				"bubble_effects":   sortedMapKeys(bubbleEffects),
				"text_styles":      sortedMapKeys(textDecorationStyles),
				"text_animations":  sortedMapKeys(textDecorationAnimations),
				"range_units":      "UTF-16 code units",
				"protocol_warning": "effects and decorations are iMessage-only",
			})
		},
	})
	var opts linqMessageBuildOptions
	preview := &cobra.Command{
		Use:         "preview",
		Short:       "Preview a message body with effects or decorations",
		Example:     `  linq-pp-cli effects preview --text "Congrats!" --effect screen:confetti --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(opts.Texts) == 0 {
				opts.Texts = []string{""}
			}
			result := buildLinqMessageBody(opts)
			return printJSONValue(cmd, result)
		},
	}
	addMessageBuilderFlags(preview, &opts)
	cmd.AddCommand(preview)
	return cmd
}

func newTypingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "typing",
		Short: "Start, stop, or pulse Linq typing indicators",
		Long: `Start, stop, or pulse outbound Linq typing indicators.

Inbound typing events are push-only webhooks. This CLI does not receive them
from Linq; use 'webhooks add-event' to subscribe your receiver, then
'typing watch' to inspect a captured webhook/debug stream.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTypingStartCmd(flags))
	cmd.AddCommand(newTypingStopCmd(flags))
	cmd.AddCommand(newTypingPulseCmd(flags))
	cmd.AddCommand(newTypingWatchCmd(flags))
	return cmd
}

func newTypingStartCmd(flags *rootFlags) *cobra.Command {
	var chatID string
	cmd := &cobra.Command{
		Use:     "start --chat-id CHAT",
		Short:   "Start a bounded iMessage typing indicator",
		Example: `  linq-pp-cli typing start --chat-id ch_123 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(chatID) == "" {
				return usageErr(fmt.Errorf("--chat-id is required"))
			}
			path := replacePathParam("/v3/chats/{chatId}/typing", "chatId", chatID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, map[string]any{}, "typing", []string{typingProtocolWarning()}, nil)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing one-to-one chat ID")
	return cmd
}

func newTypingStopCmd(flags *rootFlags) *cobra.Command {
	var chatID string
	cmd := &cobra.Command{
		Use:     "stop --chat-id CHAT",
		Short:   "Stop an iMessage typing indicator",
		Example: `  linq-pp-cli typing stop --chat-id ch_123 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(chatID) == "" {
				return usageErr(fmt.Errorf("--chat-id is required"))
			}
			path := replacePathParam("/v3/chats/{chatId}/typing", "chatId", chatID)
			return runJSONMutation(cmd, flags, http.MethodDelete, path, nil, "typing", []string{typingProtocolWarning()}, nil)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing one-to-one chat ID")
	return cmd
}

func newTypingPulseCmd(flags *rootFlags) *cobra.Command {
	var chatID string
	var dwell time.Duration
	cmd := &cobra.Command{
		Use:     "pulse --chat-id CHAT",
		Short:   "Start typing, wait a bounded dwell, then stop",
		Example: `  linq-pp-cli typing pulse --chat-id ch_123 --dwell 800ms --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(chatID) == "" {
				return usageErr(fmt.Errorf("--chat-id is required"))
			}
			dwell, err := boundedTypingDwell(dwell)
			if err != nil {
				return err
			}
			startPath := replacePathParam("/v3/chats/{chatId}/typing", "chatId", chatID)
			stopPath := startPath
			if flags.dryRun {
				return printJSONValue(cmd, map[string]any{
					"action":            "typing_pulse",
					"dry_run":           true,
					"success":           false,
					"start_path":        startPath,
					"stop_path":         stopPath,
					"dwell":             dwell.String(),
					"protocol_warnings": []string{typingProtocolWarning()},
				})
			}
			if err := sendTypingStart(cmd.Context(), flags, chatID); err != nil {
				return err
			}
			if err := sleepForContext(cmd.Context(), dwell); err != nil {
				return err
			}
			return runJSONMutation(cmd, flags, http.MethodDelete, stopPath, nil, "typing", []string{typingProtocolWarning()}, map[string]any{"pulse": true, "dwell": dwell.String()})
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing one-to-one chat ID")
	cmd.Flags().DurationVar(&dwell, "dwell", 800*time.Millisecond, "Typing dwell duration; capped at 5s")
	return cmd
}

func newLinkPreviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link-preview",
		Short: "Audit, inspect, and send rich link previews",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newLinkPreviewAuditCmd(flags))
	cmd.AddCommand(newLinkPreviewSendCmd(flags))
	cmd.AddCommand(newLinkPreviewMetadataCmd(flags))
	return cmd
}

func newLinkPreviewAuditCmd(flags *rootFlags) *cobra.Command {
	var firstOutbound bool
	cmd := &cobra.Command{
		Use:         "audit URL",
		Short:       "Validate rich link preview constraints",
		Example:     `  linq-pp-cli link-preview audit https://example.com --agent`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			result := auditRichLinkPreview(args[0], firstOutbound)
			return printJSONValue(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&firstOutbound, "first-outbound", false, "Treat this as the first outbound POST /v3/chats message and block links")
	return cmd
}

func newLinkPreviewSendCmd(flags *rootFlags) *cobra.Command {
	var chatID, rawURL, preferredService, idempotencyKey string
	var firstOutbound bool
	cmd := &cobra.Command{
		Use:     "send --chat-id CHAT --url URL",
		Short:   "Send a rich link preview to an existing chat",
		Example: `  linq-pp-cli link-preview send --chat-id ch_123 --url https://example.com --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" || rawURL == "" {
				return usageErr(fmt.Errorf("--chat-id and --url are required"))
			}
			audit := auditRichLinkPreview(rawURL, firstOutbound)
			if audit["sendable"] != true {
				return usageErr(fmt.Errorf("link preview is not sendable: %s", strings.Join(anyStringSlice(audit["errors"]), "; ")))
			}
			result := buildLinqMessageBody(linqMessageBuildOptions{
				Link:             rawURL,
				PreferredService: preferredService,
				IdempotencyKey:   idempotencyKey,
			})
			if err := requireSendableMessage(result); err != nil {
				return err
			}
			path := replacePathParam("/v3/chats/{chatId}/messages", "chatId", chatID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, result.Body, "messages", result.ProtocolWarnings, audit)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing chat ID")
	cmd.Flags().StringVar(&rawURL, "url", "", "HTTPS URL to render as a rich link preview")
	cmd.Flags().StringVar(&preferredService, "preferred-service", "", "Preferred protocol: iMessage, RCS, SMS, or omitted")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Message idempotency key")
	cmd.Flags().BoolVar(&firstOutbound, "first-outbound", false, "Block links for a first outbound POST /v3/chats opener")
	return cmd
}

func newLinkPreviewMetadataCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "metadata URL",
		Short:       "Fetch Open Graph, Twitter Card, title, and image metadata",
		Example:     `  linq-pp-cli link-preview metadata https://example.com --agent`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONValue(cmd, map[string]any{"action": "link_preview_metadata", "dry_run": true, "url": args[0], "would_fetch": true})
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			meta, err := fetchLinkPreviewMetadata(ctx, args[0])
			if err != nil {
				return err
			}
			return printJSONValue(cmd, meta)
		},
	}
	return cmd
}

func newAttachmentsPlanCmd(flags *rootFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:         "plan --file PATH",
		Short:       "Decide whether an attachment can use direct URL or should be pre-uploaded",
		Example:     `  linq-pp-cli attachments plan --file ./photo.jpg --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := attachmentPlan(file)
			if err != nil {
				return err
			}
			return printJSONValue(cmd, plan)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Local file path")
	return cmd
}

func newAttachmentsUploadCmd(flags *rootFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:     "upload --file PATH",
		Short:   "Request a pre-upload URL and upload file bytes",
		Example: `  linq-pp-cli attachments upload --file ./photo.jpg --agent --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := attachmentPlan(file)
			if err != nil {
				return err
			}
			if plan["upload_allowed"] != true {
				return usageErr(fmt.Errorf("file is not eligible for Linq pre-upload"))
			}
			body := map[string]any{
				"filename":     filepath.Base(file),
				"content_type": plan["content_type"],
				"size_bytes":   plan["size_bytes"],
			}
			if flags.dryRun {
				return printJSONValue(cmd, map[string]any{"action": "attachment_upload", "dry_run": true, "preupload_path": "/v3/attachments", "body": body, "plan": plan})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.PostWithParams(cmd.Context(), "/v3/attachments", map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var parsed map[string]any
			_ = json.Unmarshal(data, &parsed)
			uploadURL := firstNestedString(parsed, "upload_url", "uploadUrl", "url")
			if uploadURL == "" {
				return fmt.Errorf("pre-upload response did not include an upload_url")
			}
			if err := putAttachmentFile(cmd.Context(), uploadURL, parsed, file, fmt.Sprint(plan["content_type"])); err != nil {
				return err
			}
			return printJSONValue(cmd, map[string]any{"action": "attachment_upload", "status": status, "success": status >= 200 && status < 300, "preupload": parsed, "plan": plan})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Local file path")
	return cmd
}

func newAttachmentsSendURLCmd(flags *rootFlags) *cobra.Command {
	var chatID, rawURL, text, idempotencyKey string
	cmd := &cobra.Command{
		Use:     "send-url --chat-id CHAT --url URL",
		Short:   "Send text plus a public HTTPS media URL",
		Example: `  linq-pp-cli attachments send-url --chat-id ch_123 --url https://cdn.example/photo.jpg --text "Photo attached" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" || rawURL == "" {
				return usageErr(fmt.Errorf("--chat-id and --url are required"))
			}
			result := buildLinqMessageBody(linqMessageBuildOptions{Texts: optionalStringSlice(text), MediaURLs: []string{rawURL}, IdempotencyKey: idempotencyKey})
			if err := requireSendableMessage(result); err != nil {
				return err
			}
			path := replacePathParam("/v3/chats/{chatId}/messages", "chatId", chatID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, result.Body, "messages", result.ProtocolWarnings, map[string]any{"attachment_warning": attachmentLifecycleWarning()})
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing chat ID")
	cmd.Flags().StringVar(&rawURL, "url", "", "Public HTTPS media URL, up to 10MB")
	cmd.Flags().StringVar(&text, "text", "", "Optional text part before the media")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Message idempotency key")
	return cmd
}

func newAttachmentsSendIDCmd(flags *rootFlags) *cobra.Command {
	var chatID, attachmentID, text, idempotencyKey string
	cmd := &cobra.Command{
		Use:     "send-id --chat-id CHAT --attachment-id ATTACHMENT",
		Short:   "Send text plus a pre-uploaded attachment ID",
		Example: `  linq-pp-cli attachments send-id --chat-id ch_123 --attachment-id att_123 --text "Photo attached" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" || attachmentID == "" {
				return usageErr(fmt.Errorf("--chat-id and --attachment-id are required"))
			}
			result := buildLinqMessageBody(linqMessageBuildOptions{Texts: optionalStringSlice(text), AttachmentIDs: []string{attachmentID}, IdempotencyKey: idempotencyKey})
			if err := requireSendableMessage(result); err != nil {
				return err
			}
			path := replacePathParam("/v3/chats/{chatId}/messages", "chatId", chatID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, result.Body, "messages", result.ProtocolWarnings, map[string]any{"attachment_warning": attachmentLifecycleWarning()})
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing chat ID")
	cmd.Flags().StringVar(&attachmentID, "attachment-id", "", "Pre-uploaded attachment ID")
	cmd.Flags().StringVar(&text, "text", "", "Optional text part before the media")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Message idempotency key")
	return cmd
}

func newAttachmentsAuditURLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "audit-url URL",
		Short:       "Audit a public media URL before using it as an attachment",
		Example:     `  linq-pp-cli attachments audit-url https://cdn.example/photo.jpg --agent`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := args[0]
			errorsOut := []string{}
			if !isHTTPSURL(raw) {
				errorsOut = append(errorsOut, "attachment media URLs must use HTTPS")
			}
			return printJSONValue(cmd, map[string]any{
				"url":              raw,
				"sendable":         len(errorsOut) == 0,
				"errors":           errorsOut,
				"direct_url_limit": "10MB",
				"preupload_limit":  "100MB",
				"warnings":         []string{"allowlist cdn.linqapp.com for Linq attachment responses", attachmentLifecycleWarning(), "audio media parts appear as downloadable files; use voicememo for native inline playback"},
			})
		},
	}
	return cmd
}

func newAttachmentsCleanupCmd(flags *rootFlags) *cobra.Command {
	var attachmentID string
	cmd := &cobra.Command{
		Use:     "cleanup --attachment-id ATTACHMENT",
		Short:   "Delete an owned Linq attachment",
		Example: `  linq-pp-cli attachments cleanup --attachment-id att_123 --agent --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if attachmentID == "" {
				return usageErr(fmt.Errorf("--attachment-id is required"))
			}
			path := replacePathParam("/v3/attachments/{attachmentId}", "attachmentId", attachmentID)
			return runJSONMutation(cmd, flags, http.MethodDelete, path, nil, "attachments", []string{"deletion is irreversible; message history remains but attachment bytes are removed"}, nil)
		},
	}
	cmd.Flags().StringVar(&attachmentID, "attachment-id", "", "Attachment ID to delete")
	return cmd
}

func newReactCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "react",
		Short: "Add or remove validated message reactions",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReactOperationCmd(flags, "add"))
	cmd.AddCommand(newReactOperationCmd(flags, "remove"))
	cmd.AddCommand(newReactCustomCmd(flags))
	return cmd
}

func newReactOperationCmd(flags *rootFlags, operation string) *cobra.Command {
	var messageID, reactionType string
	var partIndex int
	cmd := &cobra.Command{
		Use:     operation + " --message-id MESSAGE --type TYPE",
		Short:   asciiTitle(operation) + " a built-in reaction",
		Example: "  linq-pp-cli react " + operation + " --message-id msg_123 --type like --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if messageID == "" || reactionType == "" {
				return usageErr(fmt.Errorf("--message-id and --type are required"))
			}
			reactionType = strings.ToLower(strings.TrimSpace(reactionType))
			if !validBuiltInReaction(reactionType) {
				return usageErr(fmt.Errorf("--type must be one of love, like, dislike, laugh, emphasize, question; sticker is inbound-only"))
			}
			body := reactionBody(operation, reactionType, "", partIndex, cmd.Flags().Changed("part-index"))
			path := replacePathParam("/v3/messages/{messageId}/reactions", "messageId", messageID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, body, "reactions", []string{"reactions render on iMessage/RCS; SMS support is limited"}, nil)
		},
	}
	cmd.Flags().StringVar(&messageID, "message-id", "", "Message ID to react to")
	cmd.Flags().StringVar(&reactionType, "type", "", "Built-in reaction: love, like, dislike, laugh, emphasize, question")
	cmd.Flags().IntVar(&partIndex, "part-index", 0, "0-based message part index")
	return cmd
}

func newReactCustomCmd(flags *rootFlags) *cobra.Command {
	var messageID, emoji string
	var partIndex int
	cmd := &cobra.Command{
		Use:     "custom --message-id MESSAGE --emoji EMOJI",
		Short:   "Add a custom Unicode emoji reaction",
		Example: `  linq-pp-cli react custom --message-id msg_123 --emoji "tada" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if messageID == "" || strings.TrimSpace(emoji) == "" {
				return usageErr(fmt.Errorf("--message-id and --emoji are required"))
			}
			body := reactionBody("add", "custom", emoji, partIndex, cmd.Flags().Changed("part-index"))
			path := replacePathParam("/v3/messages/{messageId}/reactions", "messageId", messageID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, body, "reactions", []string{"custom emoji reactions render on iMessage/RCS; SMS support is limited"}, nil)
		},
	}
	cmd.Flags().StringVar(&messageID, "message-id", "", "Message ID to react to")
	cmd.Flags().StringVar(&emoji, "emoji", "", "Custom Unicode emoji")
	cmd.Flags().IntVar(&partIndex, "part-index", 0, "0-based message part index")
	return cmd
}

func newContactShareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contact-share",
		Short: "Preflight and send native iMessage contact card sharing",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newContactSharePreflightCmd(flags))
	cmd.AddCommand(newContactShareSendCmd(flags))
	return cmd
}

func newContactSharePreflightCmd(flags *rootFlags) *cobra.Command {
	var chatID, from string
	cmd := &cobra.Command{
		Use:         "preflight --chat-id CHAT --from NUMBER",
		Short:       "Check contact card sharing readiness",
		Example:     `  linq-pp-cli contact-share preflight --chat-id ch_123 --from +16282893046 --agent --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" || from == "" {
				return usageErr(fmt.Errorf("--chat-id and --from are required"))
			}
			result, err := contactSharePreflight(cmd.Context(), flags, chatID, from)
			if err != nil {
				return err
			}
			return printJSONValue(cmd, result)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing iMessage chat ID")
	cmd.Flags().StringVar(&from, "from", "", "Sending phone number in E.164 format")
	return cmd
}

func newContactShareSendCmd(flags *rootFlags) *cobra.Command {
	var chatID, from string
	cmd := &cobra.Command{
		Use:     "send --chat-id CHAT --from NUMBER --yes",
		Short:   "Share the configured contact card with a chat",
		Example: `  linq-pp-cli contact-share send --chat-id ch_123 --from +16282893046 --yes --agent --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" || from == "" {
				return usageErr(fmt.Errorf("--chat-id and --from are required"))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("--yes is required to send contact-share"))
			}
			preflight, err := contactSharePreflight(cmd.Context(), flags, chatID, from)
			if err != nil {
				return err
			}
			if preflight["allowed"] != true && !flags.dryRun {
				return usageErr(fmt.Errorf("contact-share preflight failed: %s", strings.Join(anyStringSlice(preflight["blocking_reasons"]), "; ")))
			}
			path := replacePathParam("/v3/chats/{chatId}/share_contact_card", "chatId", chatID)
			return runJSONMutation(cmd, flags, http.MethodPost, path, nil, "share-contact-card", []string{"iMessage-only; call at most once per chat per day"}, preflight)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing iMessage chat ID")
	cmd.Flags().StringVar(&from, "from", "", "Sending phone number in E.164 format")
	return cmd
}

func runJSONMutation(cmd *cobra.Command, flags *rootFlags, method, path string, body any, resource string, warnings []string, extra any) error {
	if flags.dryRun {
		out := map[string]any{
			"action":            strings.ToLower(method),
			"resource":          resource,
			"path":              path,
			"body":              body,
			"dry_run":           true,
			"status":            0,
			"success":           false,
			"protocol_warnings": warnings,
		}
		if extra != nil {
			out["details"] = extra
		}
		return printJSONValue(cmd, out)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	var data json.RawMessage
	var status int
	switch method {
	case http.MethodPost:
		data, status, err = c.PostWithParams(cmd.Context(), path, map[string]string{}, body)
	case http.MethodDelete:
		if body == nil {
			data, status, err = c.DeleteWithParams(cmd.Context(), path, map[string]string{})
		} else {
			data, status, err = c.DeleteWithParamsAndBody(cmd.Context(), path, map[string]string{}, body)
		}
	default:
		return fmt.Errorf("unsupported mutation method %s", method)
	}
	if err != nil {
		return classifyAPIError(err, flags)
	}
	env := map[string]any{
		"action":            strings.ToLower(method),
		"resource":          resource,
		"path":              path,
		"status":            status,
		"success":           status >= 200 && status < 300,
		"protocol_warnings": warnings,
	}
	if extra != nil {
		env["details"] = extra
	}
	if len(data) > 0 {
		var parsed any
		if json.Unmarshal(data, &parsed) == nil {
			env["data"] = parsed
		}
	}
	return printJSONValue(cmd, env)
}

func sendTypingStart(ctx context.Context, flags *rootFlags, chatID string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	_, _, err = c.PostWithParams(ctx, replacePathParam("/v3/chats/{chatId}/typing", "chatId", chatID), map[string]string{}, map[string]any{})
	return err
}

func boundedTypingDwell(dwell time.Duration) (time.Duration, error) {
	if dwell <= 0 {
		return 0, usageErr(fmt.Errorf("typing dwell must be positive"))
	}
	if dwell > maxTypingDwell {
		return 0, usageErr(fmt.Errorf("typing dwell is capped at %s", maxTypingDwell))
	}
	return dwell, nil
}

func sleepForContext(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func typingProtocolWarning() string {
	return "typing indicators are iMessage-only, one-to-one only, auto-clear on send, and expire after about 60s; this CLI never loops indefinitely"
}

func sortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func auditRichLinkPreview(raw string, firstOutbound bool) map[string]any {
	errorsOut := []string{}
	warnings := []string{"rich previews render on iMessage/RCS; SMS falls back to a bare URL"}
	if len([]rune(raw)) > maxLinkValueChars {
		errorsOut = append(errorsOut, "URL exceeds 2,048 characters")
	}
	if !isHTTPSURL(raw) {
		errorsOut = append(errorsOut, "URL protocol must be HTTPS for preview generation")
	}
	if firstOutbound {
		errorsOut = append(errorsOut, "first outbound POST /v3/chats must not contain links; send a plain opener first, then a link preview follow-up")
	}
	return map[string]any{
		"url":      raw,
		"sendable": len(errorsOut) == 0,
		"errors":   errorsOut,
		"warnings": warnings,
		"rules": []string{
			"link part must be the only message part",
			"URL max length is 2,048 characters",
			"HTTPS is required for preview generation",
		},
	}
}

func fetchLinkPreviewMetadata(ctx context.Context, raw string) (map[string]any, error) {
	if !isHTTPSURL(raw) {
		return nil, usageErr(fmt.Errorf("metadata URL must be HTTPS"))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	html := string(body)
	meta := map[string]any{"url": raw, "http_status": resp.StatusCode}
	for _, match := range linkMetaTagRE.FindAllStringSubmatch(html, -1) {
		if len(match) < 2 {
			continue
		}
		key, value := linkMetaTagAttrs(match[1])
		switch key {
		case "og:title", "og:description", "og:image", "twitter:title", "twitter:description", "twitter:image", "description":
			meta[strings.ReplaceAll(strings.ReplaceAll(key, ":", "_"), "-", "_")] = value
		}
	}
	if match := linkTitleRE.FindStringSubmatch(html); len(match) > 1 {
		meta["title"] = strings.TrimSpace(htmlUnescape(match[1]))
	}
	if _, ok := meta["og_image"]; !ok {
		if match := linkImageRE.FindStringSubmatch(html); len(match) > 1 {
			meta["first_image"] = strings.TrimSpace(match[1])
		}
	}
	meta["has_preview_metadata"] = meta["og_title"] != nil || meta["twitter_title"] != nil || meta["title"] != nil
	return meta, nil
}

func htmlUnescape(s string) string {
	replacer := strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#34;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">")
	return replacer.Replace(s)
}

func linkMetaTagAttrs(attrs string) (string, string) {
	var key, content string
	for _, match := range linkMetaAttrRE.FindAllStringSubmatch(attrs, -1) {
		if len(match) < 3 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(match[1]))
		value := strings.TrimSpace(htmlUnescape(match[2]))
		switch name {
		case "property", "name":
			key = strings.ToLower(value)
		case "content":
			content = value
		}
	}
	return key, content
}

func attachmentPlan(file string) (map[string]any, error) {
	if strings.TrimSpace(file) == "" {
		return nil, usageErr(fmt.Errorf("--file is required"))
	}
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, usageErr(fmt.Errorf("--file must point to a file, not a directory"))
	}
	contentType := detectFileContentType(file)
	size := info.Size()
	method := "direct_url"
	if size > directAttachmentLimitBytes {
		method = "pre_upload"
	}
	return map[string]any{
		"file":                   file,
		"filename":               filepath.Base(file),
		"size_bytes":             size,
		"content_type":           contentType,
		"recommended_path":       method,
		"direct_url_allowed":     size <= directAttachmentLimitBytes,
		"upload_allowed":         size <= uploadAttachmentLimitBytes,
		"direct_url_limit_bytes": directAttachmentLimitBytes,
		"preupload_limit_bytes":  uploadAttachmentLimitBytes,
		"warnings":               []string{"direct URL sends require your own public HTTPS URL", "allowlist cdn.linqapp.com for Linq attachment responses", attachmentLifecycleWarning(), "for audio with native inline playback, use the voicememo endpoint instead of an attachment"},
	}, nil
}

func detectFileContentType(file string) string {
	if extType := mime.TypeByExtension(filepath.Ext(file)); extType != "" {
		return extType
	}
	f, err := os.Open(file)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return http.DetectContentType(buf[:n])
}

func putAttachmentFile(ctx context.Context, uploadURL string, preupload map[string]any, file, contentType string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, f)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if headers, ok := preupload["required_headers"].(map[string]any); ok {
		for key, value := range headers {
			req.Header.Set(key, fmt.Sprint(value))
		}
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upload PUT returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func attachmentLifecycleWarning() string {
	return "external URLs are fetched on every send; pre-uploaded attachment IDs are reusable, persistent by default, and ephemeral tiers may expire after 24h"
}

func optionalStringSlice(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
}

func validBuiltInReaction(reactionType string) bool {
	switch reactionType {
	case "love", "like", "dislike", "laugh", "emphasize", "question":
		return true
	}
	return false
}

func asciiTitle(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func reactionBody(operation, reactionType, emoji string, partIndex int, hasPartIndex bool) map[string]any {
	body := map[string]any{"operation": operation, "type": reactionType}
	if reactionType == "custom" {
		body["custom_emoji"] = emoji
	}
	if hasPartIndex {
		body["part_index"] = partIndex
	}
	return body
}

func contactSharePreflight(ctx context.Context, flags *rootFlags, chatID, from string) (map[string]any, error) {
	checks := map[string]string{
		"imessage_only":       "warn: native Name and Photo Sharing is iMessage-only",
		"once_per_day":        "warn: repeated calls within 24h do not prompt more than once",
		"native_not_vcf":      "info: this is iMessage Name and Photo Sharing, not a .vcf attachment",
		"prior_outbound_chat": "warn: local evidence unavailable; run sync first if this matters",
	}
	blocking := []string{}
	if !validateE164(from) {
		checks["from"] = "fail: must be E.164"
		blocking = append(blocking, "--from must be E.164")
	} else {
		checks["from"] = "pass"
	}
	if flags.dryRun {
		checks["contact_card"] = "skip: dry-run does not fetch /v3/contact_card"
		checks["active"] = "skip: dry-run does not fetch /v3/contact_card"
		return map[string]any{"chat_id": chatID, "from": from, "allowed": len(blocking) == 0, "blocking_reasons": blocking, "checks": checks}, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/v3/contact_card", map[string]string{})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var parsed any
	_ = json.Unmarshal(data, &parsed)
	card := findContactCardForFrom(parsed, from)
	if card == nil {
		checks["contact_card"] = "fail: no contact card found for from number"
		blocking = append(blocking, "no contact card exists for --from")
	} else {
		checks["contact_card"] = "pass"
		if isTruthy(card["is_active"]) {
			checks["active"] = "pass"
		} else {
			checks["active"] = "fail: contact card is not active"
			blocking = append(blocking, "contact card is not active")
		}
	}
	if messages, err := loadLocalMessages(250); err == nil {
		ev := localMessageEvidence(chatID, messages)
		if n, _ := ev["prior_outbound_messages"].(int); n > 0 {
			checks["prior_outbound_chat"] = "pass"
		} else {
			checks["prior_outbound_chat"] = "warn: local mirror has no prior outbound message for this chat"
		}
	}
	return map[string]any{"chat_id": chatID, "from": from, "allowed": len(blocking) == 0, "blocking_reasons": blocking, "checks": checks, "contact_card": card}, nil
}

func findContactCardForFrom(raw any, from string) map[string]any {
	switch v := raw.(type) {
	case map[string]any:
		if cardMatchesFrom(v, from) {
			return v
		}
		for _, key := range []string{"data", "items", "contact_cards", "contactCards"} {
			if found := findContactCardForFrom(v[key], from); found != nil {
				return found
			}
		}
	case []any:
		for _, item := range v {
			if found := findContactCardForFrom(item, from); found != nil {
				return found
			}
		}
	}
	return nil
}

func cardMatchesFrom(card map[string]any, from string) bool {
	for _, key := range []string{"from", "phone_number", "phoneNumber", "number"} {
		if strings.TrimSpace(fmt.Sprint(card[key])) == from {
			return true
		}
	}
	return false
}

func isTruthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true") || strings.EqualFold(strings.TrimSpace(t), "active")
	}
	return false
}

func anyStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		if raw == nil {
			return nil
		}
		return []string{fmt.Sprint(raw)}
	}
}

func firstNestedString(raw any, keys ...string) string {
	switch v := raw.(type) {
	case map[string]any:
		for _, key := range keys {
			if s := strings.TrimSpace(fmt.Sprint(v[key])); s != "" && s != "<nil>" {
				return s
			}
		}
		for _, value := range v {
			if s := firstNestedString(value, keys...); s != "" {
				return s
			}
		}
	case []any:
		for _, value := range v {
			if s := firstNestedString(value, keys...); s != "" {
				return s
			}
		}
	}
	return ""
}
