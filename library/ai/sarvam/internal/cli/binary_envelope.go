// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel helper: decode the client's base64 binary envelope back to raw bytes.
// The shared client wraps non-textual success bodies in
// {"_pp_binary":true,"content_type":...,"encoding":"base64","bytes":N,"data":"..."}
// so they survive the json.RawMessage contract. Commands that must emit raw
// bytes (streaming audio) decode the envelope before writing to stdout.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

var errNotBinaryEnvelope = errors.New("not a binary envelope")

// decodeBinaryEnvelope returns the decoded raw payload when data is the
// client's binary envelope; otherwise it returns data unchanged with an error
// so callers can fall through to the raw write.
func decodeBinaryEnvelope(data []byte) ([]byte, error) {
	var env struct {
		PPBinary bool   `json:"_pp_binary"`
		Encoding string `json:"encoding"`
		Data     string `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil || !env.PPBinary {
		return nil, errNotBinaryEnvelope
	}
	if env.Encoding != "base64" {
		return nil, errNotBinaryEnvelope
	}
	return base64.StdEncoding.DecodeString(env.Data)
}
