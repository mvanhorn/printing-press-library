package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

func TestNCCPLToDMY(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-09-04", "04/09/2026"},
		{"2015-12-09", "09/12/2015"},
		{"2026-01-31", "31/01/2026"},
		{"", ""},
		{"not-a-date", "not-a-date"},
		{"04/09/2026", "04/09/2026"}, // already converted; must not double-convert
	}
	for _, c := range cases {
		if got := nccplToDMY(c.in); got != c.want {
			t.Errorf("nccplToDMY(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The three date encodings are the highest-risk detail in this API: sending the
// wrong one returns an empty array with HTTP 200 rather than an error.
func TestNCCPLRequestBodyPerDateMode(t *testing.T) {
	fipi, _ := nccplResourceByName("fipi")
	body := nccplRequestBody(fipi, "2026-09-04")
	if body["fromDate"] != "04/09/2026" || body["toDate"] != "04/09/2026" {
		t.Errorf("fipi must send DD/MM/YYYY, got %v", body)
	}
	if body["type"] != "101" {
		t.Errorf("fipi must default type to 101, got %v", body["type"])
	}

	sector, _ := nccplResourceByName("fipi-sector")
	body = nccplRequestBody(sector, "2026-09-04")
	if body["fromDate"] != "2026-09-04" || body["toDate"] != "2026-09-04" {
		t.Errorf("fipi-sector must send raw YYYY-MM-DD, got %v", body)
	}
	if _, ok := body["type"]; ok {
		t.Errorf("fipi-sector must not send a type field, got %v", body)
	}

	varm, _ := nccplResourceByName("var-margins")
	body = nccplRequestBody(varm, "2026-09-04")
	if body["date"] != "2026-09-04" {
		t.Errorf("var-margins must send raw YYYY-MM-DD in date, got %v", body)
	}
}

// fipi-normal returns `records` while lipi-normal returns `data`. The asymmetry is
// upstream; assuming symmetry silently returns nothing for one of them.
func TestNCCPLNormalEnvelopeAsymmetry(t *testing.T) {
	fn, _ := nccplResourceByName("fipi-normal")
	ln, _ := nccplResourceByName("lipi-normal")
	if fn.Envelope != "records" {
		t.Errorf("fipi-normal envelope = %q, want records", fn.Envelope)
	}
	if ln.Envelope != "data" {
		t.Errorf("lipi-normal envelope = %q, want data", ln.Envelope)
	}
}

func TestNCCPLSessionDatesSkipsWeekends(t *testing.T) {
	// 2026-09-04 is a Friday; 05 Sat, 06 Sun; 07 Monday.
	got, err := nccplSessionDates("2026-09-04", "2026-09-07")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"2026-09-04", "2026-09-07"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if _, err := nccplSessionDates("2026-09-07", "2026-09-04"); err == nil {
		t.Error("reversed range must error")
	}
	if _, err := nccplSessionDates("bad", "2026-09-04"); err == nil {
		t.Error("malformed --from must error")
	}
}

func TestNCCPLRowKeyStableAndCollisionSafe(t *testing.T) {
	r, _ := nccplResourceByName("fipi")
	seen := map[string]bool{}
	k1 := nccplRowKey(r, map[string]any{"client_type": "FI", "segment": "EQUITY"}, 0, seen)
	if k1 != "FI|EQUITY" {
		t.Errorf("key = %q, want FI|EQUITY", k1)
	}
	// Identical row later in the payload must not overwrite the first.
	k2 := nccplRowKey(r, map[string]any{"client_type": "FI", "segment": "EQUITY"}, 3, seen)
	if k2 == k1 {
		t.Errorf("duplicate key %q must be disambiguated", k2)
	}
	// No key fields at all falls back to the ordinal rather than colliding on "".
	k3 := nccplRowKey(r, map[string]any{"unrelated": 1}, 7, seen)
	if k3 != "#7" {
		t.Errorf("fallback key = %q, want #7", k3)
	}
}

func TestNCCPLNumParsesStringsAndCommas(t *testing.T) {
	row := map[string]any{
		"a": float64(12.5),
		"b": "1,234.75",
		"c": "  -42 ",
		"d": "",
		"e": "abc",
	}
	if v, ok := nccplNum(row, "a"); !ok || v != 12.5 {
		t.Errorf("float passthrough failed: %v %v", v, ok)
	}
	if v, ok := nccplNum(row, "b"); !ok || v != 1234.75 {
		t.Errorf("comma-separated parse failed: %v %v", v, ok)
	}
	if v, ok := nccplNum(row, "c"); !ok || v != -42 {
		t.Errorf("negative parse failed: %v %v", v, ok)
	}
	if _, ok := nccplNum(row, "d"); ok {
		t.Error("empty string must not parse")
	}
	if _, ok := nccplNum(row, "e"); ok {
		t.Error("non-numeric must not parse")
	}
	if _, ok := nccplNum(row, "missing"); ok {
		t.Error("absent field must not parse")
	}
	// Falls through candidate names in order.
	if v, ok := nccplNum(row, "missing", "a"); !ok || v != 12.5 {
		t.Errorf("fallback candidate failed: %v %v", v, ok)
	}
}

// The net-summing helpers must exclude the pre-summed FN/LN "net" rows, or the
// invariant check double-counts a whole side and reports a false failure.
func TestNCCPLSumNetExcludesPreSummedNetRows(t *testing.T) {
	obs := []store.NCCPLObs{
		{Date: "2026-09-04", Payload: `{"client_type":"FI","net_value":100}`},
		{Date: "2026-09-04", Payload: `{"client_type":"FC","net_value":-30}`},
		{Date: "2026-09-04", Payload: `{"client_type":"FN","net_value":70}`},
		{Date: "2026-09-04", Payload: `not json`},
	}
	got := nccplSumNetByDate(obs)
	if got["2026-09-04"] != 70 {
		t.Errorf("sum = %v, want 70 (FN row must be excluded, bad JSON skipped)", got["2026-09-04"])
	}
}

func TestNCCPLSumNetByDateSectorGroups(t *testing.T) {
	obs := []store.NCCPLObs{
		{Date: "2026-09-04", Payload: `{"SECTOR_NAME":"Cement","CLIENT_TYPE":"FI","NET_VALUE":10}`},
		{Date: "2026-09-04", Payload: `{"SECTOR_NAME":"Cement","CLIENT_TYPE":"LI","NET_VALUE":-10}`},
		{Date: "2026-09-04", Payload: `{"SECTOR_NAME":"Cement","CLIENT_TYPE":"LN","NET_VALUE":-10}`},
		{Date: "2026-09-04", Payload: `{"CLIENT_TYPE":"FI","NET_VALUE":99}`},
	}
	got := nccplSumNetByDateSector(obs)
	if v := got["2026-09-04"]["Cement"]; v != 0 {
		t.Errorf("Cement net = %v, want 0 (LN excluded)", v)
	}
	if _, ok := got["2026-09-04"][""]; ok {
		t.Error("rows without a sector name must be skipped, not grouped under empty string")
	}
}

func TestNCCPLDecodedCookieMap(t *testing.T) {
	in := map[string]string{
		"XSRF-TOKEN":   "abc%3D%3D",
		"cf_clearance": "plainvalue",
		"weird":        "100%",
	}
	got := nccplDecodedCookieMap(in)
	if got["XSRF-TOKEN"] != "abc==" {
		t.Errorf("XSRF-TOKEN = %q, want abc==", got["XSRF-TOKEN"])
	}
	if got["cf_clearance"] != "plainvalue" {
		t.Errorf("unescaped value must pass through, got %q", got["cf_clearance"])
	}
	if got["weird"] != "100%" {
		t.Errorf("undecodable value must pass through unchanged, got %q", got["weird"])
	}
	if nccplDecodedCookieMap(nil) != nil {
		t.Error("nil in, nil out")
	}
}

func TestNCCPLSelectResources(t *testing.T) {
	all, err := nccplSelectResources("")
	if err != nil || len(all) != len(nccplResources) {
		t.Fatalf("empty selector must return all: %d, %v", len(all), err)
	}
	two, err := nccplSelectResources(" fipi , lipi ")
	if err != nil || len(two) != 2 {
		t.Fatalf("csv selector failed: %d %v", len(two), err)
	}
	if _, err := nccplSelectResources("fipi,nope"); err == nil {
		t.Error("unknown resource must error")
	}
}

// The dogfood-curtailed set must still exercise every request encoding and every
// response envelope, or the self-test stops proving what it claims to prove.
func TestNCCPLRepresentativeResourcesCoverEveryContract(t *testing.T) {
	rep := nccplRepresentativeResources()
	modes := map[nccplDateMode]bool{}
	envs := map[string]bool{}
	for _, r := range rep {
		modes[r.Mode] = true
		envs[r.Envelope] = true
	}
	allModes := map[nccplDateMode]bool{}
	allEnvs := map[string]bool{}
	for _, r := range nccplResources {
		allModes[r.Mode] = true
		allEnvs[r.Envelope] = true
	}
	if len(modes) != len(allModes) {
		t.Errorf("representative set covers %d date modes, want %d", len(modes), len(allModes))
	}
	if len(envs) != len(allEnvs) {
		t.Errorf("representative set covers %d envelopes, want %d", len(envs), len(allEnvs))
	}
}
