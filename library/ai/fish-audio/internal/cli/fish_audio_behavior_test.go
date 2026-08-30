// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the hand-written Fish Audio commands. Everything here
// runs against the local store or fails before any network call, so the suite
// needs no credentials.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
)

// runCLI executes one command line against a fresh root and returns stdout,
// stderr, and the error the command returned.
func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errBuf bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

// seedRenderLog builds a temporary database with two render rows and returns
// its path.
func seedRenderLog(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "renders.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening the temp store: %v", err)
	}
	defer db.Close()
	if err := db.EnsureRenderLog(context.Background()); err != nil {
		t.Fatalf("EnsureRenderLog: %v", err)
	}
	rows := []store.RenderRow{
		{
			CreatedAt: "2026-08-01T10:00:00Z", RequestHash: "hash-one", Text: "first line",
			Model: "s2.1-pro", VoiceID: "voice-a", Format: "mp3",
			BytesIn: 100, BytesOut: 4000, CostUSD: 0.0015, CostUSDPaidEquiv: 0.0015,
			FilePath: "/tmp/one.mp3", FileSHA256: "aaa", Source: "tts render",
		},
		{
			CreatedAt: "2026-08-02T10:00:00Z", RequestHash: "hash-two", Text: "second line",
			Model: "s2.1-pro-free", VoiceID: "voice-b", Format: "wav",
			BytesIn: 300, BytesOut: 9000, CostUSD: 0, CostUSDPaidEquiv: 0.0045,
			FilePath: "/tmp/two.wav", FileSHA256: "bbb", Source: "tts batch",
		},
	}
	for _, row := range rows {
		if _, err := db.InsertRenderRow(context.Background(), row); err != nil {
			t.Fatalf("InsertRenderRow: %v", err)
		}
	}
	return dbPath
}

// TestRenderLogEmptyDatabasePrintsEmptyArray is the missing-mirror contract: a
// machine caller must receive a valid empty result, not a SQLite open failure.
func TestRenderLogEmptyDatabasePrintsEmptyArray(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.db")
	out, errOut, err := runCLI(t, "render", "log", "--json", "--db", missing)
	if err != nil {
		t.Fatalf("render log on a missing database returned %v, want a clean empty result", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("stdout = %q, want []", out)
	}
	if !strings.Contains(errOut, "no local render log") {
		t.Fatalf("stderr = %q, want the missing-mirror hint", errOut)
	}
}

func TestRenderLogListsSeededRows(t *testing.T) {
	dbPath := seedRenderLog(t)
	out, _, err := runCLI(t, "render", "log", "--json", "--db", dbPath)
	if err != nil {
		t.Fatalf("render log returned %v", err)
	}
	var rows []store.RenderRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("render log did not print a JSON array: %v\n%s", err, out)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	// Newest first.
	if rows[0].RequestHash != "hash-two" {
		t.Fatalf("first row = %+v, want the newest render", rows[0])
	}
}

func TestRenderLogFiltersByVoice(t *testing.T) {
	dbPath := seedRenderLog(t)
	out, _, err := runCLI(t, "render", "log", "--json", "--db", dbPath, "--voice", "voice-a")
	if err != nil {
		t.Fatalf("render log returned %v", err)
	}
	var rows []store.RenderRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("parsing output: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0].VoiceID != "voice-a" {
		t.Fatalf("--voice filter returned %+v, want only voice-a", rows)
	}

	// Negative case: a voice that rendered nothing must return nothing rather
	// than falling back to the unfiltered list.
	out, _, err = runCLI(t, "render", "log", "--json", "--db", dbPath, "--voice", "voice-does-not-exist")
	if err != nil {
		t.Fatalf("render log returned %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("a non-matching --voice returned %q, want []", out)
	}
}

func TestRenderSpendGroupsByModel(t *testing.T) {
	dbPath := seedRenderLog(t)
	out, _, err := runCLI(t, "render", "spend", "--group-by", "model", "--json", "--db", dbPath)
	if err != nil {
		t.Fatalf("render spend returned %v", err)
	}
	var rows []store.SpendRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("parsing output: %v\n%s", err, out)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d groups, want 2: %s", len(rows), out)
	}
	byModel := map[string]store.SpendRow{}
	for _, row := range rows {
		byModel[row.Group] = row
	}
	paid := byModel["s2.1-pro"]
	if paid.Renders != 1 || paid.BytesIn != 100 || paid.CostUSD != 0.0015 {
		t.Fatalf("s2.1-pro group = %+v, want 1 render, 100 bytes, 0.0015", paid)
	}
	free := byModel["s2.1-pro-free"]
	if free.CostUSD != 0 {
		t.Fatalf("free-tier billed cost = %v, want 0", free.CostUSD)
	}
	if free.CostUSDPaidEquiv != 0.0045 {
		t.Fatalf("free-tier paid equivalent = %v, want 0.0045", free.CostUSDPaidEquiv)
	}
}

func TestRenderSpendRejectsUnknownGroup(t *testing.T) {
	dbPath := seedRenderLog(t)
	_, _, err := runCLI(t, "render", "spend", "--group-by", "wizard", "--json", "--db", dbPath)
	if err == nil {
		t.Fatal("render spend accepted an unknown --group-by")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRenderDiffShowsDeltas(t *testing.T) {
	dbPath := seedRenderLog(t)
	out, _, err := runCLI(t, "render", "diff", "1", "2", "--json", "--db", dbPath)
	if err != nil {
		t.Fatalf("render diff returned %v", err)
	}
	var view renderDiffView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("parsing output: %v\n%s", err, out)
	}
	if view.LeftID != 1 || view.RightID != 2 {
		t.Fatalf("diff ids = %d, %d; want 1, 2", view.LeftID, view.RightID)
	}
	fields := map[string]renderDiffField{}
	for _, f := range view.Fields {
		fields[f.Field] = f
	}
	if got := fields["bytes_in"]; got.Delta != "+200" || !got.Changed {
		t.Fatalf("bytes_in field = %+v, want a +200 delta", got)
	}
	if got := fields["model"]; !got.Changed || got.Left != "s2.1-pro" || got.Right != "s2.1-pro-free" {
		t.Fatalf("model field = %+v", got)
	}
	if got := fields["cost_usd"]; got.Delta != "-0.001500" {
		t.Fatalf("cost_usd delta = %q, want -0.001500", got.Delta)
	}
	if view.Changed == 0 {
		t.Fatal("diff reported no changed fields between two different renders")
	}
}

// TestRenderDiffIdenticalRowsReportNoChange is the absence-of-correctness
// case: comparing a row with itself must not invent a difference.
func TestRenderDiffIdenticalRowsReportNoChange(t *testing.T) {
	dbPath := seedRenderLog(t)
	out, _, err := runCLI(t, "render", "diff", "1", "1", "--json", "--db", dbPath)
	if err != nil {
		t.Fatalf("render diff returned %v", err)
	}
	var view renderDiffView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("parsing output: %v\n%s", err, out)
	}
	if view.Changed != 0 {
		t.Fatalf("comparing a row with itself reported %d changed fields", view.Changed)
	}
}

func TestRenderDiffMissingRowIsEmptyResult(t *testing.T) {
	dbPath := seedRenderLog(t)
	out, errOut, err := runCLI(t, "render", "diff", "1", "99", "--json", "--db", dbPath)
	if err != nil {
		t.Fatalf("render diff on a missing row should be an empty result, got %v", err)
	}
	if !strings.Contains(out, `"fields": []`) && !strings.Contains(out, `"fields":[]`) {
		t.Fatalf("stdout = %q, want an empty fields list", out)
	}
	if !strings.Contains(errOut, "render log has no row 99") {
		t.Fatalf("stderr = %q, want it to name the missing row", errOut)
	}
}

func TestRenderDiffPartialPositionalsExitTwo(t *testing.T) {
	_, _, err := runCLI(t, "render", "diff", "1")
	if err == nil {
		t.Fatal("render diff accepted one positional argument")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// TestTtsBatchDialogueRejectsS1 is the negative test for the s2-family gate.
// It must fail before the input file is opened, so a non-existent --input is
// deliberate here.
func TestTtsBatchDialogueRejectsS1(t *testing.T) {
	_, _, err := runCLI(t, "tts", "batch", "--input", "x", "--voice", "v", "--out-dir", "d", "--model", "s1", "--dialogue")
	if err == nil {
		t.Fatal("tts batch accepted --dialogue with --model s1")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(err.Error(), "s2-family") {
		t.Fatalf("error = %v, want it to name the s2-family requirement", err)
	}
	if _, statErr := os.Stat("d"); statErr == nil {
		t.Fatal("tts batch created the output directory before rejecting the model")
	}
}

func TestTtsBatchRejectsUnknownModel(t *testing.T) {
	_, _, err := runCLI(t, "tts", "batch", "--input", "x", "--voice", "v", "--out-dir", "d", "--model", "s2.1pro")
	if err == nil {
		t.Fatal("tts batch accepted --model s2.1pro")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestTtsRenderDryRunEmitsEnvelope(t *testing.T) {
	out, _, err := runCLI(t, "tts", "render", "--text", "hi", "--voice", "abc", "--out", filepath.Join(t.TempDir(), "x.mp3"), "--dry-run", "--json")
	if err != nil {
		t.Fatalf("tts render --dry-run returned %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("dry run did not print JSON: %v\n%s", err, out)
	}
	if envelope["dry_run"] != true {
		t.Fatalf("dry_run envelope = %v", envelope)
	}
	if envelope["action"] != "tts render" {
		t.Fatalf("action = %v, want \"tts render\"", envelope["action"])
	}
}

// TestNovelCommandsDryRunCleanly covers the verify probe: every hand-written
// command must short-circuit on --dry-run at exit 0 with a parseable envelope.
func TestNovelCommandsDryRunCleanly(t *testing.T) {
	commands := [][]string{
		{"tts", "render"},
		{"tts", "batch"},
		{"tts", "resolve"},
		{"render", "log"},
		{"render", "spend"},
		{"voice", "clone"},
		{"voice", "design"},
		{"voice", "design-save"},
		{"voice", "discover"},
		{"voice", "verify", "example-model-id"},
		{"wallet", "balance"},
		{"asr", "transcribe"},
	}
	for _, argv := range commands {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			args := append(append([]string{}, argv...), "--dry-run", "--json")
			out, _, err := runCLI(t, args...)
			if err != nil {
				t.Fatalf("%v --dry-run returned %v", argv, err)
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatalf("%v --dry-run did not print JSON: %v\n%s", argv, err, out)
			}
			if envelope["dry_run"] != true {
				t.Fatalf("%v --dry-run envelope = %v", argv, envelope)
			}
		})
	}
}

// TestParentShortsDoNotLeakGroupNames guards the cosmetic fix: the generated
// parents carried their capability-group labels into user-facing help.
func TestParentShortsDoNotLeakGroupNames(t *testing.T) {
	root := RootCmd()
	want := map[string]string{
		"render": "Local render history, spend, and diffs",
		"voice":  "Clone, design, discover, and verify voices",
	}
	seen := map[string]bool{}
	for _, cmd := range root.Commands() {
		if expected, ok := want[cmd.Name()]; ok {
			seen[cmd.Name()] = true
			if cmd.Short != expected {
				t.Fatalf("%s Short = %q, want %q", cmd.Name(), cmd.Short, expected)
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Fatalf("command group %q is not wired onto the root", name)
		}
	}
}

func TestFishSince(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{in: "", wantErr: false},
		{in: "30d", wantErr: false},
		{in: "12h", wantErr: false},
		{in: "2026-08-01", wantErr: false},
		{in: "yesterday", wantErr: true},
	}
	for _, tc := range cases {
		got, err := fishSince(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("fishSince(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("fishSince(%q) error = %v", tc.in, err)
		}
		if tc.in != "" && got == "" {
			t.Fatalf("fishSince(%q) returned an empty bound", tc.in)
		}
	}
}

func TestReadTextInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.txt")
	if err := os.WriteFile(path, []byte("from a file"), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	if got, err := readTextInput("inline", ""); err != nil || got != "inline" {
		t.Fatalf("readTextInput(inline) = %q, %v", got, err)
	}
	if got, err := readTextInput("", path); err != nil || got != "from a file" {
		t.Fatalf("readTextInput(file) = %q, %v", got, err)
	}
	if _, err := readTextInput("inline", path); err == nil {
		t.Fatal("readTextInput accepted both --text and --text-file")
	}
	if _, err := readTextInput("", ""); err == nil {
		t.Fatal("readTextInput accepted neither --text nor --text-file")
	}
}

func TestParseCreditValue(t *testing.T) {
	cases := []struct {
		raw     string
		want    float64
		wantErr bool
	}{
		{raw: `"12.34"`, want: 12.34},
		{raw: `12.34`, want: 12.34},
		{raw: `null`, wantErr: true},
		{raw: `{"a":1}`, wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseCreditValue(json.RawMessage(tc.raw))
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseCreditValue(%s) = %v, want an error", tc.raw, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("parseCreditValue(%s) = %v, %v; want %v", tc.raw, got, err, tc.want)
		}
	}
}

func TestBuildMultipartRepeatsFieldNames(t *testing.T) {
	body, contentType, err := buildMultipart(
		[]multipartField{{Name: "tags", Value: "one"}, {Name: "tags", Value: "two"}},
		[]multipartFile{{Name: "voices", FileName: "a.wav", Content: []byte("audio")}},
	)
	if err != nil {
		t.Fatalf("buildMultipart error = %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Fatalf("content type = %q", contentType)
	}
	text := string(body)
	if strings.Count(text, `name="tags"`) != 2 {
		t.Fatalf("repeated field name was collapsed:\n%s", text)
	}
	if !strings.Contains(text, `filename="a.wav"`) {
		t.Fatalf("file part missing:\n%s", text)
	}
}

func TestVoiceCatalogSearchRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "voices.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening the temp store: %v", err)
	}
	defer db.Close()
	if err := db.EnsureVoiceCatalog(context.Background()); err != nil {
		t.Fatalf("EnsureVoiceCatalog: %v", err)
	}
	written, err := db.UpsertVoiceCatalog(context.Background(), []store.VoiceCatalogRow{
		{ID: "v1", Title: "Warm narrator", Description: "calm documentary read", Tags: "narration calm", Source: "public"},
		{ID: "v2", Title: "Bright concierge", Description: "upbeat front desk", Tags: "service", Source: "self"},
	})
	if err != nil || written != 2 {
		t.Fatalf("UpsertVoiceCatalog = %d, %v", written, err)
	}
	results, err := db.SearchVoiceCatalog(context.Background(), "narrator", "all", 10)
	if err != nil {
		t.Fatalf("SearchVoiceCatalog: %v", err)
	}
	if len(results) != 1 || results[0].ID != "v1" {
		t.Fatalf("search returned %+v, want only v1", results)
	}
	// Negative case: a query that matches nothing must return nothing.
	results, err = db.SearchVoiceCatalog(context.Background(), "helicopter", "all", 10)
	if err != nil {
		t.Fatalf("SearchVoiceCatalog: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("a non-matching query returned %+v", results)
	}
	// Source filter.
	results, err = db.SearchVoiceCatalog(context.Background(), "", "self", 10)
	if err != nil {
		t.Fatalf("SearchVoiceCatalog: %v", err)
	}
	if len(results) != 1 || results[0].ID != "v2" {
		t.Fatalf("--source self returned %+v, want only v2", results)
	}
}

func TestRenderLogSkipIfRenderedLookup(t *testing.T) {
	dbPath := seedRenderLog(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening the temp store: %v", err)
	}
	defer db.Close()
	row, err := db.RenderRowByHash(context.Background(), "hash-one")
	if err != nil {
		t.Fatalf("RenderRowByHash: %v", err)
	}
	if row == nil || row.Text != "first line" {
		t.Fatalf("RenderRowByHash returned %+v", row)
	}
	missing, err := db.RenderRowByHash(context.Background(), "no-such-hash")
	if err != nil {
		t.Fatalf("RenderRowByHash: %v", err)
	}
	if missing != nil {
		t.Fatalf("RenderRowByHash returned %+v for an unknown hash, want nil", missing)
	}
}

// TestRenderLogInsertIsIdempotentOnHash keeps a retried render from growing
// the log by one row per attempt.
func TestRenderLogInsertIsIdempotentOnHash(t *testing.T) {
	dbPath := seedRenderLog(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening the temp store: %v", err)
	}
	defer db.Close()
	id, err := db.InsertRenderRow(context.Background(), store.RenderRow{
		RequestHash: "hash-one", Text: "first line", Model: "s2.1-pro", VoiceID: "voice-a",
		Format: "mp3", BytesIn: 100, BytesOut: 4100, CostUSD: 0.0015, CostUSDPaidEquiv: 0.0015,
	})
	if err != nil {
		t.Fatalf("InsertRenderRow: %v", err)
	}
	if id != 1 {
		t.Fatalf("re-inserting the same hash produced id %d, want the existing row 1", id)
	}
	rows, err := db.ListRenderRows(context.Background(), store.RenderLogFilter{})
	if err != nil {
		t.Fatalf("ListRenderRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("log has %d rows after a repeat insert, want 2", len(rows))
	}
}

func TestClassifyRawAPIErrorMapsStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{status: 429, want: 7},
		{status: 401, want: 4},
		{status: 403, want: 4},
		{status: 404, want: 3},
		{status: 500, want: 5},
	}
	for _, tc := range cases {
		err := classifyRawAPIError(&client.APIError{Method: "POST", Path: "/v1/tts", StatusCode: tc.status})
		if got := ExitCode(err); got != tc.want {
			t.Fatalf("status %d mapped to exit %d, want %d", tc.status, got, tc.want)
		}
	}
}

// --- Phase 4.95 regression tests ---

// TestTtsRenderRequiresOneVoiceSource covers the zero-shot path that was
// unreachable: --voice was gated as required, so --reference-audio alone could
// never run, and passing both put two competing voice sources in one body.
func TestTtsRenderRequiresOneVoiceSource(t *testing.T) {
	dir := t.TempDir()
	ref := filepath.Join(dir, "sample.wav")
	if err := os.WriteFile(ref, []byte("RIFF____WAVE"), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	out := filepath.Join(dir, "x.mp3")

	t.Run("neither is a usage error", func(t *testing.T) {
		_, _, err := runCLI(t, "tts", "render", "--text", "hi", "--out", out)
		if err == nil {
			t.Fatal("tts render accepted neither --voice nor --reference-audio")
		}
		if code := ExitCode(err); code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if !strings.Contains(err.Error(), "--voice or --reference-audio is required") {
			t.Fatalf("error = %v, want it to name both alternatives", err)
		}
	})

	t.Run("both together is a usage error", func(t *testing.T) {
		_, _, err := runCLI(t, "tts", "render", "--text", "hi", "--voice", "abc", "--reference-audio", ref, "--out", out)
		if err == nil {
			t.Fatal("tts render accepted --voice and --reference-audio together")
		}
		if code := ExitCode(err); code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if !strings.Contains(err.Error(), "not both") {
			t.Fatalf("error = %v, want it to say the two flags are exclusive", err)
		}
	})
}

// TestZeroShotRequestDropsReferenceID is the wire-level half of the same fix:
// a zero-shot body must carry references and no reference_id.
func TestZeroShotRequestDropsReferenceID(t *testing.T) {
	req := fishaudio.RenderRequest{
		Text: "hello", Format: "mp3", VoiceID: "",
		ReferenceAudio: []byte{0x01, 0x02}, ReferenceText: "sample",
	}
	body := req.Body()
	if _, present := body["reference_id"]; present {
		t.Fatalf("a zero-shot body carried reference_id: %v", body)
	}
	refs, ok := body["references"].([]map[string]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("body[references] = %v, want one inline reference", body["references"])
	}
	if !req.NeedsMsgpack() {
		t.Fatal("a zero-shot request must encode as MessagePack")
	}
}

// TestSkipIfRenderedProducesTheRequestedFile is the contract exit 0 has to
// mean: a reused render still leaves the audio at --out.
func TestSkipIfRenderedProducesTheRequestedFile(t *testing.T) {
	dir := t.TempDir()
	priorPath := filepath.Join(dir, "prior.mp3")
	priorAudio := []byte("prior audio bytes")
	if err := os.WriteFile(priorPath, priorAudio, 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	outPath := filepath.Join(dir, "wanted.mp3")

	prior := &store.RenderRow{
		ID: 7, FilePath: priorPath, Model: "s2.1-pro", VoiceID: "voice-a",
		Format: "mp3", BytesIn: 10, BytesOut: int64(len(priorAudio)),
		CostUSD: 0.00015, CostUSDPaidEquiv: 0.00015,
	}
	manifest, err := reusePriorRender(prior, outPath)
	if err != nil {
		t.Fatalf("reusePriorRender error = %v", err)
	}
	if manifest == nil {
		t.Fatal("reusePriorRender returned no manifest for a file that exists")
	}
	if manifest.File != outPath {
		t.Fatalf("manifest.File = %q, want %q", manifest.File, outPath)
	}
	if !manifest.Skipped {
		t.Fatal("a reused render must report skipped = true")
	}
	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("--out was never created: %v", err)
	}
	if string(written) != string(priorAudio) {
		t.Fatalf("--out holds %q, want the prior audio", written)
	}

	t.Run("a prior row whose file is gone falls through to a real render", func(t *testing.T) {
		missing := &store.RenderRow{ID: 8, FilePath: filepath.Join(dir, "gone.mp3")}
		got, reuseErr := reusePriorRender(missing, filepath.Join(dir, "other.mp3"))
		if reuseErr != nil {
			t.Fatalf("reusePriorRender error = %v, want a nil-nil fall-through", reuseErr)
		}
		if got != nil {
			t.Fatalf("reusePriorRender returned %+v for a missing file, want nil", got)
		}
	})

	t.Run("a prior row already at --out is returned as-is", func(t *testing.T) {
		got, reuseErr := reusePriorRender(prior, priorPath)
		if reuseErr != nil || got == nil {
			t.Fatalf("reusePriorRender = %+v, %v", got, reuseErr)
		}
		if got.File != priorPath || !got.Skipped {
			t.Fatalf("manifest = %+v", got)
		}
	})
}

// TestDedupeBatchUnitsGroupsIdenticalRequests is the spend-accuracy fix:
// duplicate lines must become one API call feeding several output files, not
// one render_log row that swallows the rest.
func TestDedupeBatchUnitsGroupsIdenticalRequests(t *testing.T) {
	mk := func(index int, text, voice, path string) batchUnit {
		return batchUnit{
			index:  index,
			lineNo: index,
			voice:  voice,
			path:   path,
			req:    fishaudio.RenderRequest{Text: text, VoiceID: voice, Model: "s2.1-pro", Format: "mp3"},
		}
	}
	units := []batchUnit{
		mk(1, "hello", "voice-a", "out/0001.mp3"),
		mk(2, "world", "voice-a", "out/0002.mp3"),
		mk(3, "hello", "voice-a", "out/0003.mp3"),
		mk(4, "hello", "voice-b", "out/0004.mp3"),
	}
	jobs := dedupeBatchUnits(units)
	if len(jobs) != 3 {
		t.Fatalf("got %d jobs, want 3 (the two identical 'hello' lines share one)", len(jobs))
	}
	if len(jobs[0].units) != 2 {
		t.Fatalf("job 0 holds %d units, want the two identical lines", len(jobs[0].units))
	}
	if jobs[0].units[0].path != "out/0001.mp3" || jobs[0].units[1].path != "out/0003.mp3" {
		t.Fatalf("job 0 paths = %q, %q", jobs[0].units[0].path, jobs[0].units[1].path)
	}
	// A different voice is a different render, not a duplicate.
	if len(jobs[2].units) != 1 {
		t.Fatalf("the same text on another voice was deduped: %+v", jobs[2])
	}
	// Input order is preserved so output file numbering stays predictable.
	if jobs[1].units[0].path != "out/0002.mp3" {
		t.Fatalf("dedupe reordered the jobs: %q", jobs[1].units[0].path)
	}
}

// TestBatchRowHashIsPerFile keeps two copies of one render from colliding on
// the store's UNIQUE request_hash, which is what made spend under-report.
func TestBatchRowHashIsPerFile(t *testing.T) {
	req := fishaudio.RenderRequest{Text: "hello", VoiceID: "voice-a", Model: "s2.1-pro", Format: "mp3"}
	hash := req.Hash()
	first := fishaudio.BatchRowHash(hash, "out/0001.mp3")
	second := fishaudio.BatchRowHash(hash, "out/0003.mp3")
	if first == second {
		t.Fatal("two output files of one render share a row hash; the log would collapse them")
	}
	if first == hash {
		t.Fatal("a batch row hash must differ from the request hash it derives from")
	}
	if first != fishaudio.BatchRowHash(hash, "out/0001.mp3") {
		t.Fatal("BatchRowHash is not stable for the same inputs")
	}
}

// TestBatchDuplicateLinesRecordSeparateRows is the end-to-end spend assertion:
// three files from two API calls must be three rows, and only the calls that
// were billed carry cost.
func TestBatchDuplicateLinesRecordSeparateRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "renders.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening the temp store: %v", err)
	}
	defer db.Close()
	if err := db.EnsureRenderLog(context.Background()); err != nil {
		t.Fatalf("EnsureRenderLog: %v", err)
	}
	req := fishaudio.RenderRequest{Text: "hello", VoiceID: "voice-a", Model: "s2.1-pro", Format: "mp3"}
	hash := req.Hash()
	for i, path := range []string{"out/0001.mp3", "out/0003.mp3"} {
		cost := 0.0015
		if i > 0 {
			cost = 0
		}
		if _, err := db.InsertRenderRow(context.Background(), store.RenderRow{
			RequestHash: fishaudio.BatchRowHash(hash, path), Text: "hello", Model: "s2.1-pro",
			VoiceID: "voice-a", Format: "mp3", BytesIn: 5, BytesOut: 100,
			CostUSD: cost, CostUSDPaidEquiv: cost, FilePath: path, Source: "tts batch",
		}); err != nil {
			t.Fatalf("InsertRenderRow: %v", err)
		}
	}
	rows, err := db.ListRenderRows(context.Background(), store.RenderLogFilter{})
	if err != nil {
		t.Fatalf("ListRenderRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("two output files produced %d row(s), want 2", len(rows))
	}
	spend, err := db.RenderSpend(context.Background(), "model", "")
	if err != nil {
		t.Fatalf("RenderSpend: %v", err)
	}
	if len(spend) != 1 || spend[0].Renders != 2 {
		t.Fatalf("spend = %+v, want one model group covering 2 rows", spend)
	}
	if spend[0].CostUSD != 0.0015 {
		t.Fatalf("spend cost = %v, want the single billed render's 0.0015", spend[0].CostUSD)
	}
}

// TestRenderDiffDryRunEmitsEnvelope covers the ordering fix: the dry-run
// short-circuit must run before the two-positional gate.
func TestRenderDiffDryRunEmitsEnvelope(t *testing.T) {
	out, _, err := runCLI(t, "render", "diff", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("render diff --dry-run returned %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("render diff --dry-run did not print JSON: %v\n%s", err, out)
	}
	if envelope["dry_run"] != true || envelope["action"] != "render diff" {
		t.Fatalf("envelope = %v", envelope)
	}

	// Without --dry-run, partial positionals are still a usage error.
	if _, _, err := runCLI(t, "render", "diff", "1"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("render diff 1 returned %v, want exit 2", err)
	}
}
