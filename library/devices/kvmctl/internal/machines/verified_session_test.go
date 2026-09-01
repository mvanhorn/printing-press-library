package machines

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifiedSessionStore_MACAtomic0600(t *testing.T) {
	dir := t.TempDir()
	store := VerifiedSessionStore{Path: filepath.Join(dir, "session.json")}
	inv := DefaultInventory()
	rec := SelectionRecord{Target: Target{Name: "pve1", Port: 1}, State: Verified, Detail: "ok", At: time.Now()}
	endpoint := "https://kvm.example:443|host=kvm.example:443"
	if err := store.Save(rec, endpoint); err != nil {
		t.Fatalf("save: %v", err)
	}
	// perms
	if st, _ := os.Stat(store.Path); st.Mode().Perm() != 0600 {
		t.Fatalf("session perm %v", st.Mode())
	}
	if st, _ := os.Stat(store.keyPath()); st.Mode().Perm() != 0600 {
		t.Fatalf("key perm %v", st.Mode())
	}
	if st, _ := os.Stat(dir); st.Mode().Perm()&0077 != 0 {
		t.Fatalf("dir perm %v", st.Mode())
	}
	// load
	got, ok, err := store.Load(endpoint, inv)
	if err != nil || !ok || got.Target.Name != "pve1" || got.State != Verified {
		t.Fatalf("load got=%v ok=%v err=%v", got, ok, err)
	}
	// tamper
	b, _ := os.ReadFile(store.Path)
	b[len(b)-2] ^= 0xff
	_ = os.WriteFile(store.Path, b, 0600)
	if _, _, err := store.Load(endpoint, inv); err == nil {
		t.Fatal("expected integrity error on tampered file")
	}
}

func TestVerifiedSessionStore_RejectsUnverifiedAndEndpointMismatch(t *testing.T) {
	dir := t.TempDir()
	store := VerifiedSessionStore{Path: filepath.Join(dir, "session.json")}
	rec := SelectionRecord{Target: Target{Name: "pve1", Port: 1}, State: SelectedUnverified, At: time.Now()}
	if err := store.Save(rec, "https://a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Fatal("unverified should not create file")
	}
	rec.State = Verified
	if err := store.Save(rec, "https://a"); err != nil {
		t.Fatal(err)
	}
	inv := DefaultInventory()
	if _, ok, _ := store.Load("https://b", inv); ok {
		t.Fatal("endpoint mismatch should not load")
	}
}

func TestVerifiedSessionStore_MaxAgeAndPortBinding(t *testing.T) {
	dir := t.TempDir()
	fixed := time.Now()
	store := VerifiedSessionStore{Path: filepath.Join(dir, "session.json"), Now: func() time.Time { return fixed }, MaxAge: time.Hour}
	inv := DefaultInventory()
	rec := SelectionRecord{Target: Target{Name: "pve2", Port: 2}, State: Verified, Detail: "x", At: fixed.Add(-2 * time.Hour)}
	if err := store.Save(rec, "https://a"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.Load("https://a", inv); ok {
		t.Fatal("expired should not load")
	}
	// fresh
	store2 := VerifiedSessionStore{Path: filepath.Join(dir, "s2.json"), Now: func() time.Time { return fixed }}
	rec.At = fixed
	if err := store2.Save(rec, "https://a"); err != nil {
		t.Fatal(err)
	}
	// wrong port inventory (tamper inv)
	inv2 := Inventory{Targets: []Target{{Name: "pve2", Port: 9, Enabled: true}}}
	if _, ok, _ := store2.Load("https://a", inv2); ok {
		t.Fatal("port mismatch should not load")
	}
}

func TestVerifiedSessionStore_LockingConcurrency(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "session.json")
	s1 := VerifiedSessionStore{Path: p}
	s2 := VerifiedSessionStore{Path: p}
	rec := SelectionRecord{Target: Target{Name: "pve3", Port: 4}, State: Verified, Detail: "c", At: time.Now()}
	done := make(chan error, 2)
	go func() { done <- s1.Save(rec, "https://a") }()
	go func() { done <- s2.Save(rec, "https://a") }()
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent save %v", err)
		}
	}
	inv := DefaultInventory()
	if _, ok, err := s1.Load("https://a", inv); !ok || err != nil {
		t.Fatalf("load after concurrent %v ok %v", err, ok)
	}
}
