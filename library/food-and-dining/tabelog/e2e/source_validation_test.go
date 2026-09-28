package e2e

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const samboaURL = "https://tabelog.com/en/tokyo/A1301/A130101/13005012/"

func workspaceEnv(t *testing.T, w *workspace, key string) string {
	t.Helper()
	for _, v := range w.env {
		if strings.HasPrefix(v, key+"=") {
			return strings.TrimPrefix(v, key+"=")
		}
	}
	t.Fatalf("workspace has no %s", key)
	return ""
}

func sourceCachePath(t *testing.T, w *workspace, sourceURL string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(sourceURL))
	want := hex.EncodeToString(sum[:]) + ".json"
	var path string
	err := filepath.WalkDir(workspaceEnv(t, w, "XDG_CACHE_HOME"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == want {
			path = p
		}
		return nil
	})
	if err != nil || path == "" {
		t.Fatalf("validated source cache %s missing: %v", want, err)
	}
	return path
}

func readCache(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDetailIdentityMismatchRetainsRawSnapshotAndNotes(t *testing.T) {
	good := readFixture(t, "samboa-detail.html")
	wrongRoute := bytes.ReplaceAll(good, []byte(samboaURL), []byte("https://tabelog.com/en/tokyo/A1301/A130103/13005012/"))
	wrongCanonical := bytes.Replace(good, []byte("<body>"), []byte(`<head><link rel="canonical" href="`+sushiURL+`"></head><body>`), 1)
	for name, wrong := range map[string][]byte{
		"different_venue":         readFixture(t, "sushi-detail.html"),
		"same_id_different_route": wrongRoute,
		"conflicting_canonical":   wrongCanonical,
	} {
		t.Run(name, func(t *testing.T) {
			r := newReplay(t, func(*http.Request) response { return response{body: good} })
			w := newWorkspace(t, r)
			before := mustSucceed(t, w.run(t, "show", samboaURL, "--agent"))
			mustSucceed(t, w.run(t, "lists", "add", "trip", "13005012", "--note", "keep my note", "--agent"))
			cachePath := sourceCachePath(t, w, samboaURL)
			cacheBefore := readCache(t, cachePath)
			r.serve(func(*http.Request) response { return response{body: wrong} })
			failed := w.run(t, "show", samboaURL, "--data-source", "live", "--agent")
			mustFail(t, failed)
			if !bytes.Contains(failed.stderr, []byte("identity")) {
				t.Fatalf("mismatched detail did not explain identity: %s", failed.stderr)
			}
			equal(t, readCache(t, cachePath), cacheBefore)
			after := mustSucceed(t, w.run(t, "show", "13005012", "--data-source", "local", "--agent"))
			equal(t, items(t, after), items(t, before))
			mustFail(t, w.run(t, "lists", "refresh", "trip", "13005012", "--agent"))
			equal(t, readCache(t, cachePath), cacheBefore)
			list := mustSucceed(t, w.run(t, "lists", "show", "trip", "--agent"))
			saved := restaurant(t, items(t, list), "13005012")
			equal(t, saved["name"], "SAMBOA BAR Ginza ten")
			equal(t, saved["note"], "keep my note")
			equal(t, saved["fetched_at"], items(t, before)[0]["fetched_at"])
			equal(t, r.count(), 3)
		})
	}
}

func TestUnfetchedWrongDetailDoesNotCreateAnIdentity(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{body: readFixture(t, "sushi-detail.html")} })
	w := newWorkspace(t, r)
	mustFail(t, w.run(t, "show", samboaURL, "--agent"))
	mustFail(t, w.run(t, "show", "13005012", "--data-source", "local", "--agent"))
	equal(t, r.count(), 1)
}

func changedSort(body []byte) []byte {
	// Keep every URL (including the rt next link) untouched: the selected UI
	// control, rather than a request URL or an inactive link, is the oracle.
	b := bytes.ReplaceAll(body, []byte("navi-rstlst__tab--rank is-active"), []byte("navi-rstlst__tab--rank"))
	return bytes.ReplaceAll(b, []byte(`navi-rstlst__tab--trend"`), []byte(`navi-rstlst__tab--trend is-active"`))
}

func TestSelectedSortDriftCannotOverwriteRankedCache(t *testing.T) {
	good := readFixture(t, "tokyo-ranked.html")
	for name, wrong := range map[string][]byte{
		"other_selected": changedSort(good),
		"no_selected":    bytes.ReplaceAll(good, []byte("navi-rstlst__tab--rank is-active"), []byte("navi-rstlst__tab--rank")),
		"two_selected":   bytes.ReplaceAll(good, []byte(`navi-rstlst__tab--trend"`), []byte(`navi-rstlst__tab--trend is-active"`)),
	} {
		t.Run(name, func(t *testing.T) {
			r := newReplay(t, func(*http.Request) response { return response{body: good} })
			w := newWorkspace(t, r)
			before := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--agent"))
			equal(t, assertMeta(t, before)["source_sort"], "highest_rated")
			cachePath := sourceCachePath(t, w, tokyoURL+"?SrtT=rt")
			cacheBefore := readCache(t, cachePath)
			r.serve(func(*http.Request) response { return response{body: wrong} })
			failed := w.run(t, "find", "--area", tokyoURL, "--data-source", "live", "--agent")
			mustFail(t, failed)
			if !bytes.Contains(failed.stderr, []byte("sort")) && !bytes.Contains(failed.stderr, []byte("ranking")) {
				t.Fatalf("sort mismatch lacked an actionable diagnostic: %s", failed.stderr)
			}
			equal(t, readCache(t, cachePath), cacheBefore)
			after := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--data-source", "local", "--agent"))
			equal(t, items(t, after), items(t, before))
			equal(t, r.count(), 2)
		})
	}
}

func TestEveryPageValidatesSelectedSort(t *testing.T) {
	first, second := readFixture(t, "ginza-bars.html"), readFixture(t, "ginza-bars-page2.html")
	handler := func(wrong bool) func(*http.Request) response {
		return func(req *http.Request) response {
			if strings.Contains(req.URL.Path, "/bar/2/") {
				if wrong {
					return response{body: changedSort(second)}
				}
				return response{body: second}
			}
			return response{body: first}
		}
	}
	r := newReplay(t, handler(false))
	w := newWorkspace(t, r)
	args := []string{"find", "--area", ginzaURL, "--cuisine", "bar", "--meal", "dinner", "--budget-max", "5000", "--limit", "25", "--max-pages", "2", "--agent"}
	before := mustSucceed(t, w.run(t, args...))
	savedBefore := mustSucceed(t, w.run(t, "show", "13224293", "--data-source", "local", "--agent"))
	secondRequest := r.seen()[1]
	secondURL := "https://tabelog.com" + secondRequest.path + "?" + secondRequest.query
	cachePath := sourceCachePath(t, w, secondURL)
	cacheBefore := readCache(t, cachePath)
	r.serve(handler(true))
	mustFail(t, w.run(t, append(args, "--data-source", "live")...))
	equal(t, readCache(t, cachePath), cacheBefore)
	savedAfter := mustSucceed(t, w.run(t, "show", "13224293", "--data-source", "local", "--agent"))
	equal(t, items(t, savedAfter), items(t, savedBefore))
	after := mustSucceed(t, w.run(t, append(args, "--data-source", "local")...))
	beforeItems, afterItems := items(t, before), items(t, after)
	for i := range beforeItems {
		// The valid first page may have been refetched before page2 failed.
		// Its independent raw timestamp advances; the rejected page2 does not.
		if i < 20 {
			delete(beforeItems[i], "fetched_at")
			delete(afterItems[i], "fetched_at")
		}
	}
	equal(t, afterItems, beforeItems)
	equal(t, assertMeta(t, after)["source_sort"], "highest_rated")
	equal(t, r.count(), 4)
}

func TestEmptySourceDoesNotInventASortControl(t *testing.T) {
	body := readFixture(t, "ginza-empty.html")
	r := newReplay(t, func(*http.Request) response { return response{body: body} })
	w := newWorkspace(t, r)
	p := mustSucceed(t, w.run(t, "find", "--area", ginzaURL, "--keyword", "zzztabeloge2ezerocandidates20260927", "--agent"))
	equal(t, len(items(t, p)), 0)
	equal(t, assertMeta(t, p)["source_sort"], "not_applicable")
}

func TestDefaultSummaryRetainsKnownFacilitiesAndHelpUsesDomainFields(t *testing.T) {
	r := tokyoReplay(t)
	w := newWorkspace(t, r)
	p := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--limit", "1", "--agent"))
	equal(t, items(t, p)[0]["facilities"], []any{"Credit card accepted", "Non smoking"})
	full := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--limit", "1", "--data-source", "local", "--agent", "--select", "items.id,items.facilities"))
	equal(t, rawItems(t, full)[0]["facilities"], items(t, p)[0]["facilities"])
	for _, args := range [][]string{{"--help"}, {"find", "--help"}} {
		help := w.run(t, args...)
		equal(t, help.code, 0)
		if !bytes.Contains(help.stdout, []byte("items.id,items.name,items.rating")) || bytes.Contains(help.stdout, []byte("title,url")) {
			t.Fatalf("misleading projection example in help: %s", help.stdout)
		}
	}
	equal(t, r.count(), 1)
}

func TestSourceCommandsIgnoreUnusedCheckpointAndExposeSnapshotFailures(t *testing.T) {
	listing, detail := readFixture(t, "tokyo-ranked.html"), readFixture(t, "sushi-detail.html")
	r := newReplay(t, func(req *http.Request) response {
		if strings.Contains(req.URL.Path, "13294162/") {
			return response{body: detail}
		}
		return response{body: listing}
	})
	w := newWorkspace(t, r)
	mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--agent"))
	var dbPath string
	err := filepath.WalkDir(workspaceEnv(t, w, "XDG_DATA_HOME"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(d.Name(), ".db") {
			dbPath = path
		}
		return err
	})
	if err != nil || dbPath == "" {
		t.Fatalf("workspace DB missing: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TRIGGER obsolete_checkpoint BEFORE INSERT ON sync_state BEGIN SELECT RAISE(ABORT,'obsolete checkpoint must not run'); END`)
	if err != nil {
		t.Fatal(err)
	}
	// A deterministic SQL trap verifies that the unused contextless operation
	// is gone, without a flaky elapsed-time assertion on SQLite instruction speed.
	mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--data-source", "local", "--timeout", "100ms", "--agent"))
	mustSucceed(t, w.run(t, "show", sushiURL, "--timeout", "100ms", "--agent"))
	_, err = db.Exec(`CREATE TRIGGER required_snapshot BEFORE UPDATE ON tabelog_snapshots BEGIN SELECT RAISE(ABORT,'required snapshot failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	failed := w.run(t, "find", "--area", tokyoURL, "--data-source", "local", "--agent")
	mustFail(t, failed)
	if !bytes.Contains(failed.stderr, []byte("required snapshot failure")) {
		t.Fatalf("normalized persistence error disappeared: %s", failed.stderr)
	}
	equal(t, r.count(), 2)
}

func TestCallerDeadlineStopsSourceBeforePersistence(t *testing.T) {
	r := newReplay(t, func(req *http.Request) response { <-req.Context().Done(); return response{status: 504} })
	w := newWorkspace(t, r)
	start := time.Now()
	mustFail(t, w.run(t, "find", "--area", tokyoURL, "--timeout", "50ms", "--agent"))
	if time.Since(start) > time.Second {
		t.Fatal("caller deadline did not bound the source operation")
	}
	equal(t, r.count(), 1)
}
