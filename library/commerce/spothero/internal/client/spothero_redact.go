package client

import (
	"bytes"
	"encoding/json"
)

var accessKeyField = []byte(`"access_key"`)

// stripAccessKey removes the per-reservation refund secret from API responses.
// Everything this client returns can reach stdout, the response cache, the local
// store and MCP callers, and none of them need it. The cancel path reads it through
// internal/consumer, a separate client, so cancelling is not affected.
func stripAccessKey(body []byte) []byte {
	if !bytes.Contains(body, accessKeyField) {
		return body
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	// UseNumber keeps large IDs exact across the round trip.
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		// Not JSON, so there is no field to find. A secret must not survive just
		// because the body was malformed, but callers treat this as raw text and
		// the API only sends this field inside JSON objects.
		return body
	}
	dropAccessKey(v)
	out, err := json.Marshal(v)
	if err != nil {
		return body
	}
	return out
}

func dropAccessKey(v any) {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "access_key")
		for _, child := range t {
			dropAccessKey(child)
		}
	case []any:
		for _, child := range t {
			dropAccessKey(child)
		}
	}
}
