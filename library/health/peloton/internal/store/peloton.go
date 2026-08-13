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

// UpsertBatchWithFacts is UpsertBatch plus a best-effort dual-write into the
// provider_payloads table the `offline` commands read from. Every write path
// in this CLI — sync's paginated batches and the live-read write-through
// cache — funnels through here so the two stores never drift the way they
// used to: a full sync landed thousands of rows in `resources` while
// `provider_payloads` (and therefore every `offline` command) stayed empty.
// The fact write is best-effort and non-atomic with the resources write: it
// mirrors the existing write-through cache's "the live/synced result already
// succeeded" tolerance for a secondary, reconstructable local index.
func (s *Store) UpsertBatchWithFacts(resourceType string, items []json.RawMessage) (int, int, error) {
	items = enrichResourceItems(resourceType, items)
	stored, extractFailures, err := s.UpsertBatch(resourceType, items)
	if err != nil {
		return stored, extractFailures, err
	}
	s.recordProviderFactsBestEffort(resourceType, items)
	return stored, extractFailures, nil
}

// UpsertWithFacts mirrors UpsertBatchWithFacts for the generic single-object
// write path (sync's single-object fallback, discriminator-resolved singles,
// and per-parent dependent fan-outs like "performance"). Unlike
// recordProviderFactsBestEffort's re-derivation from the body, this uses the
// caller-supplied id directly: callers of Upsert already know the id
// (sometimes, as with performance_graph, the response body carries no id
// field at all — the workout id comes from the request path, not the body),
// so re-deriving it here would silently drop exactly those facts.
func (s *Store) UpsertWithFacts(resourceType, id string, data json.RawMessage) error {
	if err := s.Upsert(resourceType, id, data); err != nil {
		return err
	}
	if id != "" {
		_, _ = s.RecordProviderFact(resourceType, id, data)
	}
	return nil
}

// UpsertClassesWithFacts mirrors UpsertBatchWithFacts for the typed
// single-object classes upsert.
func (s *Store) UpsertClassesWithFacts(data json.RawMessage) error {
	if err := s.UpsertClasses(data); err != nil {
		return err
	}
	s.recordProviderFactsBestEffort("classes", []json.RawMessage{data})
	return nil
}

// UpsertWorkoutsWithFacts mirrors UpsertBatchWithFacts for the typed
// single-object workouts upsert.
func (s *Store) UpsertWorkoutsWithFacts(data json.RawMessage) error {
	data = enrichWorkoutRideMetadata(data)
	if err := s.UpsertWorkouts(data); err != nil {
		return err
	}
	s.recordProviderFactsBestEffort("workouts", []json.RawMessage{data})
	return nil
}

// enrichResourceItems applies best-effort peloton-specific fixups at the
// single choke point every batch write funnels through. Currently only
// "workouts" needs one — see enrichWorkoutRideMetadata.
func enrichResourceItems(resourceType string, items []json.RawMessage) []json.RawMessage {
	if resourceType != "workouts" {
		return items
	}
	out := make([]json.RawMessage, len(items))
	for i, item := range items {
		out[i] = enrichWorkoutRideMetadata(item)
	}
	return out
}

// enrichWorkoutRideMetadata promotes ride.title/ride.id to top-level
// title/ride_id on a workout item before it reaches the typed workouts
// table's column extraction (lookupFieldValue(obj, "title"), which only
// ever looks at the top level) and the raw JSON that gets stored.
//
// Confirmed against a live GET /api/user/{user_id}/workouts?joins=ride
// response (2026-08-13): workout items carry no top-level "title" or
// "ride_id" at all — both live nested under a "ride" object ("ride.title",
// "ride.id"). Without this promotion every synced workout's title and ride
// association land null even though the API is returning the data; sync
// was already sending joins=ride's default value via the same query params
// single-fetch commands use, so the fix here is entirely in extraction, not
// in what's requested.
func enrichWorkoutRideMetadata(item json.RawMessage) json.RawMessage {
	dec := json.NewDecoder(bytes.NewReader(item))
	dec.UseNumber()
	var obj map[string]any
	if dec.Decode(&obj) != nil {
		return item
	}
	ride, ok := obj["ride"].(map[string]any)
	if !ok {
		return item
	}
	changed := false
	if v, present := obj["title"]; !present || v == nil {
		if title, ok := ride["title"]; ok && title != nil {
			obj["title"] = title
			changed = true
		}
	}
	if v, present := obj["ride_id"]; !present || v == nil {
		if rideID, ok := ride["id"]; ok && rideID != nil {
			obj["ride_id"] = rideID
			changed = true
		}
	}
	if !changed {
		return item
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return item
	}
	return json.RawMessage(out)
}

// recordProviderFactsBestEffort re-derives each item's primary key the same
// way UpsertBatch/Upsert do and records it as a provider fact. Failures
// (undecodable item, unresolvable ID) are skipped rather than propagated:
// the authoritative write to `resources`/typed tables already succeeded, and
// this secondary index exists to serve `offline` reads, not to gate sync.
func (s *Store) recordProviderFactsBestEffort(resourceType string, items []json.RawMessage) {
	for _, item := range items {
		obj, err := DecodeJSONObject(item)
		if err != nil {
			continue
		}
		id := ExtractResourceID(resourceType, obj)
		if id == "" {
			continue
		}
		_, _ = s.RecordProviderFact(resourceType, id, item)
	}
}
