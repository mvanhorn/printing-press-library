package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuoteUsesV33EndpointAndReturnsCompleteTotal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3.3/quotes" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var request quoteRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.PropertyUID != testUnitUIDs[4] || request.Guests != 2 {
			t.Fatalf("request = %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quote":{"totalAmount":159,"totalTaxAmount":19,"currency":"USD","currencySymbol":"$","rent":{"netPrice":125,"taxAmount":15,"grossPrice":140},"fees":{"cleaningFee":{"netPrice":15,"taxAmount":4,"grossPrice":19},"totalFeeAmount":19}}}`))
	}))
	defer server.Close()
	t.Setenv("HOSTFULLY_BASE_URL", server.URL+"/api/v3")
	t.Setenv("HOSTFULLY_AGENCY_ID", "agency-test")
	t.Setenv("HOSTFULLY_API_KEY", "test-key")
	flags := &rootFlags{agent: true}
	cmd := newNovelQuoteCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--unit", "4", "--check-in", "2026-09-02", "--check-out", "2026-09-03", "--guests", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"total_amount": 159`) || !strings.Contains(stdout.String(), `"source": "hostfully_live"`) {
		t.Fatalf("output = %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"base_rent": 125`) || !strings.Contains(stdout.String(), `"fees_gross": 19`) {
		t.Fatalf("breakdown output = %s", stdout.String())
	}
}

func TestQuoteHumanBreakdown(t *testing.T) {
	t.Setenv("HOSTFULLY_AGENCY_ID", "agency-test")
	enableMockForTest(t)
	flags := &rootFlags{}
	cmd := newNovelQuoteCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--unit", "4", "--check-in", "2026-09-02", "--check-out", "2026-09-03", "--guests", "2", "--breakdown"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Base rent", "Cleaning fee", "All fees", "Total taxes", "Complete total"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("missing %q in %s", want, stdout.String())
		}
	}
}

func TestRoundQuoteMoney(t *testing.T) {
	if got := roundQuoteMoney(11.12 + 12); got != 23.12 {
		t.Fatalf("roundQuoteMoney() = %v", got)
	}
}

func TestMockQuoteNeverCallsLoader(t *testing.T) {
	t.Setenv("HOSTFULLY_AGENCY_ID", "agency-test")
	enableMockForTest(t)
	flags := &rootFlags{agent: true}
	called := false
	cmd := newNovelQuoteCmdWithLoader(flags, func(_ *rootFlags, _ quoteRequest) (quoteAPIResponse, error) {
		called = true
		return quoteAPIResponse{}, nil
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--unit", "4", "--check-in", "2026-09-02", "--check-out", "2026-09-03", "--guests", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("mock quote called live loader")
	}
	if !strings.Contains(stdout.String(), `"total_amount": 134`) || !strings.Contains(stdout.String(), `"source": "mock"`) {
		t.Fatalf("output = %s", stdout.String())
	}
}
