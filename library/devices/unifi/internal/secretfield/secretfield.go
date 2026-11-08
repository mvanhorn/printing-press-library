// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output.

// Package secretfield decides what counts as a secret, in one place, because the audit
// log, the change journal, the mirror, the response cache and drift must never disagree:
// a value one of them treats as harmless ends up on disk through that one. It imports
// nothing from this module so both the client and the store can use it.
package secretfield

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// ScrubbedValue replaces a secret where nothing needs to compare it later.
const ScrubbedValue = "[scrubbed]"

// DigestPrefix marks a salted digest of a secret. It is recognizable so readers can hide
// it and the client can refuse to send it back as if it were the real value.
const DigestPrefix = "[secret:v1:"

// namePattern is deliberately broad: a scrubbed harmless field costs little, a leaked
// PSK is permanent. "pre_?shared" is here because presharedKeys ends in Keys, which the
// key rule does not match, and that spec field holds per-network WiFi passphrases.
var namePattern = regexp.MustCompile(`(?i)(pass|secret|token|psk|pre[-_]?shared|credential|cookie|auth|private|(^|[-_])key$|[a-z]Key$|signature|cert)`)

// notSecret lists spec fields the broad pattern catches by accident. Exact names only,
// so the pattern stays broad everywhere else. They matter because a scrubbed before-image
// blocks undo, which made every Protect object (modelKey) impossible to undo, and a
// scrubbed field is skipped by read-back, so a requested change to authorized was
// reported verified without being compared.
var notSecret = map[string]bool{
	"modelKey": true, "poe_passthrough": true, "nextToken": true, "inputToken": true,
	"authorized": true, "authorizedAt": true, "authorizedGuestCount": true, "authorizedGuestLimit": true,
	"authorizationMethod": true, "grantedAuthorization": true, "revokedAuthorization": true,
	"presharedKeyNetworkIds": true,
}

// voucherShape identifies a hotspot voucher, whose "code" is a network-access credential.
// By shape rather than by resource type, because the same object also arrives nested in a
// creation result and in a journal read-back. The name alone cannot decide it: the spec
// also uses code for country codes and error codes.
var voucherShape = []string{"authorizedGuestLimit", "authorizedGuestCount", "timeLimitMinutes"}

// credentialURL matches values that are secrets whatever their field is called. A Protect
// RTSPS stream URL carries its access token in the path, per createdRtspsStreams in the
// spec, and a URL with userinfo carries a password. A plain address such as the talkback
// rtp:// URL does not match.
var credentialURL = regexp.MustCompile(`(?i)^(rtsps?://|[a-z][a-z0-9+.-]*://[^/@\s]+:[^/@\s]*@)`)

// IsSecretField reports whether a field name is one the scrubbers hide.
func IsSecretField(name string) bool {
	return !notSecret[name] && namePattern.MatchString(name)
}

// IsPlaceholder reports a value that stands in for a secret and is not the secret.
func IsPlaceholder(value string) bool {
	return value == ScrubbedValue || (strings.HasPrefix(value, DigestPrefix) && strings.HasSuffix(value, "]"))
}

func isVoucher(obj map[string]any) bool {
	if _, ok := obj["code"]; !ok {
		return false
	}
	for _, field := range voucherShape {
		if _, ok := obj[field]; ok {
			return true
		}
	}
	return false
}

// strongName is the part of the pattern that means secret whatever the value's type.
// The rest (token, auth, key, cert) also names counters and ports such as total_tokens and
// authPort, where a number is that number. The spec has no numeric secrets, so this only
// guards a console that sends a numeric password.
var strongName = regexp.MustCompile(`(?i)(pass|secret|psk|pre[-_]?shared|credential|private)`)

// holdsValue: booleans and empties carry no secret and keep their type, and "no passphrase
// set" stays visible.
func holdsValue(key string, v any) bool {
	switch typed := v.(type) {
	case nil, bool:
		return false
	case string:
		return typed != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	}
	return strongName.MatchString(key)
}

// Walk returns v with every secret replaced by replace(value), and whether anything
// changed. A secret-named field is replaced whole even when it holds an object: walking
// into {"credentials":{"value":x}} reaches a leaf called value, which looks harmless.
func Walk(v any, replace func(any) any) (any, bool) {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		changed := false
		voucher := isVoucher(typed)
		for key, value := range typed {
			if (IsSecretField(key) || (voucher && key == "code")) && holdsValue(key, value) {
				next := replace(value)
				if !sameScalar(next, value) {
					changed = true
				}
				out[key] = next
				continue
			}
			next, childChanged := Walk(value, replace)
			out[key] = next
			changed = changed || childChanged
		}
		return out, changed
	case []any:
		out := make([]any, len(typed))
		changed := false
		for i, value := range typed {
			next, childChanged := Walk(value, replace)
			out[i] = next
			changed = changed || childChanged
		}
		return out, changed
	case string:
		if IsPlaceholder(typed) || credentialURL.MatchString(typed) {
			next := replace(typed)
			return next, !sameScalar(next, typed)
		}
	}
	return v, false
}

func sameScalar(a, b any) bool {
	as, aOK := a.(string)
	bs, bOK := b.(string)
	return aOK && bOK && as == bs
}

func decode(data []byte) (any, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if dec.Decode(&value) != nil {
		return nil, false
	}
	return value, true
}

// HoldsSecret reports whether a JSON document contains a real secret, not a placeholder.
func HoldsSecret(data []byte) bool {
	value, ok := decode(data)
	if !ok {
		return false
	}
	found := false
	Walk(value, func(v any) any {
		if s, isString := v.(string); !isString || !IsPlaceholder(s) {
			found = true
		}
		return v
	})
	return found
}

// HoldsPlaceholder reports a placeholder anywhere in a JSON document, whatever its field.
func HoldsPlaceholder(data []byte) bool {
	// Cheap test first: this runs on every request body.
	if !bytes.Contains(data, []byte(DigestPrefix)) && !bytes.Contains(data, []byte(ScrubbedValue)) {
		return false
	}
	value, ok := decode(data)
	if !ok {
		return false
	}
	var search func(any) bool
	search = func(v any) bool {
		switch typed := v.(type) {
		case string:
			return IsPlaceholder(typed)
		case []any:
			for _, child := range typed {
				if search(child) {
					return true
				}
			}
		case map[string]any:
			for _, child := range typed {
				if search(child) {
					return true
				}
			}
		}
		return false
	}
	return search(value)
}

// StringValues returns the secret strings in v, so text that may echo them can be cleaned.
func StringValues(v any) []string {
	var out []string
	Walk(v, func(value any) any {
		collectStrings(value, &out)
		return value
	})
	return out
}

func collectStrings(v any, out *[]string) {
	switch typed := v.(type) {
	case string:
		if !IsPlaceholder(typed) {
			*out = append(*out, typed)
		}
	case []any:
		for _, child := range typed {
			collectStrings(child, out)
		}
	case map[string]any:
		for _, child := range typed {
			collectStrings(child, out)
		}
	}
}
