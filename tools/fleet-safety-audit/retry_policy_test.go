package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewedRetryPoliciesArePinned(t *testing.T) {
	for _, tc := range []struct{ path, function, guard string }{
		{"commerce/fedex", "send", "canRetry"},
		{"project-management/paperclip-self-hosted", "doInternal", "requestCanRetry(method, readOnlyIntent)"},
		{"marketing/dataforseo", "do", "requestCanRetry(req)"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			path := "library/" + tc.path + "/internal/client/client.go"
			data, err := os.ReadFile(filepath.Join("../..", path))
			if err != nil {
				t.Fatal(err)
			}
			if got := reviewedRetryGuard(path, tc.function, data); got != tc.guard {
				t.Fatalf("guard=%q want=%q", got, tc.guard)
			}
			start := bytes.Index(data, []byte("func requestCanRetry("))
			if start < 0 {
				start = bytes.Index(data, []byte("func canRetryAmbiguousFailure("))
			}
			if start < 0 {
				t.Fatal("missing reviewed helper")
			}
			widened := append(append([]byte{}, data[:start]...), bytes.Replace(data[start:], []byte("return false"), []byte("return true"), 1)...)
			defer func() {
				if recover() == nil {
					t.Error("widened policy silently retained approval")
				}
			}()
			reviewedRetryGuard(path, tc.function, widened)
		})
	}
}

func TestTransportPositiveGuardIsAlreadySafe(t *testing.T) {
	in := bytes.Replace([]byte(retryFixture), []byte("lastErr = err\n\t\t\tcontinue"), []byte("lastErr = err\nif attempt < maxRetries && canRetryAmbiguousFailure { continue }\nreturn nil,0,lastErr"), 1)
	in = bytes.Replace(in, []byte("const maxRetries = 3"), []byte("canRetryAmbiguousFailure := method == http.MethodGet\nconst maxRetries = 3"), 1)
	in = bytes.Replace(in, []byte("resp.StatusCode >= 500 && attempt < maxRetries"), []byte("resp.StatusCode >= 500 && attempt < maxRetries && canRetryAmbiguousFailure"), 1)
	if got := retrofitRetry(in, &cluster{files: map[string]bool{}}, "client.go"); !bytes.Equal(got, in) {
		t.Fatalf("rewrote safe positive guard:\n%s", got)
	}
}

func TestDisjunctiveServerConditionIsGatedAsAWhole(t *testing.T) {
	in := bytes.Replace([]byte(retryFixture), []byte("resp.StatusCode >= 500 && attempt < maxRetries"), []byte("(resp.StatusCode >= 500 && attempt < maxRetries) || forceRetry"), 1)
	got := retrofitRetry(in, &cluster{files: map[string]bool{}}, "client.go")
	if !bytes.Contains(got, []byte("((resp.StatusCode >= 500 && attempt < maxRetries) || forceRetry) && canRetryAmbiguousFailure")) {
		t.Fatalf("OR bypasses guard:\n%s", got)
	}
}
