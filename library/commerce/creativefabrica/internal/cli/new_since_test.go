package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/snapshot"
)

func TestNewSinceCommitsSnapshotOnlyAfterSuccessfulDelivery(t *testing.T) {
	for _, tc := range []struct {
		name        string
		deliverCode int
		wantSaved   bool
	}{
		{name: "failed delivery", deliverCode: http.StatusBadGateway, wantSaved: false},
		{name: "successful delivery", deliverCode: http.StatusNoContent, wantSaved: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/sink" {
					w.WriteHeader(tc.deliverCode)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"results":[{"hits":[{"objectID":"new-1","name_en":"New font"}],"nbHits":1,"nbPages":1,"page":0,"hitsPerPage":50}]}`)
			}))
			defer srv.Close()

			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CREATIVEFABRICA_CONFIG", filepath.Join(home, "missing.toml"))
			t.Setenv("CREATIVEFABRICA_BASE_URL", srv.URL)
			t.Setenv("CREATIVEFABRICA_ALGOLIA_APP_ID", "TESTAPP")
			t.Setenv("CREATIVEFABRICA_ALGOLIA_API_KEY", "test-key")

			var flags rootFlags
			root := newRootCmd(&flags)
			root.SetArgs([]string{"--quiet", "--deliver", "webhook:" + srv.URL + "/sink", "new-since", "flowers"})
			if err := root.Execute(); err != nil {
				t.Fatalf("command execution: %v", err)
			}
			err := finishSuccessfulCommand(&flags)
			if tc.wantSaved && err != nil {
				t.Fatalf("successful delivery: %v", err)
			}
			if !tc.wantSaved && (err == nil || !strings.Contains(err.Error(), "502")) {
				t.Fatalf("failed delivery error = %v, want HTTP 502", err)
			}

			_, saved := snapshot.Open("").Get("q:flowers|d:|t:")
			if saved != tc.wantSaved {
				t.Fatalf("snapshot saved = %v, want %v", saved, tc.wantSaved)
			}
		})
	}
}
