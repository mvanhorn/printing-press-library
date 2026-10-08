package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
)

func TestTreeRecursiveBytesAndUnknownPath(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Photos", "folder", "", "", 0),
		fixtureRow("/Photos/2019", "folder", "", "", 0),
		fixtureRow("/Photos/2019/a.jpg", "file", "A", "2020-01-01T00:00:00Z", 100),
		fixtureRow("/Docs", "folder", "", "", 0),
		fixtureRow("/Docs/b.txt", "file", "B", "2020-01-01T00:00:00Z", 50),
	)
	data, err := runRead(t, "tree", "/", "--depth", "1", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got treeNode
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Files != 2 || got.Bytes != 150 || len(got.Children) != 2 || got.Children[0].PathDisplay != "/Photos" || got.Children[0].Bytes != 100 || len(got.Children[0].Children) != 0 {
		t.Fatalf("tree %+v", got)
	}
	_, err = runRead(t, "tree", "/Missing", "--db", db, "--json")
	var cliErr *cliError
	if !errors.As(err, &cliErr) || cliErr.code != 2 {
		t.Fatalf("unknown path err=%v", err)
	}
}

func TestTreeNestedRollupsOrderingAndLimit(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/A", "folder", "", "", 0),
		fixtureRow("/A/B", "folder", "", "", 0),
		fixtureRow("/A/B/C", "folder", "", "", 0),
		fixtureRow("/A/B/C/z", "file", "", "", 10),
		fixtureRow("/A/B/y", "file", "", "", 20),
		fixtureRow("/A/x", "file", "", "", 30),
		fixtureRow("/A/empty", "folder", "", "", 0),
		fixtureRow("/B", "folder", "", "", 0),
		fixtureRow("/B/b", "file", "", "", 25),
	)
	data, err := runRead(t, "tree", "/A", "--depth", "3", "--limit", "2", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got treeNode
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Files != 3 || got.Bytes != 60 || got.More != 1 || len(got.Children) != 2 {
		t.Fatalf("root tree %+v", got)
	}
	b := got.Children[0]
	if b.PathDisplay != "/A/B" || b.Files != 2 || b.Bytes != 30 || b.More != 0 || len(b.Children) != 2 {
		t.Fatalf("nested tree %+v", b)
	}
	if b.Children[0].PathDisplay != "/A/B/y" || b.Children[0].Files != 1 || b.Children[0].Bytes != 20 {
		t.Fatalf("nested file %+v", b.Children[0])
	}
	if b.Children[1].PathDisplay != "/A/B/C" || b.Children[1].Files != 1 || b.Children[1].Bytes != 10 || len(b.Children[1].Children) != 1 || b.Children[1].Children[0].PathDisplay != "/A/B/C/z" {
		t.Fatalf("deep tree %+v", b.Children[1])
	}
	if got.Children[1].PathDisplay != "/A/x" || got.Children[1].Bytes != 30 {
		t.Fatalf("tie ordering %+v", got.Children)
	}
}
