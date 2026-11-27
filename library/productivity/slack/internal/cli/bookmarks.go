// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func emitSlackReadJSON(cmd *cobra.Command, flags *rootFlags, data json.RawMessage) error {
	if flags.quiet {
		return nil
	}
	prov := DataProvenance{Source: "live"}
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		filtered := data
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		wrapped, wrapErr := wrapWithProvenance(filtered, prov)
		if wrapErr != nil {
			return wrapErr
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
}

func runSlackGetEndpoint(cmd *cobra.Command, flags *rootFlags, path string, params map[string]string) error {
	if flags.dataSource == "local" {
		return notFoundErr(fmt.Errorf("%s requires live Slack API access", path))
	}
	if flags.dryRun {
		return emitSlackReadJSON(cmd, flags, json.RawMessage(fmt.Sprintf(`{"dry_run":true,"method":"GET","path":%q}`, path)))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, err := c.Get(cmd.Context(), path, params)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if slackErr := checkSlackAPIError(data); slackErr != nil {
		return slackErr
	}
	return emitSlackReadJSON(cmd, flags, data)
}

func postBookmark(cmd *cobra.Command, flags *rootFlags, path string, body map[string]any) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, statusCode, err := c.Post(cmd.Context(), path, body)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if slackErr := checkSlackAPIError(data); slackErr != nil {
		return slackErr
	}
	if flags.quiet {
		return nil
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		filtered := data
		if flags.compact {
			filtered = compactFields(filtered)
		}
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		}
		envelope := map[string]any{
			"action":   "post",
			"resource": "bookmarks",
			"path":     path,
			"status":   statusCode,
			"success":  statusCode >= 200 && statusCode < 300,
		}
		if flags.dryRun {
			envelope["dry_run"] = true
			envelope["status"] = 0
			envelope["success"] = false
		}
		if len(data) > 0 {
			var rawParsed map[string]any
			if err := json.Unmarshal(data, &rawParsed); err == nil {
				if synthetic, _ := rawParsed["__pp_verify_synthetic__"].(bool); synthetic {
					envelope["verify_noop"] = true
					envelope["success"] = false
				}
			}
		}
		if len(filtered) > 0 {
			var parsed any
			if err := json.Unmarshal(filtered, &parsed); err == nil {
				envelope["data"] = parsed
			}
		}
		envelopeJSON, err := json.Marshal(envelope)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func newBookmarksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bookmarks",
		Short:       "List, add, edit, and remove Slack channel bookmarks",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBookmarksListCmd(flags))
	cmd.AddCommand(newBookmarksAddCmd(flags))
	cmd.AddCommand(newBookmarksEditCmd(flags))
	cmd.AddCommand(newBookmarksRemoveCmd(flags))
	return cmd
}

func newBookmarksListCmd(flags *rootFlags) *cobra.Command {
	var channelID string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List bookmarks in a channel",
		Example:     "  slack-pp-cli bookmarks list --channel C0123456789 --json",
		Annotations: map[string]string{"pp:endpoint": "bookmarks.list", "pp:method": "GET", "pp:path": "/bookmarks.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(channelID) == "" {
				return usageErr(fmt.Errorf("--channel is required"))
			}
			return runSlackGetEndpoint(cmd, flags, "/bookmarks.list", map[string]string{
				"channel_id": strings.TrimSpace(channelID),
			})
		},
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID to list bookmarks for")
	return cmd
}

func newBookmarksAddCmd(flags *rootFlags) *cobra.Command {
	var channelID, title, link, bookmarkType, emoji string
	cmd := &cobra.Command{
		Use:         "add",
		Short:       "Add a bookmark to a channel",
		Example:     "  slack-pp-cli bookmarks add --channel C0123456789 --title \"Runbook\" --link https://example.com/runbook",
		Annotations: map[string]string{"pp:endpoint": "bookmarks.add", "pp:method": "POST", "pp:path": "/bookmarks.add"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(channelID) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(link) == "" {
				return usageErr(fmt.Errorf("--channel, --title, and --link are required"))
			}
			bType := strings.TrimSpace(bookmarkType)
			if bType == "" {
				bType = "link"
			}
			body := map[string]any{
				"channel_id": strings.TrimSpace(channelID),
				"title":      strings.TrimSpace(title),
				"type":       bType,
				"link":       strings.TrimSpace(link),
			}
			if strings.TrimSpace(emoji) != "" {
				body["emoji"] = strings.TrimSpace(emoji)
			}
			return postBookmark(cmd, flags, "/bookmarks.add", body)
		},
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID")
	cmd.Flags().StringVar(&title, "title", "", "Bookmark title")
	cmd.Flags().StringVar(&link, "link", "", "Bookmark URL")
	cmd.Flags().StringVar(&bookmarkType, "type", "link", "Bookmark type (default: link)")
	cmd.Flags().StringVar(&emoji, "emoji", "", "Optional emoji tag (e.g. :book:)")
	return cmd
}

func newBookmarksEditCmd(flags *rootFlags) *cobra.Command {
	var channelID, bookmarkID, title, link, emoji string
	cmd := &cobra.Command{
		Use:         "edit",
		Short:       "Edit an existing channel bookmark",
		Example:     "  slack-pp-cli bookmarks edit --channel C0123456789 --bookmark-id Bk01234 --title \"Updated Runbook\"",
		Annotations: map[string]string{"pp:endpoint": "bookmarks.edit", "pp:method": "POST", "pp:path": "/bookmarks.edit"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(channelID) == "" || strings.TrimSpace(bookmarkID) == "" {
				return usageErr(fmt.Errorf("--channel and --bookmark-id are required"))
			}
			body := map[string]any{
				"channel_id":  strings.TrimSpace(channelID),
				"bookmark_id": strings.TrimSpace(bookmarkID),
			}
			if strings.TrimSpace(title) != "" {
				body["title"] = strings.TrimSpace(title)
			}
			if strings.TrimSpace(link) != "" {
				body["link"] = strings.TrimSpace(link)
			}
			if strings.TrimSpace(emoji) != "" {
				body["emoji"] = strings.TrimSpace(emoji)
			}
			return postBookmark(cmd, flags, "/bookmarks.edit", body)
		},
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID")
	cmd.Flags().StringVar(&bookmarkID, "bookmark-id", "", "Bookmark ID")
	cmd.Flags().StringVar(&title, "title", "", "New bookmark title")
	cmd.Flags().StringVar(&link, "link", "", "New bookmark URL")
	cmd.Flags().StringVar(&emoji, "emoji", "", "New emoji tag")
	return cmd
}

func newBookmarksRemoveCmd(flags *rootFlags) *cobra.Command {
	var channelID, bookmarkID string
	cmd := &cobra.Command{
		Use:         "remove",
		Short:       "Remove a bookmark from a channel",
		Example:     "  slack-pp-cli bookmarks remove --channel C0123456789 --bookmark-id Bk01234",
		Annotations: map[string]string{"pp:endpoint": "bookmarks.remove", "pp:method": "POST", "pp:path": "/bookmarks.remove", "mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(channelID) == "" || strings.TrimSpace(bookmarkID) == "" {
				return usageErr(fmt.Errorf("--channel and --bookmark-id are required"))
			}
			body := map[string]any{
				"channel_id":  strings.TrimSpace(channelID),
				"bookmark_id": strings.TrimSpace(bookmarkID),
			}
			return postBookmark(cmd, flags, "/bookmarks.remove", body)
		},
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID")
	cmd.Flags().StringVar(&bookmarkID, "bookmark-id", "", "Bookmark ID")
	return cmd
}
