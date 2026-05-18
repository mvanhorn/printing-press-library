package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/namecheap/internal/config"
)

func TestNamecheapPrepareRequestInjectsAuthAndCommand(t *testing.T) {
	cfg := &config.Config{APIUser: "user", APIKey: "key", ClientIP: "203.0.113.10"}
	c := New(cfg, time.Second, 0)
	path, params, err := c.prepareNamecheapRequest("/xml.response/domains/check", map[string]string{"DomainList": "example.com"})
	if err != nil {
		t.Fatalf("prepareNamecheapRequest returned error: %v", err)
	}
	if path != "/xml.response" {
		t.Fatalf("path = %q, want /xml.response", path)
	}
	want := map[string]string{
		"Command":    "namecheap.domains.check",
		"ApiUser":    "user",
		"UserName":   "user",
		"ApiKey":     "key",
		"ClientIp":   "203.0.113.10",
		"DomainList": "example.com",
	}
	for k, v := range want {
		if params[k] != v {
			t.Fatalf("params[%s] = %q, want %q (all params: %#v)", k, params[k], v, params)
		}
	}
}

func TestNamecheapXMLResponseIsConvertedToJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/xml.response" {
			t.Fatalf("path = %q, want /xml.response", got)
		}
		if got := r.URL.Query().Get("Command"); got != "namecheap.users.getBalances" {
			t.Fatalf("Command = %q", got)
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<ApiResponse Status="OK"><CommandResponse Type="namecheap.users.getBalances"><UserGetBalancesResult Currency="USD" AvailableBalance="12.34" AccountBalance="15.00" /></CommandResponse></ApiResponse>`))
	}))
	defer srv.Close()

	cfg := &config.Config{BaseURL: srv.URL, APIUser: "user", APIKey: "key", ClientIP: "203.0.113.10"}
	c := New(cfg, time.Second, 0)
	data, err := c.Get("/xml.response/users/get-balances", nil)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, data)
	}
	api := decoded["ApiResponse"].(map[string]any)
	if api["Status"] != "OK" {
		t.Fatalf("Status = %#v", api["Status"])
	}
}
