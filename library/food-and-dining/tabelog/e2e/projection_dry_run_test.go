package e2e

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"testing"
)

func TestProjectedDiscoveryDryRunPreservesPlanWithoutIO(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 500} })
	w := newWorkspace(t, r)
	taskHome := t.TempDir()
	selection := "items.id,items.name,items.rating,items.url"
	p := mustSucceed(t, w.run(t, "find", "--area", "tokyo", "--cuisine", "bar", "--meal", "dinner", "--budget-max", "5000", "--select", selection, "--dry-run", "--agent", "--home", taskHome))
	planned := rawItems(t, p)
	equal(t, len(planned), 1)
	equal(t, planned[0]["dry_run"], true)
	equal(t, object(t, planned[0]["criteria"])["select"], selection)
	equal(t, object(t, p["meta"])["coverage"], "planned")
	equal(t, r.count(), 0)
	if err := filepath.WalkDir(taskHome, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != taskHome {
			t.Errorf("dry run created persistent state: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
