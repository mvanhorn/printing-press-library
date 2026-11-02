package cli

import (
	"context"
	"testing"
)

func TestSecCIKValidation(t *testing.T) {
	for _, s := range []string{"320193", "CIK0000320193"} {
		v, e := secCIK(context.Background(), s)
		if e != nil || v != "0000320193" {
			t.Fatalf("%q: %q %v", s, v, e)
		}
	}
	if _, e := secCIK(context.Background(), "0"); e == nil {
		t.Fatal("zero CIK accepted")
	}
}
func TestSecRowsKeepsColumnValues(t *testing.T) {
	r := secRows(map[string]any{"accessionNumber": []any{"a", "b"}, "form": []any{"10-K", "10-Q"}, "filingDate": []any{"2025-01-01"}})
	if len(r) != 2 || r[1]["form"] != "10-Q" || r[0]["filingDate"] != "2025-01-01" {
		t.Fatal(r)
	}
	if _, ok := r[1]["filingDate"]; ok {
		t.Fatal("fabricated missing value")
	}
}
func TestSecDateAndArchiveSafety(t *testing.T) {
	if secRange("2025-02-01", "2025-01-01") == nil {
		t.Fatal("reversed dates accepted")
	}
	if secRange("yesterday", "") == nil {
		t.Fatal("invalid date accepted")
	}
	if secAccession.MatchString("../../etc/passwd") {
		t.Fatal("traversal accession accepted")
	}
	if secArchive("0000320193", "0000320193-25-000079") != "https://www.sec.gov/Archives/edgar/data/320193/000032019325000079/" {
		t.Fatal("bad archive")
	}
}
func TestSecFetchRejectsNonSecBeforeNetwork(t *testing.T) {
	t.Setenv("SEC_USER_AGENT", "test contact@example.invalid")
	for _, u := range []string{"https://example.com/", "http://data.sec.gov/", "https://sec.gov.attacker.test/"} {
		if _, e := secFetch(context.Background(), u); e == nil {
			t.Fatal("accepted", u)
		}
	}
}
