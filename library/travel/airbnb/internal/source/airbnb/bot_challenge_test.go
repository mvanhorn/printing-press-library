package airbnb

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
)

func mkResp(status int, headers map[string]string, body string) *http.Response {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       http.NoBody,
		Request: &http.Request{
			Header: http.Header{},
		},
	}
}

func mkRespWithCookie(status int, cookies []*http.Cookie, body string) *http.Response {
	h := http.Header{}
	for _, c := range cookies {
		h.Add("Set-Cookie", c.String())
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       http.NoBody,
		Request:    &http.Request{Header: http.Header{}},
	}
}

func TestIsBotChallenge_DatadomeCookie(t *testing.T) {
	resp := mkRespWithCookie(403, []*http.Cookie{{Name: "datadome", Value: "abc123"}}, "")
	got, ok := isBotChallenge(resp, []byte(""))
	if !ok {
		t.Fatal("expected datadome cookie to be detected")
	}
	if got.ChallengeType != "datadome" {
		t.Errorf("ChallengeType = %q, want datadome", got.ChallengeType)
	}
	if !strings.Contains(got.Remediation, "auth login --chrome") {
		t.Errorf("Remediation missing auth-login hint: %q", got.Remediation)
	}
}

func TestIsBotChallenge_DatadomeServerHeader(t *testing.T) {
	resp := mkResp(403, map[string]string{"Server": "dd-13"}, "")
	got, ok := isBotChallenge(resp, []byte(""))
	if !ok {
		t.Fatal("expected dd- server header to be detected")
	}
	if got.ChallengeType != "datadome" {
		t.Errorf("ChallengeType = %q, want datadome", got.ChallengeType)
	}
	if got.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", got.StatusCode)
	}
}

func TestIsBotChallenge_DatadomeBodyMarker(t *testing.T) {
	body := `{"url":"https://geo.captcha-delivery.com/captcha/...","initialCid":"..."}`
	resp := mkResp(403, nil, body)
	got, ok := isBotChallenge(resp, []byte(body))
	if !ok {
		t.Fatal("expected captcha-delivery URL to be detected")
	}
	if got.ChallengeType != "datadome" {
		t.Errorf("ChallengeType = %q, want datadome", got.ChallengeType)
	}
}

func TestIsBotChallenge_AkamaiTitle(t *testing.T) {
	body := `<!doctype html><html><head><title>Bot or Not? | example.com</title></head></html>`
	resp := mkResp(403, nil, body)
	got, ok := isBotChallenge(resp, []byte(body))
	if !ok {
		t.Fatal("expected Akamai 'bot or not' title to be detected")
	}
	if got.ChallengeType != "akamai" {
		t.Errorf("ChallengeType = %q, want akamai", got.ChallengeType)
	}
	if !strings.Contains(got.Remediation, "sensor cooldown") {
		t.Errorf("Remediation missing sensor cooldown hint: %q", got.Remediation)
	}
}

func TestIsBotChallenge_AkamaiCaptchaPwa(t *testing.T) {
	body := `<html><body><script src="https://.../captcha-pwa.js"></script></body></html>`
	resp := mkResp(403, nil, body)
	got, ok := isBotChallenge(resp, []byte(body))
	if !ok {
		t.Fatal("expected captcha-pwa script to be detected")
	}
	if got.ChallengeType != "akamai" {
		t.Errorf("ChallengeType = %q, want akamai", got.ChallengeType)
	}
}

func TestIsBotChallenge_GenericForbidden(t *testing.T) {
	// 403 with no challenge signatures (e.g., geographic block, auth deny)
	// should NOT be classified as bot challenge.
	body := `{"error":"forbidden","message":"Not available in your region"}`
	resp := mkResp(403, map[string]string{"Server": "nginx"}, body)
	_, ok := isBotChallenge(resp, []byte(body))
	if ok {
		t.Fatal("generic 403 should not be classified as bot challenge")
	}
}

func TestIsBotChallenge_HappyPath200(t *testing.T) {
	// 200 OK with mundane content must not trigger detection.
	body := `{"data":{"price":100,"description":"A nice listing in Mercer Island"}}`
	resp := mkResp(200, nil, body)
	_, ok := isBotChallenge(resp, []byte(body))
	if ok {
		t.Fatal("200 OK with no signatures should not match")
	}
}

func TestIsBotChallenge_NilResp(t *testing.T) {
	_, ok := isBotChallenge(nil, []byte(""))
	if ok {
		t.Fatal("nil response should return false")
	}
}

func TestBotChallengeError_ErrorsAs(t *testing.T) {
	// Verify a returned BotChallengeError participates in errors.As routing.
	src := &cliutil.BotChallengeError{
		URL:           "https://example.com/api",
		ChallengeType: "datadome",
		StatusCode:    403,
		Remediation:   "wait",
	}
	var err error = src
	var got *cliutil.BotChallengeError
	if !errors.As(err, &got) {
		t.Fatal("errors.As should match *BotChallengeError")
	}
	if got.ChallengeType != "datadome" {
		t.Errorf("ChallengeType = %q, want datadome", got.ChallengeType)
	}
}
