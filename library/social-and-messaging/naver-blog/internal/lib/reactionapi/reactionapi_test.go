package reactionapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetReactionsHappyPath(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("pool"); got != "blogid" {
			t.Errorf("pool = %q, want blogid", got)
		}
		q := r.URL.Query().Get("q")
		// q should be BLOG[selly9401_1,foodie_2]
		if !strings.HasPrefix(q, "BLOG[") || !strings.HasSuffix(q, "]") {
			t.Errorf("q param shape wrong: %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"contents": [
				{"contentsId":"selly9401_1","reactions":[{"reactionType":"like","count":5}]},
				{"contentsId":"foodie_2","reactions":[{"reactionType":"like","count":12}]}
			]
		}`))
	}))

	oldEndpoint := reactionsEndpoint
	reactionsEndpoint = "https://reaction.test/search/contents"
	t.Cleanup(func() { reactionsEndpoint = oldEndpoint })

	got, err := GetReactions(context.Background(), client, []PostKey{
		{BlogID: "selly9401", LogNo: "1"},
		{BlogID: "foodie", LogNo: "2"},
	})
	if err != nil {
		t.Fatalf("GetReactions: %v", err)
	}
	if got["selly9401_1"] != 5 {
		t.Errorf("selly9401_1 = %d, want 5", got["selly9401_1"])
	}
	if got["foodie_2"] != 12 {
		t.Errorf("foodie_2 = %d, want 12", got["foodie_2"])
	}
}

func TestGetReactionsMissingPostsAbsent(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return only the first key; the second was "deleted".
		w.Write([]byte(`{
			"contents": [
				{"contentsId":"a_1","reactions":[{"reactionType":"like","count":3}]}
			]
		}`))
	}))
	oldEndpoint := reactionsEndpoint
	reactionsEndpoint = "https://reaction.test/search/contents"
	t.Cleanup(func() { reactionsEndpoint = oldEndpoint })

	got, err := GetReactions(context.Background(), client, []PostKey{
		{BlogID: "a", LogNo: "1"},
		{BlogID: "deleted", LogNo: "2"},
	})
	if err != nil {
		t.Fatalf("GetReactions: %v", err)
	}
	if _, ok := got["a_1"]; !ok {
		t.Error("expected a_1 to be present")
	}
	if _, ok := got["deleted_2"]; ok {
		t.Error("expected deleted_2 to be absent (unknown), but it was present")
	}
}

func TestGetReactionsBatching(t *testing.T) {
	requestCount := 0
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Write([]byte(`{"contents":[]}`))
	}))
	oldEndpoint := reactionsEndpoint
	reactionsEndpoint = "https://reaction.test/search/contents"
	t.Cleanup(func() { reactionsEndpoint = oldEndpoint })

	keys := make([]PostKey, 25)
	for i := range keys {
		keys[i] = PostKey{BlogID: "b", LogNo: "x"}
	}
	if _, err := GetReactions(context.Background(), client, keys); err != nil {
		t.Fatalf("GetReactions: %v", err)
	}
	// 25 keys / MaxBatchSize=20 = 2 batches.
	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2 (batched at MaxBatchSize=%d)", requestCount, MaxBatchSize)
	}
}

func TestGetReactionsEmpty(t *testing.T) {
	got, err := GetReactions(context.Background(), http.DefaultClient, nil)
	if err != nil {
		t.Fatalf("GetReactions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestGetReactionsAPIError(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	oldEndpoint := reactionsEndpoint
	reactionsEndpoint = "https://reaction.test/search/contents"
	t.Cleanup(func() { reactionsEndpoint = oldEndpoint })

	_, err := GetReactions(context.Background(), client, []PostKey{{BlogID: "a", LogNo: "1"}})
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			resp := rr.Result()
			if resp.Body == nil {
				resp.Body = io.NopCloser(strings.NewReader(""))
			}
			return resp, nil
		}),
	}
}
