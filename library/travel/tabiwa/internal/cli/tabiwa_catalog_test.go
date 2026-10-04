package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/cliutil/testenv"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogComparisonKeepsDatedAbsenceUnknown(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("usage_date") != "" {
			w.Write([]byte(`{"response":[{"id":"J0001900","name":"ポイント券","price":"2,000P","is_point_only":true}]}`))
		} else {
			w.Write([]byte(`{"response":[{"id":"J0001900","name":"ポイント券","price":"2,000P","is_point_only":true},{"id":"J0000900","name":"現金見積券","price":"3,600円"}]}`))
		}
	}))
	defer s.Close()
	t.Setenv("TABIWA_BASE_URL", s.URL)
	cmd := RootCmd()
	cmd.SetArgs([]string{"catalog", "compare", "J0001900", "J0000900", "--region", "10", "--on", "2026-10-28", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Comparisons []struct {
			ID           string `json:"id"`
			Date         string `json:"requested_date_membership"`
			Availability string `json:"availability"`
		} `json:"comparisons"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v output=%s", err, out.String())
	}
	if requests != 2 || len(result.Comparisons) != 2 || result.Comparisons[1].Date != "not_listed" || result.Comparisons[1].Availability != "unknown" {
		t.Fatalf("wrong date/stock inference: %+v requests=%d", result, requests)
	}
}
func TestCatalogSearchHonorsScanCapBeforeOutputLimit(t *testing.T) {
	testenv.Isolate(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response":[{"id":"J0000900","name":"別の券"},{"id":"J0001900","name":"ポイント券"}]}`))
	}))
	defer s.Close()
	t.Setenv("TABIWA_BASE_URL", s.URL)
	cmd := RootCmd()
	cmd.SetArgs([]string{"catalog", "search", "ポイント", "--max-records", "1", "--limit", "3", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Products []any           `json:"products"`
		Coverage catalogCoverage `json:"coverage"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Products) != 0 || result.Coverage.Scanned != 1 || result.Coverage.Complete {
		t.Fatalf("misleading coverage %+v", result)
	}
}
