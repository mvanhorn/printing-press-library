package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSyncVerificationMockRunsGenericPipelineWithoutDomainEnrichment(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/styles" {
			_, _ = w.Write([]byte(`[{"id":"style-1","name":"Mock style"}]`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"catalog-1","name":"Mock catalog"}`))
	}))
	defer server.Close()

	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--no-learn", "sync", "--db", filepath.Join(t.TempDir(), "verify.db"), "--full"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync against verification fixture: %v", err)
	}
	if calls["/"] != 1 || calls["/styles"] != 1 {
		t.Fatalf("verification fixture calls = %#v; want one generated request per resource", calls)
	}
}
