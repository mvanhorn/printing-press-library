package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMarkdownBodyToDraftJSImageLine(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Before\n\n![body alt](./body.png)\n\nAfter")

	if len(contentState.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(contentState.Blocks))
	}
	if contentState.Blocks[1].Type != "atomic" {
		t.Fatalf("expected image line to produce an atomic block, got %q", contentState.Blocks[1].Type)
	}
	if contentState.Blocks[1].Text != " " {
		t.Fatalf("expected atomic block text to be a single space, got %q", contentState.Blocks[1].Text)
	}
	if len(contentState.Blocks[1].EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(contentState.Blocks[1].EntityRanges))
	}
	if contentState.Blocks[1].EntityRanges[0]["key"] != 0 {
		t.Fatalf("expected atomic block to reference entity key 0, got %#v", contentState.Blocks[1].EntityRanges[0]["key"])
	}
	if len(contentState.EntityMap) != 1 {
		t.Fatalf("expected one media entity, got %d", len(contentState.EntityMap))
	}
	entity := contentState.EntityMap[0]
	if entity.Value.Type != "MEDIA" {
		t.Fatalf("expected MEDIA entity, got %q", entity.Value.Type)
	}
	if entity.Value.Mutability != "Immutable" {
		t.Fatalf("expected Immutable entity, got %q", entity.Value.Mutability)
	}
	if entity.Value.Data["source_path"] != "./body.png" {
		t.Fatalf("expected source_path to be retained, got %#v", entity.Value.Data["source_path"])
	}
	if entity.Value.Data["alt_text"] != "body alt" {
		t.Fatalf("expected alt_text to be retained, got %#v", entity.Value.Data["alt_text"])
	}
}

func TestArticlesPublishMdUpdateDryRunWithFilePrintsPreview(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "article.md")
	if err := os.WriteFile(md, []byte("---\ntitle: Dry Run\ncover: ./cover.jpg\n---\n\nBody"), 0o600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"articles-publish-md", md, "--update", "123", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("articles-publish-md dry-run failed: %v\n%s", err, out.String())
	}
	if out.Len() == 0 {
		t.Fatalf("expected dry-run preview output")
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
	}
	if payload["article_id"] != "123" || payload["title"] != "Dry Run" || payload["cover"] != "./cover.jpg" {
		t.Fatalf("unexpected preview payload: %#v", payload)
	}
}

func TestArticlesPublishMdUpdateRejectsDraftFlag(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "article.md")
	if err := os.WriteFile(md, []byte("---\ntitle: Dry Run\n---\n\nBody"), 0o600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"articles-publish-md", md, "--update", "123", "--draft"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --update --draft to fail")
	}
	if !strings.Contains(err.Error(), "--update cannot be combined with --draft") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArticleMediaEntities(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("![one](./one.png)\n\n![two](./two.jpg)")
	uploads := []string{}

	err := bindArticleMediaEntities(&contentState, func(path string) (string, error) {
		uploads = append(uploads, path)
		return "media-" + path, nil
	})
	if err != nil {
		t.Fatalf("bindArticleMediaEntities returned error: %v", err)
	}
	if len(uploads) != 2 {
		t.Fatalf("expected 2 uploads, got %d", len(uploads))
	}
	if uploads[0] != "./one.png" || uploads[1] != "./two.jpg" {
		t.Fatalf("unexpected upload paths: %#v", uploads)
	}

	first := contentState.EntityMap[0].Value
	if first.Data["source_path"] != nil {
		t.Fatalf("expected source_path to be removed after bind, got %#v", first.Data["source_path"])
	}
	firstItems, ok := first.Data["media_items"].([]map[string]any)
	if !ok || len(firstItems) != 1 {
		t.Fatalf("expected first media_items, got %#v", first.Data["media_items"])
	}
	if firstItems[0]["local_media_id"] != 2 {
		t.Fatalf("expected first local_media_id 2, got %#v", firstItems[0]["local_media_id"])
	}
	if firstItems[0]["media_category"] != "DraftTweetImage" {
		t.Fatalf("expected DraftTweetImage, got %#v", firstItems[0]["media_category"])
	}
	if firstItems[0]["media_id"] != "media-./one.png" {
		t.Fatalf("expected first media_id, got %#v", firstItems[0]["media_id"])
	}

	second := contentState.EntityMap[1].Value
	secondItems, ok := second.Data["media_items"].([]map[string]any)
	if !ok || len(secondItems) != 1 {
		t.Fatalf("expected second media_items, got %#v", second.Data["media_items"])
	}
	if secondItems[0]["local_media_id"] != 4 {
		t.Fatalf("expected second local_media_id 4, got %#v", secondItems[0]["local_media_id"])
	}
	if secondItems[0]["media_id"] != "media-./two.jpg" {
		t.Fatalf("expected second media_id, got %#v", secondItems[0]["media_id"])
	}
}

func TestMarkdownBodyToDraftJSCodeFence(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Before\n\n```bash\nx-twitter-pp-cli articles-publish-md draft.md --post\n```\n\nAfter")

	if len(contentState.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(contentState.Blocks))
	}
	if contentState.Blocks[1].Type != "atomic" {
		t.Fatalf("expected fenced code to produce an atomic block, got %q", contentState.Blocks[1].Type)
	}
	if contentState.Blocks[1].Text != " " {
		t.Fatalf("expected atomic block text to be a single space, got %q", contentState.Blocks[1].Text)
	}
	if len(contentState.Blocks[1].EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(contentState.Blocks[1].EntityRanges))
	}
	if contentState.Blocks[1].EntityRanges[0]["key"] != 0 {
		t.Fatalf("expected atomic block to reference entity key 0, got %#v", contentState.Blocks[1].EntityRanges[0]["key"])
	}
	if len(contentState.EntityMap) != 1 {
		t.Fatalf("expected one markdown entity, got %d", len(contentState.EntityMap))
	}
	entity := contentState.EntityMap[0]
	if entity.Key != "0" {
		t.Fatalf("expected entity key 0, got %q", entity.Key)
	}
	if entity.Value.Type != "MARKDOWN" {
		t.Fatalf("expected MARKDOWN entity, got %q", entity.Value.Type)
	}
	if entity.Value.Mutability != "Mutable" {
		t.Fatalf("expected Mutable entity, got %q", entity.Value.Mutability)
	}
	wantMarkdown := "```bash\nx-twitter-pp-cli articles-publish-md draft.md --post\n```"
	if entity.Value.Data["markdown"] != wantMarkdown {
		t.Fatalf("unexpected markdown entity data:\nwant: %q\n got: %q", wantMarkdown, entity.Value.Data["markdown"])
	}
}

func TestMarkdownBodyToDraftJSTweetEmbedLines(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Before\n\nhttps://x.com/alice/status/2061877533885473181\n\nhttps://twitter.com/bob/status/2062703227972293057?ref=article\n\nAfter https://x.com/alice/status/1\n\nhttps://x.com/alice/status/2061877533885473181/photo/1")

	if len(contentState.Blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(contentState.Blocks))
	}
	firstTweet := requireAtomicEntity(t, contentState, 1, 0, "TWEET", "Immutable")
	if firstTweet.Data["tweet_id"] != "2061877533885473181" {
		t.Fatalf("expected first tweet_id, got %#v", firstTweet.Data["tweet_id"])
	}
	secondTweet := requireAtomicEntity(t, contentState, 2, 1, "TWEET", "Immutable")
	if secondTweet.Data["tweet_id"] != "2062703227972293057" {
		t.Fatalf("expected second tweet_id, got %#v", secondTweet.Data["tweet_id"])
	}
	if contentState.Blocks[3].Type != "unstyled" {
		t.Fatalf("expected non-standalone tweet URL to remain text, got %q", contentState.Blocks[3].Type)
	}
	if contentState.Blocks[3].Text != "After https://x.com/alice/status/1" {
		t.Fatalf("unexpected final paragraph text: %q", contentState.Blocks[3].Text)
	}
	if contentState.Blocks[4].Type != "unstyled" {
		t.Fatalf("expected media sub-page tweet URL to remain text, got %q", contentState.Blocks[4].Type)
	}
	if contentState.Blocks[4].Text != "https://x.com/alice/status/2061877533885473181/photo/1" {
		t.Fatalf("unexpected media sub-page paragraph text: %q", contentState.Blocks[4].Text)
	}
}

func TestMarkdownBodyToDraftJSDivider(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Before\n\n---\n\nAfter")

	if len(contentState.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(contentState.Blocks))
	}
	entity := requireAtomicEntity(t, contentState, 1, 0, "DIVIDER", "Immutable")
	if len(entity.Data) != 0 {
		t.Fatalf("expected empty divider data, got %#v", entity.Data)
	}
}

func TestMarkdownBodyToDraftJSTable(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Before\n\n| Feature | Status |\n|---|---:|\n| Tweet | Native embed |\n| Divider | Native rule |\n\nAfter")

	if len(contentState.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(contentState.Blocks))
	}
	entity := requireAtomicEntity(t, contentState, 1, 0, "MARKDOWN", "Mutable")
	wantMarkdown := "| Feature | Status |\n|---|---:|\n| Tweet | Native embed |\n| Divider | Native rule |"
	if entity.Data["markdown"] != wantMarkdown {
		t.Fatalf("unexpected table markdown:\nwant: %q\n got: %q", wantMarkdown, entity.Data["markdown"])
	}
}

func TestMarkdownBodyToDraftJSInlineLink(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("See [the docs](https://example.com/docs) for more")

	if len(contentState.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(contentState.Blocks))
	}
	blk := contentState.Blocks[0]
	if blk.Text != "See the docs for more" {
		t.Fatalf("expected link syntax stripped from text, got %q", blk.Text)
	}
	if len(blk.EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(blk.EntityRanges))
	}
	er := blk.EntityRanges[0]
	if er["key"] != 0 || er["offset"] != 4 || er["length"] != 8 {
		t.Fatalf("unexpected entity range: %#v", er)
	}
	if len(contentState.EntityMap) != 1 {
		t.Fatalf("expected one entity, got %d", len(contentState.EntityMap))
	}
	entity := contentState.EntityMap[0]
	if entity.Key != "0" {
		t.Fatalf("expected entity key 0, got %q", entity.Key)
	}
	if entity.Value.Type != "LINK" || entity.Value.Mutability != "Mutable" {
		t.Fatalf("expected Mutable LINK entity, got %s/%s", entity.Value.Type, entity.Value.Mutability)
	}
	if entity.Value.Data["url"] != "https://example.com/docs" {
		t.Fatalf("expected url retained verbatim (no t.co wrap), got %#v", entity.Value.Data["url"])
	}
	// entityKey is server-generated on read-back; the write input schema
	// rejects it (GRAPHQL_VALIDATION_FAILED, live-verified 2026-06-10).
	if _, present := entity.Value.Data["entityKey"]; present {
		t.Fatalf("entityKey must not be in the write payload, got %#v", entity.Value.Data)
	}
}

func TestMarkdownBodyToDraftJSMultipleLinks(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("[a](https://a.example) mid [bb](https://b.example)")

	if len(contentState.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(contentState.Blocks))
	}
	blk := contentState.Blocks[0]
	if blk.Text != "a mid bb" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.EntityRanges) != 2 || len(contentState.EntityMap) != 2 {
		t.Fatalf("expected two distinct entities, got %d ranges / %d entities", len(blk.EntityRanges), len(contentState.EntityMap))
	}
	first, second := blk.EntityRanges[0], blk.EntityRanges[1]
	if first["key"] != 0 || first["offset"] != 0 || first["length"] != 1 {
		t.Fatalf("unexpected first range: %#v", first)
	}
	if second["key"] != 1 || second["offset"] != 6 || second["length"] != 2 {
		t.Fatalf("unexpected second range: %#v", second)
	}
	if contentState.EntityMap[0].Value.Data["url"] != "https://a.example" ||
		contentState.EntityMap[1].Value.Data["url"] != "https://b.example" {
		t.Fatalf("unexpected entity urls: %#v / %#v",
			contentState.EntityMap[0].Value.Data["url"], contentState.EntityMap[1].Value.Data["url"])
	}
	for i, ent := range contentState.EntityMap {
		if _, present := ent.Value.Data["entityKey"]; present {
			t.Fatalf("entity %d: entityKey must not be in the write payload, got %#v", i, ent.Value.Data)
		}
	}
}

func TestMarkdownBodyToDraftJSLinkMultibyteOffsets(t *testing.T) {
	// Two emoji (2 UTF-16 units each) + space = offset 5; link text
	// "café ☕" = 6 UTF-16 units (é and ☕ are BMP, 1 unit each).
	contentState := MarkdownBodyToDraftJS("🚀🚀 [café ☕](https://c.example) done")

	blk := contentState.Blocks[0]
	if blk.Text != "🚀🚀 café ☕ done" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(blk.EntityRanges))
	}
	er := blk.EntityRanges[0]
	if er["offset"] != 5 || er["length"] != 6 {
		t.Fatalf("expected UTF-16 offset 5 length 6, got %#v", er)
	}
}

func TestMarkdownBodyToDraftJSLinkInBlockquote(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("> quoted [link](https://q.example) text")

	if len(contentState.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(contentState.Blocks))
	}
	blk := contentState.Blocks[0]
	if blk.Type != "blockquote" {
		t.Fatalf("expected blockquote block type preserved, got %q", blk.Type)
	}
	if blk.Text != "quoted link text" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.EntityRanges) != 1 {
		t.Fatalf("expected entity attached to blockquote, got %d ranges", len(blk.EntityRanges))
	}
	er := blk.EntityRanges[0]
	if er["offset"] != 7 || er["length"] != 4 {
		t.Fatalf("unexpected range: %#v", er)
	}
	if contentState.EntityMap[0].Value.Type != "LINK" {
		t.Fatalf("expected LINK entity, got %q", contentState.EntityMap[0].Value.Type)
	}
}

func TestMarkdownBodyToDraftJSBoldInBlockquote(t *testing.T) {
	// Regression: extractInlineStyles already runs on blockquote blocks via
	// the unconditional post-switch inline pass. Pin that behavior.
	contentState := MarkdownBodyToDraftJS("> **bold words** rest")

	blk := contentState.Blocks[0]
	if blk.Type != "blockquote" {
		t.Fatalf("expected blockquote, got %q", blk.Type)
	}
	if blk.Text != "bold words rest" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.InlineStyleRanges) != 1 {
		t.Fatalf("expected one inline style range, got %d", len(blk.InlineStyleRanges))
	}
	style := blk.InlineStyleRanges[0]
	if style.Offset != 0 || style.Length != 10 || style.Style != "Bold" {
		t.Fatalf("unexpected style range: %#v", style)
	}
}

func TestMarkdownBodyToDraftJSInlineCode(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Run `x-twitter-pp-cli` now")

	blk := contentState.Blocks[0]
	if blk.Text != "Run x-twitter-pp-cli now" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.InlineStyleRanges) != 1 {
		t.Fatalf("expected one inline code style, got %d", len(blk.InlineStyleRanges))
	}
	style := blk.InlineStyleRanges[0]
	if style.Offset != 4 || style.Length != 16 || style.Style != "CODE" {
		t.Fatalf("unexpected CODE style range: %#v", style)
	}
}

func TestMarkdownBodyToDraftJSEmptyInlineCode(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Run `` now")

	blk := contentState.Blocks[0]
	if blk.Text != "Run  now" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.InlineStyleRanges) != 1 {
		t.Fatalf("expected one empty inline code style, got %d", len(blk.InlineStyleRanges))
	}
	style := blk.InlineStyleRanges[0]
	if style.Offset != 4 || style.Length != 0 || style.Style != "CODE" {
		t.Fatalf("unexpected CODE style range: %#v", style)
	}
}

func TestMarkdownBodyToDraftJSDowngradesDeepHeadings(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("### Third\n\n#### Fourth")

	if len(contentState.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(contentState.Blocks))
	}
	for i, want := range []string{"Third", "Fourth"} {
		blk := contentState.Blocks[i]
		if blk.Type != "header-two" || blk.Text != want {
			t.Fatalf("block %d = %q/%q, want header-two/%q", i, blk.Type, blk.Text, want)
		}
	}
}

func TestMarkdownBodyToDraftJSLinkAndBoldSameSentence(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("**Bold** then [link](https://x.example) end")

	blk := contentState.Blocks[0]
	if blk.Text != "Bold then link end" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.InlineStyleRanges) != 1 {
		t.Fatalf("expected one style range, got %d", len(blk.InlineStyleRanges))
	}
	style := blk.InlineStyleRanges[0]
	if style.Offset != 0 || style.Length != 4 || style.Style != "Bold" {
		t.Fatalf("unexpected style range: %#v", style)
	}
	if len(blk.EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(blk.EntityRanges))
	}
	er := blk.EntityRanges[0]
	if er["offset"] != 10 || er["length"] != 4 {
		t.Fatalf("unexpected link range: %#v", er)
	}
}

func TestMarkdownBodyToDraftJSBoldInsideLinkText(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("[**bold** link](https://n.example)")

	blk := contentState.Blocks[0]
	if blk.Text != "bold link" {
		t.Fatalf("unexpected text: %q", blk.Text)
	}
	if len(blk.InlineStyleRanges) != 1 {
		t.Fatalf("expected style range inside link text, got %d", len(blk.InlineStyleRanges))
	}
	style := blk.InlineStyleRanges[0]
	if style.Offset != 0 || style.Length != 4 || style.Style != "Bold" {
		t.Fatalf("unexpected style range: %#v", style)
	}
	if len(blk.EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(blk.EntityRanges))
	}
	er := blk.EntityRanges[0]
	if er["offset"] != 0 || er["length"] != 9 {
		t.Fatalf("unexpected link range: %#v", er)
	}
}

func TestMarkdownBodyToDraftJSCommentLineStripped(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("Before\n\n<!-- x-loader: insert-as-code -->\n\nAfter")

	if len(contentState.Blocks) != 2 {
		t.Fatalf("expected comment line to produce no block, got %d blocks", len(contentState.Blocks))
	}
	if contentState.Blocks[0].Text != "Before" || contentState.Blocks[1].Text != "After" {
		t.Fatalf("unexpected block texts: %q / %q", contentState.Blocks[0].Text, contentState.Blocks[1].Text)
	}
}

func TestMarkdownBodyToDraftJSInlineCommentPassesThrough(t *testing.T) {
	// Chosen behavior: only lines whose trimmed form starts with <!-- are
	// stripped. A comment embedded mid-paragraph passes through as literal
	// text (the helper-side gates flag it; the converter does not attempt
	// multi-token HTML comment parsing).
	contentState := MarkdownBodyToDraftJS("Hello <!-- hidden --> world")

	if len(contentState.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(contentState.Blocks))
	}
	if contentState.Blocks[0].Text != "Hello <!-- hidden --> world" {
		t.Fatalf("expected inline comment passed through, got %q", contentState.Blocks[0].Text)
	}
}

func TestMarkdownBodyToDraftJSLinkEdgeCasesDegradeToPlainText(t *testing.T) {
	contentState := MarkdownBodyToDraftJS("[](https://e.example)\n\n[text]()")

	if len(contentState.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(contentState.Blocks))
	}
	if contentState.Blocks[0].Text != "[](https://e.example)" {
		t.Fatalf("expected empty link text to degrade to plain text, got %q", contentState.Blocks[0].Text)
	}
	if contentState.Blocks[1].Text != "[text]()" {
		t.Fatalf("expected empty url to degrade to plain text, got %q", contentState.Blocks[1].Text)
	}
	if len(contentState.EntityMap) != 0 {
		t.Fatalf("expected no entities, got %d", len(contentState.EntityMap))
	}
}

func requireAtomicEntity(t *testing.T, contentState draftContentState, blockIndex int, entityIndex int, entityType string, mutability string) draftEntityValue {
	t.Helper()
	if blockIndex >= len(contentState.Blocks) {
		t.Fatalf("block index %d out of range; got %d blocks", blockIndex, len(contentState.Blocks))
	}
	block := contentState.Blocks[blockIndex]
	if block.Type != "atomic" {
		t.Fatalf("expected block %d to be atomic, got %q", blockIndex, block.Type)
	}
	if block.Text != " " {
		t.Fatalf("expected atomic block text to be a single space, got %q", block.Text)
	}
	if len(block.EntityRanges) != 1 {
		t.Fatalf("expected one entity range, got %d", len(block.EntityRanges))
	}
	if block.EntityRanges[0]["key"] != entityIndex {
		t.Fatalf("expected atomic block to reference entity key %d, got %#v", entityIndex, block.EntityRanges[0]["key"])
	}
	if entityIndex >= len(contentState.EntityMap) {
		t.Fatalf("entity index %d out of range; got %d entities", entityIndex, len(contentState.EntityMap))
	}
	entity := contentState.EntityMap[entityIndex]
	if entity.Key != strconv.Itoa(entityIndex) {
		t.Fatalf("expected entity key %d, got %q", entityIndex, entity.Key)
	}
	if entity.Value.Type != entityType {
		t.Fatalf("expected %s entity, got %q", entityType, entity.Value.Type)
	}
	if entity.Value.Mutability != mutability {
		t.Fatalf("expected %s entity mutability, got %q", mutability, entity.Value.Mutability)
	}
	return entity.Value
}

func TestParseInlineLinkBalancedParens(t *testing.T) {
	// PATCH: Greptile P1 — parenthesized URLs must not truncate at the inner ')'.
	state := MarkdownBodyToDraftJS("see [Rust](https://en.wikipedia.org/wiki/Rust_(programming_language)) docs")
	if len(state.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(state.Blocks))
	}
	if len(state.EntityMap) != 1 {
		t.Fatalf("expected 1 LINK entity, got %d", len(state.EntityMap))
	}
	url, _ := state.EntityMap[0].Value.Data["url"].(string)
	want := "https://en.wikipedia.org/wiki/Rust_(programming_language)"
	if url != want {
		t.Fatalf("url truncated: got %q want %q", url, want)
	}
	if state.Blocks[0].Text != "see Rust docs" {
		t.Fatalf("text wrong: %q", state.Blocks[0].Text)
	}
}
