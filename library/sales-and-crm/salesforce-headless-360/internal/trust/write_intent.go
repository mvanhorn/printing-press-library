package trust

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

const WriteIntentAudience = JWSAudienceAgentMutation

var (
	ErrUnknownKID    = errors.New("UNKNOWN_KID")
	ErrWrongAudience = errors.New("WRONG_AUDIENCE")
	ErrIntentExpired = errors.New("INTENT_EXPIRED")
)

type WriteIntentClaims struct {
	Iss            string `json:"iss"`
	Sub            string `json:"sub"`
	Aud            string `json:"aud"`
	Iat            int64  `json:"iat"`
	Exp            int64  `json:"exp"`
	Jti            string `json:"jti"`
	SObject        string `json:"sobject"`
	RecordID       string `json:"record_id"`
	Operation      string `json:"operation"`
	DiffSha256     string `json:"diff_sha256"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	IfLastModified string `json:"if_last_modified,omitempty"`
}

func (s *FileSigner) SignWriteIntent(claims WriteIntentClaims) ([]byte, error) {
	return SignWriteIntent(s, claims)
}

func (s *FileSigner) VerifyWriteIntent(jws []byte) (WriteIntentClaims, error) {
	return VerifyWriteIntent(jws)
}

func SignWriteIntent(s jwsSigner, claims WriteIntentClaims) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("signer required")
	}
	if claims.Aud == "" {
		claims.Aud = WriteIntentAudience
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("marshal write intent claims: %w", err)
	}
	jws, err := SignJWS(s, payload)
	if err != nil {
		return nil, err
	}
	return []byte(jws), nil
}

func VerifyWriteIntent(jws []byte) (WriteIntentClaims, error) {
	var claims WriteIntentClaims
	kid, err := ExtractKIDUnsafe(string(jws))
	if err != nil {
		return claims, err
	}
	record, err := LoadKeyRecord(kid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return claims, ErrUnknownKID
		}
		return claims, fmt.Errorf("%w: %v", ErrUnknownKID, err)
	}
	pub, err := ParsePublicKeyPEM(record.PublicKeyPEM)
	if err != nil {
		return claims, err
	}
	payload, _, err := VerifyJWS(string(jws), pub)
	if err != nil {
		return claims, err
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, fmt.Errorf("parse write intent claims: %w", err)
	}
	if claims.Aud != WriteIntentAudience {
		return claims, ErrWrongAudience
	}
	if claims.Exp <= time.Now().UTC().Unix() {
		return claims, ErrIntentExpired
	}
	return claims, nil
}
