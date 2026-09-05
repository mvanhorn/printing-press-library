package cli

// Hand-authored tests for what `ingest` REFUSES.
//
// The owner rule this CLI exists to serve is that the store may only ever hold
// what the source actually said: never a fabricated observation, never a
// back-dated one, never a gap filled by inference. A missed observation is the
// correct outcome. Every test here therefore pins a refusal, and each asserts
// against the SQLite store as well as the command output, because a report that
// says "skipped" while a row landed anyway is the failure that matters.
//
// Named nccpl_ingest_validation_test.go, not ingest_validation_test.go, for the
// same reason the other hand-written tests here carry the nccpl_ prefix:
// internal/cli/<command>_<sub>.go is the generator's file convention for a
// SUBCOMMAND of <command>, so an unprefixed name reads as a phantom subcommand.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// ingestValidationView runs `ingest` with the given args and decodes its --json
// report, failing the test if the command errored or the output is not one
// document.
func ingestValidationView(t *testing.T, stdin string, args ...string) nccplIngestView {
	t.Helper()
	stdout, _, err := runRootArgsStdin(t, stdin, args...)
	if err != nil {
		t.Fatalf("ingest %v: %v", args, err)
	}
	var view nccplIngestView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("--json output must be one JSON document: %v (got=%q)", err, stdout)
	}
	return view
}

// storedRows reads every row the store holds for a resource, through the ordinary
// read path the analysis commands use.
func storedRows(t *testing.T, dbPath, resource string) []store.NCCPLObs {
	t.Helper()
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	ctx := context.Background()
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = db.Close() }()
	obs, err := store.NCCPLObservations(ctx, db, resource, "", "")
	if err != nil {
		t.Fatalf("read %s: %v", resource, err)
	}
	return obs
}

// ---------------------------------------------------------------------------
// Finding A: the envelope fallback used to accept an arbitrary array.
// ---------------------------------------------------------------------------

// An error response is not data. `{"success":false,"errors":[...]}` carries an
// array, and the old fallback took the first array under ANY key -- so an upstream
// failure was stored as authoritative var-margins observations for the date the
// operator named. It must be refused, with the reason reported.
func TestIngestEnvelope_ErrorsArrayIsRefusedNotStored(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	const errorBody = `{"success":false,"message":"session expired",` +
		`"errors":[{"code":"AUTH","detail":"clearance cookie rejected"}]}`

	view := ingestValidationView(t, errorBody,
		"ingest", "--no-learn", "--stdin",
		"--resource", "var-margins", "--date", "2026-09-04",
		"--db", dbPath, "--json")

	if view.TotalRows != 0 {
		t.Fatalf("total_rows = %d; an errors array must never be stored as observations", view.TotalRows)
	}
	if len(view.Ingested) != 0 {
		t.Fatalf("ingested = %+v, want nothing", view.Ingested)
	}
	if len(view.Skipped) != 1 {
		t.Fatalf("skipped = %+v, want exactly one refusal with a reason", view.Skipped)
	}
	// The reason has to name the envelope that was expected, or the operator
	// cannot tell a drifted key from a genuinely empty day.
	if !strings.Contains(view.Skipped[0], `"margins"`) {
		t.Errorf("skip reason %q does not name the expected envelope", view.Skipped[0])
	}
	if rows := storedRows(t, dbPath, "var-margins"); len(rows) != 0 {
		t.Fatalf("store holds %d var-margins row(s) after a refused body: %+v", len(rows), rows)
	}
}

// Two known envelopes in one body is ambiguous, and the old code resolved it by
// Go's RANDOMISED map iteration order -- a different resource's rows from run to
// run, from byte-identical input. Determinism is asserted by running the same body
// many times: every run must reach the same verdict with the same message.
func TestIngestEnvelope_TwoArraysAreDeterministicallyRefused(t *testing.T) {
	// `data` and `records` are both real envelope names in this API (lipi-normal
	// uses one, fipi-normal the other), so both are on the allow-list. Neither is
	// var-margins' documented `margins`, so which set of rows this body holds is
	// unknowable.
	const twoArrayBody = `{"success":true,` +
		`"data":[{"symbol":"HUBC","var_value":"15.5"}],` +
		`"records":[{"symbol":"OGDC","var_value":"12.25"}]}`

	var first string
	for i := 0; i < 40; i++ {
		home := withTempLearnHome(t)
		dbPath := filepath.Join(home, "data.db")

		view := ingestValidationView(t, twoArrayBody,
			"ingest", "--no-learn", "--stdin",
			"--resource", "var-margins", "--date", "2026-09-04",
			"--db", dbPath, "--json")

		if view.TotalRows != 0 {
			t.Fatalf("run %d: total_rows = %d; an unidentifiable body must not become data", i, view.TotalRows)
		}
		if len(view.Skipped) != 1 {
			t.Fatalf("run %d: skipped = %+v, want exactly one refusal", i, view.Skipped)
		}
		if i == 0 {
			first = view.Skipped[0]
			// The refusal must say what was ambiguous, naming both candidates.
			for _, want := range []string{"data", "records", "ambiguous"} {
				if !strings.Contains(first, want) {
					t.Fatalf("skip reason %q does not mention %q", first, want)
				}
			}
			continue
		}
		if view.Skipped[0] != first {
			t.Fatalf("run %d skip reason %q != run 0 reason %q; map iteration order is leaking into the verdict",
				i, view.Skipped[0], first)
		}
		if rows := storedRows(t, dbPath, "var-margins"); len(rows) != 0 {
			t.Fatalf("run %d: store holds %d row(s) after a refused body", i, len(rows))
		}
	}
}

// The fallback is narrowed, not deleted. A body carrying exactly ONE of the API's
// own envelope names is still accepted, so envelope drift (or a body hand-saved
// from the sibling endpoint that uses `records` where this one uses `data`) does
// not cost an observation.
func TestIngestEnvelope_SingleKnownSiblingEnvelopeStillAccepted(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	// lipi-normal's documented envelope is `data`; `records` is fipi-normal's.
	const siblingBody = `{"success":true,"records":[` +
		`{"CLIENT_TYPE":"INDIVIDUALS","MARKET_TYPE":"EQUITY","net":"12.5"}]}`

	view := ingestValidationView(t, siblingBody,
		"ingest", "--no-learn", "--stdin",
		"--resource", "lipi-normal", "--date", "2026-09-04",
		"--db", dbPath, "--json")

	if view.TotalRows != 1 {
		t.Fatalf("total_rows = %d, want 1: a lone known envelope is unambiguous (skipped=%+v)",
			view.TotalRows, view.Skipped)
	}
	rows := storedRows(t, dbPath, "lipi-normal")
	if len(rows) != 1 || rows[0].Date != "2026-09-04" {
		t.Fatalf("store holds %+v, want one row dated 2026-09-04", rows)
	}
}

// A body under a name this API never uses is not rescued by the allow-list.
func TestIngestEnvelope_UnknownEnvelopeNameIsRefused(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	const foreignBody = `{"result":[{"symbol":"HUBC","var_value":"15.5"}]}`

	view := ingestValidationView(t, foreignBody,
		"ingest", "--no-learn", "--stdin",
		"--resource", "var-margins", "--date", "2026-09-04",
		"--db", dbPath, "--json")

	if view.TotalRows != 0 {
		t.Fatalf("total_rows = %d; `result` is not an envelope this API uses", view.TotalRows)
	}
	if len(view.Skipped) != 1 || !strings.Contains(view.Skipped[0], "result") {
		t.Fatalf("skipped = %+v, want one refusal listing the keys the body did carry", view.Skipped)
	}
	if rows := storedRows(t, dbPath, "var-margins"); len(rows) != 0 {
		t.Fatalf("store holds %d row(s) after a refused body", len(rows))
	}
}

// ---------------------------------------------------------------------------
// Finding B: HAR resource inference never checked the origin.
// ---------------------------------------------------------------------------

// harEntryJSON renders one HAR entry. Kept as raw JSON rather than built from the
// parser's own structs so the fixture describes what DevTools writes, not what the
// parser happens to read.
func harEntryJSON(url, postData, body string, status int) string {
	return fmt.Sprintf(`{"request":{"method":"POST","url":%s,"postData":{"mimeType":"application/json","text":%s}},`+
		`"response":{"status":%d,"content":{"mimeType":"application/json","text":%s}}}`,
		mustJSONString(url), mustJSONString(postData), status, mustJSONString(body))
}

func mustJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func writeHAR(t *testing.T, dir string, entries ...string) string {
	t.Helper()
	path := filepath.Join(dir, "capture.har")
	har := `{"log":{"version":"1.2","creator":{"name":"test","version":"1"},"entries":[` +
		strings.Join(entries, ",") + `]}}`
	if err := os.WriteFile(path, []byte(har), 0o600); err != nil {
		t.Fatalf("write HAR: %v", err)
	}
	return path
}

const (
	genuineVarMarginsBody  = `{"success":true,"margins":[{"symbol":"HUBC","var_value":"15.5","hair_cut":"20.0"}]}`
	impostorVarMarginsBody = `{"success":true,"margins":[{"symbol":"FAKE","var_value":"99.9","hair_cut":"0.0"}]}`
	varMarginsPostBody     = `{"date":"2026-09-04"}`
)

// The core of Finding B. "Save all as HAR with content" exports every request the
// browser made, so a capture can hold a local dev server, a proxy, or a lookalike
// domain serving the same path. Only www.nccpl.com.pk over https is NCCPL; the rest
// must be refused BY NAME in the skipped list rather than silently dropped.
func TestIngestHAR_OnlyNCCPLOriginOverHTTPSIsIngested(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	harPath := writeHAR(t, home,
		// A local dev server or mock serving the identical path.
		harEntryJSON("http://localhost:3000/api/var-margins/data", varMarginsPostBody, impostorVarMarginsBody, 200),
		// A lookalike domain that a substring check on "nccpl.com.pk" accepts.
		harEntryJSON("https://nccpl.com.pk.evil.com/api/var-margins/data", varMarginsPostBody, impostorVarMarginsBody, 200),
		// A different registrable domain entirely.
		harEntryJSON("https://cdn.example.com/api/var-margins/data", varMarginsPostBody, impostorVarMarginsBody, 200),
		// The right host, but plaintext: not NCCPL speaking.
		harEntryJSON("http://www.nccpl.com.pk/api/var-margins/data", varMarginsPostBody, impostorVarMarginsBody, 200),
		// The genuine capture.
		harEntryJSON("https://www.nccpl.com.pk/api/var-margins/data", varMarginsPostBody, genuineVarMarginsBody, 200),
	)

	view := ingestValidationView(t, "", "ingest", "--no-learn", harPath, "--db", dbPath, "--json")

	if view.TotalRows != 1 {
		t.Fatalf("total_rows = %d, want 1 (only the www.nccpl.com.pk entry): ingested=%+v skipped=%+v",
			view.TotalRows, view.Ingested, view.Skipped)
	}
	if len(view.Ingested) != 1 || view.Ingested[0].Resource != "var-margins" || view.Ingested[0].Date != "2026-09-04" {
		t.Fatalf("ingested = %+v, want one var-margins/2026-09-04 batch", view.Ingested)
	}

	// The decisive assertion: the impostor rows must be absent from the store, not
	// merely absent from the report.
	rows := storedRows(t, dbPath, "var-margins")
	if len(rows) != 1 {
		t.Fatalf("store holds %d var-margins row(s), want 1: %+v", len(rows), rows)
	}
	if !strings.Contains(rows[0].Payload, "HUBC") {
		t.Errorf("stored payload = %q, want the genuine NCCPL row", rows[0].Payload)
	}
	for _, r := range rows {
		if strings.Contains(r.Payload, "FAKE") {
			t.Fatalf("a non-NCCPL origin reached the store: %q", r.Payload)
		}
	}

	// Every refusal is reported, so an operator can see what their capture held.
	if len(view.Skipped) != 4 {
		t.Fatalf("skipped = %+v, want one reason per refused origin", view.Skipped)
	}
	joined := strings.Join(view.Skipped, "\n")
	for _, want := range []string{"localhost", "nccpl.com.pk.evil.com", "cdn.example.com", "https"} {
		if !strings.Contains(joined, want) {
			t.Errorf("skipped list does not mention %q:\n%s", want, joined)
		}
	}
	// Refusals are attributed to the file they came from, like every other skip.
	for _, s := range view.Skipped {
		if !strings.HasPrefix(s, "capture.har: ") {
			t.Errorf("skip %q is not attributed to its source file", s)
		}
	}
}

// A lookalike host on its own: nothing ingested at all, and the store is never
// even created with rows in it. Pinned separately from the mixed capture so a
// regression cannot hide behind the genuine entry alongside it.
func TestIngestHAR_LookalikeHostIsRejectedOnItsOwn(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	for _, host := range []string{
		"nccpl.com.pk.evil.com", // suffix attack
		"notnccpl.com.pk",       // no dot boundary
		"nccpl.com.pk.co",       // different registrable domain
	} {
		harPath := writeHAR(t, t.TempDir(),
			harEntryJSON("https://"+host+"/api/var-margins/data", varMarginsPostBody, impostorVarMarginsBody, 200))

		view := ingestValidationView(t, "", "ingest", "--no-learn", harPath, "--db", dbPath, "--json")

		if view.TotalRows != 0 {
			t.Fatalf("host %q: total_rows = %d, want 0", host, view.TotalRows)
		}
		if len(view.Skipped) != 1 || !strings.Contains(view.Skipped[0], host) {
			t.Fatalf("host %q: skipped = %+v, want one refusal naming the host", host, view.Skipped)
		}
		if view.Note == "" {
			t.Errorf("host %q: a capture that ingested nothing must say why", host)
		}
		if rows := storedRows(t, dbPath, "var-margins"); len(rows) != 0 {
			t.Fatalf("host %q: store holds %d row(s): %+v", host, len(rows), rows)
		}
	}
}

// A genuine NCCPL subdomain over https is still accepted -- the fix checks the
// registrable domain, it does not hard-code one hostname.
func TestIngestHAR_NCCPLSubdomainIsAccepted(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	harPath := writeHAR(t, home,
		harEntryJSON("https://WWW.NCCPL.COM.PK/api/var-margins/data", varMarginsPostBody, genuineVarMarginsBody, 200))

	view := ingestValidationView(t, "", "ingest", "--no-learn", harPath, "--db", dbPath, "--json")
	if view.TotalRows != 1 {
		t.Fatalf("total_rows = %d, want 1: a genuine NCCPL host must still ingest (skipped=%+v)",
			view.TotalRows, view.Skipped)
	}
	if rows := storedRows(t, dbPath, "var-margins"); len(rows) != 1 {
		t.Fatalf("store holds %d row(s), want 1", len(rows))
	}
}

// The path must be the API's own, not merely contain it. A proxy that re-hosts the
// path under a prefix, or a mock at a near-miss path, is not this endpoint -- and
// on a non-NCCPL origin it is refused by origin anyway.
func TestIngestHAR_PathMustMatchExactly(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	harPath := writeHAR(t, home,
		harEntryJSON("https://www.nccpl.com.pk/proxy/api/var-margins/data", varMarginsPostBody, impostorVarMarginsBody, 200),
		harEntryJSON("https://www.nccpl.com.pk/api/var-margins/data-mock", varMarginsPostBody, impostorVarMarginsBody, 200),
	)

	view := ingestValidationView(t, "", "ingest", "--no-learn", harPath, "--db", dbPath, "--json")
	if view.TotalRows != 0 {
		t.Fatalf("total_rows = %d, want 0: neither path is /api/var-margins/data", view.TotalRows)
	}
	if rows := storedRows(t, dbPath, "var-margins"); len(rows) != 0 {
		t.Fatalf("store holds %d row(s): %+v", len(rows), rows)
	}
}

// An NCCPL entry that failed upstream, or whose request named no date, is reported
// rather than silently dropped: a capture that ingested nothing must be
// distinguishable from a day the market published nothing.
func TestIngestHAR_UnusableNCCPLEntriesAreReported(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	harPath := writeHAR(t, home,
		// Cloudflare challenged this one.
		harEntryJSON("https://www.nccpl.com.pk/api/var-margins/data", varMarginsPostBody, "<html>challenge</html>", 403),
		// Right origin, right path, but no settlement date to file the rows under.
		harEntryJSON("https://www.nccpl.com.pk/api/slb-market-information/data", `{}`, `{"success":true,"rows":[{"symbol":"HUBC"}]}`, 200),
	)

	view := ingestValidationView(t, "", "ingest", "--no-learn", harPath, "--db", dbPath, "--json")
	if view.TotalRows != 0 {
		t.Fatalf("total_rows = %d, want 0", view.TotalRows)
	}
	if len(view.Skipped) != 2 {
		t.Fatalf("skipped = %+v, want both unusable entries reported", view.Skipped)
	}
	joined := strings.Join(view.Skipped, "\n")
	if !strings.Contains(joined, "403") {
		t.Errorf("skipped list does not report the upstream failure:\n%s", joined)
	}
	if !strings.Contains(joined, "settlement date") {
		t.Errorf("skipped list does not report the undatable entry:\n%s", joined)
	}
	if rows := storedRows(t, dbPath, "slb"); len(rows) != 0 {
		t.Fatalf("an undated capture entry reached the store: %+v", rows)
	}
}
