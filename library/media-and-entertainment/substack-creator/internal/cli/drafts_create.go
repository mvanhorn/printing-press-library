// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-extended by Phase 5+ live discovery against the publication subdomain.
// Supports the full Substack draft field set so AI agents can author drafts
// (title, subtitle, body, audience, bylines, SEO, cover image, type, paywall,
// podcast/video metadata) over the same /api/v1/drafts endpoint.
//
// PATCH: drafts-full-fields — rewrote create + update to expose 30+ Substack
// draft fields. The generator emitted only title/subtitle/body/audience/
// section-id; without the rest, agents can't author real posts (no SEO, no
// cover image, no paywall config, no podcast fields). Also carries the
// prosemirror-converter integration for draft_body. Recorded in
// .printing-press-patches.json.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-creator/internal/client"

	"github.com/spf13/cobra"
)

// clientType is an alias to make the resolveBylines signature shorter.
type clientType = client.Client

func newDraftsCreateCmd(flags *rootFlags) *cobra.Command {
	var (
		title              string
		subtitle           string
		bodyInline         string
		bodyFile           string
		bodyJSON           string
		audience           string
		sectionID          string
		bylineIDs          []int64
		bylineFlag         string
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
		Use:   "create",
		Short: "Create a new draft with the full Substack field set.",
		Long: `Create a new draft. Supports every Substack field exposed by /api/v1/drafts:

Content:
  --title              Draft title (required unless --stdin)
  --subtitle           Draft subtitle
  --body <markdown>    Inline body — auto-converted to Substack's ProseMirror JSON
                       (paragraphs, # headings, [paywall] marker, --- horizontal rule)
  --body-file <path>   Read body from a file (markdown or ProseMirror JSON)
  --body-json <json>   Raw ProseMirror JSON document (full control, no conversion)

Authoring:
  --byline <id|email>  Author user-id (repeatable; defaults to the authenticated user)
  --type               newsletter | podcast | video | thread (default: newsletter)
  --section-id         Section ID to place the draft under

Audience & paywall:
  --audience           everyone | only_paid | founding
  --meter-type         none | etc
  --send-free-preview  When publishing, send the free preview to free subs
  --exempt-paywall     Exempt this post from archive paywall
  --free-unlock        Allow free unlock

SEO & social:
  --description        Short description (shown in archive)
  --cover-image        Cover image URL
  --cover-square       Treat cover image as square
  --seo-title          Search engine title
  --seo-description    Search engine description
  --social-title       Social-share title

Discussion:
  --comment-sort       best_first | most_recent | most_liked
  --comment-perms      everyone | only_paid | none

Visibility:
  --hide-from-feed     Hide from the publication's main feed
  --hidden             Draft visible only to the author
  --show-guest-bios    Show guest byline bios

Podcast / video:
  --podcast-url        Podcast audio URL
  --podcast-duration   Episode duration in seconds
  --podcast-episode-number / --podcast-season-number / --podcast-episode-type
  --free-podcast-url   Free preview audio URL
  --free-podcast-duration
  --video-url          Video upload URL
  --voiceover-url      Voiceover audio URL

Stdin:
  --stdin              Read the full draft body JSON from stdin (bypasses other flags)

Requires --subdomain <publication-subdomain>. The authenticated user's user_id is
auto-resolved via /subscriptions/page_v2 when --byline is not provided.`,
		Example: `  # Minimal create — title + markdown body, byline auto-resolved
  substack-creator-pp-cli drafts create --subdomain mypub-paid --title "Test draft" --body "First paragraph.

Second paragraph."

  # Full agent-friendly create — paid-only post with SEO + cover image
  substack-creator-pp-cli drafts create --subdomain mypub-paid --json \
    --title "Why X matters" --subtitle "A short analysis" \
    --body-file ./post.md --audience only_paid \
    --description "How X affects Y" \
    --cover-image https://substackcdn.com/.../cover.jpg \
    --seo-title "X explained" --seo-description "..."

  # Raw ProseMirror JSON for full control
  substack-creator-pp-cli drafts create --subdomain mypub-paid \
    --title "Custom doc" --body-json '{"type":"doc","content":[...]}'

  # Full body via stdin (legacy mode)
  echo '{"draft_title":"X","draft_bylines":[{"id":1234}], ...}' | \
    substack-creator-pp-cli drafts create --subdomain mypub-paid --stdin`,
		Annotations: map[string]string{
			"pp:endpoint":   "drafts.create",
			"pp:method":     "POST",
			"pp:path":       "/drafts",
			"pp:novel-ext":  "full-field-coverage",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("title") && !flags.dryRun {
					return usageErr(fmt.Errorf("--title is required (or use --stdin for raw body)"))
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/drafts"
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

				// Convert body inputs to ProseMirror JSON
				draftBody, err := buildDraftBody(bodyInline, bodyFile, bodyJSON)
				if err != nil {
					return usageErr(err)
				}

				// Required Substack field naming convention: draft_*
				body["draft_title"] = title
				if subtitle != "" {
					body["draft_subtitle"] = subtitle
				}
				if draftBody != "" {
					body["draft_body"] = draftBody
				}
				if sectionID != "" {
					body["draft_section_id"] = sectionID
				}
				if audience != "" {
					body["audience"] = audience
				}
				body["type"] = postType

				// Bylines: required by Substack. Auto-resolve if not provided.
				bylines, err := resolveBylines(c, bylineIDs, bylineFlag, flags.dryRun)
				if err != nil {
					return err
				}
				body["draft_bylines"] = bylines

				// Optional content fields
				if description != "" {
					body["description"] = description
				}
				if coverImage != "" {
					body["cover_image"] = coverImage
				}
				if cmd.Flags().Changed("cover-square") {
					body["cover_image_is_square"] = coverImageSquare
				}
				if cmd.Flags().Changed("cover-explicit") {
					body["cover_image_is_explicit"] = coverImageExplicit
				}
				if seoTitle != "" {
					body["search_engine_title"] = seoTitle
				}
				if seoDescription != "" {
					body["search_engine_description"] = seoDescription
				}
				if socialTitle != "" {
					body["social_title"] = socialTitle
				}
				if meterType != "" {
					body["meter_type"] = meterType
				}
				if commentSort != "" {
					body["default_comment_sort"] = commentSort
				}
				if commentPerms != "" {
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

				// Podcast fields
				if podcastURL != "" {
					body["draft_podcast_url"] = podcastURL
				}
				if podcastDuration > 0 {
					body["draft_podcast_duration"] = podcastDuration
				}
				if podcastEpNum > 0 {
					body["podcast_episode_number"] = podcastEpNum
				}
				if podcastSeasonNum > 0 {
					body["podcast_season_number"] = podcastSeasonNum
				}
				if podcastEpType != "" {
					body["podcast_episode_type"] = podcastEpType
				}
				if freePodcastURL != "" {
					body["free_podcast_url"] = freePodcastURL
				}
				if freePodcastDur > 0 {
					body["free_podcast_duration"] = freePodcastDur
				}
				if videoURL != "" {
					body["draft_video_upload_id"] = videoURL
				}
				if voiceoverURL != "" {
					body["draft_voiceover_upload_id"] = voiceoverURL
				}
			}

			data, statusCode, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Envelope output
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
					"action":   "post",
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

	cmd.Flags().StringVar(&title, "title", "", "Draft title (required)")
	cmd.Flags().StringVar(&subtitle, "subtitle", "", "Draft subtitle")
	cmd.Flags().StringVar(&bodyInline, "body", "", "Body content (markdown — converted to ProseMirror JSON)")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to body file (markdown or ProseMirror JSON)")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Raw ProseMirror JSON document for draft_body")
	cmd.Flags().StringVar(&audience, "audience", "everyone", "Audience: everyone | only_paid | founding")
	cmd.Flags().StringVar(&sectionID, "section-id", "", "Section ID to place the draft under")
	cmd.Flags().Int64SliceVar(&bylineIDs, "byline", nil, "Byline user IDs (repeatable). Defaults to authenticated user.")
	cmd.Flags().StringVar(&bylineFlag, "bylines", "", "Bylines as JSON array, e.g. '[{\"id\":123},{\"id\":456}]'")
	cmd.Flags().StringVar(&coverImage, "cover-image", "", "Cover image URL")
	cmd.Flags().BoolVar(&coverImageSquare, "cover-square", false, "Cover image is square")
	cmd.Flags().BoolVar(&coverImageExplicit, "cover-explicit", false, "Cover image is explicit")
	cmd.Flags().StringVar(&description, "description", "", "Short description shown in archive")
	cmd.Flags().StringVar(&seoTitle, "seo-title", "", "Search engine title")
	cmd.Flags().StringVar(&seoDescription, "seo-description", "", "Search engine description")
	cmd.Flags().StringVar(&socialTitle, "social-title", "", "Social-share title")
	cmd.Flags().StringVar(&postType, "type", "newsletter", "Post type: newsletter | podcast | video | thread")
	cmd.Flags().StringVar(&meterType, "meter-type", "", "Paywall meter type: none | etc")
	cmd.Flags().StringVar(&commentSort, "comment-sort", "", "Default comment sort: best_first | most_recent | most_liked")
	cmd.Flags().StringVar(&commentPerms, "comment-perms", "", "Write comment permissions: everyone | only_paid | none")
	cmd.Flags().BoolVar(&sendFreePreview, "send-free-preview", true, "Send free preview to free subscribers at publish time")
	cmd.Flags().BoolVar(&showGuestBios, "show-guest-bios", true, "Show guest byline bios")
	cmd.Flags().BoolVar(&hideFromFeed, "hide-from-feed", false, "Hide from the publication's main feed")
	cmd.Flags().BoolVar(&isDraftHidden, "hidden", false, "Draft visible only to the author")
	cmd.Flags().BoolVar(&exemptPaywall, "exempt-paywall", false, "Exempt this post from the archive paywall")
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
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read full request body as JSON from stdin (bypasses other flags)")

	return cmd
}

// resolveBylines builds the draft_bylines array. Priority:
// 1. --bylines JSON (raw array, used verbatim)
// 2. --byline (repeatable ints, formatted as [{"id":N}, ...])
// 3. Auto-resolve via /subscriptions/page_v2 (default)
func resolveBylines(c *clientType, ids []int64, raw string, dryRun bool) ([]map[string]any, error) {
	if raw != "" {
		var parsed []map[string]any
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return nil, fmt.Errorf("--bylines is not a valid JSON array: %w", err)
		}
		return parsed, nil
	}
	if len(ids) > 0 {
		out := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, map[string]any{"id": id})
		}
		return out, nil
	}
	if dryRun {
		return []map[string]any{{"id": "<auto-resolved-at-runtime>"}}, nil
	}
	uid, err := resolveOwnUserID(c)
	if err != nil {
		return nil, err
	}
	return []map[string]any{{"id": uid}}, nil
}

// keep unused imports referenced when refactoring removed inner adapters
var _ = strings.TrimSpace
var _ = strconv.ParseInt
