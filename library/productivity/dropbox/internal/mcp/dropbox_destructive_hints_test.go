package mcp

import (
	"os"
	"regexp"
	"testing"
)

// Generated endpoint tools default every non-read POST to a false
// destructive hint. Dropbox routes everything through POST, so the tools
// below are patched to true; this test fails if a regeneration drops that.
func TestDropboxMutatingToolsAreDestructive(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped while `generate --force` validates an unpatched tree; reapply patches, then run the full suite")
	}
	src, err := os.ReadFile("tools.go")
	if err != nil {
		t.Fatal(err)
	}
	tools := []string{
		"files_delete", "files_delete-batch", "files_move", "files_move-batch",
		"files_restore", "files_tags-remove", "sharing_revoke-shared-link",
		"file-requests_delete-all-closed",
	}
	starts := regexp.MustCompile(`mcplib\.NewTool\("([^"]+)"`).FindAllSubmatchIndex(src, -1)
	for _, name := range tools {
		found := false
		for i, m := range starts {
			if string(src[m[2]:m[3]]) != name {
				continue
			}
			end := len(src)
			if i+1 < len(starts) {
				end = starts[i+1][0]
			}
			block := src[m[0]:end]
			found = true
			if !regexp.MustCompile(`WithDestructiveHintAnnotation\(true\)`).Match(block) {
				t.Errorf("%s must carry WithDestructiveHintAnnotation(true)", name)
			}
		}
		if !found {
			t.Errorf("tool %s not found in tools.go", name)
		}
	}
}
