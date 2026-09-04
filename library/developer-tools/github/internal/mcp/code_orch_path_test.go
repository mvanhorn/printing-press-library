package mcp

import (
	"testing"
)

func TestCodeOrchPathSubstitutionEscapesReservedChars(t *testing.T) {
	t.Parallel()
	path := "/repos/{owner}/{repo}/contents/{path}"
	path = codeOrchFillPath(path, "owner", "cli")
	path = codeOrchFillPath(path, "repo", "cli")
	path = codeOrchFillPath(path, "path", "docs/a?b#c%")
	if path != "/repos/cli/cli/contents/docs/a%3Fb%23c%25" {
		t.Fatalf("got %q", path)
	}
}
