package cli

import (
	"encoding/json"
	"testing"
)

func TestArticleFriendlyRequestBodies(t *testing.T) {
	deleteBody, err := articleDeleteRequestBody("111")
	if err != nil {
		t.Fatalf("delete body: %v", err)
	}
	deleteVars, _ := deleteBody["variables"].(map[string]any)
	if deleteVars["articleEntityId"] != "111" {
		t.Fatalf("unexpected delete variables: %#v", deleteVars)
	}
	if deleteBody["queryId"] == "" {
		t.Fatalf("expected delete queryId")
	}

	titleBody, err := articleUpdateTitleRequestBody("222", "New")
	if err != nil {
		t.Fatalf("title body: %v", err)
	}
	titleVars, _ := titleBody["variables"].(map[string]any)
	if titleVars["articleEntityId"] != "222" || titleVars["title"] != "New" {
		t.Fatalf("unexpected title variables: %#v", titleVars)
	}

	coverBody, err := articleUpdateCoverRequestBody("333", "media-1")
	if err != nil {
		t.Fatalf("cover body: %v", err)
	}
	coverVars, _ := coverBody["variables"].(map[string]any)
	coverMedia, _ := coverVars["coverMedia"].(map[string]any)
	if coverVars["articleEntityId"] != "333" || coverMedia["media_id"] != "media-1" || coverMedia["media_category"] != "DraftTweetImage" {
		t.Fatalf("unexpected cover variables: %#v", coverVars)
	}
}

func TestArticleUpdateContentRequestBodyFromPreviewContentState(t *testing.T) {
	raw := []byte(`{
		"content_state": {
			"blocks": [
				{"data":{},"text":"Hello","key":"abcde","type":"unstyled","entity_ranges":[],"inline_style_ranges":[]}
			],
			"entityMap": [
				{"key":"0","value":{"data":{"url":"https://example.com"},"type":"LINK","mutability":"Mutable"}}
			]
		}
	}`)
	cs, err := parseArticleContentStateJSON(raw)
	if err != nil {
		t.Fatalf("parse content state: %v", err)
	}
	body, err := articleUpdateContentRequestBody("444", cs)
	if err != nil {
		t.Fatalf("content body: %v", err)
	}
	vars, _ := body["variables"].(map[string]any)
	if vars["article_entity"] != "444" {
		t.Fatalf("unexpected article_entity: %#v", vars)
	}
	requestState, _ := vars["content_state"].(map[string]any)
	if _, ok := requestState["entity_map"].([]draftEntity); !ok {
		encoded, _ := json.Marshal(requestState)
		t.Fatalf("expected snake_case entity_map in request state, got %s", encoded)
	}
}
