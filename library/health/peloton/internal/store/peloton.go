package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ProviderFact is one locally retained provider response and its provenance.
// It intentionally contains no credentials: RecordProviderFact is the only
// writer for this table and accepts response bodies only.
type ProviderFact struct {
	Family     string
	ProviderID string
	Payload    json.RawMessage
	FetchedAt  time.Time
}

// RecordProviderFact stores only provider response facts and provenance, never credentials.
func (s *Store) RecordProviderFact(family, id string, body json.RawMessage) (bool, error) {
	if family == "" || id == "" || !json.Valid(body) {
		return false, fmt.Errorf("invalid provider fact")
	}
	redacted, err := RedactProviderPayload(body)
	if err != nil {
		return false, err
	}
	h := sha256.Sum256(redacted)
	digest := hex.EncodeToString(h[:])
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var old string
	_ = s.db.QueryRow(`SELECT content_hash FROM provider_payloads WHERE family=? AND provider_id=?`, family, id).Scan(&old)
	if old == digest {
		return false, nil
	}
	_, err = s.db.Exec(`INSERT INTO provider_payloads(family,provider_id,content_hash,payload,fetched_at) VALUES(?,?,?,?,?) ON CONFLICT(family,provider_id) DO UPDATE SET content_hash=excluded.content_hash,payload=excluded.payload,fetched_at=excluded.fetched_at`, family, id, digest, redacted, time.Now().UTC())
	return err == nil, err
}

// RedactProviderPayload removes credential-shaped values before a provider
// response enters the private store. It deliberately preserves workout and
// class facts; only keys that would authenticate or replay a session are
// replaced. JSON numbers use UseNumber so IDs retain their exact spelling.
func RedactProviderPayload(body json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("decoding provider fact: %w", err)
	}
	redactProviderValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encoding redacted provider fact: %w", err)
	}
	return json.RawMessage(out), nil
}

func redactProviderValue(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if isSensitiveProviderKey(key) {
				v[key] = "[REDACTED]"
				continue
			}
			if text, ok := child.(string); ok && hasSensitiveURLQuery(text) {
				v[key] = "[REDACTED]"
				continue
			}
			redactProviderValue(child)
		}
	case []any:
		for _, child := range v {
			redactProviderValue(child)
		}
	}
}

func isSensitiveProviderKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	for _, needle := range []string{"authorization", "apikey", "cookie", "credential", "jwt", "password", "secret", "session", "signature", "token"} {
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}

func hasSensitiveURLQuery(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	for key := range u.Query() {
		if isSensitiveProviderKey(key) {
			return true
		}
	}
	return false
}

// GetProviderFact returns a retained provider fact by source family and ID.
// sql.ErrNoRows is preserved so callers can give a typed not-found result.
func (s *Store) GetProviderFact(family, id string) (ProviderFact, error) {
	var fact ProviderFact
	var payload string
	err := s.db.QueryRow(`SELECT family, provider_id, payload, fetched_at FROM provider_payloads WHERE family=? AND provider_id=?`, family, id).
		Scan(&fact.Family, &fact.ProviderID, &payload, &fact.FetchedAt)
	if err != nil {
		return ProviderFact{}, err
	}
	fact.Payload = json.RawMessage(payload)
	return fact, nil
}

// ListProviderFacts returns the retained facts for one source family. A
// non-positive limit returns all facts. Ordering is stable and factual: newest
// fetched record first, then the provider ID as a deterministic tie-breaker.
func (s *Store) ListProviderFacts(family string, limit int) ([]ProviderFact, error) {
	query := `SELECT family, provider_id, payload, fetched_at FROM provider_payloads WHERE family=? ORDER BY fetched_at DESC, provider_id ASC`
	args := []any{family}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []ProviderFact
	for rows.Next() {
		var fact ProviderFact
		var payload string
		if err := rows.Scan(&fact.Family, &fact.ProviderID, &payload, &fact.FetchedAt); err != nil {
			return nil, err
		}
		fact.Payload = json.RawMessage(payload)
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}
