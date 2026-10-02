package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTravelokaTest(t *testing.T, argv ...string) (map[string]any, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errout bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errout)
	base := []string{"--home", t.TempDir(), "--no-learn", "--json"}
	cmd.SetArgs(append(base, argv...))
	err := cmd.Execute()
	var v map[string]any
	if e := json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatalf("machine output is not JSON: %s (%v; stderr=%s)", out.String(), err, errout.String())
	}
	return v, err
}
func TestTravelokaDryRunSkipsSessionsAndSnapshotIO(t *testing.T) {
	testenv.Isolate(t)
	for _, path := range [][]string{{"resolve"}, {"flights", "search"}, {"flights", "inspect"}, {"hotels", "search"}, {"hotels", "rooms"}, {"quotes", "compare"}, {"auth", "import-session"}, {"auth", "capture", "--launch"}} {
		t.Run(strings.Join(path, "-"), func(t *testing.T) {
			v, e := runTravelokaTest(t, append(path, "--session-file", "/SIMULATED/missing-session", "--dry-run")...)
			if e != nil || v["dry_run"] != true {
				t.Fatalf("dry run performed required IO: %v %v", v, e)
			}
		})
	}
}
func TestTravelokaCLIInvalidInputsBeforeNetwork(t *testing.T) {
	testenv.Isolate(t)
	cases := []struct {
		name string
		args []string
	}{{"calendar", []string{"flights", "search", "--origin", "SIN", "--destination", "CGK", "--depart", "2026-02-30"}}, {"adult", []string{"hotels", "search", "--geo-id", "10000045", "--check-in", "2027-01-06", "--check-out", "2027-01-08", "--adults", "0"}}, {"ages", []string{"hotels", "rooms", "--property-id", "9000000001714", "--check-in", "2027-01-06", "--check-out", "2027-01-08", "--children", "2", "--child-ages", "8"}}, {"mode", []string{"flights", "inspect", "--data-source", "live"}}, {"files", []string{"auth", "import-session"}}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			v, e := runTravelokaTest(t, tt.args...)
			ae, _ := v["error"].(map[string]any)
			if e == nil || (ae["code"] != "INVALID_INPUT" && ae["code"] != "UNSUPPORTED_OPERATION") {
				t.Fatalf("invalid input was hidden or reached auth: %v %v", v, e)
			}
		})
	}
}
func TestTravelokaComparePreservesExactPricesAndRetrievals(t *testing.T) {
	testenv.Isolate(t)
	scale := 2
	s := &traveloka.Snapshot{ID: "SIMULATED-comparison", Kind: "flights", RetrievedAt: "2026-10-02T00:00:00Z", Query: traveloka.Query{Kind: "flights", Market: "SG", Locale: "en-SG", Currency: "SGD"}, Offers: []traveloka.Offer{{ID: "high", Details: map[string]any{"price_basis": "party_trip_total"}, Price: traveloka.Price{Total: &traveloka.Money{Currency: "SGD", MinorUnits: "9007199254740993", Decimals: &scale}}}, {ID: "low", Details: map[string]any{"price_basis": "party_trip_total"}, Price: traveloka.Price{Total: &traveloka.Money{Currency: "SGD", MinorUnits: "9007199254740992", Decimals: &scale}}}, {ID: "unknown"}}}
	p := filepath.Join(t.TempDir(), "SIMULATED-snapshot.json")
	if e := traveloka.SaveSnapshotFile(p, s); e != nil {
		t.Fatal(e)
	}
	v, e := runTravelokaTest(t, "quotes", "compare", "--snapshots", p, "--limit", "2")
	if e != nil {
		t.Fatal(e)
	}
	rows := v["offers"].([]any)
	a := rows[0].(map[string]any)
	o := a["offer"].(map[string]any)
	if o["id"] != "low" || a["retrieved_at"] != s.RetrievedAt || v["price_basis"] != "party_trip_total" || v["freshness"] != "saved_snapshot" || v["truncated"] != true {
		t.Fatalf("comparison lost identity/precision/provenance: %v", v)
	}
	if len(v["retrievals"].([]any)) != 1 || len(rows) != 2 {
		t.Fatal("comparison bounds or retrievals lost")
	}
}

func TestTravelokaExistingEmptyHistoryIsOrdinaryCacheState(t *testing.T) {
	testenv.Isolate(t)
	p := filepath.Join(t.TempDir(), "SIMULATED-empty.db")
	db, e := store.OpenWithContext(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	v, e := runTravelokaTest(t, "flights", "inspect", "--db", p)
	if e != nil || v["status"] != "empty_local_cache" {
		t.Fatalf("empty store became an upstream failure: %v %v", v, e)
	}
	v, e = runTravelokaTest(t, "flights", "inspect", "--db", p, "--snapshot-id", "SIMULATED-absent")
	ae, _ := v["error"].(map[string]any)
	if e == nil || ae["code"] != "NOT_FOUND" {
		t.Fatalf("explicit ID absence was hidden: %v %v", v, e)
	}
}

type travelokaTestRoundTripper func(*http.Request) (*http.Response, error)

func (fn travelokaTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func TestTravelokaRawReplayUsesPerRequestShopperContext(t *testing.T) {
	dir := t.TempDir()
	cookies := filepath.Join(dir, "example-cookies.json")
	requests := filepath.Join(dir, "SIMULATED-requests.json")
	session := filepath.Join(dir, "SIMULATED-session.json")
	if e := os.WriteFile(cookies, []byte(`[{"name":"tv_cs","value":"example-cookie","domain":".traveloka.com","path":"/","expires":-1,"secure":true}]`), 0600); e != nil {
		t.Fatal(e)
	}
	reqs := []map[string]any{{"method": "POST", "url": "https://www.traveloka.com/api/v2/airport/search-nexus", "headers": map[string]string{}, "body": `{"fields":[],"data":{"query":"Singapore","currency":"SGD"},"clientInterface":"desktop"}`}}
	b, _ := json.Marshal(reqs)
	if e := os.WriteFile(requests, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := traveloka.ImportSession(cookies, requests, session); e != nil {
		t.Fatal(e)
	}
	src, e := traveloka.NewClient(session)
	if e != nil {
		t.Fatal(e)
	}
	var currency, market, locale, payloadCurrency string
	src.SetHTTPTransport(travelokaTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		currency = r.Header.Get("Tv-Currency")
		market = r.Header.Get("Tv-Country")
		locale = r.Header.Get("Tv-Language")
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		payloadCurrency, _ = body["data"].(map[string]any)["currency"].(string)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"sections":[]}}`))}, nil
	}))
	r, e := http.NewRequestWithContext(context.WithValue(context.Background(), travelokaShopperContextKey{}, traveloka.Shopper{Market: "SG", Locale: "en-SG", Currency: "USD"}), "POST", "https://www.traveloka.com/api/v2/airport/search-nexus", strings.NewReader(`{"data":{"query":"Singapore","currency":"SGD"}}`))
	if e != nil {
		t.Fatal(e)
	}
	res, e := (&travelokaReplayTransport{source: src}).RoundTrip(r)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	if currency != "USD" || payloadCurrency != "USD" || market != "SG" || locale != "en_SG" {
		t.Fatalf("context ignored in replay: %s %s %s %s", currency, payloadCurrency, market, locale)
	}
}

func TestTravelokaRateLimitPreservesSourceTypeAndExit(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	f := &rootFlags{asJSON: true}
	source := &cliutil.RateLimitError{URL: "https://www.traveloka.com/api/v2/airport/search-nexus", Body: "SIMULATED Traveloka rate limit", Cause: &traveloka.APIError{Code: "RATE_LIMITED", Message: "Traveloka returned HTTP 429", Status: 429, Retryable: true}}
	e := travelokaFail(cmd, f, source)
	var ce *cliError
	var re *cliutil.RateLimitError
	if !errors.As(e, &ce) || ce.code != 7 || !errors.As(e, &re) || re.URL != source.URL {
		t.Fatalf("source throttle became generic API failure: %v", e)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	v := result["error"].(map[string]any)
	if v["code"] != "RATE_LIMITED" || v["status"] != float64(429) || v["retryable"] != true {
		t.Fatalf("source throttle output lost semantics: %v", v)
	}
}

// Simulated public snapshot regressions: these do not claim live source evidence.
func TestTravelokaCompareRejectsConflictingMoneyAndPriceUnits(t *testing.T) {
	testenv.Isolate(t)
	scale := 2
	for _, tc := range []struct{ name, amount, unit string }{{"conflicting-money", "500.00", "party_trip_total"}, {"per-passenger-unit", "50.00", "per_passenger"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := &traveloka.Snapshot{ID: "SIMULATED-invalid-comparison", Kind: "flights", RetrievedAt: "2026-10-02T00:00:00Z", Query: traveloka.Query{Kind: "flights", Market: "SG", Locale: "en-SG", Currency: "SGD"}, Offers: []traveloka.Offer{{ID: "SIMULATED-offer", Details: map[string]any{"price_basis": tc.unit}, Price: traveloka.Price{Total: &traveloka.Money{Currency: "SGD", Amount: tc.amount, MinorUnits: "5000", Decimals: &scale}}}}}
			p := filepath.Join(t.TempDir(), "SIMULATED-snapshot.json")
			if e := traveloka.SaveSnapshotFile(p, s); e != nil {
				t.Fatal(e)
			}
			v, e := runTravelokaTest(t, "quotes", "compare", "--snapshots", p)
			problem, _ := v["error"].(map[string]any)
			if e == nil || problem["code"] != "INVALID_INPUT" {
				t.Fatalf("invalid comparison was accepted: %v %v", v, e)
			}
		})
	}
}
func TestTravelokaCompareKeepsUnknownPriceBasisUnranked(t *testing.T) {
	testenv.Isolate(t)
	scale := 2
	money := func(v string) *traveloka.Money {
		return &traveloka.Money{Currency: "SGD", MinorUnits: v, Decimals: &scale}
	}
	s := &traveloka.Snapshot{ID: "SIMULATED-unknown-unit", Kind: "flights", RetrievedAt: "2026-10-02T00:00:00Z", Query: traveloka.Query{Kind: "flights", Currency: "SGD"}, Offers: []traveloka.Offer{{ID: "unknown-unit", Price: traveloka.Price{Total: money("1")}}, {ID: "known-total", Details: map[string]any{"price_basis": "party_trip_total"}, Price: traveloka.Price{Total: money("9000")}}}}
	p := filepath.Join(t.TempDir(), "SIMULATED-snapshot.json")
	if e := traveloka.SaveSnapshotFile(p, s); e != nil {
		t.Fatal(e)
	}
	v, e := runTravelokaTest(t, "quotes", "compare", "--snapshots", p)
	if e != nil {
		t.Fatal(e)
	}
	rows := v["offers"].([]any)
	first := rows[0].(map[string]any)
	last := rows[1].(map[string]any)
	if first["offer"].(map[string]any)["id"] != "known-total" || last["comparison_status"] != "unknown_price_basis" {
		t.Fatalf("unknown units entered ranking: %v", v)
	}
}
