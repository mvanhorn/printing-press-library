package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
)

type setCoverPoster struct {
	calls         []fakePostCall
	coverFailures int
}

func (f *setCoverPoster) Post(_ context.Context, path string, body any) (json.RawMessage, int, error) {
	op := path[strings.LastIndex(path, "/")+1:]
	m, _ := body.(map[string]any)
	f.calls = append(f.calls, fakePostCall{op: op, body: m})
	if op == "ArticleEntityUpdateCoverMedia" && f.coverFailures > 0 {
		f.coverFailures--
		return nil, http.StatusUnprocessableEntity, &client.APIError{
			Method:     http.MethodPost,
			Path:       path,
			StatusCode: http.StatusUnprocessableEntity,
			Body:       `{"errors":[{"message":"Missing ArticleMetadata","code":214}]}`,
		}
	}
	return json.RawMessage(`{"ok":true}`), http.StatusOK, nil
}

func TestSetCoverSelfHealsMissingArticleMetadata(t *testing.T) {
	poster := &setCoverPoster{coverFailures: 1}
	body := articleOpRequestBody("ArticleEntityUpdateCoverMedia", map[string]any{
		"articleEntityId": "123",
		"coverMedia":      map[string]any{"media_id": "media-1", "media_category": "DraftTweetImage"},
	})

	data, status, err := postCoverMediaWithMetadataHeal(context.Background(), poster, "123", body, func() (string, error) {
		return "Existing Title", nil
	})
	if err != nil {
		t.Fatalf("expected self-heal retry to succeed: %v", err)
	}
	if status != http.StatusOK || string(data) != `{"ok":true}` {
		t.Fatalf("unexpected retry response status=%d data=%s", status, data)
	}
	wantOps := []string{"ArticleEntityUpdateCoverMedia", "ArticleEntityUpdateTitle", "ArticleEntityUpdateCoverMedia"}
	if got := postOps(poster.calls); strings.Join(got, ",") != strings.Join(wantOps, ",") {
		t.Fatalf("ops = %v, want %v", got, wantOps)
	}
	titleVars, _ := poster.calls[1].body["variables"].(map[string]any)
	if titleVars["articleEntityId"] != "123" || titleVars["title"] != "Existing Title" {
		t.Fatalf("unexpected title variables: %#v", titleVars)
	}
}

func TestSetCoverMissingArticleMetadataNeedsResolvableTitle(t *testing.T) {
	poster := &setCoverPoster{coverFailures: 1}
	body := articleOpRequestBody("ArticleEntityUpdateCoverMedia", map[string]any{"articleEntityId": "123"})

	_, _, err := postCoverMediaWithMetadataHeal(context.Background(), poster, "123", body, func() (string, error) {
		return "", fmt.Errorf("not found")
	})
	if err == nil || !strings.Contains(err.Error(), "pass --title") {
		t.Fatalf("expected --title guidance, got %v", err)
	}
}

func TestIsMissingArticleMetadataError(t *testing.T) {
	err := &client.APIError{
		Method:     http.MethodPost,
		Path:       "/graphql/ArticleEntityUpdateCoverMedia",
		StatusCode: http.StatusUnprocessableEntity,
		Body:       `{"errors":[{"message":"Missing ArticleMetadata","code":214}]}`,
	}
	if !isMissingArticleMetadataError(err) {
		t.Fatalf("expected Missing ArticleMetadata code 214 to be recognized")
	}
	if isMissingArticleMetadataError(fmt.Errorf(`{"errors":[{"message":"Missing ArticleMetadata","code":215}]}`)) {
		t.Fatalf("unexpected match for non-214 metadata error")
	}
}

func TestArticleTitleFromSliceFindsNestedMetadataTitle(t *testing.T) {
	raw := json.RawMessage(`{"data":{"user":{"result":{"articles_article_mixer_slice":{"items":[{"article_entity_results":{"result":{"rest_id":"123","metadata":{"title":"Existing Title"}}}}]}}}}}`)
	title, found := articleTitleFromSlice(raw, "123")
	if !found || title != "Existing Title" {
		t.Fatalf("title=%q found=%v, want Existing Title true", title, found)
	}
}

func postOps(calls []fakePostCall) []string {
	ops := make([]string, 0, len(calls))
	for _, call := range calls {
		ops = append(ops, call.op)
	}
	return ops
}
