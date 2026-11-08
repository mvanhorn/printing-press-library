package mcp

import (
	"strings"
	"testing"
)

// The preview has to show the path the CLI would send, and the CLI encodes a bare
// dot segment rather than letting it walk up the proxy path.
func TestCodeOrchPreviewEncodesDotSegments(t *testing.T) {
	got := codeOrchPreviewPath("/v1/sites/{siteId}/clients", map[string]string{"siteId": ".."})
	if strings.Contains(got, "/../") || !strings.Contains(got, "%2E%2E") {
		t.Fatalf("preview path = %q", got)
	}
}
