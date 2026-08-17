package wpupload

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
)

func TestDetectMIMEType(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	tests := []struct {
		name, file string
		data       []byte
		want       string
	}{
		{name: "extension corrects generic bytes", file: "photo.webp", data: []byte("not-enough-signature"), want: "image/webp"},
		{name: "detected when extension absent", file: "upload", data: pngHeader, want: "image/png"},
		{name: "jpeg extension", file: "photo.jpg", data: []byte("plain bytes"), want: "image/jpeg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectMIMEType(tt.file, tt.data); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMediaUploadEndpoint(t *testing.T) {
	tests := []struct{ name, base, want string }{
		{name: "pretty permalinks", base: "https://example.com/wp-json", want: "https://example.com/wp-json/wp/v2/media"},
		{name: "rest route", base: "https://example.com/?rest_route=/", want: "https://example.com/?rest_route=%2Fwp%2Fv2%2Fmedia"},
		{name: "index rest route", base: "https://example.com/index.php?rest_route=", want: "https://example.com/index.php?rest_route=%2Fwp%2Fv2%2Fmedia"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mediaUploadEndpoint(tt.base)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeUploadError(t *testing.T) {
	tests := []struct {
		name, body, wantCode, wantMessage string
		status                            int
	}{
		{name: "WordPress JSON", status: http.StatusPreconditionFailed, body: `{"code":"rest_upload_hash_mismatch","message":"hash mismatch"}`, wantCode: "rest_upload_hash_mismatch", wantMessage: "hash mismatch"},
		{name: "plain body", status: http.StatusBadGateway, body: "gateway unavailable", wantMessage: "gateway unavailable"},
		{name: "empty body", status: http.StatusServiceUnavailable, wantMessage: "Service Unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeUploadError(tt.status, []byte(tt.body))
			typed, ok := err.(*APIError)
			if !ok {
				t.Fatalf("error type = %T, want *APIError", err)
			}
			if typed.Code != tt.wantCode || typed.Message != tt.wantMessage || typed.StatusCode != tt.status {
				t.Fatalf("decoded error = %#v", typed)
			}
		})
	}
}

func TestUploadFileSendsRawBodyAndIntegrityHeaders(t *testing.T) {
	body := []byte("raw-wordpress-upload")
	wantMD5 := md5.Sum(body) // #nosec G401 -- asserting the protocol header.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/wp-json/wp/v2/media" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBody) != string(body) {
			t.Fatalf("body = %q, want %q", gotBody, body)
		}
		if got := r.Header.Get("Content-Disposition"); got != `attachment; filename="media.jpg"` {
			t.Fatalf("Content-Disposition = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Content-MD5"); got != base64.StdEncoding.EncodeToString(wantMD5[:]) {
			t.Fatalf("Content-MD5 = %q", got)
		}
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("Authorization header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "media.jpg")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{BaseURL: server.URL + "/wp-json", WordpressUser: "editor", WordpressAppPassword: "secret"}
	client := New(cfg, server.Client())
	data, status, err := client.UploadFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || string(data) != `{"id":42}` {
		t.Fatalf("status/data = %d %s", status, data)
	}
}
