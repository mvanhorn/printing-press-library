package cli

// PATCH(nested-required-flags-637): a property required inside an OPTIONAL nested body object is required only when that
// object is sent; the bare command sends no object.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runNestedCmd(t *testing.T, build func(*rootFlags) *cobra.Command, args ...string) ([]map[string]any, error) {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		m := map[string]any{}
		_ = json.Unmarshal(raw, &m)
		bodies = append(bodies, m)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := build(&rootFlags{asJSON: true, noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return bodies, err
}

func TestNestedRequiredFlagsOnlyWhenTheObjectIsSent(t *testing.T) {
	// bare commands: the optional object is not sent and nothing is required
	if b, err := runNestedCmd(t, newScreenShareDeleteSessionCodeCmd, "CODE1"); err != nil || len(b) != 1 || b[0]["marks"] != nil {
		t.Errorf("bare delete-session-code: bodies %v err %v", b, err)
	}
	if b, err := runNestedCmd(t, newConduytAuthLogoutCmd); err != nil || len(b) != 1 || b[0]["screenShare"] != nil {
		t.Errorf("bare logout: bodies %v err %v", b, err)
	}
	if b, err := runNestedCmd(t, newSettingsUpdateCmd, "--timezone", "America/New_York"); err != nil || len(b) != 1 {
		t.Errorf("settings update --timezone: bodies %v err %v", b, err)
	}
	// one flag of the group: its siblings are required, nothing is sent
	b, err := runNestedCmd(t, newScreenShareDeleteSessionCodeCmd, "CODE1", "--marks-pointed", "true")
	if err == nil || !strings.Contains(err.Error(), "marks-drew") || len(b) != 0 {
		t.Errorf("--marks-pointed alone: bodies %v err %v, want a marks-drew refusal and no request", b, err)
	}
	// the whole group: drew 0 is a real count and is sent
	b, err = runNestedCmd(t, newScreenShareDeleteSessionCodeCmd, "CODE1", "--marks-pointed", "false", "--marks-drew", "0")
	if err != nil || len(b) != 1 {
		t.Fatalf("full marks: bodies %v err %v", b, err)
	}
	marks, _ := b[0]["marks"].(map[string]any)
	if marks == nil || marks["drew"] != float64(0) || marks["pointed"] != false {
		t.Errorf("full marks body = %v, want marks {drew 0, pointed false}", b[0])
	}
}
