// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package registrydb

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHexKnownPairs(t *testing.T) {
	pairs := map[string]string{
		"a00001": "N1",
		"a00724": "N1000Z",
		"a0001a": "N1AZ",
		"a008c5": "N101DQ",
		"a061d9": "N12345",
		"abcdef": "N86QU",
		"adf7c7": "N99999",
	}
	for hex, tail := range pairs {
		got, err := IcaoToTail(hex)
		if err != nil {
			t.Errorf("IcaoToTail(%s): %v", hex, err)
			continue
		}
		if got != tail {
			t.Errorf("IcaoToTail(%s) = %s, want %s", hex, got, tail)
		}
		back, err := TailToIcao(tail)
		if err != nil {
			t.Errorf("TailToIcao(%s): %v", tail, err)
			continue
		}
		if back != toUpper(hex) {
			t.Errorf("TailToIcao(%s) = %s, want %s", tail, back, toUpper(hex))
		}
	}
}

func toUpper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

func TestHexRoundtripSweep(t *testing.T) {
	// Every 1009th address across the whole US block must roundtrip.
	for v := icaoFirst; v <= icaoLast; v += 1009 {
		hex := toUpper(formatHex(v))
		tail, err := IcaoToTail(hex)
		if err != nil {
			t.Fatalf("IcaoToTail(%s): %v", hex, err)
		}
		back, err := TailToIcao(tail)
		if err != nil {
			t.Fatalf("TailToIcao(%s) [from %s]: %v", tail, hex, err)
		}
		if back != hex {
			t.Fatalf("roundtrip %s -> %s -> %s", hex, tail, back)
		}
	}
}

func formatHex(v int) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		out[i] = digits[v&0xF]
		v >>= 4
	}
	return string(out)
}

func TestHexInvalid(t *testing.T) {
	if _, err := IcaoToTail("c00001"); err == nil {
		t.Error("IcaoToTail outside US block should error")
	}
	if _, err := TailToIcao("N0"); err == nil {
		t.Error("TailToIcao N0 should error (leading digit must be 1-9)")
	}
	if _, err := TailToIcao("N1I"); err == nil {
		t.Error("TailToIcao with I should error")
	}
}

// buildFixtureZip creates a miniature ReleasableAircraft.zip.
func buildFixtureZip(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ReleasableAircraft.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	files := map[string]string{
		"MASTER.txt": "\ufeffN-NUMBER,SERIAL NUMBER,MFR MDL CODE,ENG MFR MDL,YEAR MFR,TYPE REGISTRANT,NAME,STREET,STREET2,CITY,STATE,ZIP CODE,REGION,COUNTY,COUNTRY,LAST ACTION DATE,CERT ISSUE DATE,CERTIFICATION,TYPE AIRCRAFT,TYPE ENGINE,STATUS CODE,MODE S CODE,FRACT OWNER,AIR WORTH DATE,OTHER NAMES(1),OTHER NAMES(2),OTHER NAMES(3),OTHER NAMES(4),OTHER NAMES(5),EXPIRATION DATE,UNIQUE ID,KIT MFR, KIT MODEL,MODE S CODE HEX,\n" +
			"100EX ,560-6513  ,3070032,52001,2004,3,EXAMPLE AVIATION LLC   ,1200 NW 63RD ST ,  ,OKLAHOMA CITY ,OK,73116 ,2,109,US,20260630,20260630,1T,5,5 ,V ,50170304, ,20040610,SAMPLE CO-OWNER LLC  , , , , ,20330630,01326483, , ,A00801    ,\n" +
			"172SP,17280005  ,2072704,17003,1998,3,SKYHAWK LLC ,1 MAIN ST , ,SEATTLE ,WA,98101 ,S,033,US,20240101,20200101,1 ,4,1 ,V ,50000001, ,19990101, , , , , ,20260801,00000001, , ,A00001    ,\n",
		"ACFTREF.txt": "\ufeffCODE,MFR,MODEL,TYPE-ACFT,TYPE-ENG,AC-CAT,BUILD-CERT-IND,NO-ENG,NO-SEATS,AC-WEIGHT,SPEED,TC-DATA-SHEET,TC-DATA-HOLDER,\n" +
			"3070032,TEXTRON AVIATION INC,560XL,5,5,1,0,2,12,CLASS 3,0,A22CE,TEXTRON AVIATION INC,\n" +
			"2072704,CESSNA,172S,4,1,1,0,1,4,CLASS 1,122,3A12,TEXTRON AVIATION INC,\n",
		"ENGINE.txt": "\ufeffCODE,MFR,MODEL,TYPE,HORSEPOWER,THRUST,\n" +
			"52001,PRATT & WHITNEY CANADA,PW545B,5,0,3991,\n" +
			"17003,LYCOMING,IO-360-L2A,1,180,0,\n",
		"DEREG.txt": "\ufeffN-NUMBER,SERIAL-NUMBER,MFR-MDL-CODE,STATUS-CODE,NAME,STREET-MAIL,STREET2-MAIL,CITY-MAIL,STATE-ABBREV-MAIL,ZIP-CODE-MAIL,ENG-MFR-MDL,YEAR-MFR,CERTIFICATION,REGION,COUNTY-MAIL,COUNTRY-MAIL,AIR-WORTH-DATE,CANCEL-DATE,MODE-S-CODE,INDICATOR-GROUP,EXP-COUNTRY,LAST-ACT-DATE,CERT-ISSUE-DATE,STREET-PHYSICAL,STREET2-PHYSICAL,CITY-PHYSICAL,STATE-ABBREV-PHYSICAL,ZIP-CODE-PHYSICAL,COUNTY-PHYSICAL,COUNTRY-PHYSICAL,OTHER-NAMES(1),OTHER-NAMES(2),OTHER-NAMES(3),OTHER-NAMES(4),OTHER-NAMES(5),KIT MFR, KIT MODEL,MODE S CODE HEX,\n" +
			"100EX ,560-0001  ,3070032,13,OLD OWNER LLC ,2 OAK AVE , ,DENVER ,CO,80014 ,52001,2001,1T,3,031,US,20010101,20150601,50170304, , ,20150601,20050101, , , , , , , , , , , , , , ,A00801 ,\n",
		"RESERVED.txt": "\ufeffN-NUMBER,REGISTRANT,STREET,STREET2,CITY,STATE,ZIP CODE,RSV DATE,TR,EXP DATE,N-NUM-CHG,PURGE DATE,\n" +
			"500XA,SAMPLE REGISTRANT ,1 VIEW LN , ,SEATTLE ,WA,98101 ,20250101,N,20260101, ,20270101,\n",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	zipPath := buildFixtureZip(t)
	if _, err := db.ImportZip(ctx, zipPath, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestImportAndLookup(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	ac, err := db.LookupTail(ctx, "N100EX")
	if err != nil {
		t.Fatal(err)
	}
	if ac == nil {
		t.Fatal("N100EX not found after import")
	}
	if ac.OwnerName != "EXAMPLE AVIATION LLC" {
		t.Errorf("owner = %q", ac.OwnerName)
	}
	if ac.Model != "560XL" || ac.Manufacturer != "TEXTRON AVIATION INC" {
		t.Errorf("model join = %q %q", ac.Manufacturer, ac.Model)
	}
	if ac.EngineModel != "PW545B" {
		t.Errorf("engine join = %q", ac.EngineModel)
	}
	if ac.Status != "Valid Registration" {
		t.Errorf("status = %q", ac.Status)
	}
	if ac.OwnerType != "Corporation" {
		t.Errorf("owner type = %q", ac.OwnerType)
	}
	if len(ac.OtherNames) != 1 || ac.OtherNames[0] != "SAMPLE CO-OWNER LLC" {
		t.Errorf("other names = %v", ac.OtherNames)
	}

	byHex, err := db.LookupHex(ctx, "a00801")
	if err != nil || byHex == nil || byHex.NNumber != "N100EX" {
		t.Errorf("LookupHex = %+v, err %v", byHex, err)
	}
}

func TestHistoryAndAvailability(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	events, err := db.History(ctx, "N100EX")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("history events = %d, want 2 (dereg + current)", len(events))
	}
	if events[0].Kind != "deregistered" || events[0].Owner != "OLD OWNER LLC" {
		t.Errorf("event[0] = %+v", events[0])
	}
	if events[1].Kind != "current" {
		t.Errorf("event[1] = %+v", events[1])
	}

	av, err := db.Available(ctx, "N100EX")
	if err != nil || av.Available {
		t.Errorf("N100EX availability = %+v, err %v (want unavailable)", av, err)
	}
	av, err = db.Available(ctx, "N500XA")
	if err != nil || av.Available || av.Reason != "reserved" {
		t.Errorf("N500XA availability = %+v, err %v (want reserved)", av, err)
	}
	av, err = db.Available(ctx, "N99999")
	if err != nil || !av.Available {
		t.Errorf("N99999 availability = %+v, err %v (want available)", av, err)
	}
}

func TestFleetAndModelFleet(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	rep, err := db.Fleet(ctx, "EXAMPLE", false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Count != 1 || rep.EngineClasses["jet"] != 1 {
		t.Errorf("fleet report = %+v", rep)
	}
	// Fractional co-owner match via OTHER NAMES
	rep, err = db.Fleet(ctx, "SAMPLE", false)
	if err != nil || rep.Count != 1 {
		t.Errorf("co-owner fleet = %+v, err %v", rep, err)
	}

	mf, err := db.ModelFleet(ctx, "CESSNA", "172")
	if err != nil || mf.Count != 1 || mf.RegistrantTypes["Corporation"] != 1 {
		t.Errorf("model fleet = %+v, err %v", mf, err)
	}
}

func TestExpiringAndSearch(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// N172SP expires 20260801 — within ~60 days of the frozen test clock
	// assumption is fragile; use a generous window instead.
	exp, err := db.Expiring(ctx, 3650, "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(exp) < 1 {
		t.Errorf("expiring = %+v, want at least one", exp)
	}

	res, err := db.Search(ctx, "EXAMPLE", 10)
	if err != nil || len(res) != 1 || res[0].NNumber != "N100EX" {
		t.Errorf("search = %+v, err %v", res, err)
	}
	res, err = db.Search(ctx, "SKYHAWK", 10)
	if err != nil || len(res) != 1 {
		t.Errorf("search skyhawk = %+v, err %v", res, err)
	}
}

func TestIncompleteArchiveRejected(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "partial.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Build a zip missing ENGINE.txt (a required file).
	dir := t.TempDir()
	path := filepath.Join(dir, "Partial.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("MASTER.txt")
	w.Write([]byte("\ufeffN-NUMBER,\n1,\n"))
	zw.Close()
	f.Close()

	if _, err := db.ImportZip(ctx, path, nil); err == nil {
		t.Fatal("expected error importing incomplete archive, got nil")
	}
	// A rejected partial import must NOT mark the DB synced.
	synced, err := db.Synced(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if synced {
		t.Error("Synced() = true after a rejected incomplete import; want false")
	}
}

func TestFailedReimportRollsBack(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t) // fully synced from the good fixture
	// Sanity: the good data is present and searchable.
	if ac, _ := db.LookupTail(ctx, "N100EX"); ac == nil {
		t.Fatal("precondition: N100EX should be present")
	}

	// Build a second archive that is well-formed except ENGINE.txt has a bad
	// header (no known columns) so its import fails mid-transaction.
	dir := t.TempDir()
	path := filepath.Join(dir, "Bad.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	good := map[string]string{
		"MASTER.txt":   "\ufeffN-NUMBER,\nNEW1 ,\n",
		"ACFTREF.txt":  "\ufeffCODE,MFR,MODEL,\nX,Y,Z,\n",
		"DEREG.txt":    "\ufeffN-NUMBER,\n",
		"RESERVED.txt": "\ufeffN-NUMBER,\n",
		"ENGINE.txt":   "totally-unknown-header\nrow\n", // no known columns → importCSV errors
	}
	for n, c := range good {
		w, _ := zw.Create(n)
		w.Write([]byte(c))
	}
	zw.Close()
	f.Close()

	if _, err := db.ImportZip(ctx, path, nil); err == nil {
		t.Fatal("expected the bad re-import to fail")
	}
	// The whole transaction must roll back: the ORIGINAL data is intact, not
	// the half-applied NEW1 row, and the DB still reports synced.
	synced, _ := db.Synced(ctx)
	if !synced {
		t.Error("Synced() = false after a rolled-back re-import; previous good sync should survive")
	}
	if ac, _ := db.LookupTail(ctx, "N100EX"); ac == nil {
		t.Error("original N100EX lost after a rolled-back re-import; base table was not restored")
	}
	if ac, _ := db.LookupTail(ctx, "NNEW1"); ac != nil {
		t.Error("partially-imported NEW1 row survived a rolled-back re-import")
	}
	// Search index must still match the original tables.
	if res, _ := db.Search(ctx, "EXAMPLE", 5); len(res) != 1 {
		t.Errorf("search index diverged after rollback: got %d results, want 1", len(res))
	}
}

func TestNotSynced(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.LookupTail(ctx, "N1"); err != ErrNotSynced {
		t.Errorf("LookupTail on empty db err = %v, want ErrNotSynced", err)
	}
}
