// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package mcp

// codeOrchWriteBody returns the value handed to the client layer as the
// request body for write methods (POST/PUT/PATCH). It MUST be the structured
// params map, never pre-marshaled bytes.
//
// client.do() marshals the body value exactly once. Handing it []byte makes
// json.Marshal([]byte) emit a base64-encoded JSON *string*, so the API
// receives "eyJ...==" where it expects the request object.
//
// Ported from the current generator template during the 4.31.1 reprint
// (merge reconciliation): the refreshed code-orchestration tests need it
// and the preserved code_orch.go predates it.
func codeOrchWriteBody(params map[string]any) any {
	return params
}

// codeOrchArrayBody returns the body for write methods whose spec declares a
// top-level array payload under the "body" param. Otherwise it returns params
// unchanged for codeOrchWriteBody.
func codeOrchArrayBody(params map[string]any) any {
	if v, ok := params["body"]; ok {
		if arr, ok := v.([]any); ok {
			return arr
		}
	}
	return params
}
