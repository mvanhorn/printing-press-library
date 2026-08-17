// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Expensify wire-protocol request building. The Printing Press spec models auth
// as an `Authorization: ExpensifyToken {authToken}` header for tooling purposes,
// but the live Expensify API does NOT follow REST conventions and ignores that
// header. Instead it expects a form-encoded body that carries authToken plus
// every field as form fields. This file is a hand-authored patch over the
// generated REST client (recorded in .printing-press-patches.json as
// `expensify-form-wire`); it cannot be expressed in the spec format today.
//
// Two dispatcher shapes exist:
//  1. New Expensify (www.expensify.com/api/<Command>): form-encoded body with
//     authToken + flattened fields. Nested values are JSON-encoded strings.
//  2. Integration Server (integrations.expensify.com/Integration-Server/...):
//     a single "requestJobDescription" form field holding a JSON document with
//     credentials and the command body.

package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// buildExpensifyRequest returns the URL, form-encoded body bytes, and content
// type for an Expensify API request, routed by path prefix.
func (c *Client) buildExpensifyRequest(method, path string, body any) (string, []byte, string, error) {
	if strings.HasPrefix(path, "/Integration-Server/") {
		return c.buildIntegrationServerRequest(method, path, body)
	}
	return c.buildNewExpensifyRequest(method, path, body)
}

func (c *Client) buildNewExpensifyRequest(_, path string, body any) (string, []byte, string, error) {
	targetURL := "https://www.expensify.com/api" + path
	if c.Config != nil && c.Config.BaseURL != "" && !strings.Contains(c.Config.BaseURL, "www.expensify.com/api") {
		// Allow BaseURL override for tests / verify's mock server.
		targetURL = strings.TrimSuffix(c.Config.BaseURL, "/") + path
	}

	form := url.Values{}
	// authToken: required for every New Expensify command. Skip if unset so
	// dry-run still works and unauthenticated errors surface from the server's
	// response rather than an early client-side abort.
	if c.Config != nil && c.Config.ExpensifyAuthToken != "" {
		form.Set("authToken", c.Config.ExpensifyAuthToken)
	}
	// Standard parameters every /api call includes in practice. Expensify
	// validates `referer` against a known app-name allowlist — a custom name
	// gets rejected with jsonCode 666, so we mimic the web app's "ecash".
	form.Set("platform", "web")
	form.Set("referer", "ecash")
	form.Set("api_setCookie", "false")

	// Flatten the body into form fields. Strings/numbers/bools go directly;
	// maps and slices are JSON-encoded into a single string field.
	if body != nil {
		if m, ok := body.(map[string]any); ok {
			for k, v := range m {
				if v == nil {
					continue
				}
				switch x := v.(type) {
				case string:
					if x != "" {
						form.Set(k, x)
					}
				case bool:
					form.Set(k, strconv.FormatBool(x))
				case int:
					form.Set(k, strconv.Itoa(x))
				case int64:
					form.Set(k, strconv.FormatInt(x, 10))
				case float64:
					form.Set(k, strconv.FormatFloat(x, 'f', -1, 64))
				default:
					if jb, err := json.Marshal(v); err == nil {
						form.Set(k, string(jb))
					}
				}
			}
		} else {
			if jb, err := json.Marshal(body); err == nil {
				form.Set("body", string(jb))
			}
		}
	}

	return targetURL, []byte(form.Encode()), "application/x-www-form-urlencoded", nil
}

func (c *Client) buildIntegrationServerRequest(_, path string, body any) (string, []byte, string, error) {
	targetURL := "https://integrations.expensify.com" + path
	if c.Config != nil && c.Config.BaseURL != "" && strings.Contains(c.Config.BaseURL, "integrations.expensify.com") {
		targetURL = strings.TrimSuffix(c.Config.BaseURL, "/") + strings.TrimPrefix(path, "/Integration-Server")
	}

	wrapper := map[string]any{}
	if c.Config != nil {
		wrapper["credentials"] = map[string]string{
			"partnerUserID":     c.Config.ExpensifyPartnerUserId,
			"partnerUserSecret": c.Config.ExpensifyPartnerUserSecret,
		}
	}
	if m, ok := body.(map[string]any); ok {
		for k, v := range m {
			wrapper[k] = v
		}
	}

	jb, err := json.Marshal(wrapper)
	if err != nil {
		return "", nil, "", fmt.Errorf("marshaling requestJobDescription: %w", err)
	}
	form := url.Values{}
	form.Set("requestJobDescription", string(jb))
	return targetURL, []byte(form.Encode()), "application/x-www-form-urlencoded", nil
}

// embeddedJSONCode extracts Expensify's in-body jsonCode and message. Expensify
// replies HTTP 200 with the real status in jsonCode (200 = ok, 407 = session
// expired, etc.), so a 2xx HTTP status alone does not mean success.
func embeddedJSONCode(body []byte) (int, string) {
	var obj struct {
		JSONCode float64 `json:"jsonCode"`
		Message  string  `json:"message"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return 0, ""
	}
	return int(obj.JSONCode), obj.Message
}
