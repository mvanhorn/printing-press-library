// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests. The wiring smoke test is kept; behaviour cases follow.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/cliutil/testenv"
)

// TestNovelBasketHelpWires smoke-tests that the basket command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBasketHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"basket", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("basket --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "basket", "--limit", "--db", "--record"} {
		if !strings.Contains(help, want) {
			t.Fatalf("basket --help missing %q in output:\n%s", want, help)
		}
	}
	// pflag does not synthesise negated booleans, so an example that shows
	// --no-record documents a flag that does not exist: it fails with
	// "unknown flag". The only way to turn the boolean off is --record=false.
	if strings.Contains(help, "--no-record") {
		t.Fatalf("basket --help advertises --no-record, which pflag never registers:\n%s", help)
	}
	if !strings.Contains(help, "--record=false") {
		t.Fatalf("basket --help does not show the working form --record=false:\n%s", help)
	}
}

func TestBasketBareInvocationPrintsHelp(t *testing.T) {
	out, err := runCLI(t, "basket")
	if err != nil {
		t.Fatalf("bare basket should print help and exit 0, got %v", err)
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("bare basket did not print usage:\n%s", out)
	}
}

// TestBasketDryRunDoesNotTouchTheFilesystem is the placement assertion: the
// dry-run guard runs BEFORE the list file is opened, so a dry run over a path
// that does not exist still exits 0.
func TestBasketDryRunDoesNotTouchTheFilesystem(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-not-here.txt")
	if _, err := os.Stat(missing); err == nil {
		t.Fatalf("fixture path %s unexpectedly exists", missing)
	}
	out, err := runCLI(t, "basket", missing, "--dry-run")
	if err != nil {
		t.Fatalf("basket <missing> --dry-run error = %v (the file read must come after dryRunOK)\n%s", err, out)
	}
	if exit := exitCodeFor(err); err != nil && exit != 0 {
		t.Fatalf("exit code = %d, want 0", exit)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("dry run did not report the skipped action:\n%s", out)
	}
	t.Logf("raw output: %s", strings.TrimSpace(out))
}

func TestBasketMissingFileIsUsageError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.txt")
	out, err := runCLI(t, "basket", missing, "--json")
	if err == nil {
		t.Fatalf("basket over a missing file should fail; output:\n%s", out)
	}
	if got := exitCodeFor(err); got != 2 {
		t.Fatalf("exit code = %d, want 2 (usage)", got)
	}
}

// ---------------------------------------------------------------------------
// Arbitrary-file-read guard.
//
// basket echoes every non-comment line of the file it is handed straight back
// to the caller, and it is registered as an MCP tool whose `path` argument
// comes from an untrusted caller. These cases pin each of the four gates.
// ---------------------------------------------------------------------------

// basketSecret is the marker every refused fixture contains. If it ever reaches
// stdout, the file was read and echoed.
const basketSecret = "SEKRIT-W-RCTX-VALUE"

func basketWriteFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("creating fixture dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

// TestBasketRefusesPathsItMustNotRead is the security case: a caller-supplied
// path that is not a plain shopping list is refused with exit 2 and its
// contents never appear in the output.
func TestBasketRefusesPathsItMustNotRead(t *testing.T) {
	// Redirect every user-directory lookup into a throwaway home so the
	// data-directory case is exercised against a real resolved data dir
	// without touching the operator's own.
	testenv.Isolate(t, cliutil.DataDir, cliutil.ConfigDir)
	dataDir, err := cliutil.DataDir()
	if err != nil {
		t.Fatalf("resolving data dir: %v", err)
	}

	tmp := t.TempDir()

	// A credential file exactly like the one this CLI stores.
	cookiePath := basketWriteFile(t, filepath.Join(tmp, "cookies.json"),
		`{"w-rctx":"`+basketSecret+`","wow-auth-token":"`+basketSecret+`"}`+"\n")
	// An allowlisted extension, but inside the CLI's own data directory: the
	// directory rule has to be what refuses this one, not the extension rule.
	inDataDir := basketWriteFile(t, filepath.Join(dataDir, "notes.txt"), basketSecret+"\n")
	// A DIRECTORY whose name ends in .txt, so it clears the extension gate.
	dirPath := filepath.Join(tmp, "not-a-file.txt")
	if err := os.MkdirAll(dirPath, 0o700); err != nil {
		t.Fatalf("creating directory fixture: %v", err)
	}
	// One byte over the 1 MiB cap.
	oversized := filepath.Join(tmp, "huge.txt")
	big := bytes.Repeat([]byte("milk\n"), (basketMaxListBytes/5)+1)
	if len(big) <= basketMaxListBytes {
		t.Fatalf("oversized fixture is %d bytes, not over the %d cap", len(big), basketMaxListBytes)
	}
	if err := os.WriteFile(oversized, big, 0o600); err != nil {
		t.Fatalf("writing oversized fixture: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "json credential file",
			path:    cookiePath,
			wantErr: "only .txt, .md, .list and .csv",
		},
		{
			name:    "allowlisted extension inside the CLI data directory",
			path:    inDataDir,
			wantErr: "resolves inside this CLI's own data directory",
		},
		{
			name:    "directory",
			path:    dirPath,
			wantErr: "it is a directory, not a shopping list",
		},
		{
			name:    "oversized file",
			path:    oversized,
			wantErr: "cap on a shopping list",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runCLI(t, "basket", tt.path, "--json", "--db", tempDBPath(t, "basket.db"))
			if err == nil {
				t.Fatalf("basket %s was accepted; output:\n%s", tt.path, out)
			}
			if code := exitCodeFor(err); code != 2 {
				t.Fatalf("exit code = %d, want 2 (usage); err: %v", code, err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
			if strings.Contains(out, basketSecret) || strings.Contains(err.Error(), basketSecret) {
				t.Fatalf("the refused file's contents leaked into the output:\n%s\n%v", out, err)
			}
			t.Logf("refused %s: %v", tt.name, err)
		})
	}

	// The control case: an ordinary .txt list outside the data directory is
	// still read and priced, so the guard did not simply break the command.
	t.Run("ordinary txt list still works", func(t *testing.T) {
		newWowFakeServer(t, &wowFakeServer{
			byTerm: map[string][]wowTestTile{
				"full cream milk": {
					{Stockcode: 901, Name: "Woolworths Full Cream Milk", DisplayName: "Woolworths Full Cream Milk 2L", Brand: "Woolworths", Price: 3.10, CupPrice: 1.55, CupMeasure: "1L", PackageSize: "2L", IsAvailable: true, IsInStock: true},
				},
			},
		})
		list := basketWriteFile(t, filepath.Join(t.TempDir(), "groceries.txt"), "# weekly\nfull cream milk\n")
		out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
		if err != nil {
			t.Fatalf("basket over an ordinary .txt list failed: %v\n%s", err, out)
		}
		env := decodeEnvelope(t, out)
		if got, _ := env["resolved_count"].(float64); got != 1 {
			t.Fatalf("resolved_count = %v, want 1", got)
		}
		if got, _ := env["total"].(float64); got != 3.10 {
			t.Fatalf("total = %v, want 3.10", got)
		}
		t.Logf("raw output:\n%s", out)
	})
}

// TestBasketTruncatesEchoedLineText proves an over-long line is clipped before
// it is echoed into Query, and clipped on a rune boundary rather than a byte
// one.
func TestBasketTruncatesEchoedLineText(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{byTerm: map[string][]wowTestTile{}})
	// 400 multi-byte runes: a byte slice would cut one in half.
	long := strings.Repeat("é", 400)
	list := basketWriteFile(t, filepath.Join(t.TempDir(), "long.txt"), long+"\n")

	out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	env := decodeEnvelope(t, out)
	lines, _ := env["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}
	line, _ := lines[0].(map[string]any)
	query, _ := line["query"].(string)
	if n := len([]rune(query)); n > basketMaxEchoRunes {
		t.Fatalf("echoed query is %d runes, over the %d cap", n, basketMaxEchoRunes)
	}
	if !strings.HasSuffix(query, "...") {
		t.Fatalf("echoed query %q was not marked as truncated", query)
	}
	if strings.ContainsRune(query, '�') {
		t.Fatalf("echoed query contains a replacement character, so a rune was cut in half: %q", query)
	}
	t.Logf("echoed query is %d runes: %q", len([]rune(query)), query)
}

// writeListFile writes a list fixture and returns its path.
func writeListFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("writing list fixture: %v", err)
	}
	return path
}

// TestBasketResolvesFourOfFiveLines is the resolution acceptance case: a
// five-item list where one line is deliberate nonsense. Exactly four resolve,
// the nonsense line appears as unresolved with a reason, and the total covers
// only the four.
func TestBasketResolvesFourOfFiveLines(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		byTerm: map[string][]wowTestTile{
			"full cream milk": {
				{Stockcode: 101, Name: "Woolworths Full Cream Milk", DisplayName: "Woolworths Full Cream Milk 2L", Brand: "Woolworths", Price: 3.10, CupPrice: 1.55, CupMeasure: "1L", PackageSize: "2L", IsAvailable: true, IsInStock: true},
			},
			"tim tam original": {
				{Stockcode: 102, Name: "Arnott's Tim Tam Original", DisplayName: "Arnott's Tim Tam Original Biscuits 200g", Brand: "Arnott's", Price: 6.00, CupPrice: 3.00, CupMeasure: "100G", PackageSize: "200g", IsOnSpecial: true, WasPrice: 7.00, IsAvailable: true, IsInStock: true},
			},
			"weet bix": {
				{Stockcode: 103, Name: "Weet-Bix Breakfast Cereal", DisplayName: "Weet-Bix Breakfast Cereal 1.2kg", Brand: "Weet-Bix", Price: 7.00, CupPrice: 0.58, CupMeasure: "100G", PackageSize: "1.2kg", IsAvailable: true, IsInStock: true},
			},
			"cadbury dairy milk": {
				{Stockcode: 104, Name: "Cadbury Dairy Milk Chocolate Bar", DisplayName: "Cadbury Dairy Milk Chocolate Bar 180g", Brand: "Cadbury", Price: 5.50, CupPrice: 3.06, CupMeasure: "100G", PackageSize: "180g", IsAvailable: true, IsInStock: true},
			},
		},
	})

	list := writeListFile(t,
		"# weekly shop",
		"full cream milk",
		"",
		"tim tam original",
		"weet bix",
		"cadbury dairy milk",
		"flurbleglop zzqxwv",
	)

	out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	if got, _ := env["resolved_count"].(float64); got != 4 {
		t.Fatalf("resolved_count = %v, want 4", got)
	}
	if got, _ := env["unresolved_count"].(float64); got != 1 {
		t.Fatalf("unresolved_count = %v, want 1", got)
	}
	// 3.10 + 6.00 + 7.00 + 5.50 = 21.60. A $0.00-padded unresolved line would
	// not change this sum, so the count assertions above carry that half; this
	// asserts the arithmetic is over exactly the resolved four.
	if got, _ := env["total"].(float64); got != 21.60 {
		t.Fatalf("total = %v, want 21.60", got)
	}

	lines, _ := env["lines"].([]any)
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5 (blank and comment lines are skipped, not dropped from the count)", len(lines))
	}
	unresolved := 0
	for _, raw := range lines {
		line, _ := raw.(map[string]any)
		resolved, _ := line["resolved"].(bool)
		if resolved {
			if price, _ := line["price"].(float64); price <= 0 {
				t.Fatalf("resolved line %v has price %v", line["query"], price)
			}
			continue
		}
		unresolved++
		if line["query"] != "flurbleglop zzqxwv" {
			t.Fatalf("unexpected unresolved line %v", line["query"])
		}
		reason, _ := line["reason"].(string)
		if strings.TrimSpace(reason) == "" {
			t.Fatalf("unresolved line carried no reason")
		}
		if _, hasPrice := line["price"]; hasPrice {
			t.Fatalf("unresolved line carried a price field; it must contribute nothing at all")
		}
		t.Logf("unresolved line %v: %s", line["query"], reason)
	}
	if unresolved != 1 {
		t.Fatalf("found %d unresolved lines in the rows, want 1", unresolved)
	}
	if got, _ := env["lines_skipped_blank_or_comment"].(float64); got != 2 {
		t.Fatalf("lines_skipped_blank_or_comment = %v, want 2", got)
	}
	basis, _ := env["total_basis"].(string)
	if !strings.Contains(basis, "4 of 5") {
		t.Fatalf("total_basis = %q, want it to name the real denominator", basis)
	}
}

// TestBasketRejectsPartialNonsense guards the matcher against the API's habit
// of ignoring terms it cannot match: "blorptig cheese" returns ordinary cheese,
// which must NOT be priced into the basket as though it were what was asked for.
func TestBasketRejectsPartialNonsense(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		byTerm: map[string][]wowTestTile{
			// The server "helpfully" returns cheese for a query it only
			// partially understood, exactly as the live endpoint does.
			"blorptig cheese": {
				{Stockcode: 201, Name: "Woolworths Tasty Cheese Block", DisplayName: "Woolworths Tasty Cheese Block 500g", Brand: "Woolworths", Price: 9, CupPrice: 1.8, CupMeasure: "100G", PackageSize: "500g", IsAvailable: true, IsInStock: true},
			},
		},
	})

	list := writeListFile(t, "blorptig cheese")
	out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	if got, _ := env["resolved_count"].(float64); got != 0 {
		t.Fatalf("resolved_count = %v, want 0; 'blorptig' matches nothing in the returned product, so the line must not resolve to plain cheese", got)
	}
	if got, _ := env["total"].(float64); got != 0 {
		t.Fatalf("total = %v, want 0", got)
	}
	lines, _ := env["lines"].([]any)
	line, _ := lines[0].(map[string]any)
	reason, _ := line["reason"].(string)
	if !strings.Contains(reason, "matched every word") {
		t.Fatalf("reason = %q, want it to explain that not every word matched", reason)
	}
	note, _ := env["note"].(string)
	if !strings.Contains(note, "not because the shopping is free") {
		t.Fatalf("note = %q, want it to say the zero total means nothing was priced", note)
	}
}

// TestBasketFetchFailureNeverBecomesAZeroRow is the partial-failure assertion:
// a line whose lookup failed is excluded from the total, flagged, and counted
// in fetch_failures rather than quietly priced at $0.00.
func TestBasketFetchFailureNeverBecomesAZeroRow(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		byTerm: map[string][]wowTestTile{
			"full cream milk": {
				{Stockcode: 301, Name: "Woolworths Full Cream Milk", DisplayName: "Woolworths Full Cream Milk 2L", Brand: "Woolworths", Price: 3.10, CupPrice: 1.55, CupMeasure: "1L", PackageSize: "2L", IsAvailable: true, IsInStock: true},
			},
			"white bread": {
				{Stockcode: 302, Name: "Woolworths White Bread", DisplayName: "Woolworths White Bread 700g", Brand: "Woolworths", Price: 2.50, CupPrice: 0.36, CupMeasure: "100G", PackageSize: "700g", IsAvailable: true, IsInStock: true},
			},
		},
		failTerms: map[string]bool{"white bread": true},
	})

	list := writeListFile(t, "full cream milk", "white bread")
	out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	failures, _ := env["fetch_failures"].([]any)
	if len(failures) != 1 {
		t.Fatalf("fetch_failures = %d, want 1", len(failures))
	}
	if got, _ := env["resolved_count"].(float64); got != 1 {
		t.Fatalf("resolved_count = %v, want 1", got)
	}
	if got, _ := env["total"].(float64); got != 3.10 {
		t.Fatalf("total = %v, want 3.10; the failed lookup must not enter the total at $0.00", got)
	}
	lines, _ := env["lines"].([]any)
	failed, _ := lines[1].(map[string]any)
	if fetchFailed, _ := failed["fetch_failed"].(bool); !fetchFailed {
		t.Fatalf("the failed line is not marked fetch_failed: %v", failed)
	}
	if reason, _ := failed["reason"].(string); !strings.Contains(reason, "lookup failed") {
		t.Fatalf("failed line reason = %q, want it to name the lookup failure", reason)
	}
	basis, _ := env["total_basis"].(string)
	if !strings.Contains(basis, "1 of 2") {
		t.Fatalf("total_basis = %q, want the real denominator", basis)
	}
}

// TestBasketEmptyListIsValidJSON covers the machine-caller contract on a list
// with nothing in it: valid JSON, [] lines, and a note.
func TestBasketEmptyListIsValidJSON(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{byTerm: map[string][]wowTestTile{}})

	list := writeListFile(t, "# nothing but a comment", "")
	out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)
	requireEmptyJSONArray(t, env, "lines")
	requireEmptyJSONArray(t, env, "fetch_failures")
	if note, _ := env["note"].(string); !strings.Contains(note, "no item lines") {
		t.Fatalf("note = %q, want an explanation of the empty list", note)
	}
}

// TestBasketReportsDeltaAgainstPreviousRun proves the previous-total
// comparison is real: the same list is priced twice against a moving price and
// the second run reports the movement.
func TestBasketReportsDeltaAgainstPreviousRun(t *testing.T) {
	srv := newWowFakeServer(t, &wowFakeServer{
		byTerm: map[string][]wowTestTile{
			"full cream milk": {
				{Stockcode: 401, Name: "Woolworths Full Cream Milk", DisplayName: "Woolworths Full Cream Milk 2L", Brand: "Woolworths", Price: 3.00, CupPrice: 1.5, CupMeasure: "1L", PackageSize: "2L", IsAvailable: true, IsInStock: true},
			},
		},
	})
	db := tempDBPath(t, "basket.db")
	list := writeListFile(t, "full cream milk")

	first, err := runCLI(t, "basket", list, "--json", "--db", db)
	if err != nil {
		t.Fatalf("first run error = %v\n%s", err, first)
	}
	t.Logf("run 1 raw output:\n%s", first)
	firstEnv := decodeEnvelope(t, first)
	if recorded, _ := firstEnv["recorded"].(bool); !recorded {
		t.Fatalf("first run did not record its total; the delta can never appear")
	}
	if firstEnv["previous_run"] != nil {
		t.Fatalf("first run should have no previous run, got %v", firstEnv["previous_run"])
	}

	// The shelf price moves between runs.
	srv.mu.Lock()
	srv.byTerm["full cream milk"][0].Price = 3.50
	srv.mu.Unlock()

	second, err := runCLI(t, "basket", list, "--json", "--db", db)
	if err != nil {
		t.Fatalf("second run error = %v\n%s", err, second)
	}
	t.Logf("run 2 raw output:\n%s", second)
	secondEnv := decodeEnvelope(t, second)
	prev, _ := secondEnv["previous_run"].(map[string]any)
	if prev == nil {
		t.Fatalf("second run reported no previous run")
	}
	if got, _ := prev["total"].(float64); got != 3.00 {
		t.Fatalf("previous_run.total = %v, want 3.00", got)
	}
	if got, _ := secondEnv["total"].(float64); got != 3.50 {
		t.Fatalf("total = %v, want 3.50", got)
	}
	if got, _ := secondEnv["total_delta_vs_previous"].(float64); got != 0.50 {
		t.Fatalf("total_delta_vs_previous = %v, want 0.50", got)
	}
}

// TestBasketNoRecordLeavesNoDatabase proves --record=false is honest: it does
// not create a database as a side effect of pricing a list.
func TestBasketNoRecordLeavesNoDatabase(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		byTerm: map[string][]wowTestTile{
			"full cream milk": {
				{Stockcode: 501, Name: "Woolworths Full Cream Milk", DisplayName: "Woolworths Full Cream Milk 2L", Brand: "Woolworths", Price: 3.00, CupPrice: 1.5, CupMeasure: "1L", PackageSize: "2L", IsAvailable: true, IsInStock: true},
			},
		},
	})
	db := tempDBPath(t, "basket.db")
	list := writeListFile(t, "full cream milk")

	out, err := runCLI(t, "basket", list, "--json", "--db", db, "--record=false")
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	env := decodeEnvelope(t, out)
	if recorded, _ := env["recorded"].(bool); recorded {
		t.Fatalf("--record=false still recorded the run")
	}
	if _, statErr := os.Stat(db); statErr == nil {
		t.Fatalf("--record=false created %s", db)
	}
	t.Logf("recorded=%v history_available=%v, database not created", env["recorded"], env["history_available"])
}

func TestBasketLimitCapsLinesRead(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{byTerm: map[string][]wowTestTile{}})
	list := writeListFile(t, "one", "two", "three", "four")

	out, err := runCLI(t, "basket", list, "--json", "--limit", "2", "--db", tempDBPath(t, "basket.db"))
	if err != nil {
		t.Fatalf("basket error = %v\n%s", err, out)
	}
	env := decodeEnvelope(t, out)
	if got, _ := env["lines_read"].(float64); got != 2 {
		t.Fatalf("lines_read = %v, want 2", got)
	}
}

func TestBasketTokenMatching(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		want      bool
	}{
		{"exact words", "tim tam", "Arnott's Tim Tam Original 200g", true},
		{"plural tolerated", "egg", "Free Range Eggs 12 pack", true},
		{"singular tolerated", "eggs", "Free Range Egg Carton", true},
		{"stopwords ignored", "box of tissues", "Kleenex Tissues Box 200 sheets", true},
		{"missing word rejected", "blorptig cheese", "Woolworths Tasty Cheese Block 500g", false},
		{"short junk token rejected", "zz cheese", "Woolworths Tasty Cheese Block 500g", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := basketTokens(tt.candidate)
			all := true
			for _, qt := range basketTokens(tt.query) {
				if ok, _ := basketTokenMatch(qt, candidate); !ok {
					all = false
					break
				}
			}
			if all != tt.want {
				t.Fatalf("matching %q against %q = %v, want %v", tt.query, tt.candidate, all, tt.want)
			}
		})
	}
}

func TestBasketMedian(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want float64
	}{
		{"single", []float64{5}, 5},
		{"odd", []float64{3, 1, 2}, 2},
		{"even", []float64{4, 1, 3, 2}, 2.5},
		{"outlier does not drag", []float64{5, 5, 5, 5, 1}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := basketMedian(tt.in); got != tt.want {
				t.Fatalf("basketMedian(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestBasketListKeyIsContentAddressed(t *testing.T) {
	a := basketListKey([]basketItem{{line: 1, text: "milk"}, {line: 2, text: "bread"}})
	b := basketListKey([]basketItem{{line: 9, text: "MILK"}, {line: 10, text: "Bread"}})
	c := basketListKey([]basketItem{{line: 1, text: "milk"}})
	if a != b {
		t.Fatalf("the same items at different line numbers produced different keys: %s vs %s", a, b)
	}
	if a == c {
		t.Fatalf("different lists produced the same key %s; totals would be compared across unrelated baskets", a)
	}
}

// TestBasketLiveFiveLineList is the live resolution acceptance case: four
// genuine grocery lines plus one deliberate nonsense line.
func TestBasketLiveFiveLineList(t *testing.T) {
	if !liveTestsEnabled(t) {
		return
	}
	list := writeListFile(t,
		"# live acceptance list",
		"full cream milk",
		"tim tam original",
		"weet bix",
		"cadbury dairy milk",
		"flurbleglop zzqxwv",
	)

	out, err := runCLI(t, "basket", list, "--json", "--db", tempDBPath(t, "basket.db"))
	skipIfUnreachable(t, err, out)
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	if got, _ := env["resolved_count"].(float64); got != 4 {
		t.Fatalf("resolved_count = %v, want exactly 4", got)
	}
	if got, _ := env["unresolved_count"].(float64); got != 1 {
		t.Fatalf("unresolved_count = %v, want exactly 1", got)
	}
	total, _ := env["total"].(float64)
	if total <= 0 {
		t.Fatalf("total = %v, want a real basket total", total)
	}

	sum := 0.0
	for _, raw := range env["lines"].([]any) {
		line, _ := raw.(map[string]any)
		if resolved, _ := line["resolved"].(bool); !resolved {
			if line["query"] != "flurbleglop zzqxwv" {
				t.Fatalf("unexpected unresolved line %v", line["query"])
			}
			reason, _ := line["reason"].(string)
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("unresolved line has no reason")
			}
			t.Logf("unresolved: %v -> %s", line["query"], reason)
			continue
		}
		price, _ := line["price"].(float64)
		if price <= 0 {
			t.Fatalf("resolved line %v priced at %v", line["query"], price)
		}
		sum += price
		t.Logf("resolved: %-22v -> %-55v $%.2f", line["query"], line["name"], price)
	}
	if diff := sum - total; diff > 0.011 || diff < -0.011 {
		t.Fatalf("reported total %v does not equal the sum of resolved line prices %v", total, sum)
	}
	t.Logf("total=%.2f basis=%v", total, env["total_basis"])
}
