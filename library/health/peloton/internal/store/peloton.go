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
	now := time.Now().UTC()
	if old == digest {
		// Content unchanged from what's already stored -- skip rewriting
		// the (potentially large) payload blob, but still advance
		// fetched_at: it represents "when was this record last confirmed
		// fresh against the live API," not "when did its content last
		// change," and callers (e.g. sync's --stale-before) rely on it to
		// know a record was just re-verified, not just that its bytes
		// happen to already match. Without this, a --stale-before refetch
		// of a record whose live content genuinely hasn't changed would
		// leave fetched_at untouched and the record would be reported
		// stale again on every subsequent call, forever.
		_, err := s.db.Exec(`UPDATE provider_payloads SET fetched_at=? WHERE family=? AND provider_id=?`, now, family, id)
		return false, err
	}
	_, err = s.db.Exec(`INSERT INTO provider_payloads(family,provider_id,content_hash,payload,fetched_at) VALUES(?,?,?,?,?) ON CONFLICT(family,provider_id) DO UPDATE SET content_hash=excluded.content_hash,payload=excluded.payload,fetched_at=excluded.fetched_at`, family, id, digest, redacted, now)
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

// providerKeyRedactionExclusions lists normalized keys (matching
// isSensitiveProviderKey's own normalization) that would otherwise
// substring-match a sensitive needle but are not actually credential- or
// session-shaped. "jointokens" (from Peloton's "join_tokens" field) is a
// short-lived identifier used to join a live class session in progress —
// not a credential, not long-lived, and not reusable outside that session —
// but its normalized form contains "token", so without this exclusion a
// live-class-join response field was redacted on every store write.
var providerKeyRedactionExclusions = map[string]bool{
	"jointokens": true,
}

func isSensitiveProviderKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	if providerKeyRedactionExclusions[key] {
		return false
	}
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

// ExistingProviderFactFetchedAt returns fetched_at per provider id already
// stored for a family. Dependent syncs (performance, workout_details) use
// this to skip parents that already have a fresh-enough record instead of
// always reprocessing every parent id on every invocation -- an id with
// existing data doesn't need reprocessing (unless a --stale-before cutoff
// says otherwise; see planDependentSync), and a call that gets cut off
// partway simply leaves whatever it didn't reach as pending for the next
// call, with no separate resume cursor required. Returning fetched_at
// rather than a plain presence set lets callers distinguish "has a record"
// from "has a record recent enough to trust."
func (s *Store) ExistingProviderFactFetchedAt(family string) (map[string]time.Time, error) {
	rows, err := s.db.Query(`SELECT provider_id, fetched_at FROM provider_payloads WHERE family=?`, family)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fetchedAt := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		fetchedAt[id] = at
	}
	return fetchedAt, rows.Err()
}

// ProviderFactIDsWithField returns the set of provider ids within family
// whose stored payload has a non-null value at the given top-level JSON
// field. Distinct from ExistingProviderFactFetchedAt's plain presence
// check: a dependent sync that enriches an already-present record in place
// (e.g. class detail backfilling "segments"/"target_metrics_data" onto a
// class that was previously synced only in flat list form, which shares
// the same "classes" family and provider id) needs to know which already-
// present records still lack the richer field, not merely which ids exist
// at all.
func (s *Store) ProviderFactIDsWithField(family, field string) (map[string]bool, error) {
	if !validIdentifierRE.MatchString(field) {
		return nil, fmt.Errorf("ProviderFactIDsWithField: invalid field name %q", field)
	}
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT provider_id FROM provider_payloads WHERE family=? AND json_extract(payload,'$.%s') IS NOT NULL`, field),
		family,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// ParentIDsTouchedSince returns provider_payloads ids for a family fetched
// at or after `since`. Dependent syncs use this to scope their
// parent-keyed fan-out to only the parents a specific sync invocation
// actually touched (e.g. under --latest-only, which promises a bounded
// "refresh the top" operation), rather than every parent ever synced into
// the local store.
func (s *Store) ParentIDsTouchedSince(family string, since time.Time) ([]string, error) {
	rows, err := s.db.Query(`SELECT provider_id FROM provider_payloads WHERE family=? AND fetched_at >= ?`, family, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// providerFactDateField names the JSON field within a family's payload that
// carries the record's own real-world date, for families where one exists.
// Concurrent sync workers write rows in whatever order requests happen to
// complete, not the order the underlying events occurred in, so ordering by
// fetched_at (write time) scrambles a family's natural chronological order
// (e.g. offline history listing workouts newest-first by sync completion
// time instead of by when they were actually recorded). Families with no
// entry here keep the original fetched_at-based ordering.
func providerFactDateField(family string) string {
	switch family {
	case "workouts":
		return "start_time"
	}
	return ""
}

// ListProviderFacts returns the retained facts for one source family. A
// non-positive limit returns all facts. Ordering is stable and factual:
// newest record first by the family's own date field when one is known
// (providerFactDateField), falling back to fetched_at (sync/write time) when
// the family has no natural date field or a given row's date is absent;
// provider ID is always the final deterministic tie-breaker.
func (s *Store) ListProviderFacts(family string, limit int) ([]ProviderFact, error) {
	var query string
	if dateField := providerFactDateField(family); dateField != "" {
		query = fmt.Sprintf(`SELECT family, provider_id, payload, fetched_at FROM provider_payloads WHERE family=? ORDER BY COALESCE(json_extract(payload,'$.%s'), 0) DESC, fetched_at DESC, provider_id ASC`, dateField)
	} else {
		query = `SELECT family, provider_id, payload, fetched_at FROM provider_payloads WHERE family=? ORDER BY fetched_at DESC, provider_id ASC`
	}
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
	if resourceType == "classes" {
		items = s.mergeClassesListItemsWithStoredDetail(items)
	}
	stored, extractFailures, err := s.UpsertBatch(resourceType, items)
	if err != nil {
		return stored, extractFailures, err
	}
	s.recordProviderFactsBestEffort(resourceType, items)
	return stored, extractFailures, nil
}

// mergeClassesListItemsWithStoredDetail preserves detail-only fields
// (segments, target_metrics_data, ride, averages, playlist, instructor,
// workout_share_images, related_rides, questions, and anything else the
// detail endpoint carries that the catalog list endpoint doesn't) that a
// prior classes_detail dependent-sync fetch backfilled onto a class, which
// the flat catalog list sync's own items never carry, before the flat
// sync's wholesale upsertGenericResourceTx/RecordProviderFact replace
// would otherwise strip them back out on every routine
// `sync --resources classes` / `workflow archive` call. It copies over
// EVERY existing top-level key the incoming item lacks, not a fixed
// allowlist, so a future detail-response field survives automatically
// without this function needing an update. A detail-shaped incoming item
// (recognizable by a top-level "ride" object -- the same signal
// classIsListForm/nestedIDContainerKeys use) is returned unchanged:
// there's nothing to preserve when the incoming item is already at least
// as rich as what's stored. Only called for resourceType=="classes";
// every other resource's batch write is unaffected.
func (s *Store) mergeClassesListItemsWithStoredDetail(items []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, len(items))
	for i, item := range items {
		out[i] = s.mergeClassesListItemWithStoredDetail(item)
	}
	return out
}

// classFactHasRideObject reports whether a decoded class fact's "ride"
// field is a real, non-null JSON object -- the same detail-shape signal
// internal/cli/offline.go's classIsListForm uses via a type assertion to
// map[string]any. A plain comma-ok presence check on the raw
// map[string]json.RawMessage (an earlier version of this function) isn't
// enough: it's also true for a hypothetical "ride": null value, which
// would then be wrongly treated as a real detail object -- either
// short-circuiting the merge for a still-list-shaped incoming item, or
// treating a still-list-shaped stored record as having detail fields to
// preserve when it has none.
func classFactHasRideObject(obj map[string]json.RawMessage) bool {
	raw, ok := obj["ride"]
	if !ok {
		return false
	}
	var ride map[string]any
	return json.Unmarshal(raw, &ride) == nil && ride != nil
}

func (s *Store) mergeClassesListItemWithStoredDetail(item json.RawMessage) json.RawMessage {
	var incoming map[string]json.RawMessage
	if err := json.Unmarshal(item, &incoming); err != nil {
		return item
	}
	if classFactHasRideObject(incoming) {
		return item // already detail-shaped; nothing to preserve
	}
	var plain map[string]any
	if err := json.Unmarshal(item, &plain); err != nil {
		return item
	}
	id := ExtractResourceID("classes", plain)
	if id == "" {
		return item
	}
	existing, err := s.Get("classes", id)
	if err != nil {
		return item // no prior record (or unreadable) -- nothing to merge
	}
	var existingObj map[string]json.RawMessage
	if err := json.Unmarshal(existing, &existingObj); err != nil {
		return item
	}
	if !classFactHasRideObject(existingObj) {
		return item // prior record was list-shaped too; nothing to preserve
	}
	changed := false
	for key, value := range existingObj {
		if _, present := incoming[key]; !present {
			incoming[key] = value
			changed = true
		}
	}
	if !changed {
		return item
	}
	merged, err := json.Marshal(incoming)
	if err != nil {
		return item
	}
	return merged
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

// DistinctWorkoutRideIDs returns every distinct ride/class id referenced by
// a synced workout, preferring the typed "ride_id" column but falling back
// to the raw ride.id nested field (json_extract on the workouts table's own
// "data" column) when the column is empty or missing for that row. The two
// can diverge: enrichWorkoutRideMetadata only promotes ride.id to the
// top-level ride_id column at WRITE time, so a workout whose raw JSON
// carries a "ride" object but was written before that promotion existed
// (or by any future write path that stores workouts without going through
// it) keeps a populated "ride" object with no corresponding typed column
// value. Round-12 verification NEW B found a real account where only 511
// of 843 ride-bearing workouts had the typed column populated -- callers
// that need the full set of taken classes (classes_detail's fan-out via
// planClassDetailSync) must query both, not just the typed column.
func (s *Store) DistinctWorkoutRideIDs() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT COALESCE(NULLIF("ride_id", ''), json_extract("data", '$.ride.id')) AS rid
		FROM "workouts"
		WHERE COALESCE(NULLIF("ride_id", ''), json_extract("data", '$.ride.id')) IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
// way UpsertBatch/Upsert do — including the single-key envelope-unwrap
// fallback (unwrapIDBearingEnvelopeItem) UpsertBatch falls back to when the
// outer object has no direct id — and records it as a provider fact.
// Without the unwrap fallback, an item UpsertBatch itself successfully
// stores (e.g. {"workout":{"id":"w1",...}}) would still get silently
// dropped here: the outer-envelope ExtractResourceID call that fails for
// UpsertBatch would fail identically here, but UpsertBatch has already
// moved on to the unwrapped inner object by the time it stores the row.
// Failures (undecodable item, unresolvable ID even after unwrap) are
// skipped rather than propagated: the authoritative write to
// `resources`/typed tables already succeeded, and this secondary index
// exists to serve `offline` reads, not to gate sync.
func (s *Store) recordProviderFactsBestEffort(resourceType string, items []json.RawMessage) {
	for _, item := range items {
		obj, err := DecodeJSONObject(item)
		if err != nil {
			continue
		}
		id := ExtractResourceID(resourceType, obj)
		if id == "" {
			if unwrappedObj, unwrappedItem, ok := unwrapIDBearingEnvelopeItem(resourceType, item, obj); ok {
				obj = unwrappedObj
				item = unwrappedItem
				id = ExtractResourceID(resourceType, obj)
			}
		}
		if id == "" {
			id = nestedContainerResourceID(resourceType, obj)
		}
		if id == "" {
			continue
		}
		_, _ = s.RecordProviderFact(resourceType, id, item)
	}
}

// nestedIDContainerKeys maps a resourceType to the key holding its real id
// one level down, for single-object endpoint responses whose id doesn't sit
// at the top level. unwrapIDBearingEnvelopeItem can't cover this case: it
// requires exactly one object-valued top-level field, but classes_show's
// response has many (ride, playlist, averages, segments, ...), only one of
// which ("ride") carries the id.
var nestedIDContainerKeys = map[string]string{
	"classes": "ride",
}

// nestedContainerResourceID resolves an id nested one level down per
// nestedIDContainerKeys, without disturbing the item that actually gets
// cached: unlike unwrapIDBearingEnvelopeItem, the FULL outer object is what
// gets stored (its sibling fields -- e.g. classes_show's top-level
// "segments"/"averages" alongside "ride" -- are real content offline
// readers need, e.g. offline_classes_structure, not envelope noise to
// discard).
func nestedContainerResourceID(resourceType string, obj map[string]any) string {
	key, ok := nestedIDContainerKeys[resourceType]
	if !ok {
		return ""
	}
	inner, ok := obj[key].(map[string]any)
	if !ok {
		return ""
	}
	return ExtractResourceID(resourceType, inner)
}
