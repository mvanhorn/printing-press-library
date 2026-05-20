// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// PATCH: feat-comments-and-handle-resolution — hand-authored typed endpoint for /youtube/v3/commentThreads.list. Spec entry was added to spec.yaml; rather than running a full regen (which would clobber polish fixes in videos_related.go), the typed handler is hand-authored to mirror the videos-list pattern.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newYoutubeCommentThreadsListCmd(flags *rootFlags) *cobra.Command {
	var flagVideoId string
	var flagChannelId string
	var flagId string
	var flagOrder string
	var flagSearchTerms string
	var flagTextFormat string
	var flagPart string
	var flagMaxResults int
	var flagAll bool

	cmd := &cobra.Command{
		Use:         "comment-threads-list",
		Short:       "Retrieves a list of top-level comment threads, filterable by video, channel, or thread id.",
		Example:     "  youtube-pp-cli youtube comment-threads-list --video-id dQw4w9WgXcQ --max-results 20 --order relevance",
		Annotations: map[string]string{"pp:endpoint": "youtube.commentThreads-list", "pp:method": "GET", "pp:path": "/youtube/v3/commentThreads", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("order") {
				allowedOrder := []string{"orderUnspecified", "time", "relevance"}
				validOrder := false
				for _, v := range allowedOrder {
					if flagOrder == v {
						validOrder = true
						break
					}
				}
				if !validOrder {
					fmt.Fprintf(os.Stderr, "warning: --%s %q not in allowed set %v\n", "order", flagOrder, allowedOrder)
				}
			}
			if cmd.Flags().Changed("text-format") {
				allowedTextFormat := []string{"textFormatUnspecified", "html", "plainText"}
				validTextFormat := false
				for _, v := range allowedTextFormat {
					if flagTextFormat == v {
						validTextFormat = true
						break
					}
				}
				if !validTextFormat {
					fmt.Fprintf(os.Stderr, "warning: --%s %q not in allowed set %v\n", "text-format", flagTextFormat, allowedTextFormat)
				}
			}
			if flagVideoId == "" && flagChannelId == "" && flagId == "" {
				return usageErr(fmt.Errorf("one of --video-id, --channel-id, or --id is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/youtube/v3/commentThreads"
			data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "youtube", path, map[string]string{
				"videoId":     fmt.Sprintf("%v", flagVideoId),
				"channelId":   fmt.Sprintf("%v", flagChannelId),
				"id":          fmt.Sprintf("%v", flagId),
				"order":       fmt.Sprintf("%v", flagOrder),
				"searchTerms": fmt.Sprintf("%v", flagSearchTerms),
				"textFormat":  fmt.Sprintf("%v", flagTextFormat),
				"part":        flagPart,
				"maxResults":  fmt.Sprintf("%d", flagMaxResults),
			}, nil, flagAll, "pagetoken", "nextPageToken", "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
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
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagVideoId, "video-id", "", "Return comment threads on the specified video.")
	cmd.Flags().StringVar(&flagChannelId, "channel-id", "", "Return top-level channel-page comment threads on the specified channel.")
	cmd.Flags().StringVar(&flagId, "id", "", "Comma-separated list of comment thread IDs to fetch directly.")
	cmd.Flags().StringVar(&flagOrder, "order", "", "Sort order: time or relevance.")
	cmd.Flags().StringVar(&flagSearchTerms, "search-terms", "", "Limit threads to those whose top-level comment matches these search terms.")
	cmd.Flags().StringVar(&flagTextFormat, "text-format", "", "Format of returned comment text: html or plainText.")
	cmd.Flags().StringVar(&flagPart, "part", "snippet", "Comma-separated parts to include (e.g. snippet, replies).")
	cmd.Flags().IntVar(&flagMaxResults, "max-results", 25, "Maximum number of results to return per page (1-100).")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")

	return cmd
}
