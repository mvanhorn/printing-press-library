package airbnb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

func TestIsBotChallenge_200WithDatadomeCookie(t *testing.T) {
	// Datadome's CDN-side pass-through sets a `Set-Cookie: datadome=...`
	// header on legitimate 200 responses (the cookie is a session token
	// for the Datadome layer, not a challenge marker). The detector must
	// NOT classify these as bot challenges or every Airbnb response would
	// silently break.
	body := `{"data":{"price":514}}`
	resp := mkRespWithCookie(200, []*http.Cookie{{Name: "datadome", Value: "session-token-abc"}}, body)
	_, ok := isBotChallenge(resp, []byte(body))
	if ok {
		t.Fatal("200 OK with datadome cookie should NOT trigger detection (regression test for Greptile finding on PR #740)")
	}
}

func TestIsBotChallenge_200WithDdServerHeader(t *testing.T) {
	// Same concern: a 200 from a Datadome-fronted origin will carry a
	// `Server: dd-<version>` header. The detector must NOT fire on these.
	body := `{"results":[]}`
	resp := mkResp(200, map[string]string{"Server": "dd-13"}, body)
	_, ok := isBotChallenge(resp, []byte(body))
	if ok {
		t.Fatal("200 OK with `Server: dd-*` should NOT trigger detection (regression test for Greptile finding on PR #740)")
	}
}

func TestDoReturnsBotChallengeOnFinal429Challenge(t *testing.T) {
	attempts := 0
	sleeps := 0
	body := `{"url":"https://geo.captcha-delivery.com/captcha/..."}`
	c := &Client{
		http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{
				StatusCode: 429,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		})},
		sleep: func(time.Duration) {
			sleeps++
		},
	}

	_, err := c.do(context.Background(), "GET", "https://example.com/api", airbnbUA, nil, nil)
	var challenge *cliutil.BotChallengeError
	if !errors.As(err, &challenge) {
		t.Fatalf("error = %v, want BotChallengeError", err)
	}
	if challenge.ChallengeType != "datadome" {
		t.Fatalf("ChallengeType = %q, want datadome", challenge.ChallengeType)
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want initial try plus 3 retries", attempts)
	}
	if sleeps != 3 {
		t.Fatalf("sleeps = %d, want one sleep before each retry", sleeps)
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
