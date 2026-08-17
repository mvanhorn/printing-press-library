package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/store"
)

// Shared DNS Made Easy helpers for the hand-written cross-zone commands
// (sync-records, where-used, drift, health, export, bulk-apply, acme-purge).
//
// The framework sync cannot mirror records: their list endpoint is
// hierarchical (/dns/managed/{domainId}/records) so it is skipped, and the
// generated table has no zone column. These helpers fetch domains and their
// records live, tag each record with its zone, and read/write the
// zone_records / record_snapshots extension tables (see
// internal/store/dnsmadeeasy_migrations.go).

// flexID unmarshals a DNS Made Easy numeric id that may appear as a JSON number
// or string, and always renders back as a plain decimal string.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		*f = ""
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		*f = flexID(s[1 : len(s)-1])
		return nil
	}
	*f = flexID(s)
	return nil
}

func (f flexID) String() string { return string(f) }

type dmeDomain struct {
	ID   flexID `json:"id"`
	Name string `json:"name"`
}

type dmeRecord struct {
	ID          flexID `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	TTL         int    `json:"ttl"`
	GtdLocation string `json:"gtdLocation"`
	MxLevel     int    `json:"mxLevel"`
	Priority    int    `json:"priority"`
	Weight      int    `json:"weight"`
	Port        int    `json:"port"`
	// DomainID / DomainName are filled in by the fetch loop, not the API.
	DomainID   string `json:"-"`
	DomainName string `json:"-"`
	// Raw preserves the original record JSON for --json output fidelity.
	Raw json.RawMessage `json:"-"`
}

type dmeListEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// dmeClient is the subset of *client.Client the helpers use, so tests can fake it.
type dmeClient interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

func decodeListEnvelope(body json.RawMessage) (json.RawMessage, error) {
	// DNS Made Easy list endpoints wrap results in {"data":[...],...}. Some
	// single-object endpoints return the object directly; callers that expect
	// an array pass through here.
	trimmed := body
	var env dmeListEnvelope
	if err := json.Unmarshal(body, &env); err == nil && len(env.Data) > 0 {
		return env.Data, nil
	}
	return trimmed, nil
}

// fetchAllDomains lists every managed domain (id + name).
func fetchAllDomains(ctx context.Context, c dmeClient) ([]dmeDomain, error) {
	body, err := c.Get(ctx, "/dns/managed", nil)
	if err != nil {
		return nil, err
	}
	data, err := decodeListEnvelope(body)
	if err != nil {
		return nil, err
	}
	var domains []dmeDomain
	if err := json.Unmarshal(data, &domains); err != nil {
		return nil, fmt.Errorf("parsing domains: %w", err)
	}
	return domains, nil
}

// fetchZoneRecords fetches all records for one domain, tagged with the zone.
func fetchZoneRecords(ctx context.Context, c dmeClient, dom dmeDomain) ([]dmeRecord, error) {
	path := "/dns/managed/" + dom.ID.String() + "/records"
	body, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	data, err := decodeListEnvelope(body)
	if err != nil {
		return nil, err
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("parsing records for %s: %w", dom.Name, err)
	}
	records := make([]dmeRecord, 0, len(raws))
	for _, raw := range raws {
		var r dmeRecord
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		r.DomainID = dom.ID.String()
		r.DomainName = dom.Name
		r.Raw = raw
		records = append(records, r)
	}
	return records, nil
}

// fetchAllZoneRecords fetches records across every managed domain, tagged with
// their zone. It returns a partial-failure map keyed by domain name so callers
// can report which zones could not be read instead of silently dropping them.
func fetchAllZoneRecords(ctx context.Context, c dmeClient) ([]dmeRecord, []dmeDomain, map[string]string, error) {
	domains, err := fetchAllDomains(ctx, c)
	if err != nil {
		return nil, nil, nil, err
	}
	var all []dmeRecord
	failures := map[string]string{}
	for _, dom := range domains {
		if ctx.Err() != nil {
			return all, domains, failures, ctx.Err()
		}
		recs, err := fetchZoneRecords(ctx, c, dom)
		if err != nil {
			failures[dom.Name] = err.Error()
			continue
		}
		all = append(all, recs...)
	}
	return all, domains, failures, nil
}

// writeZoneMirror replaces the zone_records table with the supplied records and
// appends a snapshot batch for drift. It runs inside a single write transaction.
func writeZoneMirror(ctx context.Context, s *store.Store, records []dmeRecord) (batchID string, err error) {
	if err := s.EnsureDNSMEExtensions(ctx); err != nil {
		return "", err
	}
	batchID = time.Now().UTC().Format("20060102T150405.000Z")
	takenAt := time.Now().UTC().Format(time.RFC3339Nano)

	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM zone_records`); err != nil {
		return "", err
	}
	insZone, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO zone_records
		(domain_id, record_id, domain_name, name, type, value, ttl, gtd_location, mx_level, priority, weight, port, data, synced_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)`)
	if err != nil {
		return "", err
	}
	defer insZone.Close()
	insSnap, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO record_snapshots
		(batch_id, taken_at, domain_id, domain_name, record_id, name, type, value, ttl)
		VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return "", err
	}
	defer insSnap.Close()

	for _, r := range records {
		raw := r.Raw
		if len(raw) == 0 {
			raw = json.RawMessage("{}")
		}
		if _, err = insZone.ExecContext(ctx, r.DomainID, r.ID.String(), r.DomainName, r.Name, r.Type, r.Value, r.TTL, r.GtdLocation, r.MxLevel, r.Priority, r.Weight, r.Port, string(raw)); err != nil {
			return "", err
		}
		if _, err = insSnap.ExecContext(ctx, batchID, takenAt, r.DomainID, r.DomainName, r.ID.String(), r.Name, r.Type, r.Value, r.TTL); err != nil {
			return "", err
		}
	}

	// Keep only the most recent 10 snapshot batches to bound growth.
	if _, err = tx.ExecContext(ctx, `DELETE FROM record_snapshots WHERE batch_id NOT IN (
		SELECT batch_id FROM record_snapshots GROUP BY batch_id ORDER BY taken_at DESC LIMIT 10)`); err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}
	return batchID, nil
}

// loadZoneRecords reads the mirrored records from zone_records. It uses the
// drain-first pattern: scan the whole result set, then close, so callers may
// issue follow-up queries safely.
func loadZoneRecords(ctx context.Context, s *store.Store) ([]dmeRecord, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT domain_id, domain_name, record_id, name, type, value, ttl, gtd_location, mx_level, priority, weight, port, data
		FROM zone_records ORDER BY domain_name, name, type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dmeRecord
	for rows.Next() {
		var r dmeRecord
		var recID, domID string
		var data string
		if err := rows.Scan(&domID, &r.DomainName, &recID, &r.Name, &r.Type, &r.Value, &r.TTL, &r.GtdLocation, &r.MxLevel, &r.Priority, &r.Weight, &r.Port, &data); err != nil {
			return nil, err
		}
		r.DomainID = domID
		r.ID = flexID(recID)
		r.Raw = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

// zoneRecordCount returns how many records are in the local mirror.
func zoneRecordCount(ctx context.Context, s *store.Store) (int, error) {
	var n int
	err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM zone_records`).Scan(&n)
	return n, err
}

// atoiSafe parses a decimal string, returning 0 on error.
func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ensure client satisfies the interface (compile-time check).
var _ dmeClient = (*client.Client)(nil)
