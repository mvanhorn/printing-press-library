package fares

import (
	"database/sql"
	"testing"
)

// seedResolveDB sets up schema and loads the exact RJFAF798 rows specified in
// the task-7 brief. Clusters and NDF are intentionally left empty.
func seedResolveDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("seedResolveDB: EnsureSchema: %v", err)
	}
	data := &FeedData{
		Locations: []Location{
			{NLC: "1072", CRS: "", Name: "LONDON TERMINALS", StartDate: "20250809", EndDate: "29991231"},
			{NLC: "3087", CRS: "PAD", Name: "LONDON PADDINGTN", StartDate: "20250807", EndDate: "29991231"},
			{NLC: "3115", CRS: "OXF", Name: "OXFORD", StartDate: "20250807", EndDate: "29991231"},
			{NLC: "3149", CRS: "RDG", Name: "READING", StartDate: "20250807", EndDate: "29991231"},
		},
		GroupMembers: []GroupMember{
			{MemberNLC: "3087", GroupNLC: "1072", EndDate: "29991231"},
		},
		Flows: []Flow{
			{FlowID: "0659482", OriginNLC: "1072", DestNLC: "3149", Route: "00000", Direction: "S", TOC: "GWR", StartDate: "20251011", EndDate: "20260704"},
			{FlowID: "0662636", OriginNLC: "1072", DestNLC: "3149", Route: "00735", Direction: "S", TOC: "SWT", StartDate: "20260301", EndDate: "29991231"},
			{FlowID: "0658315", OriginNLC: "3149", DestNLC: "1072", Route: "00000", Direction: "S", TOC: "GWR", StartDate: "20251011", EndDate: "20260704"},
			{FlowID: "0663156", OriginNLC: "3149", DestNLC: "1072", Route: "00735", Direction: "S", TOC: "SWT", StartDate: "20260301", EndDate: "29991231"},
			{FlowID: "0660133", OriginNLC: "3149", DestNLC: "3115", Route: "00000", Direction: "R", TOC: "GWR", StartDate: "20260207", EndDate: "20260704"},
		},
		Fares: []Fare{
			{FlowID: "0659482", TicketCode: "SDS", Pence: 3510, RestrictionCode: ""},
			{FlowID: "0659482", TicketCode: "SDR", Pence: 6380, RestrictionCode: ""},
			{FlowID: "0662636", TicketCode: "SDS", Pence: 2310, RestrictionCode: ""},
			{FlowID: "0662636", TicketCode: "SDR", Pence: 2600, RestrictionCode: ""},
			{FlowID: "0658315", TicketCode: "SDS", Pence: 3510, RestrictionCode: ""},
			{FlowID: "0658315", TicketCode: "SDR", Pence: 6380, RestrictionCode: ""},
			{FlowID: "0663156", TicketCode: "SDS", Pence: 2310, RestrictionCode: ""},
			{FlowID: "0663156", TicketCode: "SDR", Pence: 4450, RestrictionCode: ""},
			{FlowID: "0660133", TicketCode: "SDS", Pence: 1950, RestrictionCode: ""},
			{FlowID: "0660133", TicketCode: "SDR", Pence: 2450, RestrictionCode: ""},
		},
		Tickets: []TicketType{
			{Code: "SDS", Description: "ANYTIME DAY S", TicketClass: "", TicketType: "S"},
			{Code: "SDR", Description: "ANYTIME DAY R", TicketClass: "", TicketType: "R"},
		},
		// Clusters, NDF, Railcards, Restrictions intentionally empty.
	}
	if err := Load(db, data); err != nil {
		t.Fatalf("seedResolveDB: Load: %v", err)
	}
	return db
}

func TestResolvePADtoRDG(t *testing.T) {
	db := seedResolveDB(t)
	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(fares) != 4 {
		t.Fatalf("want 4 fares, got %d: %+v", len(fares), fares)
	}
	// Sorted by pence ascending.
	want := []ResolvedFare{
		{TicketCode: "SDS", TicketName: "ANYTIME DAY S", Route: "00735", Pence: 2310, Single: true},
		{TicketCode: "SDR", TicketName: "ANYTIME DAY R", Route: "00735", Pence: 2600, Single: false},
		{TicketCode: "SDS", TicketName: "ANYTIME DAY S", Route: "00000", Pence: 3510, Single: true},
		{TicketCode: "SDR", TicketName: "ANYTIME DAY R", Route: "00000", Pence: 6380, Single: false},
	}
	for i, w := range want {
		g := fares[i]
		if g.TicketCode != w.TicketCode || g.Route != w.Route || g.Pence != w.Pence ||
			g.TicketName != w.TicketName || g.Single != w.Single ||
			g.RestrictionCode != "" || g.RestrictionDesc != "" {
			t.Errorf("fares[%d]: want %+v, got %+v", i, w, g)
		}
	}
}

func TestResolveExpiredGroupMembershipExcluded(t *testing.T) {
	// PAD reaches the 1072-filed flows only via its London Terminals group
	// membership. Expiring that membership before the query date must drop those
	// fares entirely rather than serving them through a lapsed group.
	db := seedResolveDB(t)
	if _, err := db.Exec(
		`UPDATE rjf_group_members SET end_date = '20250101' WHERE member_nlc = '3087' AND group_nlc = '1072'`); err != nil {
		t.Fatalf("expire membership: %v", err)
	}
	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(fares) != 0 {
		t.Errorf("expired group membership: want 0 fares, got %d: %+v", len(fares), fares)
	}
}

func TestResolveRDGtoPAD(t *testing.T) {
	db := seedResolveDB(t)
	fares, err := Resolve(db, "RDG", "PAD", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(fares) != 4 {
		t.Fatalf("want 4 fares, got %d: %+v", len(fares), fares)
	}

	// Find SDS route 00735 and SDR route 00735 in results.
	var sdsR00735, sdrR00735 *ResolvedFare
	for i := range fares {
		f := &fares[i]
		if f.TicketCode == "SDS" && f.Route == "00735" {
			sdsR00735 = f
		}
		if f.TicketCode == "SDR" && f.Route == "00735" {
			sdrR00735 = f
		}
	}
	if sdsR00735 == nil {
		t.Fatal("RDG→PAD: missing SDS route 00735")
	}
	if sdrR00735 == nil {
		t.Fatal("RDG→PAD: missing SDR route 00735")
	}

	// Price symmetry on the single: both directions == 2310.
	if sdsR00735.Pence != 2310 {
		t.Errorf("RDG→PAD SDS route 00735: want 2310, got %d", sdsR00735.Pence)
	}

	// Direction asymmetry: RDG→PAD SDR route 00735 must be 4450, not 2600.
	if sdrR00735.Pence != 4450 {
		t.Errorf("RDG→PAD SDR route 00735: want 4450 (direction-sensitive), got %d", sdrR00735.Pence)
	}
}

func TestResolveRDGtoOXF(t *testing.T) {
	db := seedResolveDB(t)
	fares, err := Resolve(db, "RDG", "OXF", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(fares) != 2 {
		t.Fatalf("want 2 fares, got %d: %+v", len(fares), fares)
	}
	if fares[0].TicketCode != "SDS" || fares[0].Pence != 1950 {
		t.Errorf("fares[0]: want SDS 1950, got %s %d", fares[0].TicketCode, fares[0].Pence)
	}
	if fares[1].TicketCode != "SDR" || fares[1].Pence != 2450 {
		t.Errorf("fares[1]: want SDR 2450, got %s %d", fares[1].TicketCode, fares[1].Pence)
	}
}

func TestResolveOXFtoRDG(t *testing.T) {
	// Proves the direction='R' reverse-reuse clause: flow 0660133 is filed 3149->3115 only.
	db := seedResolveDB(t)
	fares, err := Resolve(db, "OXF", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(fares) != 2 {
		t.Fatalf("want 2 fares via reverse-R clause, got %d: %+v", len(fares), fares)
	}
	if fares[0].TicketCode != "SDS" || fares[0].Pence != 1950 {
		t.Errorf("fares[0]: want SDS 1950, got %s %d", fares[0].TicketCode, fares[0].Pence)
	}
	if fares[1].TicketCode != "SDR" || fares[1].Pence != 2450 {
		t.Errorf("fares[1]: want SDR 2450, got %s %d", fares[1].TicketCode, fares[1].Pence)
	}
}

func TestResolveNDFOverride(t *testing.T) {
	// A non-derivable-fare row for the direct station pair (PAD 3087 -> RDG 3149)
	// must surface and, when cheaper, beat the cluster-derived price at the same
	// {ticket_code, route} key. Derived SDS route 00735 is 2310; the NDF overrides
	// it to 1500.
	db := seedResolveDB(t)
	if _, err := db.Exec(
		`INSERT INTO rjf_ndf(origin_nlc,dest_nlc,route,ticket_code,pence,restriction_code,start_date,end_date)
		 VALUES('3087','3149','00735','SDS',1500,'','20250101','29991231')`); err != nil {
		t.Fatalf("insert ndf: %v", err)
	}

	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var sds00735 *ResolvedFare
	for i := range fares {
		if fares[i].TicketCode == "SDS" && fares[i].Route == "00735" {
			sds00735 = &fares[i]
		}
	}
	if sds00735 == nil {
		t.Fatal("NDF override: missing SDS route 00735")
	}
	if sds00735.Pence != 1500 {
		t.Errorf("NDF override: want SDS route 00735 = 1500 (NDF beats derived 2310), got %d", sds00735.Pence)
	}
}

func TestResolveNDFGroupNLCOverride(t *testing.T) {
	// An NFO override can be filed under a group NLC rather than the member
	// station's own NLC. PAD (3087) reaches fares only via the London Terminals
	// group (1072), so an override filed origin_nlc='1072' must apply to a
	// PAD→RDG query. Querying rjf_ndf with raw station NLCs would miss it.
	db := seedResolveDB(t)
	if _, err := db.Exec(
		`INSERT INTO rjf_ndf(origin_nlc,dest_nlc,route,ticket_code,pence,restriction_code,start_date,end_date)
		 VALUES('1072','3149','00735','SDS',1200,'','20250101','29991231')`); err != nil {
		t.Fatalf("insert ndf: %v", err)
	}

	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var sds00735 *ResolvedFare
	for i := range fares {
		if fares[i].TicketCode == "SDS" && fares[i].Route == "00735" {
			sds00735 = &fares[i]
		}
	}
	if sds00735 == nil {
		t.Fatal("group-NLC NDF override: missing SDS route 00735")
	}
	if sds00735.Pence != 1200 {
		t.Errorf("group-NLC NDF override: want SDS route 00735 = 1200 (group-filed NDF beats derived 2310), got %d", sds00735.Pence)
	}
}

func TestResolveNDFClusterNLCOverride(t *testing.T) {
	// An NFO override can also be filed under an FSC cluster ID that the origin
	// station is a date-valid member of. Adding PAD (3087) to cluster '2000' and
	// filing the override under that cluster must apply to a PAD→RDG query, the
	// same way flow-matching expands through clusters.
	db := seedResolveDB(t)
	if _, err := db.Exec(
		`INSERT INTO rjf_clusters(cluster_id,member_nlc,start_date,end_date)
		 VALUES('2000','3087','20250101','29991231')`); err != nil {
		t.Fatalf("insert cluster: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO rjf_ndf(origin_nlc,dest_nlc,route,ticket_code,pence,restriction_code,start_date,end_date)
		 VALUES('2000','3149','00735','SDS',1100,'','20250101','29991231')`); err != nil {
		t.Fatalf("insert ndf: %v", err)
	}

	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var sds00735 *ResolvedFare
	for i := range fares {
		if fares[i].TicketCode == "SDS" && fares[i].Route == "00735" {
			sds00735 = &fares[i]
		}
	}
	if sds00735 == nil {
		t.Fatal("cluster-NLC NDF override: missing SDS route 00735")
	}
	if sds00735.Pence != 1100 {
		t.Errorf("cluster-NLC NDF override: want SDS route 00735 = 1100 (cluster-filed NDF beats derived 2310), got %d", sds00735.Pence)
	}
}

func TestResolveBlankFlowDatesIncluded(t *testing.T) {
	// parseDate returns "" for blank/short RJFAF date fields. A flow stored with
	// blank start/end dates is open-ended and must still match, exactly like the
	// (col = '' OR col op ?) guard used for locations and group members. A bare
	// end_date >= ? comparison would wrongly drop it ('' sorts below any date).
	db := seedResolveDB(t)
	if _, err := db.Exec(
		`UPDATE rjf_flows SET start_date = '', end_date = '' WHERE flow_id = '0662636'`); err != nil {
		t.Fatalf("blank flow dates: %v", err)
	}
	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	var sds00735 *ResolvedFare
	for i := range fares {
		if fares[i].TicketCode == "SDS" && fares[i].Route == "00735" {
			sds00735 = &fares[i]
		}
	}
	if sds00735 == nil {
		t.Fatal("blank flow dates: SDS route 00735 should still resolve, got none")
	}
	if sds00735.Pence != 2310 {
		t.Errorf("blank flow dates: want SDS route 00735 = 2310, got %d", sds00735.Pence)
	}
}

func TestResolveBlankNDFDatesIncluded(t *testing.T) {
	// An NDF override stored with blank dates is open-ended and must surface.
	db := seedResolveDB(t)
	if _, err := db.Exec(
		`INSERT INTO rjf_ndf(origin_nlc,dest_nlc,route,ticket_code,pence,restriction_code,start_date,end_date)
		 VALUES('3087','3149','00735','SDS',1500,'','','')`); err != nil {
		t.Fatalf("insert ndf: %v", err)
	}
	fares, err := Resolve(db, "PAD", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	var sds00735 *ResolvedFare
	for i := range fares {
		if fares[i].TicketCode == "SDS" && fares[i].Route == "00735" {
			sds00735 = &fares[i]
		}
	}
	if sds00735 == nil {
		t.Fatal("blank NDF dates: SDS route 00735 should still resolve, got none")
	}
	if sds00735.Pence != 1500 {
		t.Errorf("blank NDF dates: want SDS route 00735 = 1500 (open-ended NDF), got %d", sds00735.Pence)
	}
}

func TestResolvePastDate(t *testing.T) {
	// Before every flow's start_date: must return empty slice, no error.
	db := seedResolveDB(t)
	fares, err := Resolve(db, "PAD", "RDG", "20200101")
	if err != nil {
		t.Fatalf("Resolve past date: %v", err)
	}
	if len(fares) != 0 {
		t.Errorf("past date: want 0 fares, got %d", len(fares))
	}
}

func TestResolveUnknownCRS(t *testing.T) {
	db := seedResolveDB(t)
	fares, err := Resolve(db, "ZZZ", "RDG", "20260621")
	if err != nil {
		t.Fatalf("Resolve unknown CRS: %v", err)
	}
	if len(fares) != 0 {
		t.Errorf("unknown CRS: want 0 fares, got %d", len(fares))
	}
}
