package cli

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewMirrorRepairsDisk(t *testing.T) {
	testenv.Isolate(t)
	seedNovelCommandStore(t)
	dir := t.TempDir()
	runNovelCommand(t, "mirror", "--export-dir", dir, "--no-learn")
	path := filepath.Join(dir, "documents.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, damage := range []string{"missing", "corrupt", "unchanged"} {
		t.Run(damage, func(t *testing.T) {
			if damage == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if damage == "unchanged" {
				if err := os.WriteFile(path, want, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if damage == "corrupt" {
				if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out := runNovelCommand(t, "mirror", "--export-dir", dir, "--incremental", "--no-learn")
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(want) {
				t.Fatalf("disk not repaired: %v %s", err, got)
			}
			var v struct {
				Changed []string `json:"changed"`
			}
			if err := json.Unmarshal([]byte(out), &v); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, s := range v.Changed {
				found = found || s == "documents.json"
			}
			if damage == "unchanged" {
				if len(v.Changed) != 0 {
					t.Fatalf("unchanged files rewritten: %v", v.Changed)
				}
			} else if !found {
				t.Fatalf("repair not listed: %s", out)
			}
		})
	}
}
