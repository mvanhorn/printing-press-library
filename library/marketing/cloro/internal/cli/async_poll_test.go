// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/cloro/internal/cliutil/testenv"
)

// async get and async queue return values that change between calls. Each
// call must reach the API, not return a cached response from an earlier call.
func TestAsyncPollCommandsSkipResponseCache(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		path string
	}{
		{name: "get", args: []string{"async", "get", "task-1", "--json"}, path: "/v1/async/task/task-1"},
		{name: "queue", args: []string{"async", "queue", "--json"}, path: "/v1/async/status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				n := calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"success":true,"call":%d}`, n)
			}))
			defer srv.Close()
			t.Setenv("CLORO_BASE_URL", srv.URL)
			t.Setenv("CLORO_API_KEY", "test-key")

			for want := 1; want <= 2; want++ {
				cmd := RootCmd()
				cmd.SetArgs(tc.args)
				var out, errOut bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&errOut)
				if err := cmd.Execute(); err != nil {
					t.Fatalf("call %d error = %v\nstderr: %s", want, err, errOut.String())
				}
				if !strings.Contains(out.String(), fmt.Sprintf(`"call": %d`, want)) {
					t.Fatalf("call %d output = %s, want the response of API call %d", want, out.String(), want)
				}
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("API calls = %d, want 2", got)
			}
		})
	}
}
