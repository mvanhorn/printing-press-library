package crestronstore

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestUpsertAndFindProduct(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	err := st.UpsertProducts(ctx, []Product{
		{Model: "DM-NVX-360", Description: "DM NVX 4K60 Network AV Encoder/Decoder",
			URL: "/Products/Catalog/AV-Over-IP/x/DM-NVX-360", DocumentID: "21965"},
		{Model: "UC-FCM-Z", Description: "Crestron Flex Mobile UC System",
			URL: "/Products/Catalog/Inactive/Discontinued/U/UC-FCM-Z", Discontinued: true},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	p, ok, err := st.FindProduct(ctx, "DM-NVX-360")
	if err != nil || !ok {
		t.Fatalf("FindProduct exact: ok=%v err=%v", ok, err)
	}
	if p.DocumentID != "21965" {
		t.Errorf("document id = %q, want 21965", p.DocumentID)
	}

	// Lookup must tolerate the punctuation and casing a user actually types.
	for _, q := range []string{"dm-nvx-360", "dm nvx 360", "DMNVX360"} {
		if _, ok, err := st.FindProduct(ctx, q); err != nil || !ok {
			t.Errorf("FindProduct(%q): ok=%v err=%v", q, ok, err)
		}
	}

	if _, ok, err := st.FindProduct(ctx, "ZZ-NOPE"); err != nil || ok {
		t.Errorf("unknown model should not be found: ok=%v err=%v", ok, err)
	}

	disc, err := st.ListDiscontinued(ctx, 10)
	if err != nil {
		t.Fatalf("ListDiscontinued: %v", err)
	}
	if len(disc) != 1 || disc[0].Model != "UC-FCM-Z" {
		t.Errorf("discontinued = %+v, want just UC-FCM-Z", disc)
	}
}

// The many-to-many join is the whole point of the mirror: one release covers a
// family, and every member must resolve to it.
func TestReleasesForModelFamilyJoin(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	family := []string{"TSW-570", "TSW-770", "TSW-1070", "TSS-770", "TSS-1070", "TS-770", "TS-1070"}
	err := st.UpsertReleases(ctx, []Release{
		{URL: "/Software-Firmware/Firmware/Touchpanels/TS-770/3-0", Title: "TSW-570/TSW-770/TSW-1070/... 3.0.1234",
			Version: "3.0.1234", Date: "Jun 16, 2026", Models: family},
		{URL: "/Software-Firmware/Firmware/DigitalMedia/DM-NVX-384/7-4", Title: "DM-NVX-384(C)_DM-NVX-385(C) 7.4.0255",
			Version: "7.4.0255", Date: "May 06, 2026", Models: []string{"DM-NVX-384(C)", "DM-NVX-385(C)"}},
	})
	if err != nil {
		t.Fatalf("upsert releases: %v", err)
	}

	for _, m := range family {
		rels, err := st.ReleasesForModel(ctx, m)
		if err != nil {
			t.Fatalf("ReleasesForModel(%q): %v", m, err)
		}
		if len(rels) != 1 {
			t.Errorf("%s resolved to %d releases, want 1", m, len(rels))
			continue
		}
		if rels[0].Version != "3.0.1234" {
			t.Errorf("%s version = %q, want 3.0.1234", m, rels[0].Version)
		}
	}

	// A model must not pick up an unrelated family's release.
	rels, err := st.ReleasesForModel(ctx, "TSW-1070")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rels {
		if r.Version == "7.4.0255" {
			t.Error("TSW-1070 matched the DM-NVX release")
		}
	}
}

// Re-syncing without a signed-in session must not erase notes a prior
// authenticated sync captured.
func TestUpsertReleasesPreservesNotes(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	url := "/Software-Firmware/Firmware/x/1-0"

	if err := st.UpsertReleases(ctx, []Release{{
		URL: url, Title: "X 1.0", Version: "1.0", Date: "Jan 1, 2026",
		Models: []string{"X"}, Notes: "captured notes", ChangeLog: "captured log",
	}}); err != nil {
		t.Fatal(err)
	}
	// Second sync has no notes (anonymous run).
	if err := st.UpsertReleases(ctx, []Release{{
		URL: url, Title: "X 1.0", Version: "1.0", Date: "Jan 1, 2026", Models: []string{"X"},
	}}); err != nil {
		t.Fatal(err)
	}
	rels, err := st.ReleasesForModel(ctx, "X")
	if err != nil || len(rels) != 1 {
		t.Fatalf("ReleasesForModel: %+v err=%v", rels, err)
	}
	if rels[0].Notes != "captured notes" || rels[0].ChangeLog != "captured log" {
		t.Errorf("an anonymous re-sync erased previously captured notes: %+v", rels[0])
	}
}

func TestSearchReleasesFTS(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.UpsertReleases(ctx, []Release{
		{URL: "/a", Title: "DM-NVX-384(C)_DM-NVX-385(C) 7.4.0255", Version: "7.4.0255",
			Date: "May 06, 2026", Models: []string{"DM-NVX-384(C)"},
			ChangeLog: "Fixed HDCP passthrough on the encoder input."},
		{URL: "/b", Title: "CP4N 2.8", Version: "2.8", Date: "Jun 30, 2026",
			Models: []string{"CP4N"}, ChangeLog: "Improved Dante audio routing."},
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := st.SearchReleases(ctx, "HDCP", 10)
	if err != nil {
		t.Fatalf("SearchReleases: %v", err)
	}
	if len(hits) != 1 || hits[0].Version != "7.4.0255" {
		t.Fatalf("HDCP search = %+v, want the DM-NVX release", hits)
	}

	// A hyphenated model number is an FTS5 expression unless it is quoted; this
	// used to fail with "no such column".
	if _, err := st.SearchReleases(ctx, "DM-NVX", 10); err != nil {
		t.Errorf("hyphenated query failed: %v", err)
	}

	// A term present in neither release must return nothing, not everything.
	hits, err = st.SearchReleases(ctx, "zzzznotathing", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("unmatched term returned %d hits", len(hits))
	}
	if hits == nil {
		t.Error("hits must be non-nil so JSON renders [] not null")
	}
}

func TestSearchProductsFTS(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.UpsertProducts(ctx, []Product{
		{Model: "DM-NVX-360", Description: "4K60 Network AV Encoder Decoder"},
		{Model: "FP-G1-W-T", Description: "Decorator Style Faceplate 1-Gang White Textured"},
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := st.SearchProducts(ctx, "Faceplate", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Model != "FP-G1-W-T" {
		t.Errorf("Faceplate search = %+v, want FP-G1-W-T only", hits)
	}
	// Negative: a query matching neither description must return nothing.
	hits, err = st.SearchProducts(ctx, "thermostat", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("unmatched product query returned %d hits", len(hits))
	}
}

func TestCountsAndMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	st, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertProducts(ctx, []Product{{Model: "A"}}); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	// Re-opening runs the migrations again; they must not fail or drop data.
	st2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = st2.Close() }()
	counts, err := st2.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["products"] != 1 {
		t.Errorf("products = %d after reopen, want 1", counts["products"])
	}
}

func TestFTSQueryEscaping(t *testing.T) {
	cases := map[string]string{
		"DM-NVX":    `"DM-NVX"`,
		"HDCP":      `"HDCP"`,
		"two words": `"two" AND "words"`,
		`say "hi"`:  `"say" AND """hi"""`,
		"   ":       "",
	}
	for in, want := range cases {
		if got := ftsQuery(in); got != want {
			t.Errorf("ftsQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
