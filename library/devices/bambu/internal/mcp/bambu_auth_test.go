package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/config"
)

func TestMCPGenericClientStripsBambuCredentials(t *testing.T) {
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		captured = request.Header.Clone()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	cfg := &config.Config{
		BaseURL:         server.URL,
		BambuSerial:     "PRINTERSERIAL001",
		BambuAccessCode: "LOCALCODE123",
		AuthHeaderVal:   "LOCALCODE123",
		Headers:         map[string]string{"x-bambu-access-code": "LOCALCODE123"},
	}
	c := newMCPClientFromConfig(cfg)
	if _, err := c.Get(context.Background(), "/probe", nil); err != nil {
		t.Fatal(err)
	}
	headers, _ := json.Marshal(captured)
	if captured.Get("X-Bambu-Access-Code") != "" || strings.Contains(string(headers), "PRINTERSERIAL001") || strings.Contains(string(headers), "LOCALCODE123") {
		t.Fatalf("MCP HTTP request exposed Bambu credentials: %s", headers)
	}
}
