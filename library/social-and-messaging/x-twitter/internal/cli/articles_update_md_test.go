package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeArticlePoster records every GraphQL POST and can fail per-operation.
// No network: the path's trailing segment is the operation name.
type fakeArticlePoster struct {
	calls           []fakePostCall
	fail            map[string]error
	publishResponse json.RawMessage
}

type fakePostCall struct {
	op   string
	body map[string]any
}

func (f *fakeArticlePoster) Post(_ context.Context, path string, body any) (json.RawMessage, int, error) {
	op := path[strings.LastIndex(path, "/")+1:]
	m, _ := body.(map[string]any)
	f.calls = append(f.calls, fakePostCall{op: op, body: m})
	if err := f.fail[op]; err != nil {
		return nil, 422, err
	}
	if op == "ArticleEntityPublish" && f.publishResponse != nil {
		return f.publishResponse, 200, nil
	}
	return json.RawMessage(`{}`), 200, nil
}

func (f *fakeArticlePoster) ops() []string {
	ops := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		ops = append(ops, c.op)
	}
	return ops
}

// fakeSliceResponse builds an ArticleEntitiesSlice read-shape response
// containing one article with the given entity types.
func fakeSliceResponse(t *testing.T, articleID string, entityTypes ...string) json.RawMessage {
	t.Helper()
	entityMap := []map[string]any{}
	for i, entityType := range entityTypes {
		entityMap = append(entityMap, map[string]any{
			"key":   string(rune('0' + i)),
			"value": map[string]any{"type": entityType, "mutability": "Mutable", "data": map[string]any{}},
		})
	}
	payload := map[string]any{
		"data": map[string]any{
			"user": map[string]any{
				"result": map[string]any{
					"articles_article_mixer_slice": map[string]any{
						"items": []map[string]any{{
							"article_entity_results": map[string]any{
								"result": map[string]any{
									"rest_id":       articleID,
									"content_state": map[string]any{"blocks": []any{}, "entityMap": entityMap},
								},
							},
						}},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fake slice: %v", err)
	}
	return raw
}

func emptySliceResponse(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"user": map[string]any{
				"result": map[string]any{
					"articles_article_mixer_slice": map[string]any{"items": []any{}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal empty slice: %v", err)
	}
	return raw
}

// testDeps wires a fake poster with the article living in the given
// lifecycle slice ("Draft" or "Published").
func testDeps(t *testing.T, poster *fakeArticlePoster, articleID string, lifecycle string, entityTypes ...string) articleUpdateDeps {
	t.Helper()
	return articleUpdateDeps{
		post: poster,
		fetchSlice: func(_ context.Context, requested string) (json.RawMessage, error) {
			if requested == lifecycle {
				return fakeSliceResponse(t, articleID, entityTypes...), nil
			}
			return emptySliceResponse(t), nil
		},
		uploadImage: func(path string) (string, error) {
			t.Fatalf("unexpected image upload for %s", path)
			return "", nil
		},
	}
}

func TestUpdateMarkdownArticleHappyPath(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Draft", "LINK", "MEDIA")
	contentState := MarkdownBodyToDraftJS("Hello [x](https://e.example)")

	result, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		title:        "New Title",
		contentState: contentState,
	})
	if err != nil {
		t.Fatalf("updateMarkdownArticle returned error: %v", err)
	}

	wantOps := []string{"ArticleEntityUpdateTitle", "ArticleEntityUpdateContent"}
	if strings.Join(poster.ops(), ",") != strings.Join(wantOps, ",") {
		t.Fatalf("unexpected call sequence: %v", poster.ops())
	}

	titleBody := poster.calls[0].body
	titleVars, _ := titleBody["variables"].(map[string]any)
	if titleVars["articleEntityId"] != "111" || titleVars["title"] != "New Title" {
		t.Fatalf("unexpected UpdateTitle variables: %#v", titleVars)
	}
	if queryID, _ := titleBody["queryId"].(string); queryID == "" {
		t.Fatalf("expected UpdateTitle queryId resolved through article-ops table, got empty")
	}

	contentBody := poster.calls[1].body
	contentVars, _ := contentBody["variables"].(map[string]any)
	if contentVars["article_entity"] != "111" {
		t.Fatalf("expected article_entity variable, got %#v", contentVars)
	}
	requestState, _ := contentVars["content_state"].(map[string]any)
	if requestState == nil {
		t.Fatalf("expected content_state variable, got %#v", contentVars["content_state"])
	}
	if _, ok := requestState["blocks"]; !ok {
		t.Fatalf("expected content_state.blocks, got %#v", requestState)
	}
	if _, ok := requestState["entity_map"]; !ok {
		t.Fatalf("expected snake_case content_state.entity_map, got %#v", requestState)
	}
	features, _ := contentBody["features"].(map[string]any)
	if len(features) != len(articleGraphQLFeatures()) {
		t.Fatalf("expected shared articleGraphQLFeatures map, got %#v", features)
	}

	if result.ArticleID != "111" || result.Lifecycle != "Draft" || result.Republished {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.URL != "https://x.com/compose/article/edit/111" {
		t.Fatalf("unexpected result url: %q", result.URL)
	}
}

func TestUpdateMarkdownArticleUploadsNewImages(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Draft")
	uploads := []string{}
	deps.uploadImage = func(path string) (string, error) {
		uploads = append(uploads, path)
		return "media-999", nil
	}
	contentState := MarkdownBodyToDraftJS("Updated\n\n![stat block](./stats.png)")

	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: contentState,
	}); err != nil {
		t.Fatalf("updateMarkdownArticle returned error: %v", err)
	}
	if len(uploads) != 1 || uploads[0] != "./stats.png" {
		t.Fatalf("expected one upload of ./stats.png, got %#v", uploads)
	}

	contentVars, _ := poster.calls[len(poster.calls)-1].body["variables"].(map[string]any)
	requestState, _ := contentVars["content_state"].(map[string]any)
	entities, ok := requestState["entity_map"].([]draftEntity)
	if !ok || len(entities) != 1 {
		t.Fatalf("expected one entity in entity_map, got %#v", requestState["entity_map"])
	}
	mediaItems, ok := entities[0].Value.Data["media_items"].([]map[string]any)
	if !ok || len(mediaItems) != 1 || mediaItems[0]["media_id"] != "media-999" {
		t.Fatalf("expected MEDIA entity rebound to uploaded media id, got %#v", entities[0].Value.Data)
	}
}

func TestUpdateMarkdownArticleUpdatesCoverWhenPresent(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Draft")
	uploads := []string{}
	deps.uploadImage = func(path string) (string, error) {
		uploads = append(uploads, path)
		return "media-cover", nil
	}

	result, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		coverPath:    "./cover.jpg",
		contentState: MarkdownBodyToDraftJS("Updated"),
	})
	if err != nil {
		t.Fatalf("updateMarkdownArticle returned error: %v", err)
	}
	if strings.Join(uploads, ",") != "./cover.jpg" {
		t.Fatalf("expected cover upload only, got %#v", uploads)
	}
	wantOps := []string{"ArticleEntityUpdateContent", "ArticleEntityUpdateCoverMedia"}
	if strings.Join(poster.ops(), ",") != strings.Join(wantOps, ",") {
		t.Fatalf("unexpected call sequence: %v", poster.ops())
	}
	coverVars, _ := poster.calls[1].body["variables"].(map[string]any)
	if coverVars["articleEntityId"] != "111" {
		t.Fatalf("unexpected cover variables: %#v", coverVars)
	}
	coverMedia, _ := coverVars["coverMedia"].(map[string]any)
	if coverMedia["media_id"] != "media-cover" || coverMedia["media_category"] != "DraftTweetImage" {
		t.Fatalf("unexpected coverMedia: %#v", coverMedia)
	}
	if result.CoverMediaID != "media-cover" {
		t.Fatalf("CoverMediaID = %q, want media-cover", result.CoverMediaID)
	}
}

func TestUpdateMarkdownArticleMissingArticleID(t *testing.T) {
	poster := &fakeArticlePoster{}
	fetchCalls := 0
	deps := articleUpdateDeps{
		post: poster,
		fetchSlice: func(_ context.Context, _ string) (json.RawMessage, error) {
			fetchCalls++
			return emptySliceResponse(t), nil
		},
	}

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{articleID: "  "})
	if err == nil || !strings.Contains(err.Error(), "--article-id is required") {
		t.Fatalf("expected --article-id error, got %v", err)
	}
	if fetchCalls != 0 || len(poster.calls) != 0 {
		t.Fatalf("expected no network intent, got %d fetches / %d posts", fetchCalls, len(poster.calls))
	}
}

func TestUpdateMarkdownArticleDraftOnlyDefault(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Draft")

	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
	}); err != nil {
		t.Fatalf("updateMarkdownArticle returned error: %v", err)
	}
	for _, op := range poster.ops() {
		if op == "ArticleEntityUnpublish" || op == "ArticleEntityPublish" {
			t.Fatalf("expected no publish lifecycle calls without --republish, got %v", poster.ops())
		}
	}
}

func TestUpdateMarkdownArticleRepublishSequence(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Published")

	result, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
		republish:    true,
	})
	if err != nil {
		t.Fatalf("updateMarkdownArticle returned error: %v", err)
	}
	wantOps := []string{"ArticleEntityUnpublish", "ArticleEntityUpdateContent", "ArticleEntityPublish"}
	if strings.Join(poster.ops(), ",") != strings.Join(wantOps, ",") {
		t.Fatalf("expected Unpublish -> UpdateContent -> Publish, got %v", poster.ops())
	}
	if !result.Republished || result.Lifecycle != "Published" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestUpdateMarkdownArticleTitleSkippedWhenAbsent(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Draft")

	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
	}); err != nil {
		t.Fatalf("updateMarkdownArticle returned error: %v", err)
	}
	for _, op := range poster.ops() {
		if op == "ArticleEntityUpdateTitle" {
			t.Fatalf("expected UpdateTitle skipped without a frontmatter title, got %v", poster.ops())
		}
	}
}

func TestUpdateMarkdownArticleUpdateContentErrorSurfaces(t *testing.T) {
	poster := &fakeArticlePoster{
		fail: map[string]error{
			"ArticleEntityUpdateContent": errors.New(`POST ArticleEntityUpdateContent returned HTTP 422: {"errors":[{"message":"content_state cannot be null"}]}`),
		},
	}
	deps := testDeps(t, poster, "111", "Draft")

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
	})
	if err == nil || !strings.Contains(err.Error(), "content_state cannot be null") {
		t.Fatalf("expected GraphQL error body to surface, got %v", err)
	}
}

func TestUpdateMarkdownArticlePreflightRefusesUnknownEntities(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Draft", "LINK", "POLL")

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
	})
	if err == nil || !strings.Contains(err.Error(), "POLL") || !strings.Contains(err.Error(), "--replace-unknown-entities") {
		t.Fatalf("expected unknown-entity refusal naming POLL, got %v", err)
	}
	if len(poster.calls) != 0 {
		t.Fatalf("expected no mutations after refusal, got %v", poster.ops())
	}

	// With the override flag the update proceeds.
	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:              "111",
		contentState:           MarkdownBodyToDraftJS("body"),
		replaceUnknownEntities: true,
	}); err != nil {
		t.Fatalf("expected --replace-unknown-entities to proceed, got %v", err)
	}
	if len(poster.calls) == 0 {
		t.Fatalf("expected mutations with override flag")
	}
}

func TestUpdateMarkdownArticlePreflightAllowsConverterEmitSet(t *testing.T) {
	poster := &fakeArticlePoster{}
	// TWEET is in the converter's emit set (markdown-authored tweet embeds
	// round-trip), so it must NOT trip the preflight.
	deps := testDeps(t, poster, "111", "Draft", "LINK", "MARKDOWN", "MEDIA", "TWEET", "DIVIDER")

	if _, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
	}); err != nil {
		t.Fatalf("expected converter-emitted entity types to pass preflight, got %v", err)
	}
	if len(poster.calls) == 0 {
		t.Fatalf("expected update to proceed")
	}
}

func TestUpdateMarkdownArticlePublishedRequiresRepublish(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "111", "Published")

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "111",
		contentState: MarkdownBodyToDraftJS("body"),
	})
	if err == nil || !strings.Contains(err.Error(), "articles update-md <markdown-file> --article-id 111 --republish") {
		t.Fatalf("expected published-article refusal to name update-md --republish, got %v", err)
	}
	if len(poster.calls) != 0 {
		t.Fatalf("expected no mutations, got %v", poster.ops())
	}
}

func TestUpdateMarkdownArticleNotFound(t *testing.T) {
	poster := &fakeArticlePoster{}
	deps := articleUpdateDeps{
		post: poster,
		fetchSlice: func(_ context.Context, _ string) (json.RawMessage, error) {
			return emptySliceResponse(t), nil
		},
	}

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "404404",
		contentState: MarkdownBodyToDraftJS("body"),
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if len(poster.calls) != 0 {
		t.Fatalf("expected no mutations, got %v", poster.ops())
	}
}

func TestUpdateMarkdownArticleRepublishOnDraftRefuses(t *testing.T) {
	// PATCH: Greptile P1 — --republish on a Draft must refuse, not silently publish.
	poster := &fakeArticlePoster{}
	deps := testDeps(t, poster, "222", "Draft")

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "222",
		contentState: MarkdownBodyToDraftJS("body"),
		republish:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "not published") {
		t.Fatalf("expected refusal for --republish on a draft, got err=%v", err)
	}
	if len(poster.ops()) != 0 {
		t.Fatalf("expected zero mutations after refusal, got %v", poster.ops())
	}
}

func TestUpdateMarkdownArticleRepublishRestoresOnFailure(t *testing.T) {
	// PATCH: Greptile P1 — a failure after Unpublish must attempt restore publish.
	poster := &fakeArticlePoster{fail: map[string]error{"ArticleEntityUpdateContent": fmt.Errorf("boom")}}
	deps := testDeps(t, poster, "333", "Published")

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "333",
		contentState: MarkdownBodyToDraftJS("body"),
		republish:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "re-published with its prior content") {
		t.Fatalf("expected restore-publish wrap, got err=%v", err)
	}
	wantOps := []string{"ArticleEntityUnpublish", "ArticleEntityUpdateContent", "ArticleEntityPublish"}
	if strings.Join(poster.ops(), ",") != strings.Join(wantOps, ",") {
		t.Fatalf("expected restore publish after failure, got %v", poster.ops())
	}
}

func TestUpdateMarkdownArticleRepublishFinalPublishFailureHasContext(t *testing.T) {
	poster := &fakeArticlePoster{fail: map[string]error{"ArticleEntityPublish": fmt.Errorf("publish down")}}
	deps := testDeps(t, poster, "444", "Published")

	_, err := updateMarkdownArticle(context.Background(), deps, articleUpdateOptions{
		articleID:    "444",
		contentState: MarkdownBodyToDraftJS("body"),
		republish:    true,
	})
	if err == nil {
		t.Fatalf("expected final publish failure")
	}
	for _, want := range []string{"was updated but final publish failed", "left UNPUBLISHED", "articles update-md --article-id 444 --republish"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
	wantOps := []string{"ArticleEntityUnpublish", "ArticleEntityUpdateContent", "ArticleEntityPublish"}
	if strings.Join(poster.ops(), ",") != strings.Join(wantOps, ",") {
		t.Fatalf("expected final publish failure after update, got %v", poster.ops())
	}
}
