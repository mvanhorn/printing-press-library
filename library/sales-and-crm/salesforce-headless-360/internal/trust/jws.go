package trust

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrSignatureInvalid = errors.New("SIGNATURE_INVALID")

const (
	JWSAudienceAgentContext  = "agent-context"
	JWSAudienceAgentMutation = "agent-mutation"
)

type jwsSigner interface {
	Sign(payload []byte) ([]byte, error)
	KID() string
}

// b64url encodes bytes using unpadded URL-safe base64 per RFC 7515.
func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func b64urlDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// SignJWS produces a compact JWS (header.payload.signature) over payloadJSON
// using the given signer. The header is:
//
//	{"alg":"EdDSA","typ":"JWT","kid":"<kid>"}
//
// payloadJSON is the byte-serialized claims object. Callers construct it
// with encoding/json.
func SignJWS(s jwsSigner, payloadJSON []byte) (string, error) {
	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": s.KID(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	signingInput := b64url(headerJSON) + "." + b64url(payloadJSON)
	sig, err := s.Sign([]byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return signingInput + "." + b64url(sig), nil
}

// VerifyJWS verifies a compact JWS using publicKey. Returns the decoded
// payload, header, and an error if verification fails. Only EdDSA is
// supported.
func VerifyJWS(jws string, publicKey ed25519.PublicKey) (payload []byte, header map[string]any, err error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("invalid JWS: expected 3 segments")
	}
	headerBytes, err := b64urlDecode(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("decode header: %w", err)
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, fmt.Errorf("parse header: %w", err)
	}
	if alg, _ := header["alg"].(string); alg != "EdDSA" {
		return nil, header, fmt.Errorf("unsupported alg: %v", header["alg"])
	}
	payload, err = b64urlDecode(parts[1])
	if err != nil {
		return nil, header, fmt.Errorf("decode payload: %w", err)
	}
	sig, err := b64urlDecode(parts[2])
	if err != nil {
		return nil, header, fmt.Errorf("decode signature: %w", err)
	}
	signingInput := parts[0] + "." + parts[1]
	if !ed25519.Verify(publicKey, []byte(signingInput), sig) {
		return nil, header, ErrSignatureInvalid
	}
	return payload, header, nil
}

// ExtractKIDUnsafe parses the JWS header without verifying the signature.
// Used by verify commands to look up the right public key before calling
// VerifyJWS.
func ExtractKIDUnsafe(jws string) (string, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid JWS")
	}
	headerBytes, err := b64urlDecode(parts[0])
	if err != nil {
		return "", err
	}
	var header struct {
		KID string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", err
	}
	return header.KID, nil
}
