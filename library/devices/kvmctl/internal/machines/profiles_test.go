package machines

import (
	"os"
	"testing"
)

func TestProfileAndStatePersistenceArePrivateAndAtomic(t *testing.T) {
	d := t.TempDir()
	p := ProfileStore{Path: d + "/profiles.json"}
	if err := p.Save(DefaultInventory()); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p.Path)
	if st.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
	got, err := p.Load()
	if err != nil || len(got.Targets) != 4 {
		t.Fatalf("%v %#v", err, got)
	}
	ss := TargetStateStore{Path: d + "/state.json"}
	if err := ss.Save("pve2"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ss.Load(); got != "pve2" {
		t.Fatal(got)
	}
}
