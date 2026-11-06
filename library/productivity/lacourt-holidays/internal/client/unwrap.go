// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"encoding/json"
)

// unwrapJSONStringBody decodes a body that is a JSON string whose contents are
// themselves a JSON array or object. The LASC holidays endpoint returns
// "[{\"date\":...}]" (a double-encoded array); callers want the array.
// Bodies that are not a string-wrapped array/object are returned unchanged.
func unwrapJSONStringBody(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) < 2 || trimmed[0] != '"' {
		return body
	}
	var inner string
	if err := json.Unmarshal(trimmed, &inner); err != nil {
		return body
	}
	innerBytes := bytes.TrimSpace([]byte(inner))
	if len(innerBytes) == 0 || (innerBytes[0] != '[' && innerBytes[0] != '{') || !json.Valid(innerBytes) {
		return body
	}
	return innerBytes
}
