package edeka

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSessionPathFollowsConfigDir: without EDEKA_SESSION_FILE the session and
// the saved market live in the resolved config dir, so a different --home or
// EDEKA_CONFIG_DIR never shares another customer's login.
func TestSessionPathFollowsConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EDEKA_SESSION_FILE", "")
	t.Setenv("EDEKA_CONFIG_DIR", dir)
	p, err := SessionPath()
	if err != nil || p != filepath.Join(dir, "session.json") {
		t.Fatalf("SessionPath = %q, %v", p, err)
	}
	m, err := statePath()
	if err != nil || m != filepath.Join(dir, "market.txt") {
		t.Fatalf("statePath = %q, %v", m, err)
	}
	t.Setenv("EDEKA_SESSION_FILE", filepath.Join(dir, "customer-a.json"))
	if p, _ := SessionPath(); p != filepath.Join(dir, "customer-a.json") {
		t.Fatalf("EDEKA_SESSION_FILE ignored: %q", p)
	}
}

func TestClearSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := ClearSession(path); err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if err := (&Session{Cookies: []Cookie{{Name: "sid", Value: "v", Domain: "m.edeka.shop", Path: "/"}}}).Save(path); err != nil {
		t.Fatal(err)
	}
	if err := ClearSession(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(path); err != ErrNoSession {
		t.Fatalf("session still there: %v", err)
	}
}

// TestSessionWriteBackMergesIntoFile: a generated command's response that
// renews the shop session must reach the session file; without a session
// file nothing is created.
func TestSessionWriteBackMergesIntoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "rotated", Path: "/"})
		io.WriteString(w, "{}")
	}))
	defer srv.Close()
	target, _ := url.Parse(srv.URL)
	host := "test-market.edeka.shop"

	path := filepath.Join(t.TempDir(), "session.json")
	if err := (&Session{Cookies: []Cookie{{Name: "sid", Value: "old", Domain: host, Path: "/"}}}).Save(path); err != nil {
		t.Fatal(err)
	}
	hc := &http.Client{Transport: SessionWriteBack(rewriteTransport{target}, path)}
	resp, err := hc.Get("https://" + host + "/index.php")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	back, _ := LoadSession(path)
	if len(back.Cookies) != 1 || back.Cookies[0].Value != "rotated" {
		t.Fatalf("rotated cookie not merged: %+v", back.Cookies)
	}

	missing := filepath.Join(t.TempDir(), "none.json")
	hc = &http.Client{Transport: SessionWriteBack(rewriteTransport{target}, missing)}
	if resp, err := hc.Get("https://" + host + "/"); err == nil {
		resp.Body.Close()
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("write-back created a session file: %v", err)
	}
}

func TestShopUsesConnSettings(t *testing.T) {
	t.Cleanup(func() { Configure(ConnSettings{Rate: -1}) })
	Configure(ConnSettings{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "proxy.example"}}, Timeout: 5 * time.Second, Rate: 0})
	sh, err := OpenWith(&Session{}, "", "test-market.edeka.shop")
	if err != nil {
		t.Fatal(err)
	}
	tr := sh.rec.next.(shopOnlyTransport).next.(*http.Transport)
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.ServerName != "proxy.example" {
		t.Fatalf("TLS settings not applied: %+v", tr.TLSClientConfig)
	}
	if sh.HTTP.Timeout != 5*time.Second {
		t.Fatalf("--timeout not applied: %s", sh.HTTP.Timeout)
	}
	if sh.limiter != nil {
		t.Fatal("--rate-limit 0 must turn pacing off")
	}
	if shopLimiter(-1) == nil || shopLimiter(1.5) == nil {
		t.Fatal("default and explicit rates need a limiter")
	}
}

func TestWordMatchTwoLetterWords(t *testing.T) {
	if !WordMatch("ei", "EDEKA Bio Eier 10 Stueck") {
		t.Fatal("'ei' must match 'Eier'")
	}
	if WordMatch("ei", "Weizenmehl Type 405") {
		t.Fatal("two-letter words must not match inside a word")
	}
	if WordMatch("e", "Eier") {
		t.Fatal("single letters are ignored")
	}
}

// Synthetic product pages: the main basket form decides the weight flag,
// recommendation tiles with their own weight inputs do not.
const weightDetailHTML = `<html><body><h1 id="productTitle">Test Brie</h1>
<form class="js-oxProductForm" method="post"><input type="hidden" name="aid" value="3000003MXKG">
<input type="text" name="persparam[Gewicht]" value="100"></form></body></html>`

const pieceDetailHTML = `<html><body><h1 id="productTitle">Test Milch</h1>
<form class="js-oxProductForm" method="post"><input type="hidden" name="aid" value="1000001MXST">
<input type="text" name="am" value="1"></form>
<div class="related"><form><input type="hidden" name="aid" value="3000003MXKG">
<input type="text" name="persparam[Gewicht]" value="100"></form></div></body></html>`

func TestParseProductDetailWeightArticle(t *testing.T) {
	d, err := ParseProductDetail(weightDetailHTML)
	if err != nil || !d.WeightArticle || d.ID != "3000003MXKG" {
		t.Fatalf("weight page: %+v %v", d, err)
	}
	d, err = ParseProductDetail(pieceDetailHTML)
	if err != nil || d.WeightArticle || d.ID != "1000001MXST" {
		t.Fatalf("piece page: %+v %v", d, err)
	}
}

// TestPersistAfterLogoutDoesNotRestoreSession: a command that was already
// running when 'auth logout' removed the session must not write it back.
func TestPersistAfterLogoutDoesNotRestoreSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s := &Session{Market: "a", Cookies: []Cookie{{Name: "sid", Value: "v1", Domain: "a.edeka.shop", Path: "/"}}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	sh, err := OpenWith(s, path, "a.edeka.shop")
	if err != nil {
		t.Fatal(err)
	}
	if err := ClearSession(path); err != nil {
		t.Fatal(err)
	}
	if err := sh.Persist(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Persist restored the cleared session: %v", err)
	}
}
