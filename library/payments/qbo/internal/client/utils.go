// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// binaryResponseEnvelope wraps a non-textual success body so it survives the
// json.RawMessage contract every consumer (CLI output, --json, MCP tools)
// depends on. Without it, raw bytes (PDF, zip, image) are corrupted by
// sanitizeJSONResponse and emitted as invalid JSON. The _pp_binary
// discriminator lets callers and agents detect and base64-decode the payload.
type binaryResponseEnvelope struct {
	PPBinary    bool   `json:"_pp_binary"`
	ContentType string `json:"content_type"`
	Encoding    string `json:"encoding"`
	Bytes       int    `json:"bytes"`
	Data        string `json:"data"`
}

// isBinaryResponseContentType reports whether a successful response with this
// Content-Type must be base64-wrapped instead of treated as text/JSON. It is
// deliberately narrow: JSON, */*, XML, and every text/* type (including
// text/html, so response_format:html CLIs are untouched) pass through
// unchanged. Only genuinely binary payloads are wrapped.
func isBinaryResponseContentType(ct string) bool {
	mt := strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	if mt == "" {
		return false
	}
	switch {
	case mt == "application/json", mt == "text/json", mt == "*/*":
		return false
	case strings.HasPrefix(mt, "text/"):
		return false
	case strings.HasSuffix(mt, "+json"), strings.HasSuffix(mt, "+xml"):
		return false
	case mt == "application/xml", mt == "application/xhtml+xml":
		return false
	case mt == "application/javascript", mt == "application/ecmascript",
		mt == "application/x-www-form-urlencoded", mt == "application/graphql":
		return false
	}
	return true
}

// wrapBinaryResponse marshals body into a self-describing base64 envelope.
func wrapBinaryResponse(ct string, body []byte) (json.RawMessage, error) {
	out, err := json.Marshal(binaryResponseEnvelope{
		PPBinary:    true,
		ContentType: ct,
		Encoding:    "base64",
		Bytes:       len(body),
		Data:        base64.StdEncoding.EncodeToString(body),
	})
	if err != nil {
		return nil, fmt.Errorf("encoding binary response: %w", err)
	}
	return json.RawMessage(out), nil
}

// sanitizeJSONResponse strips known JSONP/XSSI prefixes and UTF-8 BOM from
// response bodies so that downstream JSON parsing succeeds. For clean JSON
// responses these checks are no-ops.
func sanitizeJSONResponse(body []byte) []byte {
	// UTF-8 BOM
	body = bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF"))

	// JSONP/XSSI prefixes, ordered longest-first where prefixes overlap
	prefixes := [][]byte{
		[]byte(")]}'\n"),
		[]byte(")]}'"),
		[]byte("{}&&"),
		[]byte("for(;;);"),
		[]byte("while(1);"),
	}
	for _, p := range prefixes {
		if bytes.HasPrefix(body, p) {
			body = bytes.TrimPrefix(body, p)
			body = bytes.TrimLeft(body, " \t\r\n")
			break
		}
	}
	return body
}

// maskToken redacts all but the last 4 characters of a token for safe display.
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

type maskedError struct {
	msg string
}

func (e maskedError) Error() string {
	return e.msg
}

func (c *Client) displayURL(rawURL string, extraCredentials ...string) string {
	return c.maskCredentialText(rawURL, extraCredentials...)
}

func (c *Client) maskError(err error, extraCredentials ...string) error {
	if err == nil {
		return nil
	}
	raw := err.Error()
	msg := c.maskCredentialText(raw, extraCredentials...)
	if msg == raw {
		return err
	}
	return maskedError{msg: msg}
}

func (c *Client) maskCredentialText(text string, extraCredentials ...string) string {
	if text == "" {
		return text
	}
	type credentialMask struct {
		needle      string
		replacement string
	}
	var masks []credentialMask
	seen := map[string]struct{}{}
	addValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		replacement := maskToken(value)
		addMask := func(needle string) {
			if needle == "" {
				return
			}
			if _, ok := seen[needle]; ok {
				return
			}
			seen[needle] = struct{}{}
			masks = append(masks, credentialMask{needle: needle, replacement: replacement})
		}
		addMask(value)
		if escaped := url.QueryEscape(value); escaped != value {
			addMask(escaped)
		}
		if escaped := url.PathEscape(value); escaped != value {
			addMask(escaped)
		}
	}
	addCredential := func(value string) {
		value = strings.TrimSpace(value)
		addValue(value)
		if _, token, ok := strings.Cut(value, " "); ok {
			addValue(token)
		}
	}
	for _, value := range extraCredentials {
		addCredential(value)
	}
	if c != nil && c.Config != nil {
		addCredential(c.Config.AuthHeaderVal)
		addCredential(c.Config.AuthHeader())
		addCredential(c.Config.AccessToken)
		addCredential(c.Config.RefreshToken)
		addCredential(c.Config.ClientID)
		addCredential(c.Config.ClientSecret)
		addCredential(c.Config.QboRealmId)
	}
	sort.SliceStable(masks, func(i, j int) bool {
		return len(masks[i].needle) > len(masks[j].needle)
	})
	masked := text
	for _, mask := range masks {
		masked = strings.ReplaceAll(masked, mask.needle, mask.replacement)
	}
	return masked
}

func truncateBody(b []byte) string {
	const maxBytes = 4096
	if len(b) <= maxBytes {
		return string(b)
	}
	return strings.ToValidUTF8(string(b[:maxBytes]), "") + "..."
}

// The response cache stores text/JSON bodies as <hash>.json. Binary callers
// opt out so opaque blobs do not land under a misleading extension.
func (c *Client) wantsBinaryResponse(headers map[string]string) bool {
	binaryResponse := false
	if c != nil && c.Config != nil {
		if value, ok := binaryResponseHeaderValue(c.Config.Headers); ok {
			binaryResponse = value
		}
	}
	if value, ok := binaryResponseHeaderValue(headers); ok {
		binaryResponse = value
	}
	return binaryResponse
}

func binaryResponseHeaderValue(headers map[string]string) (bool, bool) {
	found := false
	for k, v := range headers {
		if strings.EqualFold(k, BinaryResponseHeader) {
			found = true
			if strings.EqualFold(v, "true") {
				return true, true
			}
		}
	}
	return false, found
}
