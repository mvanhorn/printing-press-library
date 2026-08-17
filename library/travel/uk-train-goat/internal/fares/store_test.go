package fares

import (
	"database/sql"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

func TestEnsureSchemaIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("first EnsureSchema: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("second EnsureSchema (idempotent): %v", err)
	}
}

func TestLoadRowCountsAndReplacement(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// First load: 2 locations, 2 flows, 2 fares, 1 NDF, 1 ticket, 1 railcard, 2 clusters, 1 group member, 1 restriction.
	data1 := &FeedData{
		Locations: []Location{
			{NLC: "1234", CRS: "LNP", Name: "London Paddington", StartDate: "20200101", EndDate: "99991231"},
			{NLC: "5678", CRS: "BRI", Name: "Bristol Temple Meads", StartDate: "20200101", EndDate: "99991231"},
		},
		Flows: []Flow{
			{FlowID: "F001", OriginNLC: "1234", DestNLC: "5678", Route: "00000", Direction: "S", TOC: "GW", StartDate: "20200101", EndDate: "99991231"},
			{FlowID: "F002", OriginNLC: "5678", DestNLC: "1234", Route: "00000", Direction: "S", TOC: "GW", StartDate: "20200101", EndDate: "99991231"},
		},
		Fares: []Fare{
			{FlowID: "F001", TicketCode: "SDS", Pence: 1500, RestrictionCode: ""},
			{FlowID: "F002", TicketCode: "SDS", Pence: 1500, RestrictionCode: ""},
		},
		NDF: []NonDerivableFare{
			{OriginNLC: "1234", DestNLC: "5678", Route: "00000", TicketCode: "SDS", Pence: 1600, RestrictionCode: "", StartDate: "20200101", EndDate: "99991231"},
		},
		Tickets: []TicketType{
			{Code: "SDS", Description: "Super Day Single", TicketClass: "2", TicketType: "S"},
		},
		Railcards: []Railcard{
			{Code: "YNG", Description: "16-25 Railcard", MinPence: 0, DiscountPct: 33},
		},
		Clusters: []ClusterMember{
			{ClusterID: "C01", MemberNLC: "1234"},
			{ClusterID: "C01", MemberNLC: "5678"},
		},
		GroupMembers: []GroupMember{
			{MemberNLC: "1234", GroupNLC: "G01", EndDate: "99991231"},
		},
		Restrictions: []Restriction{
			{Code: "R01", Description: "Outward restriction"},
		},
	}

	if err := Load(db, data1); err != nil {
		t.Fatalf("Load data1: %v", err)
	}

	counts := map[string]int{
		"rjf_locations":     2,
		"rjf_flows":         2,
		"rjf_fares":         2,
		"rjf_ndf":           1,
		"rjf_ticket_types":  1,
		"rjf_railcards":     1,
		"rjf_clusters":      2,
		"rjf_group_members": 1,
		"rjf_restrictions":  1,
	}
	for table, want := range counts {
		var got int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Errorf("counting %s: %v", table, err)
			continue
		}
		if got != want {
			t.Errorf("%s: want %d rows, got %d", table, want, got)
		}
	}

	// Second load: fewer rows — assert replacement, not append.
	data2 := &FeedData{
		Locations: []Location{
			{NLC: "9999", CRS: "MAN", Name: "Manchester Piccadilly", StartDate: "20200101", EndDate: "99991231"},
		},
		Flows:        []Flow{},
		Fares:        []Fare{},
		NDF:          []NonDerivableFare{},
		Tickets:      []TicketType{},
		Railcards:    []Railcard{},
		Clusters:     []ClusterMember{},
		GroupMembers: []GroupMember{},
		Restrictions: []Restriction{},
	}

	if err := Load(db, data2); err != nil {
		t.Fatalf("Load data2: %v", err)
	}

	var locCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM rjf_locations").Scan(&locCount); err != nil {
		t.Fatalf("counting rjf_locations after replacement: %v", err)
	}
	if locCount != 1 {
		t.Errorf("after replacement: want 1 location, got %d", locCount)
	}

	// Other tables should be empty now.
	for _, table := range []string{"rjf_flows", "rjf_fares", "rjf_ndf", "rjf_ticket_types", "rjf_railcards", "rjf_clusters", "rjf_group_members", "rjf_restrictions"} {
		var cnt int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&cnt); err != nil {
			t.Errorf("counting %s: %v", table, err)
			continue
		}
		if cnt != 0 {
			t.Errorf("%s: want 0 rows after replacement, got %d", table, cnt)
		}
	}
}

func TestLoadGroupMembersRoundTrip(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	data := &FeedData{
		GroupMembers: []GroupMember{
			{MemberNLC: "3087", GroupNLC: "1072", EndDate: "29991231"},
		},
	}

	if err := Load(db, data); err != nil {
		t.Fatalf("Load with GroupMembers: %v", err)
	}

	var memberNLC, groupNLC, endDate string
	err := db.QueryRow("SELECT member_nlc, group_nlc, end_date FROM rjf_group_members").Scan(&memberNLC, &groupNLC, &endDate)
	if err != nil {
		t.Fatalf("QueryRow rjf_group_members: %v", err)
	}

	if memberNLC != "3087" {
		t.Errorf("member_nlc: want 3087, got %s", memberNLC)
	}
	if groupNLC != "1072" {
		t.Errorf("group_nlc: want 1072, got %s", groupNLC)
	}
	if endDate != "29991231" {
		t.Errorf("end_date: want 29991231, got %s", endDate)
	}
}

func TestReadMetaRoundTrip(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// Empty DB: ReadMeta should return found==false, no error.
	_, found, err := ReadMeta(db)
	if err != nil {
		t.Fatalf("ReadMeta on empty: %v", err)
	}
	if found {
		t.Fatalf("ReadMeta on empty: expected found=false, got true")
	}

	// Write meta and read back.
	m1 := FeedMeta{
		Sequence:     "42",
		LastModified: "20260101",
		PublishDate:  "20260102",
		SyncedAt:     "2026-06-20T12:00:00Z",
	}
	if err := WriteMeta(db, m1); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	got, found, err := ReadMeta(db)
	if err != nil {
		t.Fatalf("ReadMeta after write: %v", err)
	}
	if !found {
		t.Fatalf("ReadMeta: expected found=true after WriteMeta")
	}
	if got != m1 {
		t.Errorf("ReadMeta round-trip: want %+v, got %+v", m1, got)
	}

	// Overwrite with different values; assert still exactly 1 row.
	m2 := FeedMeta{
		Sequence:     "43",
		LastModified: "20260201",
		PublishDate:  "20260202",
		SyncedAt:     "2026-06-20T13:00:00Z",
	}
	if err := WriteMeta(db, m2); err != nil {
		t.Fatalf("WriteMeta second: %v", err)
	}

	got2, found2, err := ReadMeta(db)
	if err != nil {
		t.Fatalf("ReadMeta after second write: %v", err)
	}
	if !found2 {
		t.Fatalf("ReadMeta: expected found=true after second WriteMeta")
	}
	if got2 != m2 {
		t.Errorf("ReadMeta second round-trip: want %+v, got %+v", m2, got2)
	}

	var rowCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM rjf_meta").Scan(&rowCount); err != nil {
		t.Fatalf("counting rjf_meta: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("rjf_meta: want exactly 1 row, got %d", rowCount)
	}
}

// TestLoadToleratesDatedDuplicateCodes pins the regression where the real
// RJFAF feed carries multiple dated rows per code for the three lookup
// tables. Each table declares code as PRIMARY KEY, so Load must upsert
// (last-write-wins) rather than fail on a UNIQUE constraint.
func TestLoadToleratesDatedDuplicateCodes(t *testing.T) {
	db := openTestDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	data := &FeedData{
		Tickets: []TicketType{
			{Code: "SDS", Description: "ANYTIME DAY S", TicketClass: "", TicketType: "S"},
			{Code: "SDS", Description: "Anytime Day Single", TicketClass: "", TicketType: "S"},
		},
		Railcards: []Railcard{
			{Code: "NEW", Description: "Network Railcard (old)", MinPence: 0, DiscountPct: 34},
			{Code: "NEW", Description: "Network Railcard", MinPence: 0, DiscountPct: 34},
		},
		Restrictions: []Restriction{
			{Code: "R01", Description: "Outward restriction (old)"},
			{Code: "R01", Description: "Outward restriction"},
		},
	}

	if err := Load(db, data); err != nil {
		t.Fatalf("Load with dated duplicate codes: %v", err)
	}

	// Exactly one surviving row per code, holding the last-written description.
	cases := []struct {
		table, code, wantDesc string
	}{
		{"rjf_ticket_types", "SDS", "Anytime Day Single"},
		{"rjf_railcards", "NEW", "Network Railcard"},
		{"rjf_restrictions", "R01", "Outward restriction"},
	}
	for _, c := range cases {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+c.table+" WHERE code=?", c.code).Scan(&count); err != nil {
			t.Fatalf("counting %s code=%s: %v", c.table, c.code, err)
		}
		if count != 1 {
			t.Errorf("%s code=%s: want exactly 1 row, got %d", c.table, c.code, count)
		}
		var desc string
		if err := db.QueryRow("SELECT description FROM "+c.table+" WHERE code=?", c.code).Scan(&desc); err != nil {
			t.Fatalf("reading %s description code=%s: %v", c.table, c.code, err)
		}
		if desc != c.wantDesc {
			t.Errorf("%s code=%s description: want %q (last-write-wins), got %q", c.table, c.code, c.wantDesc, desc)
		}
	}
}
