// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED, not generated. Keeps live session credentials out of stdout.

package cli

import "encoding/json"

// sessionCredentialKeys are the fields of a DSM login payload that are live
// credentials. Both are persisted to the session file by the client, so no
// caller has a reason to read them off stdout, and both grant API access on
// their own until the session is logged out or expires.
var sessionCredentialKeys = []string{"sid", "synotoken"}

// redactSessionCredentials replaces the value of every credential field in a
// DSM login payload with "***", leaving the rest of the object untouched so
// callers still see account, device_id and the success flag. Input that is not
// a JSON object is returned unchanged: this is an output filter, not a parser,
// and it must never turn a printable response into an error.
// DSM wraps the login payload in {"data":{...},"success":true}, and the
// generated command prints that envelope verbatim, so a top-level-only sweep
// would miss the credentials one level down. Nesting is walked instead of
// special-casing "data": the depth is a DSM implementation detail.
func redactSessionCredentials(data json.RawMessage) json.RawMessage {
	obj, changed := redactSessionCredentialsIn(data)
	if !changed {
		return data
	}
	redacted, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return redacted
}

// redactSessionCredentialsIn returns the redacted object and whether anything
// was replaced. Non-object input reports no change so the caller can hand back
// the original bytes untouched.
func redactSessionCredentialsIn(data json.RawMessage) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, false
	}
	changed := false
	for key, raw := range obj {
		if containsString(sessionCredentialKeys, key) {
			obj[key] = json.RawMessage(`"***"`)
			changed = true
			continue
		}
		if nested, nestedChanged := redactSessionCredentialsIn(raw); nestedChanged {
			if encoded, err := json.Marshal(nested); err == nil {
				obj[key] = encoded
				changed = true
			}
		}
	}
	return obj, changed
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
