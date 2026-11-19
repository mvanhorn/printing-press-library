package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

// TestLogoutClearsImportedSession: after 'auth logout' no shopping command
// may still log in from the imported session file.
func TestLogoutClearsImportedSession(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "session.json")
	t.Setenv("EDEKA_SESSION_FILE", path)
	if err := (&edeka.Session{Cookies: []edeka.Cookie{{Name: "sid", Value: "v", Domain: "m.edeka.shop", Path: "/"}}}).Save(path); err != nil {
		t.Fatal(err)
	}
	if out, err := runRoot(t, "auth", "logout"); err != nil {
		t.Fatalf("auth logout: %v\n%s", err, out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("imported session survived logout: %v", err)
	}
}

// TestOrderConfirmRefusesEarliest: a confirmation must not pick whatever slot
// is earliest now; it needs the exact slot of the accepted preview.
func TestOrderConfirmRefusesEarliest(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("EDEKA_SESSION_FILE", filepath.Join(t.TempDir(), "none.json"))
	out, err := runRoot(t, "order", "place", "--slot", "earliest", "--payment", "mxcash", "--confirm", "--expect-total", "597")
	if err == nil || !strings.Contains(out+err.Error(), "exact slot") {
		t.Fatalf("want refusal naming the exact slot, got err=%v out=%s", err, out)
	}
}

func TestBadCACertIsConfigError(t *testing.T) {
	testenv.Isolate(t)
	t.Cleanup(func() { edeka.Configure(edeka.ConnSettings{Rate: -1}) })
	flags := &rootFlags{caCert: filepath.Join(t.TempDir(), "missing.pem"), rateLimit: -1}
	var ce *cliError
	if err := applyShopConn(flags); !errors.As(err, &ce) {
		t.Fatalf("want a typed config error, got %v", err)
	}
}

// TestCollectOffersContinuation: a --limit cut inside a page must continue at
// the first offer not returned, never skip the rest of that page.
func TestCollectOffersContinuation(t *testing.T) {
	pages := map[int]edeka.Grid{}
	for p := 0; p < 2; p++ {
		g := edeka.Grid{HasNext: p == 0}
		for i := 0; i < 32; i++ {
			g.Products = append(g.Products, edeka.Product{ID: fmt.Sprintf("%d%03dMXST", p+1, i), Title: "Offer"})
		}
		pages[p] = g
	}
	fetch := func(p int) (edeka.Grid, error) { return pages[p], nil }
	seen := map[string]bool{}
	page, skip := 0, 0
	for round := 0; round < 10; round++ {
		w, err := collectOffers(fetch, page, skip, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range w.Offers {
			if seen[o.ID] {
				t.Fatalf("offer %s returned twice", o.ID)
			}
			seen[o.ID] = true
		}
		if w.NextPage == nil {
			break
		}
		page, skip = *w.NextPage, w.NextSkip
	}
	if len(seen) != 64 {
		t.Fatalf("following next_page/next_skip returned %d of 64 offers", len(seen))
	}
	w, _ := collectOffers(fetch, 0, 0, 32, "")
	if w.NextPage == nil || *w.NextPage != 1 || w.NextSkip != 0 {
		t.Fatalf("whole page: next=%v skip=%d", w.NextPage, w.NextSkip)
	}
}
