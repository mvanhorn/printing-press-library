// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWriteKeyAliasesAndNilConfig(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	for _, verb := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, tc := range []struct {
			name    string
			cfg     *config.Config
			headers map[string]string
			want    string
		}{
			{"nil config", nil, nil, ""},
			{"empty aliases", &config.Config{Headers: map[string]string{"IDEMPOTENCY-KEY": "", "x-idempotency-key": " "}}, map[string]string{"idempotency-key": "", "X-IDEMPOTENCY-KEY": ""}, ""},
			{"mixed override", &config.Config{Headers: map[string]string{"idempotency-key": "config-key"}}, map[string]string{"iDeMpOtEnCy-KeY": " override-key ", "Idempotency-Key": ""}, "override-key"},
			{"x alias", &config.Config{}, map[string]string{"x-IDEMPOTENCY-key": "alias-key"}, "alias-key"},
		} {
			t.Run(verb+"/"+tc.name, func(t *testing.T) {
				original := map[string]string{}
				for k, v := range tc.headers {
					original[k] = v
				}
				calls := 0
				var keys []string
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					keys = append(keys, r.Header.Get("Idempotency-Key"))
					if len(r.Header.Values("Idempotency-Key")) != 1 || keys[len(keys)-1] == "" || len(r.Header.Values("X-Idempotency-Key")) != 0 {
						t.Error("noncanonical or empty wire identity", r.Header)
					}
					w.Header().Set("Content-Type", "application/json")
					if calls == 1 {
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(429)
						fmt.Fprint(w, `{"error":"rate_limited"}`)
						return
					}
					fmt.Fprint(w, `{}`)
				}))
				defer s.Close()
				c := New(&config.Config{BaseURL: s.URL}, time.Second, 1000)
				c.Config = tc.cfg
				c.NoCache = true
				if _, _, err := c.do(context.Background(), verb, "/v1/test", nil, nil, tc.headers); err != nil {
					t.Fatal(err)
				}
				if calls != 2 || keys[0] != keys[1] || (tc.want != "" && keys[0] != tc.want) {
					t.Fatal(keys, tc.want)
				}
				if tc.headers != nil && !reflect.DeepEqual(original, tc.headers) {
					t.Fatal("caller headers mutated")
				}
			})
		}
	}
}
func TestConflictingWriteAliasesRefuseAndReadsHaveNoKey(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Idempotency-Key") != "" || r.Header.Get("X-Idempotency-Key") != "" {
			t.Error("read carries write key")
		}
		fmt.Fprint(w, `{}`)
	}))
	defer s.Close()
	c := New(&config.Config{BaseURL: s.URL, Headers: map[string]string{"idempotency-key": "configured"}}, time.Second, 1000)
	c.NoCache = true
	if _, _, err := c.PostWithHeaders(t.Context(), "/v1/test", nil, map[string]string{"Idempotency-Key": "a", "IDEMPOTENCY-KEY": "b"}); err == nil || !strings.Contains(err.Error(), "conflicting") || calls != 0 {
		t.Fatal(err, calls)
	}
	if _, err := c.GetWithHeaders(t.Context(), "/v1/test", nil, map[string]string{"X-Idempotency-Key": "read-override"}); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
}
