// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored support code for the novel (transcendence) commands: topology,
// drift, newcomer, port-audit, guest report, rule-predict.
// Not generator output.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

// errNoSitesLocal means the local mirror database exists but has no synced
// site rows yet (e.g. sync was interrupted before the sites resource
// finished, or another command created an empty schema-only DB file).
// Callers should treat this the same as "no local mirror at all" — a
// graceful empty result plus a sync hint, not a hard error — via
// isNoLocalDataYet below.
var errNoSitesLocal = errors.New("no sites found in local mirror; run 'unifi-pp-cli sync' first")

// isNoLocalDataYet reports whether err represents "the local mirror has no
// usable data yet," so a novel command can fall back to its graceful
// no-data envelope instead of a hard failure.
func isNoLocalDataYet(err error) bool {
	return errors.Is(err, errNoSitesLocal)
}

// openNovelStore opens the local SQLite mirror read-only for a novel command.
// Every novel command in this CLI is local-mirror-only: it never calls the
// live API itself (topology is the one exception, documented on its own
// command, because device-to-device uplink data is not present in any
// synced list response). Returns (nil, nil) when the DB file does not exist
// yet, signaling "not synced" to the caller so it can print sync guidance
// instead of a raw open error.
func openNovelStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("unifi-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, nil
	}
	return store.OpenReadOnlyContext(ctx, dbPath)
}

// resourceRows reads every row of a typed local-mirror table (devices,
// clients, networks, wifi, dns, radius, wans, firewall, hotspot, switching,
// vpn, acl_rules, device_tags, traffic_matching_lists), scoped to one site,
// keyed by the entity's real id (the "id" field inside data) — NOT the SQL
// "id" column. store.resourceStorageID() writes a synthetic composite key
// (entity id + a NUL byte + a scope suffix) into the "id" column so a single-
// column PRIMARY KEY stays unique across sites/resources; every FK-style
// reference elsewhere in the API (e.g. a client's uplinkDeviceId) uses the
// real entity id, so that is the key novel commands need to join on.
func resourceRows(ctx context.Context, db *sql.DB, table, siteID string) (map[string]json.RawMessage, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT data FROM "%s" WHERE sites_id = ?`, table), siteID) //nolint:gosec // table is one of a fixed internal allowlist of known typed table names, never user input
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", table, err)
	}
	raw := make([]json.RawMessage, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan %s row: %w", table, err)
		}
		raw = append(raw, json.RawMessage(data))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate %s rows: %w", table, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close %s rows: %w", table, err)
	}
	return keyByEmbeddedID(raw), nil
}

// keyByEmbeddedID re-keys a slice of JSON row payloads by their own "id"
// field, skipping rows with no id (which should not occur for these
// resources, but a silently dropped row beats a silently wrong key).
func keyByEmbeddedID(rows []json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(rows))
	for _, data := range rows {
		var withID struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(data, &withID) != nil || withID.ID == "" {
			continue
		}
		out[withID.ID] = data
	}
	return out
}

// genericResourceRows reads rows from the fallback generic "resources" table
// for resource kinds the generator routed there instead of a typed table
// (observed for firewall zones/policies in this API, under resource_type
// values "v1_sites_firewall_zones" / "v1_sites_firewall_policies" rather than
// the typed "firewall" table). Every row's data embeds a "sites_id" field
// regardless of routing, so site scoping works the same way here.
func genericResourceRows(ctx context.Context, db *sql.DB, resourceType, siteID string) (map[string]json.RawMessage, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = ? AND json_extract(data, '$.sites_id') = ?`, resourceType, siteID)
	if err != nil {
		return nil, fmt.Errorf("query resources(%s): %w", resourceType, err)
	}
	raw := make([]json.RawMessage, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan resources(%s) row: %w", resourceType, err)
		}
		raw = append(raw, json.RawMessage(data))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate resources(%s) rows: %w", resourceType, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close resources(%s) rows: %w", resourceType, err)
	}
	return keyByEmbeddedID(raw), nil
}

// resolveSiteIDLocal resolves a --site flag value (a site UUID, its
// internalReference like "default", or its display name) against the
// locally synced sites list. Empty siteArg auto-selects when exactly one
// site is synced. Sites live in the generic "resources" table under
// resource_type "sites" (this API's single top-level, non-nested resource).
func resolveSiteIDLocal(ctx context.Context, db *sql.DB, siteArg string) (id string, name string, err error) {
	rows, err := db.QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = 'sites'`)
	if err != nil {
		return "", "", fmt.Errorf("query sites: %w", err)
	}
	defer rows.Close()

	type site struct {
		ID                string `json:"id"`
		Name              string `json:"name"`
		InternalReference string `json:"internalReference"`
	}
	var sites []site
	for rows.Next() {
		var rowID string
		var data string
		if scanErr := rows.Scan(&rowID, &data); scanErr != nil {
			return "", "", fmt.Errorf("scan site row: %w", scanErr)
		}
		var s site
		if jsonErr := json.Unmarshal([]byte(data), &s); jsonErr != nil {
			continue
		}
		if s.ID == "" {
			s.ID = rowID
		}
		sites = append(sites, s)
	}
	if err := rows.Err(); err != nil {
		return "", "", fmt.Errorf("iterate site rows: %w", err)
	}

	if siteArg == "" {
		switch len(sites) {
		case 0:
			return "", "", errNoSitesLocal
		case 1:
			return sites[0].ID, sites[0].Name, nil
		default:
			names := make([]string, 0, len(sites))
			for _, s := range sites {
				names = append(names, fmt.Sprintf("%s (%s)", s.InternalReference, s.ID))
			}
			return "", "", fmt.Errorf("multiple sites synced, pass --site: %v", names)
		}
	}
	for _, s := range sites {
		if s.ID == siteArg || s.InternalReference == siteArg || s.Name == siteArg {
			return s.ID, s.Name, nil
		}
	}
	return "", "", notFoundErr(fmt.Errorf("site %q not found in local mirror", siteArg))
}

// novelSnapshotDir returns (creating if needed) the directory novel
// point-in-time snapshots are persisted under, alongside the SQLite data
// directory.
func novelSnapshotDir() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	snapDir := filepath.Join(dir, "novel-snapshots")
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		return "", err
	}
	return snapDir, nil
}

// resourceSnapshot is a point-in-time capture of one typed/generic table for
// one site, persisted to disk by drift so the next drift run has a prior
// state to diff against. The local SQLite mirror itself only ever holds
// current state (sync upserts in place), so drift/newcomer maintain their
// own history on top of it — see drift.go's doc comment for the exact
// semantics this implies.
type resourceSnapshot struct {
	CapturedAt time.Time                  `json:"captured_at"`
	Entities   map[string]json.RawMessage `json:"entities"`
}

func snapshotPath(dir, table, siteID string) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s.json", table, siteID))
}

func loadResourceSnapshot(path string) (*resourceSnapshot, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built from an internal allowlist + a synced site id, never raw user input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap resourceSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing snapshot %s: %w", path, err)
	}
	return &snap, nil
}

func saveResourceSnapshot(path string, snap resourceSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// sortedKeys returns the map's keys in a stable, sorted order so command
// output (and tests) are deterministic.
func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
