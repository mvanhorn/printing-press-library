package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// DecodeBinaryResponse restores bytes from the client's JSON transport envelope.
// Text downloads that were not wrapped pass through unchanged.
func DecodeBinaryResponse(data []byte) ([]byte, error) {
	var envelope binaryResponseEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil || !envelope.PPBinary {
		return data, nil
	}
	if envelope.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported binary encoding %q", envelope.Encoding)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Data)
	if err != nil {
		return nil, fmt.Errorf("invalid binary payload: %w", err)
	}
	if len(payload) != envelope.Bytes {
		return nil, fmt.Errorf("binary payload length mismatch: got %d, want %d", len(payload), envelope.Bytes)
	}
	return payload, nil
}
