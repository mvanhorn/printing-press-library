package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWheelogTransportWhitelistBeforeOutput(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		bad    bool
	}{{200, `{"result":[{"resultCode":0}],"content":{"spot":{"id":166345,"name":"Public restroom","spotCategory":{"category":"toilet","questionList":[]},"user":{"email":"PRIVATE"},"outline":"PRIVATE"}}}`, false}, {200, `{"result":[{"resultCode":"FAIL","message":"PRIVATE"}],"content":{}}`, true}, {503, `PRIVATE`, false}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("OS_CODE") != "3" || r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				t.Error("missing source headers")
			}
			w.Header().Set("Set-Cookie", "PRIVATE")
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		httpClient := &http.Client{Transport: WheelogTransport{}}
		request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+wheelogDetailPath, strings.NewReader(url.Values{"spotId": {"166345"}}.Encode()))
		response, err := httpClient.Do(request)
		if (err != nil) != tc.bad {
			t.Fatalf("unexpected error %v", err)
		}
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if strings.Contains(string(body), "PRIVATE") || response.Header.Get("Set-Cookie") != "" {
				t.Fatalf("private material survived %s", body)
			}
		}
		server.Close()
	}
}

func TestWheelogTransportRejectsUnsupportedRequests(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{{"POST", "/account/delete", ""}, {"GET", wheelogDetailPath, ""}, {"POST", wheelogDetailPath, "spotId=0"}, {"POST", wheelogSearchPath, "pagenum=6"}} {
		r, _ := http.NewRequest(tc.method, "https://app.wheelog.com"+tc.path, strings.NewReader(tc.body))
		if _, err := (WheelogTransport{}).RoundTrip(r); err == nil {
			t.Fatalf("accepted unsupported request %+v", tc)
		}
	}
}
