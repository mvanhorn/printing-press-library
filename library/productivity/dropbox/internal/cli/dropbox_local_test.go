package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

func TestOpenIndexStaleHintSeparatesIncompleteFromOld(t *testing.T) {
	testenv.Isolate(t)
	for _, tc := range []struct {
		name     string
		state    store.DropboxIndexState
		wantHint string
	}{
		{"incomplete", store.DropboxIndexState{Root: "/incomplete", LastFullAt: time.Now().UTC().Format(time.RFC3339), Complete: false}, "index is incomplete; run: dropbox-pp-cli index"},
		{"incomplete and old", store.DropboxIndexState{Root: "/incomplete", LastFullAt: time.Now().UTC().Add(-26 * time.Hour).Format(time.RFC3339), Complete: false}, "index is incomplete; run: dropbox-pp-cli index"},
		{"old", store.DropboxIndexState{Root: "/old", LastFullAt: time.Now().UTC().Add(-26 * time.Hour).Format(time.RFC3339), Complete: true}, "index is 26 hours old; run: dropbox-pp-cli index"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := seedIndex(t)
			db, err := store.OpenWithContext(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SetDropboxIndexState(context.Background(), tc.state); err != nil {
				db.Close()
				t.Fatal(err)
			}
			db.Close()
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)
			opened, found, err := openIndex(cmd, &rootFlags{}, path)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("seeded index missing")
			}
			opened.Close()
			if got := strings.TrimSpace(stderr.String()); got != tc.wantHint {
				t.Fatalf("hint = %q, want %q", got, tc.wantHint)
			}
		})
	}
}
