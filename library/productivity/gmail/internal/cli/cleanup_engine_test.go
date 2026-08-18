// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral acceptance tests for the cleanup engine: httptest fake Gmail
// + temp gauth dir, driven end-to-end through the real command tree.
// Covers plan freezing, nonce lifecycle, tampered plans, drift refusal,
// execution + ledger, undo (incl. conflicts), crash recovery, and the
// apply flock.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// fakeMsg is one fake mailbox message. ExtraHeaders lets unsub-engine
// tests attach List-Unsubscribe / DKIM-Signature / Authentication-Results
// headers; each pair is emitted verbatim after From/Subject (duplicates
// allowed, as on the wire).
type fakeMsg struct {
	From, Subject string
	InternalDate  int64
	ThreadID      string
	Size          int64
	ExtraHeaders  [][2]string
}

// fakeGmail is a concurrency-safe in-memory Gmail API double.
type fakeGmail struct {
	mu             sync.Mutex
	profileEmail   string
	historyID      string
	labels         map[string][]string // message id -> labelIds
	meta           map[string]fakeMsg
	order          []string // listing order (newest first)
	labelDefs      []gmailLabel
	historyRecords []map[string]any
	requests       []string
	nextLabelSeq   int
}

func (f *fakeGmail) log(r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
}

func (f *fakeGmail) reqLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeGmail) resetLog() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = nil
}

func (f *fakeGmail) setLabels(id string, labels []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.labels[id] = append([]string(nil), labels...)
}

func (f *fakeGmail) getLabels(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.labels[id]...)
}

func (f *fakeGmail) countRequests(prefix string) int {
	n := 0
	for _, r := range f.reqLog() {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeGmail) messageJSON(id string) map[string]any {
	m := f.meta[id]
	headers := []map[string]string{
		{"name": "From", "value": m.From},
		{"name": "Subject", "value": m.Subject},
	}
	for _, h := range m.ExtraHeaders {
		headers = append(headers, map[string]string{"name": h[0], "value": h[1]})
	}
	return map[string]any{
		"id":           id,
		"threadId":     m.ThreadID,
		"labelIds":     f.labels[id],
		"snippet":      "snippet of " + id,
		"sizeEstimate": m.Size,
		"internalDate": fmt.Sprintf("%d", m.InternalDate),
		"payload": map[string]any{
			"headers": headers,
		},
	}
}

// addMsg appends one message to the fake mailbox (listing order preserved).
func (f *fakeGmail) addMsg(id string, labels []string, m fakeMsg) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = append(f.order, id)
	f.labels[id] = append([]string(nil), labels...)
	f.meta[id] = m
}

// setExtraHeaders replaces one message's extra headers in place (the live
// view changes; the local store keeps whatever the last sync captured).
func (f *fakeGmail) setExtraHeaders(id string, headers [][2]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.meta[id]
	m.ExtraHeaders = headers
	f.meta[id] = m
}

func (f *fakeGmail) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)

		path := strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me")
		switch {
		case path == "/profile" && r.Method == "GET":
			writeJSON(w, map[string]any{"emailAddress": f.profileEmail, "historyId": f.historyID})

		case path == "/messages" && r.Method == "GET":
			var msgs []map[string]string
			for _, id := range f.order {
				if _, ok := f.labels[id]; ok {
					msgs = append(msgs, map[string]string{"id": id})
				}
			}
			writeJSON(w, map[string]any{"messages": msgs})

		case path == "/messages/batchModify" && r.Method == "POST":
			var body struct {
				IDs    []string `json:"ids"`
				Add    []string `json:"addLabelIds"`
				Remove []string `json:"removeLabelIds"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, id := range body.IDs {
				if cur, ok := f.labels[id]; ok {
					f.labels[id] = applyLabelDelta(cur, body.Add, body.Remove)
				}
			}
			w.WriteHeader(http.StatusNoContent)

		case path == "/history" && r.Method == "GET":
			writeJSON(w, map[string]any{"history": f.historyRecords, "historyId": f.historyID})

		case path == "/labels" && r.Method == "GET":
			writeJSON(w, map[string]any{"labels": f.labelDefs})

		case path == "/labels" && r.Method == "POST":
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.nextLabelSeq++
			l := gmailLabel{ID: fmt.Sprintf("Label_%d", f.nextLabelSeq+100), Name: body.Name, Type: "user"}
			f.labelDefs = append(f.labelDefs, l)
			writeJSON(w, l)

		case strings.HasPrefix(path, "/labels/"):
			id := strings.TrimPrefix(path, "/labels/")
			idx := -1
			for i, l := range f.labelDefs {
				if l.ID == id {
					idx = i
					break
				}
			}
			if idx < 0 {
				http.Error(w, `{"error":{"code":404,"message":"label not found"}}`, http.StatusNotFound)
				return
			}
			switch r.Method {
			case "GET":
				writeJSON(w, f.labelDefs[idx])
			case "PATCH", "PUT":
				var body struct {
					Name string `json:"name"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.Name != "" {
					f.labelDefs[idx].Name = body.Name
				}
				writeJSON(w, f.labelDefs[idx])
			default:
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
			}

		case strings.HasPrefix(path, "/messages/") && r.Method == "GET":
			id := strings.TrimPrefix(path, "/messages/")
			if _, ok := f.labels[id]; !ok {
				http.Error(w, `{"error":{"code":404,"message":"not found"}}`, http.StatusNotFound)
				return
			}
			writeJSON(w, f.messageJSON(id))

		case strings.HasSuffix(path, "/trash") && r.Method == "POST":
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/messages/"), "/trash")
			cur, ok := f.labels[id]
			if !ok {
				http.Error(w, `{"error":{"code":404,"message":"not found"}}`, http.StatusNotFound)
				return
			}
			f.labels[id] = applyLabelDelta(cur, []string{"TRASH"}, []string{"INBOX"})
			writeJSON(w, f.messageJSON(id))

		case strings.HasSuffix(path, "/untrash") && r.Method == "POST":
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/messages/"), "/untrash")
			cur, ok := f.labels[id]
			if !ok {
				http.Error(w, `{"error":{"code":404,"message":"not found"}}`, http.StatusNotFound)
				return
			}
			f.labels[id] = applyLabelDelta(cur, nil, []string{"TRASH"})
			writeJSON(w, f.messageJSON(id))

		case strings.HasSuffix(path, "/modify") && r.Method == "POST":
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/messages/"), "/modify")
			cur, ok := f.labels[id]
			if !ok {
				http.Error(w, `{"error":{"code":404,"message":"not found"}}`, http.StatusNotFound)
				return
			}
			var body struct {
				Add    []string `json:"addLabelIds"`
				Remove []string `json:"removeLabelIds"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.labels[id] = applyLabelDelta(cur, body.Add, body.Remove)
			writeJSON(w, f.messageJSON(id))

		default:
			http.Error(w, fmt.Sprintf(`{"error":{"code":400,"message":"fake gmail: unhandled %s %s"}}`, r.Method, r.URL.Path), http.StatusBadRequest)
		}
	})
}

// engineFixture wires a fake Gmail, a temp auth dir (profiles.yaml,
// client.json, token), and a temp --home into runnable CLI invocations.
type engineFixture struct {
	fake    *fakeGmail
	ts      *httptest.Server
	authDir string
	home    string
}

const engineTestEmail = "cleanup@example.com"

func newEngineFixture(t *testing.T) *engineFixture {
	t.Helper()
	fake := &fakeGmail{
		profileEmail: engineTestEmail,
		historyID:    "1000",
		labels:       map[string][]string{},
		meta:         map[string]fakeMsg{},
		labelDefs: []gmailLabel{
			{ID: "Label_1", Name: "Newsletters", Type: "user"},
			{ID: "UNREAD", Name: "UNREAD", Type: "system"},
			{ID: "INBOX", Name: "INBOX", Type: "system"},
		},
	}
	base := int64(1_700_000_000_000)
	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("m%d", i)
		fake.order = append(fake.order, id)
		fake.labels[id] = []string{"INBOX", "CATEGORY_PROMOTIONS", "UNREAD"}
		fake.meta[id] = fakeMsg{
			From:         fmt.Sprintf("Promo Sender <promo%d@example.com>", i),
			Subject:      fmt.Sprintf("Big sale %d", i),
			InternalDate: base + int64(i)*1000,
			ThreadID:     "t" + id,
			Size:         2048,
		}
	}
	ts := httptest.NewServer(fake.handler())
	t.Cleanup(ts.Close)

	root := t.TempDir()
	authDir := filepath.Join(root, "gauth")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(authDir, "tokens"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	profiles := "accounts:\n  - name: test\n    email: " + engineTestEmail + "\n    role: cleanup\n"
	if err := os.WriteFile(filepath.Join(authDir, "profiles.yaml"), []byte(profiles), 0o600); err != nil {
		t.Fatal(err)
	}
	clientJSON := `{"installed":{"client_id":"cid.apps.googleusercontent.com","project_id":"x","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","client_secret":"csecret","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(filepath.Join(authDir, "client.json"), []byte(clientJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	token := `{"access_token":"engine-test-token","token_type":"Bearer","refresh_token":"rt","expiry":"2099-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(authDir, "tokens", "test.json"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GMAIL_BASE_URL", ts.URL)
	// Hermetic: never inherit verify/dogfood modes from the runner's shell.
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.VerifyLiveHTTPEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "")
	// Home override is process-global (set by PersistentPreRunE from
	// --home); pin it for direct store access and restore after the test.
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)

	return &engineFixture{fake: fake, ts: ts, authDir: authDir, home: home}
}

// runCLI executes the real command tree in-process.
func (fx *engineFixture) runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	full := append([]string(nil), args...)
	full = append(full, "--auth-dir", fx.authDir, "--home", fx.home, "--json", "--no-learn")
	root.SetArgs(full)
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
	}
	return out.String(), errBuf.String(), code
}

func (fx *engineFixture) openStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(defaultDBPath("gmail-pp-cli"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustParseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, s)
	}
	return v
}

// planForTrash runs `cleanup plan --q ... --action trash` and returns
// (planSha, nonce, parsed output).
func planForTrash(t *testing.T, fx *engineFixture) (string, string, map[string]any) {
	t.Helper()
	out, stderr, code := fx.runCLI(t, "cleanup", "plan", "--account", "test",
		"--q", "category:promotions", "--action", "trash")
	if code != 0 {
		t.Fatalf("cleanup plan exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	sha, _ := parsed["plan_sha"].(string)
	nonce, _ := parsed["nonce"].(string)
	if sha == "" || nonce == "" {
		t.Fatalf("plan output missing sha/nonce: %s", out)
	}
	return sha, nonce, parsed
}

func TestCleanupEngine_PlanApplyUndo_EndToEnd(t *testing.T) {
	fx := newEngineFixture(t)

	// ---- plan ----
	sha, nonce, parsed := planForTrash(t, fx)
	if got := parsed["total"].(float64); got != 4 {
		t.Fatalf("plan total = %v, want 4", got)
	}
	samples := parsed["samples"].([]any)
	if len(samples) != 4 {
		t.Fatalf("samples len = %d, want 4", len(samples))
	}
	planPath := filepath.Join(fx.authDir, "plans", sha+".json")
	info, err := os.Stat(planPath)
	if err != nil {
		t.Fatalf("plan file missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("plan file mode = %v, want 0600", info.Mode().Perm())
	}
	// The file name IS the sha of the content.
	if _, err := loadPlanFile(fx.authDir, sha); err != nil {
		t.Fatalf("freshly written plan failed its own hash check: %v", err)
	}
	// Nonce row exists, unused, bound to this plan.
	db := fx.openStore(t)
	var used int
	var boundSha string
	if err := db.DB().QueryRow(`SELECT used, plan_sha FROM mail_nonces WHERE nonce = ?`, nonce).Scan(&used, &boundSha); err != nil {
		t.Fatalf("nonce row: %v", err)
	}
	if used != 0 || boundSha != sha {
		t.Fatalf("nonce row used=%d sha=%s, want unused + %s", used, boundSha, sha)
	}

	// ---- (a) wrong token: refused, zero API calls ----
	fx.fake.resetLog()
	out, _, code := fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", strings.Repeat("de", 16))
	if code != 4 {
		t.Fatalf("wrong token exit = %d, want 4\n%s", code, out)
	}
	if got := fx.fake.reqLog(); len(got) != 0 {
		t.Fatalf("wrong token made %d API calls, want 0: %v", len(got), got)
	}

	// ---- (b) expired nonce: refused ----
	expired := strings.Repeat("ab", 16)
	if err := db.CreateMailNonce(expired, sha, "test", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	out, _, code = fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", expired)
	if code != 4 {
		t.Fatalf("expired nonce exit = %d, want 4\n%s", code, out)
	}

	// ---- (c) tampered plan file: refused ----
	orig, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, append(orig, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, code = fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 4 {
		t.Fatalf("tampered plan exit = %d, want 4\n%s", code, out)
	}
	if err := os.WriteFile(planPath, orig, 0o600); err != nil {
		t.Fatal(err)
	}

	// ---- (d) valid token: mutations hit exactly the frozen ids ----
	fx.fake.resetLog()
	out, stderr, code := fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 0 {
		t.Fatalf("apply exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	applied := mustParseJSON(t, out)
	if got := applied["applied"].(float64); got != 4 {
		t.Fatalf("applied = %v, want 4\n%s", got, out)
	}
	ledgerID, _ := applied["ledger_id"].(string)
	if ledgerID == "" {
		t.Fatalf("apply output missing ledger_id: %s", out)
	}
	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("m%d", i)
		if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/" + id + "/trash"); n != 1 {
			t.Fatalf("trash calls for %s = %d, want 1\nlog: %v", id, n, fx.fake.reqLog())
		}
		labels := fx.fake.getLabels(id)
		if !hasLabel(labels, "TRASH") || hasLabel(labels, "INBOX") {
			t.Fatalf("%s labels after apply = %v, want TRASH without INBOX", id, labels)
		}
	}
	if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/batchModify"); n != 0 {
		t.Fatalf("trash apply must not use batchModify (R2-C5); saw %d calls", n)
	}
	// Ledger rows written with the introduced delta + pre-placement.
	entries, err := db.ListMailLedgerEntries(ledgerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("ledger entries = %d, want 4", len(entries))
	}
	for _, e := range entries {
		if e.Kind != "trash" || !hasLabel(e.DeltaAdd, "TRASH") {
			t.Fatalf("ledger entry %s kind/delta wrong: %+v", e.ID, e)
		}
		if !hasLabel(e.PrePlacement, "INBOX") || !hasLabel(e.PrePlacement, "CATEGORY_PROMOTIONS") {
			t.Fatalf("ledger entry %s pre_placement = %v, want INBOX + CATEGORY_PROMOTIONS", e.ID, e.PrePlacement)
		}
	}

	// ---- (e) re-apply the same token: refused (used), no new mutations ----
	fx.fake.resetLog()
	out, _, code = fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 4 {
		t.Fatalf("re-apply used token exit = %d, want 4\n%s", code, out)
	}
	if n := fx.fake.countRequests("POST "); n != 0 {
		t.Fatalf("used token made %d POSTs, want 0: %v", n, fx.fake.reqLog())
	}

	// ---- undo: one id mutated since -> conflict; rest restored ----
	// Simulate an external untrash of m2 between apply and undo.
	fx.fake.setLabels("m2", []string{"CATEGORY_PROMOTIONS", "UNREAD"})
	fx.fake.resetLog()
	out, stderr, code = fx.runCLI(t, "undo", "--ledger", ledgerID)
	if code != 0 {
		t.Fatalf("undo exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	undone := mustParseJSON(t, out)
	if got := undone["undone"].(float64); got != 3 {
		t.Fatalf("undone = %v, want 3\n%s", got, out)
	}
	conflicts := undone["conflicts"].([]any)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want exactly m2", conflicts)
	}
	if got := conflicts[0].(map[string]any)["id"].(string); got != "m2" {
		t.Fatalf("conflict id = %s, want m2", got)
	}
	for _, id := range []string{"m1", "m3", "m4"} {
		if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/" + id + "/untrash"); n != 1 {
			t.Fatalf("untrash calls for %s = %d, want 1", id, n)
		}
		labels := fx.fake.getLabels(id)
		if hasLabel(labels, "TRASH") || !hasLabel(labels, "INBOX") || !hasLabel(labels, "CATEGORY_PROMOTIONS") {
			t.Fatalf("%s labels after undo = %v, want placement restored without TRASH", id, labels)
		}
	}
	if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/m2/untrash"); n != 0 {
		t.Fatalf("conflicted m2 must not be untrashed; saw %d calls", n)
	}
}

func TestCleanupApply_RefusesOnDrift(t *testing.T) {
	fx := newEngineFixture(t)
	sha, nonce, _ := planForTrash(t, fx)

	// A history record now touches a planned id.
	fx.fake.mu.Lock()
	fx.fake.historyRecords = []map[string]any{
		{
			"labelsRemoved": []map[string]any{
				{"message": map[string]any{"id": "m3", "labelIds": []string{"INBOX", "CATEGORY_PROMOTIONS"}}, "labelIds": []string{"UNREAD"}},
			},
		},
	}
	fx.fake.mu.Unlock()

	fx.fake.resetLog()
	out, _, code := fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 4 {
		t.Fatalf("drifted apply exit = %d, want 4\n%s", code, out)
	}
	if n := fx.fake.countRequests("POST "); n != 0 {
		t.Fatalf("drifted apply issued %d mutations, want 0: %v", n, fx.fake.reqLog())
	}
	for i := 1; i <= 4; i++ {
		if hasLabel(fx.fake.getLabels(fmt.Sprintf("m%d", i)), "TRASH") {
			t.Fatalf("m%d was trashed despite drift refusal", i)
		}
	}
}

func TestCleanupRecover_CompletesCrashedApply(t *testing.T) {
	fx := newEngineFixture(t)
	sha, nonce, _ := planForTrash(t, fx)

	// Simulate a crash mid-apply: authorization happened, one chunk was
	// durably marked 'applying', two ids were already trashed, then the
	// process died. (Rows inserted directly, as the acceptance requires.)
	db := fx.openStore(t)
	applyID, err := db.AuthorizeMailApply(nonce, sha, "test", "trash", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := loadPlanFile(fx.authDir, sha)
	if err != nil {
		t.Fatal(err)
	}
	chunks := buildChunksFromPlan(plan, applyID)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for 4 ids, got %d", len(chunks))
	}
	chunks[0].State = store.MailChunkStateApplying
	if err := db.InsertMailApplyChunks(chunks); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMailApplyState(applyID, store.MailApplyStateApplying); err != nil {
		t.Fatal(err)
	}
	ledgerID := "feedc0defeedc0de"
	if err := db.CreateMailLedger(store.MailLedger{LedgerID: ledgerID, Account: "test", PlanSha: sha, ApplyID: applyID, Action: "trash"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMailApplyLedger(applyID, ledgerID); err != nil {
		t.Fatal(err)
	}
	// The crashed run got m1 and m2 done before dying.
	fx.fake.setLabels("m1", []string{"CATEGORY_PROMOTIONS", "UNREAD", "TRASH"})
	fx.fake.setLabels("m2", []string{"CATEGORY_PROMOTIONS", "UNREAD", "TRASH"})

	fx.fake.resetLog()
	out, stderr, code := fx.runCLI(t, "cleanup", "recover")
	if code != 0 {
		t.Fatalf("recover exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	// Already-trashed ids are re-checked, not re-trashed.
	for _, id := range []string{"m1", "m2"} {
		if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/" + id + "/trash"); n != 0 {
			t.Fatalf("recover re-trashed already-trashed %s (%d calls)", id, n)
		}
	}
	for _, id := range []string{"m3", "m4"} {
		if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/" + id + "/trash"); n != 1 {
			t.Fatalf("recover trash calls for %s = %d, want 1\nlog: %v", id, n, fx.fake.reqLog())
		}
		if !hasLabel(fx.fake.getLabels(id), "TRASH") {
			t.Fatalf("%s not trashed after recover", id)
		}
	}
	// Chunk done, apply done, ledger complete (all four ids).
	got, err := db.ListMailApplyChunks(applyID)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].State != store.MailChunkStateDone {
		t.Fatalf("chunk state after recover = %s, want done", got[0].State)
	}
	a, err := db.GetMailApply(applyID)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != store.MailApplyStateDone {
		t.Fatalf("apply state after recover = %s, want done", a.State)
	}
	entries, err := db.ListMailLedgerEntries(ledgerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("ledger entries after recover = %d, want 4", len(entries))
	}
	// Idempotent: a second recover is a no-op with nothing left.
	out, _, code = fx.runCLI(t, "cleanup", "recover")
	if code != 0 {
		t.Fatalf("second recover exit = %d, want 0\n%s", code, out)
	}
	second := mustParseJSON(t, out)
	if got := second["count"].(float64); got != 0 {
		t.Fatalf("second recover count = %v, want 0", got)
	}
}

func TestCleanupApply_LockBusy(t *testing.T) {
	fx := newEngineFixture(t)
	sha, nonce, _ := planForTrash(t, fx)

	// Hold the flock in this process; apply must exit 7 without mutating.
	lockPath := filepath.Join(fx.authDir, "apply.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("test could not take the lock: %v", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	fx.fake.resetLog()
	out, _, code := fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 7 {
		t.Fatalf("apply with held lock exit = %d, want 7\n%s", code, out)
	}
	if n := fx.fake.countRequests("POST "); n != 0 {
		t.Fatalf("locked-out apply issued %d mutations, want 0", n)
	}

	// Release; the same token must now work (it was never burned).
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	out, _, code = fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 0 {
		t.Fatalf("apply after lock release exit = %d, want 0\n%s", code, out)
	}
}

func TestCleanupApply_LabelActionUsesBatchModify(t *testing.T) {
	fx := newEngineFixture(t)
	out, stderr, code := fx.runCLI(t, "cleanup", "plan", "--account", "test",
		"--q", "category:promotions", "--action", "label", "--add", "Newsletters", "--remove", "UNREAD")
	if code != 0 {
		t.Fatalf("label plan exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	sha := parsed["plan_sha"].(string)
	nonce := parsed["nonce"].(string)

	fx.fake.resetLog()
	out, _, code = fx.runCLI(t, "cleanup", "apply", "--plan", sha, "--token", nonce)
	if code != 0 {
		t.Fatalf("label apply exit = %d\n%s", code, out)
	}
	if n := fx.fake.countRequests("POST /gmail/v1/users/me/messages/batchModify"); n != 1 {
		t.Fatalf("batchModify calls = %d, want 1 (500-chunk)", n)
	}
	for i := 1; i <= 4; i++ {
		labels := fx.fake.getLabels(fmt.Sprintf("m%d", i))
		if !hasLabel(labels, "Label_1") || hasLabel(labels, "UNREAD") {
			t.Fatalf("m%d labels = %v, want +Label_1 -UNREAD", i, labels)
		}
	}
}

func TestRulesRun_MergedPlanAndPlanOnly(t *testing.T) {
	fx := newEngineFixture(t)

	if _, _, code := fx.runCLI(t, "rules", "add", "--name", "promos", "--q", "category:promotions", "--action", "trash"); code != 0 {
		t.Fatalf("rules add promos failed")
	}
	if _, _, code := fx.runCLI(t, "rules", "add", "--name", "newsletters", "--q", "list:unsubscribe", "--action", "label", "--add", "Newsletters"); code != 0 {
		t.Fatalf("rules add newsletters failed")
	}

	// --plan-only first (identical rule content hashes to the same plan
	// sha across runs, so this must precede the token-minting run): plan
	// written, NO nonce minted.
	out, stderr, code := fx.runCLI(t, "rules", "run", "--account", "test", "--plan-only")
	if code != 0 {
		t.Fatalf("rules run --plan-only exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed := mustParseJSON(t, out)
	if parsed["nonce"] != nil {
		t.Fatalf("--plan-only must not mint a token: %s", out)
	}
	if parsed["plan_sha"] == nil {
		t.Fatalf("--plan-only must still write the plan: %s", out)
	}
	planOnlySha := parsed["plan_sha"].(string)
	db := fx.openStore(t)
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM mail_nonces WHERE plan_sha = ?`, planOnlySha).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("--plan-only minted %d nonce rows, want 0", n)
	}

	// Default run mints a token; ids dedupe first-rule-wins (both queries
	// match the same 4 fake messages, so rule 2 gets zero).
	out, stderr, code = fx.runCLI(t, "rules", "run", "--account", "test")
	if code != 0 {
		t.Fatalf("rules run exit = %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	parsed = mustParseJSON(t, out)
	if parsed["nonce"] == nil || parsed["plan_sha"] == nil {
		t.Fatalf("rules run must mint a token by default: %s", out)
	}
	perGroup := parsed["per_group"].([]any)
	if len(perGroup) != 2 {
		t.Fatalf("per_group len = %d, want 2", len(perGroup))
	}
	g0 := perGroup[0].(map[string]any)
	g1 := perGroup[1].(map[string]any)
	if g0["rule"].(string) != "promos" || g0["count"].(float64) != 4 {
		t.Fatalf("group 0 = %v, want promos count 4", g0)
	}
	if g1["rule"].(string) != "newsletters" || g1["count"].(float64) != 0 {
		t.Fatalf("group 1 = %v, want newsletters count 0 (deduped first-rule-wins)", g1)
	}
	if got := parsed["total"].(float64); got != 4 {
		t.Fatalf("total = %v, want 4", got)
	}
}

func TestLabelsCreateIdempotentAndRenameUndo(t *testing.T) {
	fx := newEngineFixture(t)

	// Existing name (case-insensitive) returns the existing label.
	out, _, code := fx.runCLI(t, "labels", "create", "--account", "test", "--name", "newsletters")
	if code != 0 {
		t.Fatalf("labels create exit = %d\n%s", code, out)
	}
	created := mustParseJSON(t, out)
	if created["existing"] != true || created["id"].(string) != "Label_1" {
		t.Fatalf("labels create should return the existing Label_1: %s", out)
	}
	// New name creates.
	out, _, code = fx.runCLI(t, "labels", "create", "--account", "test", "--name", "Receipts")
	if code != 0 {
		t.Fatalf("labels create exit = %d\n%s", code, out)
	}
	created = mustParseJSON(t, out)
	if created["created"] != true {
		t.Fatalf("labels create should create Receipts: %s", out)
	}

	// Rename + undo round-trip, with conflict detection.
	out, _, code = fx.runCLI(t, "labels", "rename", "--account", "test", "--id", "Label_1", "--to", "Old Newsletters")
	if code != 0 {
		t.Fatalf("labels rename exit = %d\n%s", code, out)
	}
	renamed := mustParseJSON(t, out)
	ledgerID := renamed["ledger_id"].(string)
	out, _, code = fx.runCLI(t, "undo", "--ledger", ledgerID)
	if code != 0 {
		t.Fatalf("undo rename exit = %d\n%s", code, out)
	}
	res := mustParseJSON(t, out)
	if res["undone"].(float64) != 1 {
		t.Fatalf("rename undo should undo 1 entry: %s", out)
	}
	fx.fake.mu.Lock()
	name := fx.fake.labelDefs[0].Name
	fx.fake.mu.Unlock()
	if name != "Newsletters" {
		t.Fatalf("label name after undo = %q, want Newsletters", name)
	}
}

// TestRawMutationCommandsAbsent pins acceptance item 8 in-process: the
// pruned raw mutation subcommands are unknown commands (non-zero exit,
// with and without --help), while reads and the engine commands resolve.
func TestRawMutationCommandsAbsent(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	deny := [][]string{
		{"messages", "modify", "--help"},
		{"messages", "trash", "--help"},
		{"messages", "untrash", "--help"},
		{"messages", "batch-modify", "--help"},
		{"messages", "batchModify", "--help"},
		{"threads", "modify", "--help"},
		{"threads", "trash", "--help"},
		{"threads", "untrash", "--help"},
		{"labels", "update", "--help"},
		{"labels", "patch", "--help"},
		{"labels", "delete", "--help"},
		{"import", "--help"},
		{"messages", "modify"},
		{"threads", "trash"},
		{"labels", "delete"},
	}
	for _, d := range deny {
		os.Args = append([]string{"gmail-pp-cli"}, append(append([]string(nil), d...), "--no-learn")...)
		if err := Execute(); err == nil {
			t.Fatalf("%v: expected non-zero exit (unknown command), got success", d)
		}
	}
	allow := [][]string{
		{"messages", "list", "--help"},
		{"messages", "get", "--help"},
		{"threads", "get", "--help"},
		{"labels", "list", "--help"},
		{"labels", "create", "--help"},
		{"labels", "rename", "--help"},
		{"cleanup", "plan", "--help"},
		{"cleanup", "apply", "--help"},
		{"cleanup", "recover", "--help"},
		{"undo", "--help"},
		{"rules", "run", "--help"},
	}
	for _, a := range allow {
		os.Args = append([]string{"gmail-pp-cli"}, append(append([]string(nil), a...), "--no-learn")...)
		if err := Execute(); err != nil {
			t.Fatalf("%v: expected success, got %v", a, err)
		}
	}
}
