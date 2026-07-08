package agent

import (
	"context"
	"io"
	"strings"
	"testing"
)

type mapContentFetcher map[string]string

func (m mapContentFetcher) FetchContentVersion(_ context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m[id])), nil
}

func TestHashContentVersionStreamsBytes(t *testing.T) {
	sha, size, err := HashContentVersion(context.Background(), mapContentFetcher{"068A": "hello"}, "068A")
	if err != nil {
		t.Fatalf("HashContentVersion: %v", err)
	}
	if size != 5 {
		t.Fatalf("size mismatch: %d", size)
	}
	if sha != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("sha mismatch: %s", sha)
	}
}
