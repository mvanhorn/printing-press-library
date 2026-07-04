// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored dependent-resource sync for the Sutra Partner API.
//
// Every Sutra path is scoped under /{partnerId}/, and two collections are
// parent-keyed beyond that: reservations live under a class
// (/{partnerId}/classes/{classId}/reservations) and rooms live under a
// location (/{partnerId}/locations/{locationId}/rooms). The generated flat
// sync worker pool cannot fill these because their paths need a parent ID it
// does not have. syncDependentChildren iterates the already-synced parent
// tables and fetches each child collection, injecting the parent key the
// typed upserts require (reservations.classes_id, rooms.locations_id).
//
// This lives in its own file so it survives `generate --force` regen-merge as
// a whole hand-authored unit; the call site in sync.go is the only generated
// edit.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/internal/store"
)

// decodeSutraCursor URL-decodes a pagination cursor before it is re-encoded by
// the HTTP client's query builder. The Sutra API returns
// pagination.nextStartAfterId already %2F-encoded (e.g.
// "instructor_dashboards%2F<partner>%2Fclients%2F<id>"); the generated client
// builds query strings with url.Values.Encode(), which would re-encode the "%"
// to "%25" and produce a double-encoded "%252F" the API treats as an unknown
// cursor, returning an empty page and silently capping sync at the first page.
// Decoding here yields the literal slashes so the client single-encodes them
// back to %2F (the form the API accepts). Falls back to the raw value when it
// is not valid percent-encoding, so opaque cursors are left untouched.
func decodeSutraCursor(s string) string {
	if s == "" {
		return s
	}
	if dec, err := url.QueryUnescape(s); err == nil {
		return dec
	}
	return s
}

// sutraSyncClient is the subset of the generated *client.Client used by
// dependent sync.
type sutraSyncClient interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// dependentSyncResources are parent-keyed collections handled by
// syncDependentChildren rather than the flat worker pool.
var dependentSyncResources = map[string]bool{
	"reservations": true,
	"rooms":        true,
}

// partitionSyncResources splits the resolved resource list into flat resources
// (handled by the generic worker pool) and dependent resources. Naming a
// parent cascades to its dependent: "classes" pulls in "reservations",
// "locations" pulls in "rooms", matching the documented sync scoping contract.
func partitionSyncResources(resources []string) (flat []string, dependents []string) {
	depSet := map[string]bool{}
	for _, r := range resources {
		if dependentSyncResources[r] {
			depSet[r] = true
			continue
		}
		flat = append(flat, r)
		switch r {
		case "classes":
			depSet["reservations"] = true
		case "locations":
			depSet["rooms"] = true
		}
	}
	// Stable, parent-before-child order.
	for _, r := range []string{"reservations", "rooms"} {
		if depSet[r] {
			dependents = append(dependents, r)
		}
	}
	return flat, dependents
}

// syncDependentChildren fetches reservations (per class) and rooms (per
// location) from the live API and upserts them. It returns aggregate counts
// folded into the sync summary by the caller. Each dependent resource emits
// the same sync_start / sync_summary-shaped NDJSON events as the flat loop.
func syncDependentChildren(ctx context.Context, c sutraSyncClient, db *store.Store, dependents []string, maxPages int, syncEvents io.Writer) (records, success, warned, errored int) {
	if len(dependents) == 0 {
		return 0, 0, 0, 0
	}
	partnerID := os.Getenv("SUTRA_PARTNER_ID")
	if partnerID == "" {
		// Real syncs are gated on SUTRA_PARTNER_ID upstream; reaching here with
		// an empty value means a verify/mock run, where dependents add nothing.
		return 0, 0, 0, 0
	}

	for _, dep := range dependents {
		emitDepSyncStart(syncEvents, dep)
		var n int
		var err error
		switch dep {
		case "reservations":
			n, err = syncReservationsForClasses(ctx, c, db, partnerID, maxPages)
		case "rooms":
			n, err = syncRoomsForLocations(ctx, c, db, partnerID, maxPages)
		default:
			continue
		}
		if err != nil {
			emitDepSyncError(syncEvents, dep, err)
			errored++
			continue
		}
		records += n
		success++
		emitDepSyncDone(syncEvents, dep, n)
	}
	return records, success, warned, errored
}

// syncReservationsForClasses iterates synced class IDs and fetches each class's
// reservations, injecting classes_id (the column the typed upsert reads) before
// storing.
func syncReservationsForClasses(ctx context.Context, c sutraSyncClient, db *store.Store, partnerID string, maxPages int) (int, error) {
	classIDs, err := parentRowIDs(ctx, db, "classes")
	if err != nil {
		return 0, err
	}
	classIDs = capParentFanout(classIDs)
	total := 0
	for _, classID := range classIDs {
		path := "/" + partnerID + "/classes/" + classID + "/reservations"
		items, err := fetchAllSutraPages(ctx, c, path, maxPages)
		if err != nil {
			return total, fmt.Errorf("fetching reservations for class %s: %w", classID, err)
		}
		for _, item := range items {
			enriched, err := injectStringField(item, "classes_id", classID)
			if err != nil {
				continue
			}
			if err := db.UpsertReservations(enriched); err != nil {
				return total, fmt.Errorf("storing reservation for class %s: %w", classID, err)
			}
			total++
		}
	}
	return total, nil
}

// syncRoomsForLocations iterates synced location IDs and fetches each
// location's rooms, injecting locations_id before storing.
func syncRoomsForLocations(ctx context.Context, c sutraSyncClient, db *store.Store, partnerID string, maxPages int) (int, error) {
	locationIDs, err := parentRowIDs(ctx, db, "locations")
	if err != nil {
		return 0, err
	}
	locationIDs = capParentFanout(locationIDs)
	total := 0
	for _, locationID := range locationIDs {
		path := "/" + partnerID + "/locations/" + locationID + "/rooms"
		items, err := fetchAllSutraPages(ctx, c, path, maxPages)
		if err != nil {
			return total, fmt.Errorf("fetching rooms for location %s: %w", locationID, err)
		}
		for _, item := range items {
			enriched, err := injectStringField(item, "locations_id", locationID)
			if err != nil {
				continue
			}
			if err := db.UpsertRooms(enriched); err != nil {
				return total, fmt.Errorf("storing room for location %s: %w", locationID, err)
			}
			total++
		}
	}
	return total, nil
}

// capParentFanout bounds the per-parent request fan-out under live dogfood so a
// studio with hundreds of classes cannot blow the matrix timeout. Real syncs
// (no dogfood env) are uncapped.
func capParentFanout(ids []string) []string {
	if cliutil.IsDogfoodEnv() && len(ids) > 3 {
		return ids[:3]
	}
	return ids
}

// parentRowIDs returns the primary-key IDs from a synced parent table. table is
// an internal constant ("classes" | "locations"), never user input.
func parentRowIDs(ctx context.Context, db *store.Store, table string) ([]string, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT id FROM "`+table+`"`)
	if err != nil {
		return nil, fmt.Errorf("listing %s ids: %w", table, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// sutraPageEnvelope mirrors the Sutra list response shape:
// {"items":[...], "pagination":{"nextStartAfterId": "...|null", "hasMore": bool}}.
type sutraPageEnvelope struct {
	Items      []json.RawMessage `json:"items"`
	Pagination struct {
		NextStartAfterID string `json:"nextStartAfterId"`
		HasMore          bool   `json:"hasMore"`
	} `json:"pagination"`
}

// fetchAllSutraPages walks the cursor-paginated list endpoint at path using the
// Sutra start_after / pagination.nextStartAfterId / pagination.hasMore contract,
// returning every item across pages (bounded by maxPages).
func fetchAllSutraPages(ctx context.Context, c sutraSyncClient, path string, maxPages int) ([]json.RawMessage, error) {
	if maxPages <= 0 {
		maxPages = 50
	}
	var all []json.RawMessage
	cursor := ""
	for page := 0; page < maxPages; page++ {
		params := map[string]string{"limit": "100"}
		if cursor != "" {
			params["start_after"] = decodeSutraCursor(cursor)
		}
		data, err := c.Get(ctx, path, params)
		if err != nil {
			return all, err
		}
		// Dry-run/mock sentinel: nothing to page.
		if isDryRunResponse(data) {
			return all, nil
		}
		var env sutraPageEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			// Tolerate a bare array response shape.
			var arr []json.RawMessage
			if json.Unmarshal(data, &arr) == nil {
				return append(all, arr...), nil
			}
			return all, nil
		}
		all = append(all, env.Items...)
		if !env.Pagination.HasMore || env.Pagination.NextStartAfterID == "" {
			break
		}
		cursor = env.Pagination.NextStartAfterID
	}
	return all, nil
}

// injectStringField adds key=value into a JSON object, used to stamp the parent
// foreign key the typed upserts read (classes_id / locations_id).
func injectStringField(data json.RawMessage, key, value string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	obj[key] = encoded
	return json.Marshal(obj)
}

func emitDepSyncStart(w io.Writer, resource string) {
	if humanFriendly {
		return
	}
	fmt.Fprintf(w, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
}

func emitDepSyncDone(w io.Writer, resource string, n int) {
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "  %s: %d synced (done)\n", resource, n)
		return
	}
	fmt.Fprintf(w, `{"event":"sync_resource_done","resource":"%s","records":%d}`+"\n", resource, n)
}

func emitDepSyncError(w io.Writer, resource string, err error) {
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "  %s: error: %v\n", resource, err)
		return
	}
	fmt.Fprintf(w, "%s\n", syncErrorJSON(resource, "", err))
}
