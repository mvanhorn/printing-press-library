package dropbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanRejectsUnpairedFolderDeleteAttestation(t *testing.T) {
	files, bytes := 1, int64(10)
	for _, tc := range []struct {
		name  string
		files *int
		bytes *int64
		valid bool
	}{
		{"neither", nil, nil, true},
		{"both", &files, &bytes, true},
		{"files only", &files, nil, false},
		{"bytes only", nil, &bytes, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []Op{{Op: "delete", Path: "/A", ExpectFiles: tc.files, ExpectBytes: tc.bytes}}}
			err := p.ValidateShape()
			if tc.valid && err != nil || !tc.valid && (err == nil || !strings.Contains(err.Error(), "expect_files") || !strings.Contains(err.Error(), "expect_bytes")) {
				t.Fatalf("valid=%t err=%v", tc.valid, err)
			}
		})
	}
}

func TestPlanShapeAndRoundTrip(t *testing.T) {
	p := Plan{Version: 1, CreatedAt: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), Source: "organize", Ops: []Op{{Op: "mkdir", Path: "/Photos/2019"}, {Op: "move", From: "/a.jpg", To: "/Photos/2019/a.jpg", Rev: "rev1"}, {Op: "delete", Path: "/old.jpg", Rev: "rev2"}, {Op: "revoke_link", URL: "https://dropbox.test/link"}}}
	if err := p.ValidateShape(); err != nil {
		t.Fatal(err)
	}
	bad := p
	bad.Ops = []Op{{Op: "move", From: "/a"}}
	if bad.ValidateShape() == nil {
		t.Fatal("move without destination accepted")
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := WritePlan(path, p); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	got, err := ReadPlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Ops) != len(p.Ops) || got.Ops[1] != p.Ops[1] || !got.CreatedAt.Equal(p.CreatedAt) {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestPlanRejectsNoncanonicalPaths(t *testing.T) {
	for _, bad := range []string{"relative", "/a/", "/a//b", "/a/./b", "/a/../b"} {
		for _, op := range []Op{{Op: "delete", Path: bad}, {Op: "move", From: bad, To: "/dest"}, {Op: "move", From: "/source", To: bad}, {Op: "delete", Path: "/source", Keeper: bad}} {
			p := Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []Op{op}}
			if err := p.ValidateShape(); err == nil {
				t.Errorf("accepted %+v", op)
			}
		}
	}
}

func TestPlanRoundTripsDeleteSafetyFields(t *testing.T) {
	files, bytes := 2, int64(1024)
	p := Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "dupes", Ops: []Op{{Op: "delete", Path: "/Duplicates", ExpectFiles: &files, ExpectBytes: &bytes, Keeper: "/Keep/copy", ContentHash: "hash"}}}
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := WritePlan(file, p); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPlan(file)
	if err != nil {
		t.Fatal(err)
	}
	op := got.Ops[0]
	if op.ExpectFiles == nil || *op.ExpectFiles != files || op.ExpectBytes == nil || *op.ExpectBytes != bytes || op.Keeper != "/Keep/copy" || op.ContentHash != "hash" {
		t.Fatalf("op=%+v", op)
	}
}

func TestEmptyPlanRoundTrip(t *testing.T) {
	p := Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "mess", Ops: []Op{}}
	file := filepath.Join(t.TempDir(), "empty.json")
	if err := WritePlan(file, p); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPlan(file)
	if err != nil || got.Ops == nil || len(got.Ops) != 0 {
		t.Fatalf("empty plan=%+v err=%v", got, err)
	}
}
