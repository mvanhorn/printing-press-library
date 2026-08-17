package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/store"
	"github.com/spf13/cobra"
)

func TestHealthRejectsInvertedTTLBand(t *testing.T) {
	var flags rootFlags
	cmd := newNovelHealthCmd(&flags)
	cmd.SetArgs([]string{"--min-ttl", "86400", "--max-ttl", "300"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error when --min-ttl exceeds --max-ttl, got nil")
	}
}

func TestHasDMARCPolicyRequiresNameAndVersionPrefix(t *testing.T) {
	tests := []struct {
		name string
		rec  dmeRecord
		want bool
	}{
		{"valid policy", dmeRecord{Name: "_dmarc", Type: "TXT", Value: `"v=DMARC1; p=reject"`}, true},
		{"policy at wrong name", dmeRecord{Name: "@", Type: "TXT", Value: "v=DMARC1; p=reject"}, false},
		{"non-policy at dmarc name", dmeRecord{Name: "_dmarc", Type: "TXT", Value: "ownership verification"}, false},
		{"invalid version prefix", dmeRecord{Name: "_dmarc", Type: "TXT", Value: "v=DMARC10; p=reject"}, false},
		{"wrong record type", dmeRecord{Name: "_dmarc", Type: "SPF", Value: "v=DMARC1; p=reject"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasDMARCPolicy(tt.rec); got != tt.want {
				t.Fatalf("hasDMARCPolicy(%+v) = %v, want %v", tt.rec, got, tt.want)
			}
		})
	}
}

func TestBindRecordLine(t *testing.T) {
	cases := []struct {
		name string
		rec  dmeRecord
		want string
	}{
		{"apex A", dmeRecord{Name: "", Type: "A", Value: "52.10.4.7", TTL: 300}, "@\t300\tIN\tA\t52.10.4.7"},
		{"www CNAME", dmeRecord{Name: "www", Type: "CNAME", Value: "example.com", TTL: 1800}, "www\t1800\tIN\tCNAME\texample.com."},
		{"MX", dmeRecord{Name: "", Type: "MX", Value: "mail.example.com", TTL: 3600, MxLevel: 10}, "@\t3600\tIN\tMX\t10 mail.example.com."},
		{"SRV", dmeRecord{Name: "_sip._tcp", Type: "SRV", Value: "sip.example.com", TTL: 3600, Priority: 10, Weight: 60, Port: 5060}, "_sip._tcp\t3600\tIN\tSRV\t10 60 5060 sip.example.com."},
		{"ANAME commented", dmeRecord{Name: "", Type: "ANAME", Value: "origin.example.net", TTL: 300}, "; UNSUPPORTED: @ ANAME origin.example.net. (DNS Made Easy apex flattening is not representable in standard BIND syntax)"},
		{"TXT quoted", dmeRecord{Name: "", Type: "TXT", Value: "v=spf1 include:_spf.example.com ~all", TTL: 3600}, "@\t3600\tIN\tTXT\t\"v=spf1 include:_spf.example.com ~all\""},
		{"zero ttl defaults", dmeRecord{Name: "a", Type: "A", Value: "1.2.3.4", TTL: 0}, "a\t1800\tIN\tA\t1.2.3.4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bindRecordLine(c.rec); got != c.want {
				t.Errorf("bindRecordLine = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBindQuoteTXT(t *testing.T) {
	// Short bare value -> single quoted string.
	if got := bindQuoteTXT("v=spf1 -all"); got != `"v=spf1 -all"` {
		t.Errorf("short: got %q", got)
	}
	// Embedded quote/backslash escaped.
	if got := bindQuoteTXT(`a"b\c`); got != `"a\"b\\c"` {
		t.Errorf("escape: got %q", got)
	}
	if got := bindQuoteTXT("a\tb\nc\x00\x7f"); got != `"a\009b\010c\000\127"` {
		t.Errorf("control characters: got %q", got)
	}
	// DME multi-string form is recombined then re-chunked (content preserved).
	if got := bindQuoteTXT(`"v=DKIM1; k=rsa; " "p=ABC"`); got != `"v=DKIM1; k=rsa; p=ABC"` {
		t.Errorf("multi-string: got %q", got)
	}
	// A >255-octet value must split into multiple <=255 character-strings.
	long := strings.Repeat("k", 600)
	got := bindQuoteTXT(long)
	// Recover the content by concatenating the quoted segments; each must be <=255.
	if txtRawContent(got) != long {
		t.Errorf("long: round-trip content mismatch")
	}
	segs := strings.Count(got, `"`) / 2
	if segs != 3 { // 600 -> 255 + 255 + 90
		t.Errorf("long: expected 3 segments, got %d (%q...)", segs, got[:40])
	}
	for _, part := range strings.Split(got, `" "`) {
		p := strings.Trim(part, `"`)
		if len(p) > 255 {
			t.Errorf("long: a segment exceeds 255 octets (%d)", len(p))
		}
	}
}

func TestApplyValue(t *testing.T) {
	cases := []struct {
		cur, match, set string
		contains        bool
		want            string
	}{
		{"52.10.4.7", "52.10.4.7", "52.10.4.9", false, "52.10.4.9"},
		{"host-52.10.4.7.internal", "52.10.4.7", "52.10.4.9", true, "host-52.10.4.9.internal"},
		{"52.10.4.7", "52.10.4.7", "52.10.4.9", true, "52.10.4.9"},
	}
	for _, c := range cases {
		if got := applyValue(c.cur, c.match, c.set, c.contains); got != c.want {
			t.Errorf("applyValue(%q,%q,%q,%v) = %q, want %q", c.cur, c.match, c.set, c.contains, got, c.want)
		}
	}
}

func TestPointsIntoManagedZone(t *testing.T) {
	zones := map[string]bool{"example.com": true, "example.org": true}
	cases := []struct {
		target string
		want   bool
	}{
		{"example.com", true},
		{"www.example.com", true},
		{"api.example.org", true},
		{"external.net", false},
		{"notexample.com", false},
	}
	for _, c := range cases {
		if got := pointsIntoManagedZone(c.target, zones); got != c.want {
			t.Errorf("pointsIntoManagedZone(%q) = %v, want %v", c.target, got, c.want)
		}
	}
}

func TestRecordFQDN(t *testing.T) {
	if got := recordFQDN(dmeRecord{Name: "", DomainName: "example.com"}); got != "example.com" {
		t.Errorf("apex FQDN = %q", got)
	}
	if got := recordFQDN(dmeRecord{Name: "www", DomainName: "example.com"}); got != "www.example.com" {
		t.Errorf("www FQDN = %q", got)
	}
}

// TestZoneMirrorRoundTrip exercises the write/read/snapshot path against a real
// temp SQLite store: writeZoneMirror populates zone_records + a snapshot batch,
// loadZoneRecords reads them back, and recentBatches sees the batch.
func TestZoneMirrorRoundTrip(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	recs := []dmeRecord{
		{ID: "1", DomainID: "100", DomainName: "example.com", Name: "www", Type: "A", Value: "52.10.4.7", TTL: 300, Raw: json.RawMessage(`{"id":1,"name":"www","type":"A","value":"52.10.4.7","ttl":300}`)},
		{ID: "2", DomainID: "100", DomainName: "example.com", Name: "", Type: "MX", Value: "mail.example.com", TTL: 3600, MxLevel: 10, Raw: json.RawMessage(`{"id":2,"type":"MX","value":"mail.example.com","ttl":3600,"mxLevel":10}`)},
		{ID: "3", DomainID: "200", DomainName: "example.org", Name: "api", Type: "A", Value: "52.10.4.7", TTL: 300, Raw: json.RawMessage(`{"id":3,"name":"api","type":"A","value":"52.10.4.7","ttl":300}`)},
	}
	batch, err := writeZoneMirror(ctx, s, recs)
	if err != nil {
		t.Fatalf("writeZoneMirror: %v", err)
	}
	if batch == "" {
		t.Fatal("empty batch id")
	}

	got, err := loadZoneRecords(ctx, s)
	if err != nil {
		t.Fatalf("loadZoneRecords: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("loaded %d records, want 3", len(got))
	}
	n, err := zoneRecordCount(ctx, s)
	if err != nil || n != 3 {
		t.Fatalf("zoneRecordCount = %d, %v", n, err)
	}
	batches, err := recentBatches(ctx, s, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0] != batch {
		t.Fatalf("recentBatches = %v, want [%s]", batches, batch)
	}

	// Two records across two zones share value 52.10.4.7 — the where-used core.
	shared := 0
	for _, r := range got {
		if r.Value == "52.10.4.7" {
			shared++
		}
	}
	if shared != 2 {
		t.Errorf("expected 2 records with shared value, got %d", shared)
	}
}

func TestWhereUsedTypeBoundaryAndTruncation(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	recs := []dmeRecord{
		{ID: "1", DomainID: "100", DomainName: "example.com", Name: "www", Type: "A", Value: "52.10.4.7", TTL: 300},
		{ID: "2", DomainID: "100", DomainName: "example.com", Name: "mail", Type: "MX", Value: "52.10.4.7", TTL: 300},
		{ID: "3", DomainID: "200", DomainName: "example.org", Name: "api", Type: "A", Value: "52.10.4.7", TTL: 300},
		{ID: "4", DomainID: "300", DomainName: "example.net", Name: "txt", Type: "TXT", Value: "unrelated", TTL: 300},
	}
	if _, err := writeZoneMirror(ctx, s, recs); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	flags := rootFlags{asJSON: true}
	cmd := newNovelWhereUsedCmd(&flags)
	cmd.SetArgs([]string{"52.10.4.7", "--type", "A", "--limit", "1", "--db", dbPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var view whereUsedView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("decode output %q: %v", out.String(), err)
	}
	if view.ScannedRecs != 2 || view.ScannedZones != 2 {
		t.Fatalf("scan boundary = %d records across %d zones, want 2 across 2", view.ScannedRecs, view.ScannedZones)
	}
	if view.TotalMatches != 2 || !view.Truncated || len(view.Matches) != 1 {
		t.Fatalf("match state = total %d truncated %v returned %d, want 2/true/1", view.TotalMatches, view.Truncated, len(view.Matches))
	}
	if !strings.Contains(view.Note, "first 1 of 2") {
		t.Fatalf("truncation note = %q", view.Note)
	}
}

func TestSnapRowsDifferIncludesRecordIdentity(t *testing.T) {
	base := snapRow{zone: "example.com", name: "www", typ: "A", value: "192.0.2.1", ttl: 300}
	if snapRowsDiffer(base, base) {
		t.Fatal("identical snapshot rows reported as changed")
	}
	changes := []snapRow{
		{zone: "example.net", name: "www", typ: "A", value: "192.0.2.1", ttl: 300},
		{zone: "example.com", name: "api", typ: "A", value: "192.0.2.1", ttl: 300},
		{zone: "example.com", name: "www", typ: "AAAA", value: "192.0.2.1", ttl: 300},
		{zone: "example.com", name: "www", typ: "A", value: "192.0.2.2", ttl: 300},
		{zone: "example.com", name: "www", typ: "A", value: "192.0.2.1", ttl: 600},
	}
	for _, changed := range changes {
		if !snapRowsDiffer(base, changed) {
			t.Fatalf("snapshot change was missed: before=%+v after=%+v", base, changed)
		}
	}
}

func TestHealthZoneFilterScanBoundary(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	recs := []dmeRecord{
		{ID: "1", DomainID: "100", DomainName: "example.com", Name: "www", Type: "A", Value: "192.0.2.1", TTL: 300},
		{ID: "2", DomainID: "200", DomainName: "example.org", Name: "www", Type: "A", Value: "192.0.2.2", TTL: 300},
	}
	if _, err := writeZoneMirror(ctx, s, recs); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	flags := rootFlags{asJSON: true}
	cmd := newNovelHealthCmd(&flags)
	cmd.SetArgs([]string{"--zone", "example.com", "--db", dbPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var view healthView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("decode output %q: %v", out.String(), err)
	}
	if view.ScannedZones != 1 || view.ScannedRecs != 1 {
		t.Fatalf("filtered health scan = %d records across %d zones, want 1 across 1", view.ScannedRecs, view.ScannedZones)
	}
}

func TestAcmeDeleteSummaryReportsSuccessfulAndFailedZones(t *testing.T) {
	got := acmeDeleteSummary(3, 1, 2)
	want := "deleted 3 record(s) across 1 zone(s); 2 zone(s) failed"
	if got != want {
		t.Fatalf("acmeDeleteSummary = %q, want %q", got, want)
	}
}

func TestAcmeTargetVerbReflectsPerTargetOutcome(t *testing.T) {
	if got := acmeTargetVerb(false, ""); got != "WOULD DELETE" {
		t.Fatalf("preview verb = %q", got)
	}
	if got := acmeTargetVerb(true, "deleted"); got != "DELETED" {
		t.Fatalf("successful apply verb = %q", got)
	}
	if got := acmeTargetVerb(true, "failed"); got != "FAILED" {
		t.Fatalf("failed apply verb = %q", got)
	}
}

func TestMissingMirrorPreservesEachCommandJSONShape(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing.db")
	tests := []struct {
		name       string
		wantPrefix byte
		newCmd     func(*rootFlags) *cobra.Command
		args       []string
	}{
		{"where-used object", '{', newNovelWhereUsedCmd, []string{"192.0.2.1", "--db", dbPath}},
		{"drift object", '{', newNovelDriftCmd, []string{"--db", dbPath}},
		{"health object", '{', newNovelHealthCmd, []string{"--db", dbPath}},
		{"export array", '[', newNovelExportCmd, []string{"example.com", "--format", "json", "--db", dbPath}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := rootFlags{asJSON: true}
			cmd := tt.newCmd(&flags)
			cmd.SetArgs(tt.args)
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			got := bytes.TrimSpace(stdout.Bytes())
			if len(got) == 0 || got[0] != tt.wantPrefix || !json.Valid(got) {
				t.Fatalf("stdout = %q, want valid JSON starting with %q; stderr = %q", got, tt.wantPrefix, stderr.String())
			}
		})
	}
}
