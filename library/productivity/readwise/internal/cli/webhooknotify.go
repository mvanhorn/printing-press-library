// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored: webhook delivery for the watch subsystem. Builds a signed
// JSON payload (HMAC-SHA256 → X-Readwise-Signature), validates the target
// URL against SSRF, and never fires real POSTs under the verify harness.

package cli

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
)

// watchDocItem is one document in a delivery payload.
type watchDocItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	URL     string `json:"url"`
	Summary string `json:"summary,omitempty"` // only when include_text
}

// watchHLItem is one highlight in a delivery payload.
type watchHLItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"`
	Text   string `json:"text,omitempty"` // only when include_text
}

// watchPayload is the POST body sent to each webhook.
type watchPayload struct {
	Kind        string `json:"kind"`
	Watch       string `json:"watch"`
	Count       int    `json:"count"`
	GeneratedAt string `json:"generated_at"`
	Items       any    `json:"items"`
}

// validWebhookURL rejects non-https and SSRF-prone targets. .local mDNS
// rejection is additive hardening beyond the RA baseline.
func validWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid webhook URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook URL must be https (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL has no host")
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("webhook URL may not target localhost")
	}
	// .local mDNS rejection — additive hardening (R4)
	if strings.HasSuffix(strings.ToLower(host), ".local") {
		return fmt.Errorf("webhook URL may not target .local (mDNS) addresses")
	}
	if ip := net.ParseIP(host); ip != nil && isWatchBlockedIP(ip) {
		return fmt.Errorf("webhook URL may not target a private/loopback/link-local address")
	}
	return nil
}

// isWatchBlockedIP reports whether an IP is in a range a webhook must never reach.
func isWatchBlockedIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range []string{"100.64.0.0/10", "0.0.0.0/8", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4", "64:ff9b::/96"} {
		if netip.MustParsePrefix(prefix).Contains(addr) {
			return true
		}
	}
	v6 := ip.To16()
	if ip.To4() == nil && len(v6) == 16 && v6[0] == 0x20 && v6[1] == 0x02 {
		return isWatchBlockedIP(net.IP(v6[2:6]))
	}
	return false
}

// redactWebhookURL never exposes userinfo, capability paths, query tokens or fragments.
func redactWebhookURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "[invalid webhook URL]"
	}
	suffix := ""
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		suffix = "/…"
	}
	return u.Scheme + "://" + u.Host + suffix
}

// safeWebhookClient returns an http.Client hardened against SSRF: a Control
// hook checks the POST-DNS-resolution dial address, caps redirects (≤5), and
// re-validates every redirect hop.
var webhookClientFactory = safeWebhookClient

func safeWebhookClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_ /*network*/, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("bad dial address %q: %w", address, err)
			}
			if isWatchBlockedIP(net.ParseIP(host)) {
				return fmt.Errorf("refusing to connect to blocked address %s", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many webhook redirects")
			}
			for _, previous := range via {
				if !strings.EqualFold(req.URL.Host, previous.URL.Host) {
					req.Header.Del("X-Readwise-Signature")
					break
				}
			}
			return validWebhookURL(req.URL.String())
		},
	}
}

// webhookWantsKind reports whether a webhook's events filter matches watchKind.
// "*" or empty matches everything; otherwise comma-separated allowlist.
func webhookWantsKind(filter, watchKind string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" || filter == "*" {
		return true
	}
	for _, t := range strings.Split(filter, ",") {
		if strings.EqualFold(strings.TrimSpace(t), watchKind) {
			return true
		}
	}
	return false
}

// signPayload computes HMAC-SHA256 over body with secret, returning hex.
func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// buildDocPayload builds a watchPayload for a docs watch.
// includeText adds "summary" when true; never includes notes/reading_progress/timestamps.
func buildDocPayload(watchName string, items []store.DocItem, includeText bool) ([]byte, error) {
	payloadItems := make([]watchDocItem, 0, len(items))
	for _, d := range items {
		item := watchDocItem{ID: d.ID, Title: d.Title, Author: d.Author, URL: d.URL}
		if includeText {
			item.Summary = d.Summary
		}
		payloadItems = append(payloadItems, item)
	}
	p := watchPayload{
		Kind:        "docs",
		Watch:       watchName,
		Count:       len(items),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Items:       payloadItems,
	}
	body, err := json.Marshal(p)
	if err == nil && len(body) > 256*1024 {
		return nil, fmt.Errorf("webhook payload exceeds 256 KiB cap")
	}
	return body, err
}

// buildHLPayload builds a watchPayload for a highlights watch.
// includeText adds "text" when true; never includes notes/timestamps.
func buildHLPayload(watchName string, items []store.HLItem, includeText bool) ([]byte, error) {
	payloadItems := make([]watchHLItem, 0, len(items))
	for _, h := range items {
		item := watchHLItem{ID: h.ID, Title: h.Title, Author: h.Author, URL: h.URL}
		if includeText {
			item.Text = h.Text
		}
		payloadItems = append(payloadItems, item)
	}
	p := watchPayload{
		Kind:        "highlights",
		Watch:       watchName,
		Count:       len(items),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Items:       payloadItems,
	}
	body, err := json.Marshal(p)
	if err == nil && len(body) > 256*1024 {
		return nil, fmt.Errorf("webhook payload exceeds 256 KiB cap")
	}
	return body, err
}

// postWebhook POSTs body to target with optional HMAC signature header.
// Uses the hardened safeWebhookClient. Logs url+status on failure (never body/secret).
func postWebhook(ctx context.Context, target, secret string, body []byte) error {
	if err := validWebhookURL(target); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("invalid webhook request to %s", redactWebhookURL(target))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "readwise-pp-cli")
	if secret != "" {
		req.Header.Set("X-Readwise-Signature", "sha256="+signPayload(secret, body))
	}
	c := webhookClientFactory()
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request to %s failed", redactWebhookURL(target))
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 64*1024)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// fireWatchWebhooks sends body to every webhook that matches watchKind.
// Under IsVerifyEnv() it prints "would POST to <url> (<n> items)" and returns
// success without making any network call. Never gates on IsDogfoodEnv().
// errW receives url+status error lines (never body/secret).
// Returns the number of webhooks attempted.
func fireWatchWebhooks(ctx context.Context, errW io.Writer, st *store.Store, watchKind string, body []byte, count int, failedIDs ...*[]int64) (int, error) {
	hooks, err := st.ListWebhooks(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	failed := 0
	for _, h := range hooks {
		if !webhookWantsKind(h.Events, watchKind) {
			continue
		}
		if cliutil.IsVerifyEnv() {
			// Network suppression: print intent only, never payload values
			fmt.Fprintf(errW, "would POST to %s (%d items)\n", redactWebhookURL(h.URL), count)
			sent++
			continue
		}
		recipientBody, err := webhookRecipientBody(body, h.IncludeText)
		if err != nil {
			return sent, err
		}
		deliveryErr := postWebhook(ctx, h.URL, h.Secret, recipientBody)
		if deliveryErr != nil {
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return sent, ctx.Err()
			case <-timer.C:
			}
			deliveryErr = postWebhook(ctx, h.URL, h.Secret, recipientBody)
		}
		if err := deliveryErr; err != nil {
			// failure log = url+status only (R13)
			fmt.Fprintf(errW, "webhook #%d failed: %v\n", h.ID, err)
			failed++
			for _, ids := range failedIDs {
				*ids = append(*ids, h.ID)
			}
			continue
		}
		sent++
	}
	if failed > 0 {
		return sent, fmt.Errorf("%d matching webhook(s) failed after retry; items retained", failed)
	}
	if sent == 0 {
		return 0, fmt.Errorf("no matching webhook succeeded (no matching hooks or delivery failed)")
	}
	return sent, nil
}

// webhookRecipientBody enforces the stored recipient consent at the delivery boundary.
func webhookRecipientBody(body []byte, includeText bool) ([]byte, error) {
	if includeText {
		return body, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if raw, ok := payload["items"]; ok {
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			delete(item, "text")
			delete(item, "summary")
		}
		encoded, err := json.Marshal(items)
		if err != nil {
			return nil, err
		}
		payload["items"] = encoded
	}
	return json.Marshal(payload)
}
