package jalan

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestFetchDocumentCharsetIgnoresScriptCharset(t *testing.T) {
	original := `<html><head><meta http-equiv="Content-Type" content="text/html; charset=Shift_JIS"><script charset=utf-8 src="/public.js"></script></head><body>箱根の宿 髙橋</body></html>`
	raw, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(original))
	if err != nil {
		t.Fatal(err)
	}
	c := testClient(t, func(*http.Request) (*http.Response, error) {
		response := httpResult(http.StatusOK, string(raw))
		response.Header.Set("Content-Type", "text/html; charset=Windows-31J")
		return response, nil
	})
	session := c.newSession()
	decoded, err := c.fetch(context.Background(), "https://www.jalan.net/yad385995/plan/", session)
	if err != nil {
		t.Fatalf("fetch document decoding failed: %v (cause %v)", err, errors.Unwrap(err))
	}
	if decoded != original || session.requests != 1 {
		t.Fatalf("document encoding changed: decoded=%q requests=%d", decoded, session.requests)
	}
}

func TestDocumentEncodingAlternatives(t *testing.T) {
	for _, tc := range []struct{ name, body, header string }{
		{"cp932 extension without script", `<meta charset=Shift_JIS>箱根 髙橋`, "text/html; charset=Windows-31J"},
		{"ordinary Japanese with script", `<meta charset=Shift_JIS><script charset=utf-8></script>箱根`, "text/html; charset=Windows-31J"},
		{"shift_jis header with script", `<meta charset=Shift_JIS><script charset=utf-8></script>箱根`, "text/html; charset=Shift_JIS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeHTML(raw, tc.header)
			if err != nil || decoded != tc.body {
				t.Fatalf("document decode=%q error=%v", decoded, err)
			}
		})
	}
}

func TestDocumentCharsetUsesMIMEBeforeMetaAndRejectsInvalidUTF8(t *testing.T) {
	original := `<meta charset=utf-8><script charset=utf-8></script>箱根 髙橋`
	raw, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(original))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeHTML(raw, "text/html; charset=Windows-31J")
	if err != nil || decoded != original {
		t.Fatalf("MIME precedence lost: decoded=%q error=%v", decoded, err)
	}
	if _, err := decodeHTML(raw, "text/html; charset=UTF-8"); err == nil {
		t.Fatal("invalid explicitly UTF-8 document silently decoded")
	}
	// Invalid bytes outside the encoding prescan must also be rejected.
	damaged := append([]byte(strings.Repeat(" ", 2048)), 0x81)
	if _, err := decodeHTML(damaged, "text/html; charset=UTF-8"); err == nil {
		t.Fatal("invalid late UTF-8 byte silently decoded")
	}
}

func TestDocumentMetaCharsetIgnoresScriptAndParseErrorsDoNotRetry(t *testing.T) {
	original := `<meta charset=Shift_JIS><script charset=utf-8></script>箱根`
	raw, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(original))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeHTML(raw, "text/html")
	if err != nil || decoded != original {
		t.Fatalf("document meta charset lost: decoded=%q error=%v", decoded, err)
	}
	c := testClient(t, func(*http.Request) (*http.Response, error) {
		response := httpResult(200, string(raw))
		response.Header.Set("Content-Type", "text/html; charset=UTF-8")
		return response, nil
	})
	session := c.newSession()
	_, err = c.fetch(context.Background(), "https://www.jalan.net/yad385995/plan/", session)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "parse_failure" || session.requests != 1 || !strings.Contains(typed.Hint, "HTML document encoding utf-8") {
		t.Fatalf("safe encoding diagnostic or retry bound lost: error=%v requests=%d", err, session.requests)
	}
}
