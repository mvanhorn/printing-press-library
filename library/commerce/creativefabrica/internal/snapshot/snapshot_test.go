package snapshot

import (
	"testing"
)

func TestDiff(t *testing.T) {
	cases := []struct {
		name    string
		prior   []string
		current []string
		want    []string
	}{
		{"all new", nil, []string{"a", "b"}, []string{"a", "b"}},
		{"none new", []string{"a", "b"}, []string{"a", "b"}, nil},
		{"some new", []string{"a"}, []string{"a", "b", "c"}, []string{"b", "c"}},
		{"removed ignored", []string{"a", "b"}, []string{"a"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Diff(c.prior, c.current)
			if len(got) != len(c.want) {
				t.Fatalf("Diff=%v want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("Diff=%v want %v", got, c.want)
				}
			}
		})
	}
}

func TestPutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	key := "q:flowers|d:|t:"
	if _, ok := s.Get(key); ok {
		t.Fatal("expected no snapshot before Put")
	}
	if err := s.Put(key, []string{"3", "1", "2"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	snap, ok := s.Get(key)
	if !ok {
		t.Fatal("expected snapshot after Put")
	}
	if len(snap.ObjectIDs) != 3 || snap.ObjectIDs[0] != "1" {
		t.Errorf("stored ids = %v (want sorted)", snap.ObjectIDs)
	}
}
