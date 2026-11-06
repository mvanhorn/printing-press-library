// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicPathParametersRemainSingleEncodedSegments(t *testing.T) {
	for _, tc := range []struct{ id, encoded string }{
		{".", "%2E"},
		{"..", "%2E%2E"},
		{"a/b?c#d% e", "a%2Fb%3Fc%23d%25%20e"},
	} {
		t.Run(tc.encoded, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.RequestURI != "/v1/videos/"+tc.encoded {
					t.Errorf("request URI = %q, want encoded single segment %q", r.RequestURI, tc.encoded)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"video":{"id":"fixture"}}`)
			}))
			defer s.Close()
			if _, err := runLessonCommand(newVideosGetCmd(lessonTestFlags(t, s)), tc.id); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("requests = %d, want 1", calls)
			}
		})
	}
}
