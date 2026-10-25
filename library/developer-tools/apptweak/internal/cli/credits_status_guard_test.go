package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCreditsStatusGuardedFreeRoute(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "journal")
	costs := filepath.Join(dir, "costs")
	if err := os.WriteFile(journal, []byte("{\"run_id\":\"test\",\"limit\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"GET https://public-api.apptweak.com/api/public/apptweak/usage/credits":0}`)
	if err := os.WriteFile(costs, data, 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(journal)
	st := info.Sys().(*syscall.Stat_t)
	sum := sha256.Sum256(data)
	t.Setenv("APPTWEAK_CREDIT_BUDGET_FILE", journal)
	t.Setenv("APPTWEAK_CREDIT_COSTS_FILE", costs)
	t.Setenv("APPTWEAK_CREDIT_RUN_ID", "test")
	t.Setenv("APPTWEAK_CREDIT_BUDGET_ID", fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	t.Setenv("APPTWEAK_CREDIT_COSTS_SHA256", hex.EncodeToString(sum[:]))
	hits := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":{"current_monthly_api_credits":3}}`)), Header: make(http.Header), Request: r}, nil
	})}
	if _, err := fetchCreditsStatus(httpClient, context.Background(), "test-key"); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
	content, _ := os.ReadFile(journal)
	if strings.Count(string(content), "\n") != 1 {
		t.Fatalf("free route reserved credits: %s", content)
	}
	if err := os.WriteFile(journal, []byte("corrupt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchCreditsStatus(httpClient, context.Background(), "test-key"); err == nil {
		t.Fatal("corrupt journal allowed free route")
	}
	if hits != 1 {
		t.Fatal("network reached with corrupt journal")
	}
}
