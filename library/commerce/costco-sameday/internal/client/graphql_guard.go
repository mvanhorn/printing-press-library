// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored GraphQL safety guards (charge consent + variables fold).
// Preserved across regenerate via .printing-press-patches records.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type ctxKeyDeclaredOp struct{}
type ctxKeyChargeConsent struct{}

// chargeConsent records the two order-place gate flags. FinalizeCheckout is
// sent only when BOTH are true; either one alone is refused.
type chargeConsent struct {
	yes           bool
	confirmCharge bool
}

// WithDeclaredGraphQLOperation locks outbound GraphQL requests in ctx to op.
// Generated commands should pass their declared default; overrides and stdin
// bodies that name a different operation are rejected in doInternal.
func WithDeclaredGraphQLOperation(ctx context.Context, op string) context.Context {
	op = strings.TrimSpace(op)
	if op == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyDeclaredOp{}, op)
}

// WithChargeConsent records the caller's --yes and --confirm-charge flags.
// The client refuses any request that targets FinalizeCheckout unless BOTH
// are true. Only `order place` sets this; every other command, raw stdin
// body, --operation-name override, and MCP tool reaches the client without
// it and is refused.
func WithChargeConsent(ctx context.Context, yes, confirmCharge bool) context.Context {
	return context.WithValue(ctx, ctxKeyChargeConsent{}, chargeConsent{yes: yes, confirmCharge: confirmCharge})
}

func declaredGraphQLOperation(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	op, _ := ctx.Value(ctxKeyDeclaredOp{}).(string)
	return op
}

func chargeConsentFrom(ctx context.Context) chargeConsent {
	if ctx == nil {
		return chargeConsent{}
	}
	c, _ := ctx.Value(ctxKeyChargeConsent{}).(chargeConsent)
	return c
}

var graphqlParamMetaKeys = map[string]struct{}{
	"operationName": {},
	"extensions":    {},
	"variables":     {},
	"query":         {},
	"rawQuery":      {},
}

// foldGraphQLVariables moves non-meta query params into the GraphQL variables
// JSON object so flag values reach the server. Preserves object/array/number/
// boolean types when the flag value is valid JSON; otherwise keeps a string.
func foldGraphQLVariables(params map[string]string) map[string]string {
	if len(params) == 0 {
		return params
	}
	updated := make(map[string]string, len(params))
	vars := map[string]any{}
	if raw := params["variables"]; raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &vars)
		if vars == nil {
			vars = map[string]any{}
		}
	}
	for k, v := range params {
		if _, meta := graphqlParamMetaKeys[k]; meta {
			updated[k] = v
			continue
		}
		if v == "" {
			continue
		}
		vars[k] = decodeGraphQLVariableValue(v)
	}
	if len(vars) > 0 {
		b, err := json.Marshal(vars)
		if err == nil {
			updated["variables"] = string(b)
		}
	} else if updated["variables"] == "" {
		updated["variables"] = "{}"
	}
	return updated
}

func decodeGraphQLVariableValue(raw string) any {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Only JSON-decode when the flag value is already JSON-shaped. Bare
	// digit strings (shop ids, postal codes) stay strings so GraphQL String
	// fields are not widened to Int unexpectedly.
	if s[0] == '{' || s[0] == '[' || s[0] == '"' || s == "true" || s == "false" || s == "null" {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err == nil {
			return v
		}
	}
	return raw
}

const finalizeCheckoutOperation = "FinalizeCheckout"

// finalizeCheckoutTextRe matches the charge mutation by operation name or by
// GraphQL field name in query text (finalizeCheckout), case-insensitively.
var finalizeCheckoutTextRe = regexp.MustCompile(`(?i)finalize[_\-\s]*checkout`)

// collectGraphQLOperationNames returns every operationName the request
// carries: the query param plus every operationName in the JSON body,
// including batched (array) bodies. The server may honour any of them, so
// safety checks must consider all of them, not just the first.
func collectGraphQLOperationNames(params map[string]string, body []byte) []string {
	var names []string
	if params != nil {
		if op := strings.TrimSpace(params["operationName"]); op != "" {
			names = append(names, op)
		}
	}
	if len(body) == 0 {
		return names
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return names
	}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if op, ok := t["operationName"].(string); ok && strings.TrimSpace(op) != "" {
				names = append(names, strings.TrimSpace(op))
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(payload)
	return names
}

// requestTargetsFinalizeCheckout reports whether an outgoing request could
// execute the FinalizeCheckout charge mutation by ANY route: operationName
// (query param or body, batched or not), GraphQL query text, the persisted
// query hash for FinalizeCheckout, or the URL path/query string itself.
// JSON-shaped values are decoded so \u-escaped names are still caught.
// Deliberately over-broad: a false positive refuses a request, a false
// negative could charge a card.
func requestTargetsFinalizeCheckout(path string, params map[string]string, body []byte, finalizeHashes []string) bool {
	hashes := make([]string, 0, len(finalizeHashes))
	for _, h := range finalizeHashes {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			hashes = append(hashes, h)
		}
	}
	matchText := func(s string) bool {
		if s == "" {
			return false
		}
		if finalizeCheckoutTextRe.MatchString(s) {
			return true
		}
		lower := strings.ToLower(s)
		for _, h := range hashes {
			if strings.Contains(lower, h) {
				return true
			}
		}
		return false
	}
	var matchJSON func(v any) bool
	matchJSON = func(v any) bool {
		switch t := v.(type) {
		case string:
			return matchText(t)
		case map[string]any:
			for k, val := range t {
				if matchText(k) || matchJSON(val) {
					return true
				}
			}
		case []any:
			for _, item := range t {
				if matchJSON(item) {
					return true
				}
			}
		}
		return false
	}
	matchValue := func(s string) bool {
		if matchText(s) {
			return true
		}
		if unescaped, err := url.QueryUnescape(s); err == nil && unescaped != s && matchText(unescaped) {
			return true
		}
		var decoded any
		if json.Unmarshal([]byte(s), &decoded) == nil && matchJSON(decoded) {
			return true
		}
		return false
	}
	if matchValue(path) {
		return true
	}
	for k, v := range params {
		if matchValue(k) || matchValue(v) {
			return true
		}
	}
	if len(body) > 0 && matchValue(string(body)) {
		return true
	}
	return false
}

// enforceChargeGate is the single lowest-layer charge gate. doInternal runs
// it on every outgoing request (any path, any verb, any caller: generated
// commands, stdin bodies, --operation-name overrides, MCP tools) before the
// verify short-circuit, dry-run, and network send, and again after the
// persisted-query hash is injected.
func enforceChargeGate(ctx context.Context, path string, params map[string]string, body []byte, finalizeHashes []string) error {
	if !requestTargetsFinalizeCheckout(path, params, body, finalizeHashes) {
		return nil
	}
	consent := chargeConsentFrom(ctx)
	if consent.yes && consent.confirmCharge {
		return nil
	}
	missing := []string{}
	if !consent.yes {
		missing = append(missing, "--yes")
	}
	if !consent.confirmCharge {
		missing = append(missing, "--confirm-charge")
	}
	return fmt.Errorf("refusing FinalizeCheckout without charge consent (missing %s): only `order place --yes --confirm-charge` may charge; raw checkout, stdin, --operation-name, and MCP passthrough cannot", strings.Join(missing, " and "))
}

// enforceGraphQLOperationSafety rejects operation names that disagree with a
// declared command lock (every operationName in params and body is checked).
func enforceGraphQLOperationSafety(ctx context.Context, params map[string]string, body []byte) error {
	declared := declaredGraphQLOperation(ctx)
	if declared == "" {
		return nil
	}
	for _, op := range collectGraphQLOperationNames(params, body) {
		if op != declared {
			return fmt.Errorf("refusing GraphQL operation %q: this command is locked to %q (use the narrative order commands for charge/cancel flows)", op, declared)
		}
	}
	return nil
}

// coalesceGraphQLPOSTBody ensures POST /graphql sends operationName + variables
// in the JSON body (Costco Same-Day expects body variables, not loose query params).
func coalesceGraphQLPOSTBody(params map[string]string, body []byte) ([]byte, map[string]string, error) {
	params = foldGraphQLVariables(params)
	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return body, params, nil // leave non-JSON bodies alone
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if op := strings.TrimSpace(params["operationName"]); op != "" {
		if existing, _ := payload["operationName"].(string); existing == "" {
			payload["operationName"] = op
		}
	}
	if _, ok := payload["variables"]; !ok {
		vars := map[string]any{}
		if raw := params["variables"]; raw != "" {
			_ = json.Unmarshal([]byte(raw), &vars)
		}
		payload["variables"] = vars
	}
	if _, ok := payload["extensions"]; !ok {
		if raw := params["extensions"]; raw != "" {
			var ext any
			if err := json.Unmarshal([]byte(raw), &ext); err == nil {
				payload["extensions"] = ext
			}
		}
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return body, params, err
	}
	// Keep operationName + extensions + variables as query companions for persisted-query GETs;
	// for POST, strip folded variable keys already moved into body (meta keys may remain).
	return out, params, nil
}

// GraphQLResponseSucceeded reports whether a GraphQL HTTP response actually
// succeeded: 2xx, no top-level errors, and not a verify-mode synthetic noop.
func GraphQLResponseSucceeded(status int, data []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	if len(data) == 0 {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	if isVerifySynthetic(payload) || jsonErrorPresent(payload["errors"]) {
		return false
	}
	return payload["data"] != nil
}

// RESTResponseSucceeded reports whether a REST mutating response succeeded:
// 2xx, not a verify-mode synthetic noop, and not a JSON error body
// (non-empty "error"/"errors", "success"/"ok": false, or an error "status").
func RESTResponseSucceeded(status int, data []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return true // empty 2xx is the normal REST cancel reply
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return true // non-JSON 2xx body still counts as HTTP success
	}
	if isVerifySynthetic(payload) {
		return false
	}
	if jsonErrorPresent(payload["error"]) || jsonErrorPresent(payload["errors"]) {
		return false
	}
	for _, key := range []string{"success", "ok"} {
		if v, ok := payload[key].(bool); ok && !v {
			return false
		}
	}
	if s, ok := payload["status"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "error", "failed", "failure", "fail":
			return false
		}
	}
	return true
}

func isVerifySynthetic(payload map[string]any) bool {
	syn, _ := payload["__pp_verify_synthetic__"].(bool)
	return syn
}

// jsonErrorPresent treats any non-empty error value as an error.
func jsonErrorPresent(v any) bool {
	switch e := v.(type) {
	case nil:
		return false
	case bool:
		return e
	case string:
		return strings.TrimSpace(e) != ""
	case []any:
		return len(e) > 0
	case map[string]any:
		return len(e) > 0
	default:
		return true
	}
}

// Charge outcome for a FinalizeCheckout reply.
const (
	ChargeConfirmed   = "charged"     // 2xx, no errors, data.finalizeCheckout present
	ChargeNotCharged  = "not_charged" // non-2xx, GraphQL errors, payload errors, or synthetic verify reply
	ChargeUnconfirmed = "unconfirmed" // 2xx without errors but no finalizeCheckout result
)

// ClassifyFinalizeCheckoutResponse decides whether a FinalizeCheckout reply
// proves a charge. Only ChargeConfirmed may be reported as charged: true.
// ChargeUnconfirmed means the server did not report an error, but the
// expected order result is missing, so the caller must check order history
// before retrying (a blind retry could double-charge).
func ClassifyFinalizeCheckoutResponse(status int, data []byte) string {
	if status < 200 || status >= 300 || len(strings.TrimSpace(string(data))) == 0 {
		return ChargeNotCharged
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ChargeUnconfirmed
	}
	if isVerifySynthetic(payload) || jsonErrorPresent(payload["errors"]) {
		return ChargeNotCharged
	}
	dataObj, _ := payload["data"].(map[string]any)
	if dataObj == nil {
		return ChargeUnconfirmed
	}
	var result map[string]any
	for k, v := range dataObj {
		if strings.EqualFold(k, "finalizeCheckout") {
			result, _ = v.(map[string]any)
		}
	}
	if result == nil {
		return ChargeUnconfirmed
	}
	for _, key := range []string{"errors", "userErrors", "error"} {
		if jsonErrorPresent(result[key]) {
			return ChargeNotCharged
		}
	}
	return ChargeConfirmed
}
