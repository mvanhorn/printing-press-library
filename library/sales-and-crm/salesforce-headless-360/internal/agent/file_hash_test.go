package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestContentDocumentLinkAssemblyHashesContentVersion(t *testing.T) {
	raw := json.RawMessage(`{
		"Id": "06A_LINK",
		"Title": "contract.pdf",
		"LatestPublishedVersionId": "068_VERSION"
	}`)

	file, ok, err := fileRefFromContentDocumentLink(context.Background(), raw, mapContentFetcher{"068_VERSION": "hello"})
	if err != nil {
		t.Fatalf("fileRefFromContentDocumentLink: %v", err)
	}
	if !ok {
		t.Fatal("expected file ref")
	}
	if file.ContentVersionID != "068_VERSION" {
		t.Fatalf("content version = %s", file.ContentVersionID)
	}
	if file.SizeBytes != 5 {
		t.Fatalf("size = %d", file.SizeBytes)
	}
	if file.SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("sha256 = %s", file.SHA256)
	}
}
