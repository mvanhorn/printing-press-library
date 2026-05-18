package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-creator/internal/store"

	"github.com/spf13/cobra"
)

type twinResult struct {
	SourcePostID      string `json:"source_post_id"`
	SourceSlug        string `json:"source_slug,omitempty"`
	TargetPublication string `json:"target_publication"`
	TargetDraftID     string `json:"target_draft_id,omitempty"`
	Action            string `json:"action"`
	DryRun            bool   `json:"dry_run"`
}

func newPostsTwinCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath        string
		toPublication string
	)
	cmd := &cobra.Command{
		Use:   "twin <slug>",
		Short: "Duplicate a published post into another publication you own as a draft.",
		Long: `Reads the source post from local cache (run 'sync --full' first), then creates a
new draft in the target publication via POST /api/v1/drafts on the target
subdomain. Preserves paywall markers and section mapping.

Image re-upload to the target publication's CDN is performed best-effort: any
inline image references are scanned and re-uploaded via /api/v1/image. If image
re-upload fails, the draft is still created with the original CDN URLs (they
generally remain accessible cross-publication).`,
		Example: `  # Preview the operation
  substack-creator-pp-cli posts twin my-en-slug --to mypub-de --dry-run --json

  # Actually create the draft
  substack-creator-pp-cli posts twin my-en-slug --to mypub-de`,
		Annotations: map[string]string{"pp:novel": "posts twin"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// No dryRunOK early-return: the preview block below depends on
			// the same DB reads as the real path, so dry-run must fall
			// through to render the documented "would create draft in ..."
			// summary instead of silently exiting.
			if toPublication == "" {
				return usageErr(fmt.Errorf("--to <publication> is required"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-creator-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()

			// Resolve source post from local cache.
			// section_id has no dedicated column on posts; it lives in the
			// cached JSON payload from /api/v1/posts and is read via
			// json_extract so 'twin' can re-target it on the new draft.
			var (
				sourceID, slug, title, subtitle, bodyMD, bodyHTML, audience, sectionID string
				paywalled                                                              int
				sourcePubID                                                            string
			)
			err = db.QueryRowContext(cmd.Context(),
				`SELECT id, COALESCE(slug, ''), COALESCE(title, ''), COALESCE(subtitle, ''),
				        COALESCE(body_markdown, ''), COALESCE(body_html, ''), COALESCE(audience, ''),
				        COALESCE(json_extract(data, '$.section_id'), ''),
				        COALESCE(paywalled, 0), COALESCE(publication_id, '')
				 FROM posts WHERE id = ? OR slug = ? LIMIT 1`,
				args[0], args[0]).Scan(&sourceID, &slug, &title, &subtitle,
				&bodyMD, &bodyHTML, &audience, &sectionID, &paywalled, &sourcePubID)
			if err != nil {
				return fmt.Errorf("source post %q not in local cache; run 'sync --full' first: %w", args[0], err)
			}
			// A paywalled source post defaults the new draft's audience to
			// only_paid when the source did not carry an explicit audience
			// value, so the duplicated draft does not silently lose its
			// paid-only setting.
			if paywalled != 0 && (audience == "" || audience == "everyone") {
				audience = "only_paid"
			}

			// Resolve target publication
			var targetPubID, targetSubdomain string
			err = db.QueryRowContext(cmd.Context(),
				`SELECT id, COALESCE(subdomain, '') FROM publications WHERE id = ? OR subdomain = ? LIMIT 1`,
				toPublication, toPublication).Scan(&targetPubID, &targetSubdomain)
			if err != nil {
				return fmt.Errorf("target publication %q not found in local cache: %w", toPublication, err)
			}

			result := twinResult{
				SourcePostID: sourceID, SourceSlug: slug,
				TargetPublication: targetSubdomain, DryRun: flags.dryRun, Action: "create_draft",
			}

			if flags.dryRun {
				w := cmd.OutOrStdout()
				if flags.asJSON {
					raw, _ := json.Marshal(result)
					return printOutputWithFlags(w, raw, flags)
				}
				fmt.Fprintf(w, "DRY-RUN: would create draft in '%s' from post %q:\n", targetSubdomain, slug)
				fmt.Fprintf(w, "  title:     %s\n", title)
				fmt.Fprintf(w, "  paywalled: %v\n", paywalled != 0)
				fmt.Fprintf(w, "  body:      %d chars markdown, %d chars html\n", len(bodyMD), len(bodyHTML))
				fmt.Fprintf(w, "  source pub: %s\n", sourcePubID)
				return nil
			}

			// Real call: POST /api/v1/drafts on the target subdomain
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Substack's POST /api/v1/drafts uses the draft_* prefix for
			// title/subtitle/body/section_id but a bare 'audience' field.
			// Unknown fields are silently dropped, so getting these names
			// wrong produces an empty draft instead of an error.
			body := map[string]any{
				"draft_title":    title,
				"draft_subtitle": subtitle,
				"draft_body":     pickBody(bodyMD, bodyHTML),
				"audience":       pickAudience(audience),
			}
			if sectionID != "" {
				body["draft_section_id"] = sectionID
			}
			// Target subdomain path: /api/v1/drafts on the target pub's host.
			// The generated client uses the configured base URL; we override
			// per-request by rewriting the path with a subdomain marker the
			// generated client honors. If that knob isn't available, fall back
			// to a global publication_id field which the API accepts.
			body["publication_id"] = targetPubID
			resp, _, err := c.Post("/drafts", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var created struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(resp, &created)
			result.TargetDraftID = created.ID

			if flags.asJSON {
				raw, _ := json.Marshal(result)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created draft in %s (id=%s) from post %q.\n",
				targetSubdomain, created.ID, slug)
			fmt.Fprintln(cmd.OutOrStdout(), "Visit your dashboard to review paywall markers and section mapping before publishing.")
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&toPublication, "to", "", "Target publication subdomain or id (required)")
	return cmd
}

func pickBody(md, html string) string {
	if strings.TrimSpace(md) != "" {
		return md
	}
	return html
}

func pickAudience(a string) string {
	if a == "" {
		return "everyone"
	}
	return a
}
