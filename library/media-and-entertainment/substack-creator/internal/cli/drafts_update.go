// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-extended (Phase 5+): exposes the full Substack field set for PUT /drafts/{id}.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newDraftsUpdateCmd(flags *rootFlags) *cobra.Command {
	var (
		title              string
		subtitle           string
		bodyInline         string
		bodyFile           string
		bodyJSON           string
		audience           string
		sectionID          string
		coverImage         string
		coverImageSquare   bool
		coverImageExplicit bool
		description        string
		seoTitle           string
		seoDescription     string
		socialTitle        string
		postType           string
		meterType          string
		commentSort        string
		commentPerms       string
		sendFreePreview    bool
		showGuestBios      bool
		hideFromFeed       bool
		isDraftHidden      bool
		exemptPaywall      bool
		freeUnlock         bool
		podcastEpNum       int
		podcastSeasonNum   int
		podcastEpType      string
		podcastURL         string
		podcastDuration    int
		freePodcastURL     string
		freePodcastDur     int
		videoURL           string
		voiceoverURL       string
		stdinBody          bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing draft.",
		Long: `Update an existing draft. All flags are optional — only changed fields are sent.
Supports the same field set as 'drafts create': content, audience, SEO, podcast
metadata, comment settings, visibility flags. See 'drafts create --help' for the
full field list.`,
		Example: `  substack-creator-pp-cli drafts update 12345 --subdomain mypub-paid --title "New title"
  substack-creator-pp-cli drafts update 12345 --subdomain mypub-paid --body-file ./post.md --audience only_paid`,
		Annotations: map[string]string{
			"pp:endpoint":  "drafts.update",
			"pp:method":    "PUT",
			"pp:path":      "/drafts/{id}",
			"pp:novel-ext": "full-field-coverage",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/drafts/{id}"
			path = replacePathParam(path, "id", args[0])
			var body map[string]any

			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
			} else {
				body = map[string]any{}
				if cmd.Flags().Changed("title") {
					body["draft_title"] = title
				}
				if cmd.Flags().Changed("subtitle") {
					body["draft_subtitle"] = subtitle
				}
				if cmd.Flags().Changed("body") || cmd.Flags().Changed("body-file") || cmd.Flags().Changed("body-json") {
					draftBody, err := buildDraftBody(bodyInline, bodyFile, bodyJSON)
					if err != nil {
						return usageErr(err)
					}
					if draftBody != "" {
						body["draft_body"] = draftBody
					}
				}
				if cmd.Flags().Changed("audience") {
					body["audience"] = audience
				}
				if cmd.Flags().Changed("section-id") {
					body["draft_section_id"] = sectionID
				}
				if cmd.Flags().Changed("type") {
					body["type"] = postType
				}
				if cmd.Flags().Changed("description") {
					body["description"] = description
				}
				if cmd.Flags().Changed("cover-image") {
					body["cover_image"] = coverImage
				}
				if cmd.Flags().Changed("cover-square") {
					body["cover_image_is_square"] = coverImageSquare
				}
				if cmd.Flags().Changed("cover-explicit") {
					body["cover_image_is_explicit"] = coverImageExplicit
				}
				if cmd.Flags().Changed("seo-title") {
					body["search_engine_title"] = seoTitle
				}
				if cmd.Flags().Changed("seo-description") {
					body["search_engine_description"] = seoDescription
				}
				if cmd.Flags().Changed("social-title") {
					body["social_title"] = socialTitle
				}
				if cmd.Flags().Changed("meter-type") {
					body["meter_type"] = meterType
				}
				if cmd.Flags().Changed("comment-sort") {
					body["default_comment_sort"] = commentSort
				}
				if cmd.Flags().Changed("comment-perms") {
					body["write_comment_permissions"] = commentPerms
				}
				if cmd.Flags().Changed("send-free-preview") {
					body["should_send_free_preview"] = sendFreePreview
				}
				if cmd.Flags().Changed("show-guest-bios") {
					body["show_guest_bios"] = showGuestBios
				}
				if cmd.Flags().Changed("hide-from-feed") {
					body["hide_from_feed"] = hideFromFeed
				}
				if cmd.Flags().Changed("hidden") {
					body["is_draft_hidden"] = isDraftHidden
				}
				if cmd.Flags().Changed("exempt-paywall") {
					body["exempt_from_archive_paywall"] = exemptPaywall
				}
				if cmd.Flags().Changed("free-unlock") {
					body["free_unlock_required"] = freeUnlock
				}
				if cmd.Flags().Changed("podcast-url") {
					body["draft_podcast_url"] = podcastURL
				}
				if cmd.Flags().Changed("podcast-duration") {
					body["draft_podcast_duration"] = podcastDuration
				}
				if cmd.Flags().Changed("podcast-episode-number") {
					body["podcast_episode_number"] = podcastEpNum
				}
				if cmd.Flags().Changed("podcast-season-number") {
					body["podcast_season_number"] = podcastSeasonNum
				}
				if cmd.Flags().Changed("podcast-episode-type") {
					body["podcast_episode_type"] = podcastEpType
				}
				if cmd.Flags().Changed("free-podcast-url") {
					body["free_podcast_url"] = freePodcastURL
				}
				if cmd.Flags().Changed("free-podcast-duration") {
					body["free_podcast_duration"] = freePodcastDur
				}
				if cmd.Flags().Changed("video-url") {
					body["draft_video_upload_id"] = videoURL
				}
				if cmd.Flags().Changed("voiceover-url") {
					body["draft_voiceover_upload_id"] = voiceoverURL
				}

				if len(body) == 0 {
					return usageErr(fmt.Errorf("at least one field flag must be supplied (or use --stdin)"))
				}
			}

			data, statusCode, err := c.Put(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "put",
					"resource": "drafts",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, _ := json.Marshal(envelope)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&subtitle, "subtitle", "", "New subtitle")
	cmd.Flags().StringVar(&bodyInline, "body", "", "New body (markdown — converted to ProseMirror JSON)")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to body file (markdown or ProseMirror JSON)")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Raw ProseMirror JSON document")
	cmd.Flags().StringVar(&audience, "audience", "", "Audience: everyone | only_paid | founding")
	cmd.Flags().StringVar(&sectionID, "section-id", "", "Section ID")
	cmd.Flags().StringVar(&coverImage, "cover-image", "", "Cover image URL")
	cmd.Flags().BoolVar(&coverImageSquare, "cover-square", false, "Cover image is square")
	cmd.Flags().BoolVar(&coverImageExplicit, "cover-explicit", false, "Cover image is explicit")
	cmd.Flags().StringVar(&description, "description", "", "Short description")
	cmd.Flags().StringVar(&seoTitle, "seo-title", "", "Search engine title")
	cmd.Flags().StringVar(&seoDescription, "seo-description", "", "Search engine description")
	cmd.Flags().StringVar(&socialTitle, "social-title", "", "Social-share title")
	cmd.Flags().StringVar(&postType, "type", "", "Post type: newsletter | podcast | video | thread")
	cmd.Flags().StringVar(&meterType, "meter-type", "", "Paywall meter type")
	cmd.Flags().StringVar(&commentSort, "comment-sort", "", "Default comment sort")
	cmd.Flags().StringVar(&commentPerms, "comment-perms", "", "Write comment permissions")
	cmd.Flags().BoolVar(&sendFreePreview, "send-free-preview", false, "Send free preview to free subscribers at publish time")
	cmd.Flags().BoolVar(&showGuestBios, "show-guest-bios", false, "Show guest byline bios")
	cmd.Flags().BoolVar(&hideFromFeed, "hide-from-feed", false, "Hide from main feed")
	cmd.Flags().BoolVar(&isDraftHidden, "hidden", false, "Visible only to the author")
	cmd.Flags().BoolVar(&exemptPaywall, "exempt-paywall", false, "Exempt from archive paywall")
	cmd.Flags().BoolVar(&freeUnlock, "free-unlock", false, "Allow free unlock")
	cmd.Flags().IntVar(&podcastEpNum, "podcast-episode-number", 0, "Podcast episode number")
	cmd.Flags().IntVar(&podcastSeasonNum, "podcast-season-number", 0, "Podcast season number")
	cmd.Flags().StringVar(&podcastEpType, "podcast-episode-type", "", "Podcast episode type")
	cmd.Flags().StringVar(&podcastURL, "podcast-url", "", "Podcast audio URL")
	cmd.Flags().IntVar(&podcastDuration, "podcast-duration", 0, "Podcast duration in seconds")
	cmd.Flags().StringVar(&freePodcastURL, "free-podcast-url", "", "Free podcast preview URL")
	cmd.Flags().IntVar(&freePodcastDur, "free-podcast-duration", 0, "Free podcast preview duration in seconds")
	cmd.Flags().StringVar(&videoURL, "video-url", "", "Video upload ID/URL")
	cmd.Flags().StringVar(&voiceoverURL, "voiceover-url", "", "Voiceover audio upload ID")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read full request body as JSON from stdin")

	return cmd
}
