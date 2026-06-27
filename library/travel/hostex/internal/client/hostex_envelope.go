package client

import "encoding/json"

// hostexEnvelope is Hostex's uniform response wrapper. Every Hostex v3 call
// returns HTTP 200 and signals the real outcome via error_code in the body.
// error_code mirrors HTTP status semantics: 200 (observed live, with
// error_msg "Done.") and other 2xx/3xx — plus a legacy 0 — mean success, while
// >= 400 means failure (401 auth, 404 not-found, 422 validation, 420
// subscription, 429 rate-limit, 5xx server/upstream).
type hostexEnvelope struct {
	ErrorCode *int   `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

// hostexAppError inspects a 200-status JSON body for an application-level
// failure. It returns ok=true only when the body is a JSON object carrying an
// error_code field; code is that value (0 on success) and msg is error_msg.
// Bodies that are not error_code envelopes (arrays, binary wrappers, the
// verify short-circuit synthetic envelope) return ok=false and are left
// untouched by the caller.
func hostexAppError(body []byte) (code int, msg string, ok bool) {
	var env hostexEnvelope
	if err := json.Unmarshal(body, &env); err != nil || env.ErrorCode == nil {
		return 0, "", false
	}
	return *env.ErrorCode, env.ErrorMsg, true
}
