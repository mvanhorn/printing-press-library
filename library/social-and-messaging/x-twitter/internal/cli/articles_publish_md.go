// articles publish-md — markdown to X Article publishing wrapper.
//
// Parses a markdown file with frontmatter, converts body to Draft.js
// content_state JSON, and prints the constructed payload in dry-run mode.
//
// CURRENT SCOPE: article body + media. Supported block types:
// paragraph, header-one, header-two, unordered-list-item, ordered-list-item,
// blockquote, fenced code blocks, markdown table blocks, markdown image blocks,
// tweet embeds, dividers, plus bold/italic inline styles and [text](url)
// inline links (LINK entities), and inline `code` spans. HTML comment lines
// are stripped. Cover image uses the captured upload + UpdateCoverMedia flow.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
	"github.com/spf13/cobra"
)

type articleFrontmatter struct {
	Title   string   `yaml:"title"`
	Cover   string   `yaml:"cover"`
	Tags    []string `yaml:"tags"`
	Summary string   `yaml:"summary"`
}

type articleParsed struct {
	Frontmatter articleFrontmatter
	Body        string // markdown body (post-frontmatter)
}

type draftBlock struct {
	Data              map[string]any   `json:"data"`
	Text              string           `json:"text"`
	Key               string           `json:"key"`
	Type              string           `json:"type"`
	EntityRanges      []map[string]any `json:"entity_ranges"`
	InlineStyleRanges []inlineStyle    `json:"inline_style_ranges"`
}

type inlineStyle struct {
	Length int    `json:"length"`
	Offset int    `json:"offset"`
	Style  string `json:"style"`
}

type draftContentState struct {
	Blocks    []draftBlock  `json:"blocks"`
	EntityMap []draftEntity `json:"entityMap"`
}

type draftEntity struct {
	Key   string           `json:"key"`
	Value draftEntityValue `json:"value"`
}

type draftEntityValue struct {
	Data       map[string]any `json:"data"`
	Type       string         `json:"type"`
	Mutability string         `json:"mutability"`
}

func newNovelArticlesPublishMdCmd(flags *rootFlags) *cobra.Command {
	var post bool
	var draft bool
	var updateID string
	var replaceUnknownEntities bool
	cmd := &cobra.Command{
		Use:     "articles-publish-md <markdown-file>",
		Short:   "Convert a markdown file to an X Article (preview by default; --draft or --post to write)",
		Long:    "Parses frontmatter (title, cover, tags) and body, converts the body to the Draft.js content_state JSON X's Articles editor accepts. Previews the payload by default (no API call); pass --draft to save a new draft, --post to create and publish publicly, or --update <article_id> to update an existing draft in place.",
		Example: "  x-twitter-pp-cli articles-publish-md draft.md\n  x-twitter-pp-cli articles-publish-md draft.md --update 1750000000000000000",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Dry-run probes call with no file arg: short-circuit before the
				// required-arg check so verify can exercise the command cleanly.
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
			if strings.TrimSpace(updateID) != "" {
				if post {
					return usageErr(fmt.Errorf("--update cannot be combined with --post; use `articles update-md --article-id %s --republish` for published articles", strings.TrimSpace(updateID)))
				}
				if draft {
					return usageErr(fmt.Errorf("--update cannot be combined with --draft; --update already targets an existing draft (id %s); drop --draft", strings.TrimSpace(updateID)))
				}
				payload := map[string]any{
					"article_id":    strings.TrimSpace(updateID),
					"title":         parsed.Frontmatter.Title,
					"cover":         parsed.Frontmatter.Cover,
					"content_state": cs,
				}
				if flags.dryRun {
					enc := json.NewEncoder(cmd.OutOrStdout())
					if !flags.asJSON {
						fmt.Fprintln(cmd.OutOrStdout(), "── Article update payload (dry-run, no API call) ──")
						enc.SetIndent("", "  ")
					}
					return enc.Encode(payload)
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
					articleID:              strings.TrimSpace(updateID),
					title:                  parsed.Frontmatter.Title,
					coverPath:              parsed.Frontmatter.Cover,
					contentState:           cs,
					replaceUnknownEntities: replaceUnknownEntities,
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "updated draft article %s\n%s\n", result.ArticleID, result.URL)
				return nil
			}
			payload := map[string]any{
				"title":         parsed.Frontmatter.Title,
				"cover":         parsed.Frontmatter.Cover,
				"tags":          parsed.Frontmatter.Tags,
				"summary":       parsed.Frontmatter.Summary,
				"content_state": cs,
			}
			if (!post && !draft) || flags.dryRun {
				enc := json.NewEncoder(cmd.OutOrStdout())
				// Machine output (--json/--agent): emit the bare payload so an
				// agent can json.load it. Banner + trailing prose are human-only.
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "── Article payload (preview) ──")
					enc.SetIndent("", "  ")
				}
				if err := enc.Encode(payload); err != nil {
					return err
				}
			}
			if !post && !draft {
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "(preview only — pass --draft to save a draft, or --post to publish)")
				}
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "(--dry-run set, skipping article write)")
				return nil
			}
			if os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				fmt.Fprintln(cmd.OutOrStdout(), "verify-env: skipping article write")
				return nil
			}
			result, err := publishMarkdownArticle(cmd.Context(), flags, parsed.Frontmatter.Title, parsed.Frontmatter.Cover, cs, post)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			verb := "created draft"
			if post {
				verb = "published"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s article %s\n%s\n", verb, result.ArticleID, result.URL)
			return nil
		},
	}
	cmd.Flags().BoolVar(&draft, "draft", false, "Create the article as a draft without publishing")
	cmd.Flags().BoolVar(&post, "post", false, "Create and publish the article publicly (default: preview only)")
	cmd.Flags().StringVar(&updateID, "update", "", "Update an existing draft article by rest_id instead of creating a new draft")
	cmd.Flags().BoolVar(&replaceUnknownEntities, "replace-unknown-entities", false, "With --update, proceed even when the target draft contains entity types the converter cannot reproduce")
	return cmd
}

type publishedArticleResult struct {
	ArticleID    string `json:"article_id"`
	URL          string `json:"url"`
	Title        string `json:"title"`
	CoverMediaID string `json:"cover_media_id,omitempty"`
}

// articleGraphQLPoster is the minimal client surface the shared article
// GraphQL composition helpers need. *client.Client satisfies it; tests inject
// a fake transport so no network is involved.
type articleGraphQLPoster interface {
	Post(ctx context.Context, path string, body any) (json.RawMessage, int, error)
}

// PATCH: Shared X Articles GraphQL composition. Both articles-publish-md
// (create) and articles update-md (in-place update) compose the same proven
// mutations through these helpers; queryIds resolve through the client's
// article-ops table instead of hardcoded literals that drift when X redeploys.
func articleOpRequestBody(opName string, variables map[string]any) map[string]any {
	return map[string]any{
		"variables": variables,
		"features":  articleGraphQLFeatures(),
		"queryId":   client.ArticleOpQueryID(opName),
	}
}

func createArticleDraftEntity(ctx context.Context, p articleGraphQLPoster) (string, error) {
	body := articleOpRequestBody("ArticleEntityDraftCreate", map[string]any{
		"content_state": map[string]any{"blocks": []any{}, "entity_map": []any{}},
		"title":         "",
	})
	createData, _, err := p.Post(ctx, client.ArticleOpURL("ArticleEntityDraftCreate"), body)
	if err != nil {
		return "", err
	}
	articleID := articleIDFromCreateResponse(createData)
	if articleID == "" {
		return "", fmt.Errorf("create draft response did not include article rest_id")
	}
	return articleID, nil
}

func updateArticleEntityTitle(ctx context.Context, p articleGraphQLPoster, articleID string, title string) error {
	body := articleOpRequestBody("ArticleEntityUpdateTitle", map[string]any{
		"articleEntityId": articleID,
		"title":           title,
	})
	_, _, err := p.Post(ctx, client.ArticleOpURL("ArticleEntityUpdateTitle"), body)
	return err
}

// updateArticleEntityContent sends the proven UpdateContent variable shape:
// {content_state: {blocks, entity_map}, article_entity: <id>}. This replaces
// the entire body of the target draft.
func updateArticleEntityContent(ctx context.Context, p articleGraphQLPoster, articleID string, contentState draftContentState) error {
	body := articleOpRequestBody("ArticleEntityUpdateContent", map[string]any{
		"content_state":  articleContentStateRequest(contentState),
		"article_entity": articleID,
	})
	_, _, err := p.Post(ctx, client.ArticleOpURL("ArticleEntityUpdateContent"), body)
	return err
}

func updateArticleEntityCoverMedia(ctx context.Context, p articleGraphQLPoster, articleID string, mediaID string) error {
	body := articleOpRequestBody("ArticleEntityUpdateCoverMedia", map[string]any{
		"articleEntityId": articleID,
		"coverMedia": map[string]any{
			"media_id":       mediaID,
			"media_category": "DraftTweetImage",
		},
	})
	_, _, err := p.Post(ctx, client.ArticleOpURL("ArticleEntityUpdateCoverMedia"), body)
	return err
}

func publishArticleEntity(ctx context.Context, p articleGraphQLPoster, articleID string) (json.RawMessage, error) {
	body := articleOpRequestBody("ArticleEntityPublish", map[string]any{
		"articleEntityId":   articleID,
		"visibilitySetting": "Public",
	})
	publishData, _, err := p.Post(ctx, client.ArticleOpURL("ArticleEntityPublish"), body)
	return publishData, err
}

func unpublishArticleEntity(ctx context.Context, p articleGraphQLPoster, articleID string) error {
	body := articleOpRequestBody("ArticleEntityUnpublish", map[string]any{
		"articleEntityId": articleID,
	})
	_, _, err := p.Post(ctx, client.ArticleOpURL("ArticleEntityUnpublish"), body)
	return err
}

// PATCH: Wire the X-specific Articles create/update/publish GraphQL sequence.
func publishMarkdownArticle(ctx context.Context, flags *rootFlags, title string, coverPath string, contentState draftContentState, publish bool) (*publishedArticleResult, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("frontmatter title is required when creating an article draft or post")
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}

	articleID, err := createArticleDraftEntity(ctx, c)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}

	if err := updateArticleEntityTitle(ctx, c, articleID, title); err != nil {
		return nil, classifyAPIError(err, flags)
	}

	// PATCH: Bind local markdown image placeholders to uploaded X Article MEDIA entities.
	if err := bindArticleMediaEntities(&contentState, func(p string) (string, error) {
		return c.UploadArticleImage(ctx, p)
	}); err != nil {
		return nil, classifyAPIError(err, flags)
	}

	if err := updateArticleEntityContent(ctx, c, articleID, contentState); err != nil {
		return nil, classifyAPIError(err, flags)
	}

	var coverMediaID string
	if strings.TrimSpace(coverPath) != "" {
		coverMediaID, err = c.UploadArticleImage(ctx, coverPath)
		if err != nil {
			return nil, classifyAPIError(err, flags)
		}
		if err := updateArticleEntityCoverMedia(ctx, c, articleID, coverMediaID); err != nil {
			return nil, classifyAPIError(err, flags)
		}
	}

	if !publish {
		// Draft-only: stop before ArticleEntityPublish. The draft is created
		// and fully populated (title, content, cover) but never made public.
		return &publishedArticleResult{
			ArticleID:    articleID,
			URL:          "https://x.com/compose/article/edit/" + articleID,
			Title:        title,
			CoverMediaID: coverMediaID,
		}, nil
	}

	publishData, err := publishArticleEntity(ctx, c, articleID)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	if publishedID := articleIDFromPublishResponse(publishData); publishedID != "" {
		articleID = publishedID
	}

	return &publishedArticleResult{
		ArticleID:    articleID,
		URL:          "https://x.com/i/article/" + articleID,
		Title:        title,
		CoverMediaID: coverMediaID,
	}, nil
}

func articleGraphQLFeatures() map[string]any {
	return map[string]any{
		"profile_label_improvements_pcf_label_in_post_enabled":              true,
		"responsive_web_profile_redirect_enabled":                           false,
		"rweb_tipjar_consumption_enabled":                                   false,
		"verified_phone_label_enabled":                                      false,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"responsive_web_graphql_timeline_navigation_enabled":                true,
	}
}

func articleContentStateRequest(cs draftContentState) map[string]any {
	// X's ArticleEntityUpdateContent rejects null blocks/entity_map ("cannot be
	// null"). A plain-text article has no entities, so cs.EntityMap is a nil
	// slice that JSON-encodes as null — force empty arrays instead.
	blocks := cs.Blocks
	if blocks == nil {
		blocks = []draftBlock{}
	}
	entityMap := cs.EntityMap
	if entityMap == nil {
		entityMap = []draftEntity{}
	}
	return map[string]any{
		"blocks":     blocks,
		"entity_map": entityMap,
	}
}

func articleIDFromCreateResponse(data []byte) string {
	var response struct {
		Data struct {
			CreateDraft struct {
				ArticleEntityResults struct {
					Result struct {
						RestID string `json:"rest_id"`
					} `json:"result"`
				} `json:"article_entity_results"`
			} `json:"articleentity_create_draft"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &response) != nil {
		return ""
	}
	return response.Data.CreateDraft.ArticleEntityResults.Result.RestID
}

func articleIDFromPublishResponse(data []byte) string {
	var response struct {
		Data struct {
			Publish struct {
				ArticleEntityResults struct {
					Result struct {
						RestID string `json:"rest_id"`
					} `json:"result"`
				} `json:"article_entity_results"`
			} `json:"articleentity_publish"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &response) != nil {
		return ""
	}
	return response.Data.Publish.ArticleEntityResults.Result.RestID
}

// ParseArticleMarkdown extracts frontmatter and body from a markdown string.
// Frontmatter is delimited by --- on its own line at the start.
func ParseArticleMarkdown(s string) (*articleParsed, error) {
	out := &articleParsed{}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		// Find closing ---
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end > 0 {
			fm := strings.Join(lines[1:end], "\n")
			parseFrontmatter(fm, &out.Frontmatter)
			out.Body = strings.Join(lines[end+1:], "\n")
			return out, nil
		}
	}
	out.Body = s
	return out, nil
}

// parseFrontmatter does a minimal YAML-subset parse: scalar strings, simple
// inline arrays. Sufficient for title/cover/summary/tags.
func parseFrontmatter(yamlSrc string, fm *articleFrontmatter) {
	for _, line := range strings.Split(yamlSrc, "\n") {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"' `)
		switch key {
		case "title":
			fm.Title = val
		case "cover":
			fm.Cover = val
		case "summary":
			fm.Summary = val
		case "tags":
			val = strings.TrimPrefix(val, "[")
			val = strings.TrimSuffix(val, "]")
			for _, tag := range strings.Split(val, ",") {
				t := strings.TrimSpace(strings.Trim(tag, `"' `))
				if t != "" {
					fm.Tags = append(fm.Tags, t)
				}
			}
		}
	}
}

// MarkdownBodyToDraftJS converts a markdown body to a Draft.js content_state.
// Supports: paragraph, header-one (# ), header-two (## and deeper headings),
// ordered-list-item, blockquote, markdown image lines, standalone tweet URLs,
// standalone dividers (---), markdown tables, plus inline bold (**...**) and
// italic (*...*) and code (`...`). Fenced code blocks and markdown tables are
// emitted as X Articles MARKDOWN entities bound to atomic Draft.js blocks.
// Image lines are emitted as placeholder MEDIA entities in dry-run output, then
// rebound to uploaded media IDs before live publish. Setext headings are
// intentionally unsupported; use ## for header-two because --- is reserved for
// dividers.
func MarkdownBodyToDraftJS(md string) draftContentState {
	cs := draftContentState{}
	lines := strings.Split(md, "\n")
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimRight(raw, " \t")
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		// PATCH: Strip HTML comment lines (`<!-- ... -->`) so loader directives
		// never ship as visible article text. Only whole comment lines are
		// stripped; an inline comment mid-paragraph passes through as literal
		// text (multi-token HTML parsing is out of converter scope).
		if strings.HasPrefix(trim, "<!--") {
			continue
		}
		if strings.HasPrefix(trim, "```") {
			codeLines := []string{}
			openingFence := trim
			for i++; i < len(lines); i++ {
				codeLine := strings.TrimRight(lines[i], " \t")
				if strings.HasPrefix(strings.TrimSpace(codeLine), "```") {
					break
				}
				codeLines = append(codeLines, codeLine)
			}
			appendMarkdownEntityBlock(&cs, openingFence+"\n"+strings.Join(codeLines, "\n")+"\n```")
			continue
		}
		if alt, path, ok := parseMarkdownImageLine(trim); ok {
			appendArticleMediaEntityBlock(&cs, path, alt)
			continue
		}
		if tableLines, next, ok := collectMarkdownTable(lines, i); ok {
			appendMarkdownEntityBlock(&cs, strings.Join(tableLines, "\n"))
			i = next - 1
			continue
		}
		if tweetID, ok := parseTweetStatusLine(trim); ok {
			appendTweetEntityBlock(&cs, tweetID)
			continue
		}
		if trim == "---" {
			appendDividerEntityBlock(&cs)
			continue
		}
		blk := draftBlock{
			Data:              map[string]any{},
			Key:               randBlockKey(),
			Type:              "unstyled",
			EntityRanges:      []map[string]any{},
			InlineStyleRanges: []inlineStyle{},
		}
		switch {
		case strings.HasPrefix(trim, "# "):
			blk.Type = "header-one"
			blk.Text = strings.TrimSpace(trim[2:])
		case strings.HasPrefix(trim, "## "):
			blk.Type = "header-two"
			blk.Text = strings.TrimSpace(trim[3:])
		case strings.HasPrefix(trim, "###"):
			if headingText, ok := downgradeMarkdownHeading(trim); ok {
				blk.Type = "header-two"
				blk.Text = headingText
			} else {
				blk.Text = trim
			}
		case strings.HasPrefix(trim, "> "):
			blk.Type = "blockquote"
			blk.Text = strings.TrimSpace(trim[2:])
		case strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* "):
			blk.Type = "unordered-list-item"
			blk.Text = strings.TrimSpace(trim[2:])
		case len(trim) > 2 && trim[0] >= '0' && trim[0] <= '9' && (strings.HasPrefix(trim[1:], ". ") || strings.HasPrefix(trim[2:], ". ")):
			blk.Type = "ordered-list-item"
			dot := strings.Index(trim, ". ")
			blk.Text = strings.TrimSpace(trim[dot+2:])
		default:
			blk.Text = trim
		}
		// PATCH: Run the combined inline pass (bold/italic + [text](url) links)
		// on every non-atomic block, including blockquotes, and attach LINK
		// entities with UTF-16 entity_ranges.
		var links []linkSpan
		blk.Text, blk.InlineStyleRanges, links = extractInlineSpans(blk.Text)
		for _, ln := range links {
			entityIndex := len(cs.EntityMap)
			cs.EntityMap = append(cs.EntityMap, draftEntity{
				Key: strconv.Itoa(entityIndex),
				Value: draftEntityValue{
					// Read-back of a live composer-authored draft (article
					// 2064864205749280768) shows {entityKey: <uuid>, url: <url>},
					// but entityKey is server-generated: ArticleEntityUpdateContent
					// rejects it with GRAPHQL_VALIDATION_FAILED (HTTP 422) at
					// entity_map[N].value.data.entityKey. Write url only, verbatim
					// (not t.co-wrapped); the server owns entityKey.
					Data:       map[string]any{"url": ln.URL},
					Type:       "LINK",
					Mutability: "Mutable",
				},
			})
			blk.EntityRanges = append(blk.EntityRanges, map[string]any{
				"key":    entityIndex,
				"offset": ln.Offset,
				"length": ln.Length,
			})
		}
		cs.Blocks = append(cs.Blocks, blk)
	}
	return cs
}

func appendMarkdownEntityBlock(cs *draftContentState, markdown string) {
	appendAtomicEntityBlock(cs, "MARKDOWN", "Mutable", map[string]any{"markdown": markdown})
}

func appendArticleMediaEntityBlock(cs *draftContentState, sourcePath string, altText string) {
	data := map[string]any{"source_path": sourcePath}
	if altText != "" {
		data["alt_text"] = altText
	}
	appendAtomicEntityBlock(cs, "MEDIA", "Immutable", data)
}

func appendTweetEntityBlock(cs *draftContentState, tweetID string) {
	appendAtomicEntityBlock(cs, "TWEET", "Immutable", map[string]any{"tweet_id": tweetID})
}

func appendDividerEntityBlock(cs *draftContentState) {
	appendAtomicEntityBlock(cs, "DIVIDER", "Immutable", map[string]any{})
}

func appendAtomicEntityBlock(cs *draftContentState, entityType string, mutability string, data map[string]any) {
	entityIndex := len(cs.EntityMap)
	cs.EntityMap = append(cs.EntityMap, draftEntity{
		Key: strconv.Itoa(entityIndex),
		Value: draftEntityValue{
			Data:       data,
			Type:       entityType,
			Mutability: mutability,
		},
	})
	cs.Blocks = append(cs.Blocks, draftBlock{
		Data:              map[string]any{},
		Text:              " ",
		Key:               randBlockKey(),
		Type:              "atomic",
		EntityRanges:      []map[string]any{{"key": entityIndex, "offset": 0, "length": 1}},
		InlineStyleRanges: []inlineStyle{},
	})
}

func bindArticleMediaEntities(cs *draftContentState, upload func(string) (string, error)) error {
	imageIndex := 0
	for i := range cs.EntityMap {
		entity := &cs.EntityMap[i].Value
		if entity.Type != "MEDIA" {
			continue
		}
		sourcePath, _ := entity.Data["source_path"].(string)
		if strings.TrimSpace(sourcePath) == "" {
			continue
		}
		mediaID, err := upload(sourcePath)
		if err != nil {
			return fmt.Errorf("upload article body image %s: %w", sourcePath, err)
		}
		entity.Data = articleMediaEntityData(mediaID, imageIndex)
		entity.Type = "MEDIA"
		entity.Mutability = "Immutable"
		imageIndex++
	}
	return nil
}

func articleMediaEntityData(mediaID string, imageIndex int) map[string]any {
	return map[string]any{
		"media_items": []map[string]any{{
			"local_media_id": 2 + imageIndex*2,
			"media_category": "DraftTweetImage",
			"media_id":       mediaID,
		}},
	}
}

func parseMarkdownImageLine(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "![") || !strings.HasSuffix(line, ")") {
		return "", "", false
	}
	closeAlt := strings.Index(line, "](")
	if closeAlt < 2 {
		return "", "", false
	}
	altText := line[2:closeAlt]
	sourcePath := strings.TrimSpace(line[closeAlt+2 : len(line)-1])
	if sourcePath == "" || strings.ContainsAny(sourcePath, "\t\n\r") {
		return "", "", false
	}
	return altText, sourcePath, true
}

func parseTweetStatusLine(line string) (string, bool) {
	u, err := url.Parse(line)
	if err != nil {
		return "", false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host != "x.com" && host != "twitter.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] != "status" {
		return "", false
	}
	tweetID := parts[2]
	if tweetID == "" {
		return "", false
	}
	for _, r := range tweetID {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return tweetID, true
}

func collectMarkdownTable(lines []string, start int) ([]string, int, bool) {
	if start+1 >= len(lines) {
		return nil, start, false
	}
	first := strings.TrimRight(lines[start], " \t")
	second := strings.TrimRight(lines[start+1], " \t")
	if !isMarkdownTableRow(first) || !isMarkdownTableSeparatorRow(second) {
		return nil, start, false
	}

	tableLines := []string{first, second}
	next := start + 2
	for next < len(lines) {
		line := strings.TrimRight(lines[next], " \t")
		if strings.TrimSpace(line) == "" || !isMarkdownTableRow(line) {
			break
		}
		tableLines = append(tableLines, line)
		next++
	}
	return tableLines, next, true
}

func isMarkdownTableRow(line string) bool {
	return len(splitMarkdownTableRow(line)) >= 2
}

func isMarkdownTableSeparatorRow(line string) bool {
	cells := splitMarkdownTableRow(line)
	if len(cells) < 2 {
		return false
	}
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		trimmed = strings.TrimPrefix(trimmed, ":")
		trimmed = strings.TrimSuffix(trimmed, ":")
		if len(trimmed) < 3 || strings.Trim(trimmed, "-") != "" {
			return false
		}
	}
	return true
}

func splitMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	if !strings.Contains(trimmed, "|") {
		return nil
	}
	return strings.Split(trimmed, "|")
}

func downgradeMarkdownHeading(line string) (string, bool) {
	hashes := 0
	for hashes < len(line) && line[hashes] == '#' {
		hashes++
	}
	if hashes < 3 {
		return "", false
	}
	if hashes >= len(line) || line[hashes] != ' ' {
		return "", false
	}
	text := strings.TrimSpace(line[hashes+1:])
	if text == "" {
		return "", false
	}
	return text, true
}

// linkSpan describes one [text](url) span found by extractInlineSpans, with
// Offset/Length in UTF-16 code units over the cleaned block text.
type linkSpan struct {
	Offset int
	Length int
	URL    string
}

// PATCH: extractInlineSpans is the combined inline pass: bold/italic markers
// plus [text](url) inline links. Link text is recursively cleaned through
// extractInlineStyles so `[**bold** link](url)` yields both the LINK range and
// the inner Bold range with correct UTF-16 offsets. Image syntax (`![alt](p)`)
// is left untouched, and degenerate links (`[](url)`, `[text]()`) degrade to
// plain text.
func extractInlineSpans(s string) (string, []inlineStyle, []linkSpan) {
	ranges := []inlineStyle{}
	links := []linkSpan{}
	out := strings.Builder{}
	i := 0
	for i < len(s) {
		if next, ok := consumeInlineCode(s, i, &out, &ranges); ok {
			i = next
			continue
		}
		// Bold first (**...**) so it doesn't get consumed as italic.
		if i+2 <= len(s) && s[i:i+2] == "**" {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				inner := s[i+2 : i+2+end]
				offset := utf16Len(out.String())
				out.WriteString(inner)
				ranges = append(ranges, inlineStyle{Offset: offset, Length: utf16Len(inner), Style: "Bold"})
				i = i + 2 + end + 2
				continue
			}
		}
		// Italic (*...*), single asterisk
		if s[i] == '*' && (i == 0 || s[i-1] != '*') {
			end := strings.Index(s[i+1:], "*")
			if end > 0 && (i+1+end+1 >= len(s) || s[i+1+end+1] != '*') {
				inner := s[i+1 : i+1+end]
				offset := utf16Len(out.String())
				out.WriteString(inner)
				ranges = append(ranges, inlineStyle{Offset: offset, Length: utf16Len(inner), Style: "Italic"})
				i = i + 1 + end + 1
				continue
			}
		}
		// Inline link [text](url). Skip image syntax (preceding '!').
		if s[i] == '[' && (i == 0 || s[i-1] != '!') {
			if inner, linkURL, advance, ok := parseInlineLinkAt(s, i); ok {
				cleanedInner, innerStyles := extractInlineStyles(inner)
				offset := utf16Len(out.String())
				out.WriteString(cleanedInner)
				for _, st := range innerStyles {
					st.Offset += offset
					ranges = append(ranges, st)
				}
				links = append(links, linkSpan{Offset: offset, Length: utf16Len(cleanedInner), URL: linkURL})
				i += advance
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String(), ranges, links
}

// parseInlineLinkAt parses a [text](url) span starting at s[i] (which must be
// '['). Returns the raw link text, the url, and how many bytes the whole span
// consumes. Empty link text or empty url returns ok=false so the caller copies
// the characters through as plain text.
func parseInlineLinkAt(s string, i int) (inner string, linkURL string, advance int, ok bool) {
	closeText := findInlineLinkTextClose(s, i)
	if closeText < 0 {
		return "", "", 0, false
	}
	inner = s[i+1 : closeText]
	if strings.TrimSpace(inner) == "" {
		return "", "", 0, false
	}
	urlStart := closeText + 2
	// PATCH: balance nested parens so URLs like .../Rust_(programming_language)
	// are not truncated at the inner ')' (Greptile P1 on PR #1141).
	closeURL := -1
	depth := 0
	for j := urlStart; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				closeURL = j - urlStart
			} else {
				depth--
			}
		}
		if closeURL >= 0 {
			break
		}
	}
	if closeURL < 0 {
		return "", "", 0, false
	}
	linkURL = strings.TrimSpace(s[urlStart : urlStart+closeURL])
	if linkURL == "" {
		return "", "", 0, false
	}
	advance = urlStart + closeURL + 1 - i
	return inner, linkURL, advance, true
}

func findInlineLinkTextClose(s string, open int) int {
	depth := 0
	for j := open + 1; j+1 < len(s); j++ {
		switch s[j] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
				continue
			}
			if s[j+1] == '(' {
				return j
			}
		}
	}
	return -1
}

func consumeInlineCode(s string, i int, out *strings.Builder, ranges *[]inlineStyle) (int, bool) {
	if i >= len(s) || s[i] != '`' {
		return i, false
	}
	end := strings.Index(s[i+1:], "`")
	if end < 0 {
		return i, false
	}
	inner := s[i+1 : i+1+end]
	offset := utf16Len(out.String())
	out.WriteString(inner)
	*ranges = append(*ranges, inlineStyle{Offset: offset, Length: utf16Len(inner), Style: "CODE"})
	return i + 1 + end + 1, true
}

// extractInlineStyles scans a string for **bold** and *italic* markers and
// returns the cleaned text plus the inline_style_ranges that describe them.
//
// Offsets/lengths use UTF-16 code units to match Draft.js — JS strings are
// indexed by UTF-16 code units. Byte-based offsets would be wrong for
// non-ASCII text (every emoji, accented char, CJK char throws off the math).
func extractInlineStyles(s string) (string, []inlineStyle) {
	ranges := []inlineStyle{}
	out := strings.Builder{}
	i := 0
	for i < len(s) {
		if next, ok := consumeInlineCode(s, i, &out, &ranges); ok {
			i = next
			continue
		}
		// Bold first (**...**) so it doesn't get consumed as italic.
		if i+2 <= len(s) && s[i:i+2] == "**" {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				inner := s[i+2 : i+2+end]
				offset := utf16Len(out.String())
				out.WriteString(inner)
				ranges = append(ranges, inlineStyle{Offset: offset, Length: utf16Len(inner), Style: "Bold"})
				i = i + 2 + end + 2
				continue
			}
		}
		// Italic (*...*), single asterisk
		if s[i] == '*' && (i == 0 || s[i-1] != '*') {
			end := strings.Index(s[i+1:], "*")
			if end >= 0 && end > 0 && (i+1+end+1 >= len(s) || s[i+1+end+1] != '*') {
				inner := s[i+1 : i+1+end]
				offset := utf16Len(out.String())
				out.WriteString(inner)
				ranges = append(ranges, inlineStyle{Offset: offset, Length: utf16Len(inner), Style: "Italic"})
				i = i + 1 + end + 1
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String(), ranges
}

// utf16Len returns the number of UTF-16 code units required to encode s.
// Equivalent to s.length in JavaScript — a BMP rune counts as 1, a
// supplementary rune (emoji, etc.) counts as 2.
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// randBlockKey produces a 5-char alphanumeric key in the shape Draft.js uses.
func randBlockKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
