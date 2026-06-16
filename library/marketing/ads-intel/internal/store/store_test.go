package store

import "testing"

func TestSaveLoadAndSnapshots(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveProfile(Profile{Name: "demo", AccountID: "123"}); err != nil {
		t.Fatal(err)
	}
	ps, err := s.ListProfiles()
	if err != nil || len(ps) != 1 {
		t.Fatalf("profiles failed: %v %#v", err, ps)
	}
	d := Fixture("demo")
	if err := s.SaveData(d); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadData("demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Campaigns) == 0 || loaded.Account.Status != "active" {
		t.Fatalf("bad fixture load: %#v", loaded)
	}
	snaps, err := s.LatestSnapshots("demo", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].SchemaVersion != SnapshotSchemaVersion {
		t.Fatalf("snapshot not written: %#v", snaps)
	}
}
