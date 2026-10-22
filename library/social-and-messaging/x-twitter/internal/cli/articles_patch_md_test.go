package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// patchFixtureSlice is a Published ArticleEntitiesSlice read-shape response
// with the fields a composer-made article carries: server entityKey uuids,
// block data mentions/urls, a MEDIA image, a LINK and a bold range.
const patchFixtureSlice = `{"data":{"user":{"result":{"articles_article_mixer_slice":{"items":[{"article_entity_results":{"result":{
  "rest_id":"999",
  "content_state":{
    "blocks":[
      {"key":"aaaaa","text":"Install Tailscale","type":"header-two","data":{},"entityRanges":[],"inlineStyleRanges":[]},
      {"key":"bbbbb","text":"Thanks @friend for the tip, read the docs here.","type":"unstyled","data":{"mentions":[{"fromIndex":7,"toIndex":14,"text":"friend"}]},"entityRanges":[{"key":0,"offset":37,"length":4}],"inlineStyleRanges":[{"offset":0,"length":6,"style":"Bold"}]},
      {"key":"ccccc","text":" ","type":"atomic","data":{},"entityRanges":[{"key":1,"offset":0,"length":1}],"inlineStyleRanges":[]},
      {"key":"ddddd","text":"I installed five apps:","type":"unstyled","data":{},"entityRanges":[],"inlineStyleRanges":[]},
      {"key":"eeeee","text":"The checklist","type":"header-two","data":{},"entityRanges":[],"inlineStyleRanges":[]}
    ],
    "entityMap":[
      {"key":"0","value":{"type":"LINK","mutability":"Mutable","data":{"entityKey":"11111111-uuid","url":"https://example.com/docs"}}},
      {"key":"1","value":{"type":"MEDIA","mutability":"Immutable","data":{"entityKey":"22222222-uuid","mediaItems":[{"localMediaId":"3","mediaCategory":"DraftTweetImage","mediaId":"2064751876030058496"}]}}}
    ]
  }
}}}]}}}}}`

func patchFixtureDeps(t *testing.T, poster *fakeArticlePoster) articleUpdateDeps {
	t.Helper()
	return articleUpdateDeps{
		post: poster,
		fetchSlice: func(_ context.Context, lifecycle string) (json.RawMessage, error) {
			if lifecycle == "Published" {
				return json.RawMessage(patchFixtureSlice), nil
			}
			return emptySliceResponse(t), nil
		},
		uploadImage: func(path string) (string, error) {
			t.Fatalf("unexpected image upload for %s", path)
			return "", nil
		},
	}
}

func loadPatchFixture(t *testing.T) draftContentState {
	t.Helper()
	cs, lifecycle, _, err := loadArticleContentState(context.Background(), patchFixtureDeps(t, &fakeArticlePoster{}), "999")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	if lifecycle != "Published" {
		t.Fatalf("lifecycle = %q, want Published", lifecycle)
	}
	return cs
}

func TestReadContentStateToWriteDropsServerOwnedFields(t *testing.T) {
	cs := loadPatchFixture(t)
	raw, _ := json.Marshal(articleContentStateRequest(cs))
	s := string(raw)
	for _, banned := range []string{"entityKey", "mentions", "mediaItems", "entityRanges", "inlineStyleRanges", "localMediaId"} {
		if strings.Contains(s, banned) {
			t.Errorf("write shape still contains read-shape field %q: %s", banned, s)
		}
	}
	link := cs.EntityMap[0].Value
	if link.Type != "LINK" || link.Data["url"] != "https://example.com/docs" || len(link.Data) != 1 {
		t.Errorf("LINK data = %#v, want url only", link.Data)
	}
	media := cs.EntityMap[1].Value.Data["media_items"].([]map[string]any)[0]
	if media["media_id"] != "2064751876030058496" || media["local_media_id"] != 3 || media["media_category"] != "DraftTweetImage" {
		t.Errorf("MEDIA item = %#v", media)
	}
	if cs.Blocks[1].EntityRanges[0]["key"] != 0 || cs.Blocks[2].EntityRanges[0]["key"] != 1 {
		t.Errorf("entity range keys not remapped: %#v %#v", cs.Blocks[1].EntityRanges, cs.Blocks[2].EntityRanges)
	}
	if len(cs.Blocks[1].Data) != 0 {
		t.Errorf("block data should be empty on write (server derives mentions), got %#v", cs.Blocks[1].Data)
	}
}

func TestApplyArticlePatchInsertsEmbedsAndKeepsExistingEntities(t *testing.T) {
	cs := loadPatchFixture(t)
	ops, err := parseArticlePatch(strings.Join([]string{
		"<!-- replace: I installed five apps: => I installed four apps: -->",
		"",
		"<!-- after: Thanks @friend for the tip -->",
		"Here is the post:",
		"",
		"https://x.com/someone/status/2103588600076214414",
		"",
		"<!-- before: The checklist -->",
		"## New section",
		"",
		"https://x.com/someone/status/2112996650289451225",
	}, "\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, summary, err := applyArticlePatch(cs, ops)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	var texts []string
	for _, b := range out.Blocks {
		texts = append(texts, b.Type+":"+b.Text)
	}
	want := []string{
		"header-two:Install Tailscale",
		"unstyled:Thanks @friend for the tip, read the docs here.",
		"unstyled:Here is the post:",
		"atomic: ",
		"atomic: ",
		"unstyled:I installed four apps:",
		"header-two:New section",
		"atomic: ",
		"header-two:The checklist",
	}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks =\n%s\nwant\n%s", strings.Join(texts, "\n"), strings.Join(want, "\n"))
	}
	// Original entities keep their indexes; new TWEETs are appended.
	if out.EntityMap[0].Value.Type != "LINK" || out.EntityMap[1].Value.Type != "MEDIA" {
		t.Fatalf("existing entities moved: %#v", out.EntityMap[:2])
	}
	if out.Blocks[3].EntityRanges[0]["key"] != 2 || out.EntityMap[2].Value.Data["tweet_id"] != "2103588600076214414" {
		t.Errorf("first embed wired to %#v / %#v", out.Blocks[3].EntityRanges, out.EntityMap[2])
	}
	if out.Blocks[4].EntityRanges[0]["key"] != 1 {
		t.Errorf("original image block lost its MEDIA entity: %#v", out.Blocks[4].EntityRanges)
	}
	if out.Blocks[7].EntityRanges[0]["key"] != 3 || out.EntityMap[3].Value.Data["tweet_id"] != "2112996650289451225" {
		t.Errorf("second embed wired to %#v / %#v", out.Blocks[7].EntityRanges, out.EntityMap[3])
	}
	if summary.AddedEntities["TWEET"] != 2 || summary.BlocksBefore != 5 || summary.BlocksAfter != 9 {
		t.Errorf("summary = %#v", summary)
	}
	keys := map[string]bool{}
	for _, b := range out.Blocks {
		if keys[b.Key] {
			t.Errorf("duplicate block key %s", b.Key)
		}
		keys[b.Key] = true
	}
}

func TestReplaceShiftsRangesAfterTheEdit(t *testing.T) {
	cs := loadPatchFixture(t)
	ops, _ := parseArticlePatch("<!-- replace: for the tip => for the excellent tip -->")
	out, _, err := applyArticlePatch(cs, ops)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	b := out.Blocks[1]
	if b.Text != "Thanks @friend for the excellent tip, read the docs here." {
		t.Fatalf("text = %q", b.Text)
	}
	// LINK on "docs" moves right by len("excellent ") = 10; Bold on "Thanks" stays.
	if b.EntityRanges[0]["offset"] != 47 || b.EntityRanges[0]["length"] != 4 {
		t.Errorf("link range = %#v, want offset 47 length 4", b.EntityRanges[0])
	}
	if b.InlineStyleRanges[0].Offset != 0 || b.InlineStyleRanges[0].Length != 6 {
		t.Errorf("bold range = %#v", b.InlineStyleRanges[0])
	}
	if got := b.Text[47:51]; got != "docs" {
		t.Errorf("link now covers %q", got)
	}
}

func TestApplyArticlePatchRefusesUnsafeEdits(t *testing.T) {
	cases := map[string]string{
		"missing anchor":   "<!-- after: No such paragraph -->\nhello",
		"ambiguous anchor": "<!-- after: I -->\nhello",
		"replace twice":    "<!-- replace: e => E -->",
		"replace cut link": "<!-- replace: the do => a do -->",
	}
	for name, patch := range cases {
		t.Run(name, func(t *testing.T) {
			ops, err := parseArticlePatch(patch)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if _, _, err := applyArticlePatch(loadPatchFixture(t), ops); err == nil {
				t.Fatalf("expected refusal")
			}
		})
	}
}

func TestParseArticlePatchRejectsMalformedFiles(t *testing.T) {
	for name, patch := range map[string]string{
		"no directives":   "just text",
		"text first":      "hello\n<!-- after: Install -->\nx",
		"empty insert":    "<!-- after: Install -->\n\n",
		"replace no sep":  "<!-- replace: five four -->",
		"replace w/ body": "<!-- replace: a => b -->\nstray",
	} {
		if _, err := parseArticlePatch(patch); err == nil {
			t.Errorf("%s: expected parse error", name)
		}
	}
}

func TestReadContentStateRefusesUnknownEntityTypes(t *testing.T) {
	raw := strings.Replace(patchFixtureSlice, `"type":"LINK"`, `"type":"POLL"`, 1)
	deps := articleUpdateDeps{fetchSlice: func(_ context.Context, lifecycle string) (json.RawMessage, error) {
		if lifecycle == "Published" {
			return json.RawMessage(raw), nil
		}
		return emptySliceResponse(t), nil
	}}
	if _, _, _, err := loadArticleContentState(context.Background(), deps, "999"); err == nil || !strings.Contains(err.Error(), "POLL") {
		t.Fatalf("expected POLL refusal, got %v", err)
	}
}

func TestPatchPublishedArticleUsesRepublishSequence(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := patchFixtureDeps(t, poster)
	cs, _, fp, err := loadArticleContentState(context.Background(), deps, "999")
	if err != nil {
		t.Fatal(err)
	}
	ops, _ := parseArticlePatch("<!-- after: Install Tailscale -->\nhttps://x.com/someone/status/2103588600076214414")
	patched, _, err := applyArticlePatch(cs, ops)
	if err != nil {
		t.Fatal(err)
	}
	result, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{articleID: "999", contentState: patched, republish: true, allowedEntityTypes: patchCarrySet(), baseFingerprint: fp})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := strings.Join(poster.ops(), ","); got != "ArticleEntityUnpublish,ArticleEntityUpdateContent,ArticleEntityPublish" {
		t.Fatalf("ops = %s (no title call expected)", got)
	}
	body, _ := json.Marshal(poster.calls[1].body)
	for _, want := range []string{`"media_id":"2064751876030058496"`, `"tweet_id":"2103588600076214414"`, `"url":"https://example.com/docs"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("UpdateContent body missing %s", want)
		}
	}
	if !result.Republished {
		t.Errorf("result not marked republished")
	}
	// Without --republish a published article is refused before any write.
	poster2 := &fakeArticlePoster{}
	if _, err := updateMarkdownArticle(context.Background(), patchFixtureDeps(t, poster2), articleUpdateOptions{articleID: "999", contentState: patched}); err == nil || len(poster2.calls) != 0 {
		t.Fatalf("expected refusal with zero writes, got err=%v calls=%v", err, poster2.ops())
	}
}

func TestPatchRefusesWhenArticleChangedSinceRead(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := patchFixtureDeps(t, poster)
	cs, _, fp, err := loadArticleContentState(context.Background(), deps, "999")
	if err != nil {
		t.Fatal(err)
	}
	// The composer saves an edit between our read and the write.
	edited := strings.Replace(patchFixtureSlice, "The checklist", "The new checklist", 1)
	deps.fetchSlice = func(_ context.Context, lifecycle string) (json.RawMessage, error) {
		if lifecycle == "Published" {
			return json.RawMessage(edited), nil
		}
		return emptySliceResponse(t), nil
	}
	_, err = updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{articleID: "999", contentState: cs, republish: true, allowedEntityTypes: patchCarrySet(), baseFingerprint: fp})
	if err == nil || !strings.Contains(err.Error(), "changed since it was read") {
		t.Fatalf("expected changed-body refusal, got %v", err)
	}
	if len(poster.calls) != 0 {
		t.Fatalf("expected zero writes, got %v", poster.ops())
	}
}

func TestPatchCarriesTwemojiArticles(t *testing.T) {
	withEmoji := strings.Replace(patchFixtureSlice, `{"key":"0","value":{"type":"LINK"`, `{"key":"9","value":{"type":"TWEMOJI","mutability":"Immutable","data":{"url":"https://abs.twimg.com/emoji/v2/svg/1f99e.svg"}}},{"key":"0","value":{"type":"LINK"`, 1)
	poster := &fakeArticlePoster{}
	deps := articleUpdateDeps{post: poster, fetchSlice: func(_ context.Context, lifecycle string) (json.RawMessage, error) {
		if lifecycle == "Published" {
			return json.RawMessage(withEmoji), nil
		}
		return emptySliceResponse(t), nil
	}}
	cs, _, fp, err := loadArticleContentState(context.Background(), deps, "999")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{articleID: "999", contentState: cs, republish: true, allowedEntityTypes: patchCarrySet(), baseFingerprint: fp}); err != nil {
		t.Fatalf("TWEMOJI article should patch, got %v", err)
	}
	body, _ := json.Marshal(poster.calls[1].body)
	if !strings.Contains(string(body), "1f99e.svg") {
		t.Errorf("TWEMOJI entity not carried over")
	}
	// update-md keeps its stricter converter-only preflight.
	poster2 := &fakeArticlePoster{}
	deps.post = poster2
	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{articleID: "999", contentState: cs, republish: true}); err == nil || len(poster2.calls) != 0 {
		t.Fatalf("update-md should still refuse TWEMOJI, got err=%v calls=%v", err, poster2.ops())
	}
}

func TestParseArticlePatchIgnoresDirectivesInsideFences(t *testing.T) {
	ops, err := parseArticlePatch(strings.Join([]string{
		"<!-- after: Install Tailscale -->",
		"Example patch file:",
		"",
		"```markdown",
		"<!-- replace: old => new -->",
		"```",
	}, "\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ops) != 1 || ops[0].Kind != "after" || !strings.Contains(ops[0].Markdown, "<!-- replace: old => new -->") {
		t.Fatalf("fenced directive was not kept as content: %#v", ops)
	}
	// An unclosed fence must not turn a sample into a live replace either.
	ops, err = parseArticlePatch("<!-- after: Install Tailscale -->\n```text\n<!-- replace: Install => Remove -->")
	if err != nil || len(ops) != 1 {
		t.Fatalf("unclosed fence: ops=%#v err=%v", ops, err)
	}
}

func TestReplaceRefusesOverlappingMatches(t *testing.T) {
	blocks := []draftBlock{{Key: "a", Type: "unstyled", Text: "banana", EntityRanges: []map[string]any{}, InlineStyleRanges: []inlineStyle{}}}
	if err := replaceInBlocks(blocks, articlePatchOp{Kind: "replace", Anchor: "ana", New: "X", Line: 1}); err == nil {
		t.Fatalf("overlapping matches should be ambiguous, got %q", blocks[0].Text)
	}
}
