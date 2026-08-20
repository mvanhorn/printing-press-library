package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"
	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/store"
)

// openLocalStore opens the local SQLite mirror at the default path,
// returning a nil-safe "no local mirror" signal so callers can print the
// standard sync hint and return an empty result instead of a raw SQLite
// open failure.
func openLocalStore(ctx context.Context) (*store.Store, bool, error) {
	dbPath := defaultDBPath("fpi-india-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, false, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, true, err
	}
	return db, true, nil
}

// loadNetInvestmentRows reads every synced net_investment record matching
// periodType/currency, ordered by period ascending.
func loadNetInvestmentRows(db *store.Store, periodType, currency string) ([]nsdl.NetInvestmentRow, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'net_investment'
		   AND json_extract(data, '$.period_type') = ?
		   AND json_extract(data, '$.currency') = ?`,
		periodType, currency,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []nsdl.NetInvestmentRow
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if !raw.Valid {
			continue
		}
		var r nsdl.NetInvestmentRow
		if err := json.Unmarshal([]byte(raw.String), &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period < out[j].Period })
	return out, rows.Err()
}

// aucSnapshotRecord is one synced AUC row (country or category), keyed with
// the synced_date each syncAUC call attached.
type aucSnapshotRecord struct {
	By         string
	Key        string // Country or Category name
	SyncedDate string
	Fields     map[string]string
}

func loadAUCSnapshots(db *store.Store, by string) ([]aucSnapshotRecord, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'auc'
		   AND json_extract(data, '$.by') = ?`, by)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []aucSnapshotRecord
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if !raw.Valid {
			continue
		}
		var fields map[string]string
		if err := json.Unmarshal([]byte(raw.String), &fields); err != nil {
			continue
		}
		key := fields["Country"]
		if key == "" {
			key = fields["Category"]
		}
		out = append(out, aucSnapshotRecord{
			By:         fields["by"],
			Key:        key,
			SyncedDate: fields["synced_date"],
			Fields:     fields,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SyncedDate < out[j].SyncedDate })
	return out, rows.Err()
}

// sectorSnapshotRecord is one synced sector row for one fortnight.
type sectorSnapshotRecord struct {
	PeriodLabel string
	SectorName  string
	Fields      map[string]string
}

func loadSectorSnapshots(db *store.Store) ([]sectorSnapshotRecord, error) {
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = 'sector'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sectorSnapshotRecord
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if !raw.Valid {
			continue
		}
		var fields map[string]string
		if err := json.Unmarshal([]byte(raw.String), &fields); err != nil {
			continue
		}
		name := fields["Sectors"]
		if name == "" {
			name = fields["Sector"]
		}
		// The sector table ends with a "Grand Total" summary row (a market-
		// wide aggregate, not a real sector), and column alignment for that
		// row can leave its label under whatever key normally holds the
		// sector name. Excluding it here (rather than in the parser) keeps
		// the raw synced record available for `sector` while stopping the
		// aggregate from posing as a sector in rotation rankings.
		if name == "" || strings.Contains(strings.ToLower(name), "total") {
			continue
		}
		out = append(out, sectorSnapshotRecord{
			PeriodLabel: fields["period_label"],
			SectorName:  name,
			Fields:      fields,
		})
	}
	return out, rows.Err()
}

// sectorTotal extracts a representative net-investment figure for a sector
// row: the last (rightmost / most-recent) column whose composite name
// contains "Total", falling back to the largest numeric field present. The
// sector table's header nesting is deep enough that exact column identity
// varies release to release; a "biggest total-shaped number" heuristic is
// robust to that drift where an exact key match would not be.
func sectorTotal(fields map[string]string) (float64, bool) {
	best := 0.0
	found := false
	// map iteration order is randomized, so ties in abs(n) need a
	// stable secondary key (lexicographically smallest "Total*" column
	// name) or which one wins would vary between identical runs.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(k) < 5 || k[:5] != "Total" {
			continue
		}
		n, ok := nsdl.ParseNumber(fields[k])
		if !ok {
			continue
		}
		if !found || abs(n) > abs(best) {
			best = n
			found = true
		}
	}
	return best, found
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// filmLimitRecord mirrors the synced typed limits table row shape (see the
// spec's FilmLimit type / db.UpsertLimits).
type filmLimitRecord struct {
	ISIN           string
	Issuer         string
	NRILimit       string
	FPILimit       string
	SectoralCap    string
	MonitoredLimit string
	Remarks        string
	ReportingDate  string
}

func loadLimitsMatching(db *store.Store, needle string) ([]filmLimitRecord, error) {
	rows, err := db.DB().Query(
		`SELECT film_isin, film_issuer, film_nri_limit, film_fpi_limit, film_sectorial_cap,
		        film_monitorfpi_limit, film_remarks, reporting_date
		   FROM "limits"
		   WHERE film_issuer LIKE '%' || ? || '%' OR film_remarks LIKE '%' || ? || '%'`,
		needle, needle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []filmLimitRecord
	for rows.Next() {
		var r filmLimitRecord
		var isin, issuer, nri, fpi, sect, mon, rem, date sql.NullString
		if err := rows.Scan(&isin, &issuer, &nri, &fpi, &sect, &mon, &rem, &date); err != nil {
			continue
		}
		r.ISIN, r.Issuer, r.NRILimit, r.FPILimit = isin.String, issuer.String, nri.String, fpi.String
		r.SectoralCap, r.MonitoredLimit, r.Remarks, r.ReportingDate = sect.String, mon.String, rem.String, date.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func usageErrf(format string, args ...any) error {
	return usageErr(fmt.Errorf(format, args...))
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return data
}
