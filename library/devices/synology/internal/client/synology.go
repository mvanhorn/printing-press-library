// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.

// HAND-AUTHORED. Not emitted by CLI Printing Press; diff this file (and the
// marked regions of session.go and client.go) against a fresh `generate
// --force` before accepting a regeneration.
//
// DSM speaks a namespaced RPC dialect that the generic session_handshake
// transport does not cover on its own:
//
//   - Login is a real credential exchange, not a token GET. It answers with
//     {"success":true,"data":{"sid":"...","synotoken":"..."}}.
//   - Failures arrive as HTTP 200 with {"success":false,"error":{"code":N}}.
//     Code 119 means the sid expired, which is the signal to relogin.
//   - DSM 7.3.2 and newer reject calls that omit the X-SYNO-TOKEN header.
//
// This file holds the DSM-specific pieces; session.go and client.go call into
// it at a handful of clearly marked points.

package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/synology/internal/config"
)

// DSM carries the account password in the login request's query string, so any
// text that quotes a request URL can leak it: Go's *url.Error embeds the real
// URL, and a dry-run prints the outgoing parameters. dsmSecretQueryPattern
// finds those parameters wherever they appear in free text.
var dsmSecretQueryPattern = regexp.MustCompile(`(?i)\b(passwd|password|otp_code)=[^&\s"']*`)

// dsmSecretParamKeys are the login parameters whose values must never be
// printed, even when the surrounding text has no "key=value" shape.
var dsmSecretParamKeys = map[string]struct{}{
	"passwd":   {},
	"password": {},
	"otp_code": {},
}

// scrubDSMSecrets replaces every password-carrying query parameter in text with
// a redacted placeholder. It is deliberately value-blind: it matches by
// parameter name, so it works for error strings whose credential value the
// caller never had a chance to register as a mask.
func scrubDSMSecrets(text string) string {
	if text == "" {
		return text
	}
	return dsmSecretQueryPattern.ReplaceAllStringFunc(text, func(match string) string {
		name, _, _ := strings.Cut(match, "=")
		return name + "=***"
	})
}

// scrubDSMSecretError returns err with every password-carrying query parameter
// redacted. It returns err unchanged when there is nothing to redact, so the
// original error's type and wrapping survive the common case.
func scrubDSMSecretError(err error) error {
	if err == nil {
		return nil
	}
	raw := err.Error()
	scrubbed := scrubDSMSecrets(raw)
	if scrubbed == raw {
		return err
	}
	return fmt.Errorf("%s", scrubbed)
}

// isDSMSecretParam reports whether a request parameter's value must be redacted
// before it is printed.
func isDSMSecretParam(key string) bool {
	_, ok := dsmSecretParamKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// DSM error codes worth naming. Every other code falls back to the raw number,
// which is still more useful than a bare "success: false".
const (
	dsmErrSessionExpired = 119
	dsmErrNoPermission   = 105
)

var dsmErrorMessages = map[int]string{
	100:                  "unknown error",
	101:                  "invalid parameter",
	102:                  "the requested API does not exist on this NAS - the package that provides it may not be installed",
	103:                  "the requested method does not exist",
	104:                  "the requested API version is not supported by this DSM",
	dsmErrNoPermission:   "insufficient permission - this account may not be an administrator, or the API needs a privilege it was not granted",
	106:                  "session timeout",
	107:                  "session interrupted by a duplicate login",
	dsmErrSessionExpired: "session expired",
}

// dsmAuthErrorMessages covers the 4xx block, which DSM defines only for
// SYNO.API.Auth. Every other namespace reuses those same numbers for its own
// meanings - SYNO.FileStation.List answers 400 for an invalid file-operation
// parameter, not for a bad password - so rendering them globally told users to
// re-check credentials that were in fact fine. Applied only to auth calls.
var dsmAuthErrorMessages = map[int]string{
	400: "invalid account or password",
	401: "the account is disabled",
	402: "permission denied",
	403: "a one-time password is required - pass --otp-code",
	404: "the one-time password was rejected",
	406: "enforce-2FA is on but the account has no OTP enrolled",
	407: "blocked by the DSM IP auto-block list",
	408: "the account password has expired and must be changed in DSM",
}

// dsmResponse is the envelope every entry.cgi call answers with.
type dsmResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code   int `json:"code"`
		Errors any `json:"errors,omitempty"`
	} `json:"error"`
}

// dsmErrorCode reports the DSM error code carried by an HTTP 200 body. ok is
// false for anything that is not a failed DSM envelope: a successful call, a
// non-JSON body, a JSON array, or a binary download.
func dsmErrorCode(body []byte) (int, bool) {
	trimmed := strings.TrimLeft(string(body), " \t\r\n")
	if !strings.HasPrefix(trimmed, "{") {
		return 0, false
	}
	var resp dsmResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false
	}
	if resp.Success || resp.Error == nil {
		return 0, false
	}
	return resp.Error.Code, true
}

// dsmUnwrapSuccess strips the {"success":true,"data":{...}} envelope every
// entry.cgi call answers with, so callers, the local cache and the MCP server
// all see the payload itself rather than a wrapper. Without this every command
// printed an object, which also meant the generated table renderer - it only
// fires for a JSON array - never ran for any command.
//
// Anything that is not a successful envelope carrying data is returned
// untouched: non-JSON and binary bodies, JSON arrays, failed envelopes (those
// never reach here, dsmErrorCode intercepts them) and successes with no data
// such as logout, which legitimately answer {"success":true}.
func dsmUnwrapSuccess(body []byte) []byte {
	trimmed := strings.TrimLeft(string(body), " \t\r\n")
	if !strings.HasPrefix(trimmed, "{") {
		return body
	}
	var resp dsmResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}
	if !resp.Success || len(resp.Data) == 0 || string(resp.Data) == "null" {
		return body
	}
	return resp.Data
}

// dsmErrorText renders a DSM error code as a sentence a user can act on.
func dsmErrorText(code int, path string) string {
	if msg, ok := dsmErrorMessages[code]; ok {
		return fmt.Sprintf("DSM error %d: %s", code, msg)
	}
	if strings.Contains(path, "api=SYNO.API.Auth") {
		if msg, ok := dsmAuthErrorMessages[code]; ok {
			return fmt.Sprintf("DSM error %d: %s", code, msg)
		}
	}
	return fmt.Sprintf("DSM error %d", code)
}

// isDSMAuthCall reports whether a request path is a SYNO.API.Auth call for the
// given method, so the transport can adopt or drop the session it produces.
func isDSMAuthCall(path, method string) bool {
	return strings.Contains(path, "api=SYNO.API.Auth") && strings.Contains(path, "method="+method)
}

// dsmCredentials carries what a login needs. Credentials come from flags (via
// `session login`) or from the environment, never from a file this CLI writes:
// the session record on disk holds the sid and the SynoToken, never a password.
type dsmCredentials struct {
	Account    string
	Password   string
	OTPCode    string
	DeviceID   string
	DeviceName string
}

// dsmCredentialsFromEnv reads the credentials used for an unattended relogin.
// A run that was never given credentials cannot self-heal an expired session;
// EnsureToken turns that into a "run session login" message rather than a
// silent retry loop.
// envOrDotenv prefers the real environment and falls back to this CLI's
// printing-press .env file. Hand-added, and resolved per field on purpose: an
// account exported in the shell still combines with a password that only lives
// in the file.
func envOrDotenv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return config.DotenvLookup(key)
}

func dsmCredentialsFromEnv() dsmCredentials {
	return dsmCredentials{
		Account:    envOrDotenv("SYNOLOGY_ACCOUNT"),
		Password:   envOrDotenv("SYNOLOGY_PASSWORD"),
		OTPCode:    envOrDotenv("SYNOLOGY_OTP_CODE"),
		DeviceID:   envOrDotenv("SYNOLOGY_DEVICE_ID"),
		DeviceName: envOrDotenv("SYNOLOGY_DEVICE_NAME"),
	}
}

func (c dsmCredentials) complete() bool {
	return c.Account != "" && c.Password != ""
}

// dsmLoginURL builds the SYNO.API.Auth login call against the configured NAS.
// The base URL is per-user, so this is resolved at runtime instead of being
// baked in as a constant the way the generic handshake does it.
func dsmLoginURL(baseURL string, creds dsmCredentials) string {
	q := url.Values{}
	q.Set("api", "SYNO.API.Auth")
	q.Set("method", "login")
	q.Set("version", "7")
	q.Set("session", "FileStation")
	q.Set("format", "sid")
	q.Set("enable_syno_token", "yes")
	q.Set("account", creds.Account)
	q.Set("passwd", creds.Password)
	if creds.OTPCode != "" {
		q.Set("otp_code", creds.OTPCode)
	}
	if creds.DeviceID != "" {
		q.Set("device_id", creds.DeviceID)
	}
	if creds.DeviceName != "" {
		q.Set("device_name", creds.DeviceName)
	}
	return strings.TrimRight(baseURL, "/") + "/webapi/entry.cgi?" + q.Encode()
}

// dsmLoginResult is the useful part of a successful login.
type dsmLoginResult struct {
	SID       string `json:"sid"`
	SynoToken string `json:"synotoken"`
	DeviceID  string `json:"did"`
}

// parseDSMLogin turns a login response body into a sid plus a SynoToken.
func parseDSMLogin(body []byte) (dsmLoginResult, error) {
	var out dsmLoginResult
	if code, failed := dsmErrorCode(body); failed {
		return out, fmt.Errorf("DSM login rejected: %s", dsmErrorText(code, "api=SYNO.API.Auth"))
	}
	var resp dsmResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return out, fmt.Errorf("parsing DSM login response: %w", err)
	}
	if !resp.Success {
		return out, fmt.Errorf("DSM login failed without an error code")
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return out, fmt.Errorf("parsing DSM login payload: %w", err)
	}
	if out.SID == "" {
		return out, fmt.Errorf("DSM login returned no session id")
	}
	return out, nil
}
