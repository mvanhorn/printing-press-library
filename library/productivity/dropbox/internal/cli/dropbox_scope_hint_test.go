package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
)

func TestDropboxMissingScopeHint(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped while `generate --force` validates an unpatched tree; reapply patches, then run the full suite")
	}
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{400, `{"error_summary":"missing_scope/..","error":{".tag":"missing_scope","required_scope":"sharing.read"}}`, "sharing.read"},
		{401, `{"error_summary":"missing_scope/..","message":"Your app does not have the required scope 'files.content.read'"}`, "files.content.read"},
	} {
		err := classifyAPIErrorOnly(&client.APIError{Method: "POST", Path: "/files/download", StatusCode: tc.status, Body: tc.body})
		if ExitCode(err) != 4 || !strings.Contains(err.Error(), "lacks scope "+tc.want) || !strings.Contains(err.Error(), "App Console Permissions tab, click Submit") || strings.Contains(err.Error(), "token may have expired") {
			t.Fatalf("status %d: %v", tc.status, err)
		}
	}
}
