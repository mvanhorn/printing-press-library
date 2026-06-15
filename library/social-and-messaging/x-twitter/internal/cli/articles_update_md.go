// PATCH: articles update-md — first-class in-place X Article draft update.
//
// Updates an existing draft's title, body, and inline images from a markdown
// file by reusing the create path's proven GraphQL composition (UpdateTitle +
// media binding + UpdateContent with {content_state: {blocks, entity_map},
// article_entity}). UpdateContent replaces the ENTIRE body: before writing,
// the command preflights the target draft's content_state and refuses when it
// finds entity types the markdown converter cannot reproduce (composer-only
// artifacts would be silently destroyed), unless --replace-unknown-entities
// is passed. Draft-only by default; --republish opts into the
// Unpublish -> UpdateContent -> Publish sequence required for published
// articles.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
	"github.com/spf13/cobra"
)

// converterEntityEmitSet lists the entity types MarkdownBodyToDraftJS can
// emit. The update preflight treats anything outside this set as a
// composer-only artifact that a full-body replacement would destroy.
func converterEntityEmitSet() map[string]bool {
	return map[string]bool{
		"LINK":     true,
		"MARKDOWN": true,
		"MEDIA":    true,
		"TWEET":    true,
		"DIVIDER":  true,
	}
}

// articleUpdateDeps carries the injectable client surface so tests run with
// fakes and no network.
type articleUpdateDeps struct {
	post        articleGraphQLPoster
	fetchSlice  func(ctx context.Context, lifecycle string) (json.RawMessage, error)
	uploadImage func(path string) (string, error)
}

type articleUpdateOptions struct {
	articleID              string
	title                  string
	coverPath              string
	contentState           draftContentState
	republish              bool
	replaceUnknownEntities bool
}

type articleUpdateResult struct {
	ArticleID    string `json:"article_id"`
	URL          string `json:"url"`
	Title        string `json:"title,omitempty"`
	CoverMediaID string `json:"cover_media_id,omitempty"`
	Lifecycle    string `json:"lifecycle"`
	Republished  bool   `json:"republished"`
}

func newArticlesUpdateMdCmd(flags *rootFlags) *cobra.Command {
	var articleID string
	var republish bool
	var replaceUnknownEntities bool
	cmd := &cobra.Command{
		Use:   "update-md <markdown-file>",
		Short: "Update an existing X Article draft in place from a markdown file (replaces the whole body)",
		Long: "Parses frontmatter (title, cover) and body, converts the body to Draft.js content_state, uploads any new inline images, " +
			"and rewrites the target draft via ArticleEntityUpdateTitle + ArticleEntityUpdateContent + ArticleEntityUpdateCoverMedia when cover is present.\n\n" +
			"WARNING: this REPLACES THE ENTIRE BODY of the target draft. Anything added in the composer that the markdown " +
			"converter cannot reproduce (polls, composer-only embeds, unknown entity types) is destroyed by the update. The " +
			"command preflights the draft's content_state and refuses when it finds such entity types unless " +
			"--replace-unknown-entities is passed.\n\n" +
			"Draft-only by default: published articles are refused unless --republish is passed, which runs the " +
			"Unpublish -> UpdateContent -> Publish sequence (the article is briefly unpublished).",
		Example: "  x-twitter-pp-cli articles update-md draft.md --article-id 1750000000000000000",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Dry-run probes call with no file arg: short-circuit before
				// the required-arg check so verify can exercise the command.
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			parsed, err := ParseArticleMarkdown(string(data))
			if err != nil {
				return err
			}
			cs := MarkdownBodyToDraftJS(parsed.Body)
			if flags.dryRun {
				payload := map[string]any{
					"article_id":    articleID,
					"title":         parsed.Frontmatter.Title,
					"cover":         parsed.Frontmatter.Cover,
					"content_state": cs,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "── Article update payload (dry-run, no API call) ──")
					enc.SetIndent("", "  ")
				}
				return enc.Encode(payload)
			}
			if strings.TrimSpace(articleID) == "" {
				return usageErr(fmt.Errorf("--article-id is required (find it via `articles list --agent`)"))
			}
			if os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				fmt.Fprintln(cmd.OutOrStdout(), "verify-env: skipping article update")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			deps := articleUpdateDeps{
				post: c,
				fetchSlice: func(ctx context.Context, lifecycle string) (json.RawMessage, error) {
					return fetchArticleSlice(ctx, c, lifecycle)
				},
				uploadImage: func(p string) (string, error) {
					return c.UploadArticleImage(cmd.Context(), p)
				},
			}
			result, err := updateMarkdownArticle(cmd.Context(), deps, articleUpdateOptions{
				articleID:              articleID,
				title:                  parsed.Frontmatter.Title,
				coverPath:              parsed.Frontmatter.Cover,
				contentState:           cs,
				republish:              republish,
				replaceUnknownEntities: replaceUnknownEntities,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			verb := "updated draft"
			if result.Republished {
				verb = "updated and republished"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s article %s\n%s\n", verb, result.ArticleID, result.URL)
			return nil
		},
	}
	cmd.Flags().StringVar(&articleID, "article-id", "", "rest_id of the existing article to update (required; find it via articles list)")
	cmd.Flags().BoolVar(&republish, "republish", false, "Allow updating a published article via Unpublish -> UpdateContent -> Publish (briefly unpublishes it)")
	cmd.Flags().BoolVar(&replaceUnknownEntities, "replace-unknown-entities", false, "Proceed even when the target draft contains entity types the converter cannot reproduce (destroys them)")
	return cmd
}

// updateMarkdownArticle drives the in-place update against injected deps.
// Order on the wire: preflight read, optional Unpublish (republish only),
// UpdateTitle (when a title is present), image upload + MEDIA binding,
// UpdateContent, optional Publish (republish only).
func updateMarkdownArticle(ctx context.Context, deps articleUpdateDeps, opts articleUpdateOptions) (*articleUpdateResult, error) {
	if strings.TrimSpace(opts.articleID) == "" {
		return nil, usageErr(fmt.Errorf("--article-id is required (find it via `articles list --agent`)"))
	}

	scan, err := preflightArticleEntities(ctx, deps, opts.articleID)
	if err != nil {
		return nil, err
	}
	if len(scan.unknownTypes) > 0 && !opts.replaceUnknownEntities {
		return nil, fmt.Errorf("article %s contains entity types the markdown converter cannot reproduce (%s); update-md replaces the whole body and would destroy them. Pass --replace-unknown-entities to overwrite anyway, or edit in the composer",
			opts.articleID, strings.Join(scan.unknownTypes, ", "))
	}
	if scan.lifecycle == "Published" && !opts.republish {
		return nil, fmt.Errorf("article %s is published; pass --republish to run Unpublish -> UpdateContent -> Publish (it is briefly unpublished), or edit a draft instead", opts.articleID)
	}

	if opts.republish && scan.lifecycle != "Published" {
		// PATCH: --republish on a non-Published article would fall through to
		// Unpublish (likely a no-op) + Publish, silently publishing a draft
		// (Greptile P1 on PR #1141). Restrict the republish lane to Published.
		return nil, fmt.Errorf("article %s is not published (lifecycle %q); --republish only applies to published articles. Drop the flag to update the draft in place", opts.articleID, scan.lifecycle)
	}
	if opts.republish {
		if err := unpublishArticleEntity(ctx, deps.post, opts.articleID); err != nil {
			return nil, err
		}
	}

	// PATCH: once unpublished, a mid-sequence failure must not strand the
	// article in the unpublished state (Greptile P1 on PR #1141). On any
	// failure between Unpublish and the final Publish, attempt a best-effort
	// restore publish and report both outcomes.
	restoreOnErr := func(err error) error {
		if !opts.republish {
			return err
		}
		if _, pubErr := publishArticleEntity(ctx, deps.post, opts.articleID); pubErr != nil {
			return fmt.Errorf("%w; restore publish also failed (%v) — article %s is left UNPUBLISHED, re-publish it in the composer or via `articles publish`", err, pubErr, opts.articleID)
		}
		return fmt.Errorf("%w (the article was re-published with its prior content)", err)
	}

	if strings.TrimSpace(opts.title) != "" {
		if err := updateArticleEntityTitle(ctx, deps.post, opts.articleID, opts.title); err != nil {
			return nil, restoreOnErr(err)
		}
	}

	contentState := opts.contentState
	if err := bindArticleMediaEntities(&contentState, deps.uploadImage); err != nil {
		return nil, restoreOnErr(err)
	}

	if err := updateArticleEntityContent(ctx, deps.post, opts.articleID, contentState); err != nil {
		return nil, restoreOnErr(err)
	}

	var coverMediaID string
	if strings.TrimSpace(opts.coverPath) != "" {
		mediaID, err := deps.uploadImage(opts.coverPath)
		if err != nil {
			return nil, restoreOnErr(err)
		}
		coverMediaID = mediaID
		if err := updateArticleEntityCoverMedia(ctx, deps.post, opts.articleID, mediaID); err != nil {
			return nil, restoreOnErr(err)
		}
	}

	result := &articleUpdateResult{
		ArticleID:    opts.articleID,
		URL:          "https://x.com/compose/article/edit/" + opts.articleID,
		Title:        opts.title,
		CoverMediaID: coverMediaID,
		Lifecycle:    "Draft",
	}
	if opts.republish {
		publishData, err := publishArticleEntity(ctx, deps.post, opts.articleID)
		if err != nil {
			return nil, err
		}
		articleID := opts.articleID
		if publishedID := articleIDFromPublishResponse(publishData); publishedID != "" {
			articleID = publishedID
		}
		result.ArticleID = articleID
		result.URL = "https://x.com/i/article/" + articleID
		result.Lifecycle = "Published"
		result.Republished = true
	}
	return result, nil
}

type articleEntityScan struct {
	lifecycle    string
	unknownTypes []string
}

// preflightArticleEntities locates the target article in the user's Draft
// then Published slices and reports any entity types outside the converter's
// emit set.
func preflightArticleEntities(ctx context.Context, deps articleUpdateDeps, articleID string) (*articleEntityScan, error) {
	for _, lifecycle := range []string{"Draft", "Published"} {
		raw, err := deps.fetchSlice(ctx, lifecycle)
		if err != nil {
			return nil, fmt.Errorf("preflight %s articles: %w", lifecycle, err)
		}
		if scan, found := scanArticleSliceForEntity(raw, articleID); found {
			scan.lifecycle = lifecycle
			return scan, nil
		}
	}
	return nil, fmt.Errorf("article %s not found in your Draft or Published articles; check the id via `articles list --agent`", articleID)
}

// scanArticleSliceForEntity parses an ArticleEntitiesSlice response (read
// shape: camelCase entityMap of {key, value{type,...}}) looking for the
// target rest_id, and collects entity types outside the converter emit set.
func scanArticleSliceForEntity(raw json.RawMessage, articleID string) (*articleEntityScan, bool) {
	var response struct {
		Data struct {
			User struct {
				Result struct {
					Slice struct {
						Items []struct {
							Results struct {
								Result struct {
									RestID       string `json:"rest_id"`
									ContentState struct {
										EntityMap []struct {
											Key   string `json:"key"`
											Value struct {
												Type string `json:"type"`
											} `json:"value"`
										} `json:"entityMap"`
									} `json:"content_state"`
								} `json:"result"`
							} `json:"article_entity_results"`
						} `json:"items"`
					} `json:"articles_article_mixer_slice"`
				} `json:"result"`
			} `json:"user"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return nil, false
	}
	emitSet := converterEntityEmitSet()
	for _, item := range response.Data.User.Result.Slice.Items {
		article := item.Results.Result
		if article.RestID != articleID {
			continue
		}
		unknown := map[string]bool{}
		for _, entity := range article.ContentState.EntityMap {
			if !emitSet[entity.Value.Type] {
				unknown[entity.Value.Type] = true
			}
		}
		scan := &articleEntityScan{}
		for entityType := range unknown {
			scan.unknownTypes = append(scan.unknownTypes, entityType)
		}
		sort.Strings(scan.unknownTypes)
		return scan, true
	}
	return nil, false
}

// fetchArticleSlice reads the user's article slice for one lifecycle, live
// (no response cache: the preflight must see the draft's current entities).
func fetchArticleSlice(ctx context.Context, c *client.Client, lifecycle string) (json.RawMessage, error) {
	cookies, err := client.LoadCookieAuth()
	if err != nil {
		return nil, err
	}
	userID := cookies.ArticleUserID()
	if userID == "" {
		return nil, fmt.Errorf("articles update-md requires the signed-in X user id; re-run `x-twitter-pp-cli auth login --chrome` to refresh cookies with the twid user id")
	}
	variables, err := json.Marshal(map[string]string{"userId": userID, "lifecycle": lifecycle})
	if err != nil {
		return nil, err
	}
	return c.GetNoCache(ctx, client.ArticleOpURL("ArticleEntitiesSlice"), map[string]string{"variables": string(variables)})
}
