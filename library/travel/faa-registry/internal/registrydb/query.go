// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package registrydb

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Aircraft is an offline registration record joined to its model and engine
// reference rows.
type Aircraft struct {
	NNumber        string   `json:"n_number"`
	SerialNumber   string   `json:"serial_number,omitempty"`
	Manufacturer   string   `json:"manufacturer,omitempty"`
	Model          string   `json:"model,omitempty"`
	YearMfr        string   `json:"year_mfr,omitempty"`
	AircraftType   string   `json:"aircraft_type,omitempty"`
	EngineType     string   `json:"engine_type,omitempty"`
	EngineMfr      string   `json:"engine_manufacturer,omitempty"`
	EngineModel    string   `json:"engine_model,omitempty"`
	Horsepower     string   `json:"horsepower,omitempty"`
	Thrust         string   `json:"thrust,omitempty"`
	Seats          string   `json:"seats,omitempty"`
	Speed          string   `json:"cruise_speed_mph,omitempty"`
	OwnerName      string   `json:"owner_name,omitempty"`
	OwnerType      string   `json:"owner_type,omitempty"`
	Street         string   `json:"street,omitempty"`
	City           string   `json:"city,omitempty"`
	State          string   `json:"state,omitempty"`
	ZipCode        string   `json:"zip_code,omitempty"`
	Country        string   `json:"country,omitempty"`
	Region         string   `json:"region,omitempty"`
	OtherNames     []string `json:"other_owner_names,omitempty"`
	Status         string   `json:"status,omitempty"`
	CertIssueDate  string   `json:"certificate_issue_date,omitempty"`
	ExpirationDate string   `json:"expiration_date,omitempty"`
	LastActionDate string   `json:"last_action_date,omitempty"`
	AirWorthDate   string   `json:"airworthiness_date,omitempty"`
	Airworthiness  string   `json:"airworthiness_class,omitempty"`
	ModeSHex       string   `json:"mode_s_hex,omitempty"`
	ModeSOctal     string   `json:"mode_s_octal,omitempty"`
	FractOwner     string   `json:"fractional_owner,omitempty"`
	KitMfr         string   `json:"kit_manufacturer,omitempty"`
	KitModel       string   `json:"kit_model,omitempty"`
}

const aircraftSelect = `
SELECT m.n_number, m.serial_number, COALESCE(a.mfr,''), COALESCE(a.model,''),
	m.year_mfr, m.type_aircraft, m.type_engine,
	COALESCE(e.mfr,''), COALESCE(e.model,''), COALESCE(e.horsepower,''), COALESCE(e.thrust,''),
	COALESCE(a.no_seats,''), COALESCE(a.speed,''),
	m.name, m.type_registrant, m.street, m.city, m.state, m.zip_code, m.country, m.region,
	m.other_name_1, m.other_name_2, m.other_name_3, m.other_name_4, m.other_name_5,
	m.status_code, m.cert_issue_date, m.expiration_date, m.last_action_date,
	m.air_worth_date, m.certification, m.mode_s_code_hex, m.mode_s_code, m.fract_owner,
	m.kit_mfr, m.kit_model
FROM faa_master m
LEFT JOIN faa_acftref a ON a.code = m.mfr_mdl_code
LEFT JOIN faa_engine e ON e.code = m.eng_mfr_mdl
`

// fmtFAADate renders the registry's YYYYMMDD date serials as ISO 8601
// (YYYY-MM-DD); anything else passes through unchanged.
func fmtFAADate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return s
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return s
		}
	}
	return s[:4] + "-" + s[4:6] + "-" + s[6:]
}

func scanAircraft(rows *sql.Rows) (*Aircraft, error) {
	var ac Aircraft
	var typeReg, status, cert string
	var on [5]string
	if err := rows.Scan(&ac.NNumber, &ac.SerialNumber, &ac.Manufacturer, &ac.Model,
		&ac.YearMfr, &ac.AircraftType, &ac.EngineType,
		&ac.EngineMfr, &ac.EngineModel, &ac.Horsepower, &ac.Thrust,
		&ac.Seats, &ac.Speed,
		&ac.OwnerName, &typeReg, &ac.Street, &ac.City, &ac.State, &ac.ZipCode, &ac.Country, &ac.Region,
		&on[0], &on[1], &on[2], &on[3], &on[4],
		&status, &ac.CertIssueDate, &ac.ExpirationDate, &ac.LastActionDate,
		&ac.AirWorthDate, &cert, &ac.ModeSHex, &ac.ModeSOctal, &ac.FractOwner,
		&ac.KitMfr, &ac.KitModel); err != nil {
		return nil, err
	}
	ac.NNumber = "N" + ac.NNumber
	ac.OwnerType = DecodeRegistrantType(typeReg)
	ac.AircraftType = DecodeAircraftType(ac.AircraftType)
	ac.EngineType = DecodeEngineType(ac.EngineType)
	ac.Region = DecodeRegion(ac.Region)
	ac.Status = DecodeStatus(status)
	ac.Airworthiness = DecodeAirworthinessClass(cert)
	ac.CertIssueDate = fmtFAADate(ac.CertIssueDate)
	ac.ExpirationDate = fmtFAADate(ac.ExpirationDate)
	ac.LastActionDate = fmtFAADate(ac.LastActionDate)
	ac.AirWorthDate = fmtFAADate(ac.AirWorthDate)
	for _, n := range on {
		if n != "" {
			ac.OtherNames = append(ac.OtherNames, n)
		}
	}
	return &ac, nil
}

func (d *DB) queryAircraft(ctx context.Context, where string, args ...any) ([]*Aircraft, error) {
	rows, err := d.db.QueryContext(ctx, aircraftSelect+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Aircraft
	for rows.Next() {
		ac, err := scanAircraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ac)
	}
	return out, rows.Err()
}

// LookupTail returns the offline record for an N-number, or nil when absent.
func (d *DB) LookupTail(ctx context.Context, tail string) (*Aircraft, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	acs, err := d.queryAircraft(ctx, `WHERE m.n_number = ?`, NormalizeTail(tail))
	if err != nil || len(acs) == 0 {
		return nil, err
	}
	return acs[0], nil
}

// LookupHex returns the offline record for a Mode S hex address, or nil.
func (d *DB) LookupHex(ctx context.Context, hex string) (*Aircraft, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	h := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hex), "0x")))
	acs, err := d.queryAircraft(ctx, `WHERE m.mode_s_code_hex = ?`, h)
	if err != nil || len(acs) == 0 {
		return nil, err
	}
	return acs[0], nil
}

// ownerWhere matches an owner name against the registrant and all five
// other-name columns, prefix-style (the registry stores ALL CAPS).
const ownerWhere = `(m.name LIKE ? OR m.other_name_1 LIKE ? OR m.other_name_2 LIKE ?
	OR m.other_name_3 LIKE ? OR m.other_name_4 LIKE ? OR m.other_name_5 LIKE ?)`

func ownerArgs(owner string) []any {
	p := strings.ToUpper(strings.TrimSpace(owner)) + "%"
	return []any{p, p, p, p, p, p}
}

// FleetAircraft lists every aircraft where the owner appears as registrant or
// co-owner.
func (d *DB) FleetAircraft(ctx context.Context, owner string) ([]*Aircraft, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	return d.queryAircraft(ctx, `WHERE `+ownerWhere+` ORDER BY m.n_number`, ownerArgs(owner)...)
}

// FleetReport aggregates an owner's fleet.
type FleetReport struct {
	Owner         string         `json:"owner"`
	Count         int            `json:"count"`
	Models        []ModelCount   `json:"models,omitempty"`
	EngineClasses map[string]int `json:"engine_classes,omitempty"`
	States        map[string]int `json:"states,omitempty"`
	AvgSeats      float64        `json:"avg_seats,omitempty"`
	AvgYear       float64        `json:"avg_year_built,omitempty"`
	Oldest        string         `json:"oldest_year,omitempty"`
	Newest        string         `json:"newest_year,omitempty"`
	Aircraft      []*Aircraft    `json:"aircraft,omitempty"`
}

// ModelCount is one model's share of a fleet.
type ModelCount struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Count        int    `json:"count"`
}

// Fleet builds the aggregate fleet report for an owner. includeAircraft
// attaches the per-tail rows.
func (d *DB) Fleet(ctx context.Context, owner string, includeAircraft bool) (*FleetReport, error) {
	acs, err := d.FleetAircraft(ctx, owner)
	if err != nil {
		return nil, err
	}
	rep := &FleetReport{
		Owner:         strings.ToUpper(strings.TrimSpace(owner)),
		Count:         len(acs),
		EngineClasses: map[string]int{},
		States:        map[string]int{},
	}
	modelCounts := map[string]*ModelCount{}
	var seatsSum, seatsN, yearSum, yearN float64
	minYear, maxYear := 9999, 0
	for _, ac := range acs {
		key := ac.Manufacturer + "\x00" + ac.Model
		if mc, ok := modelCounts[key]; ok {
			mc.Count++
		} else {
			modelCounts[key] = &ModelCount{Manufacturer: ac.Manufacturer, Model: ac.Model, Count: 1}
		}
		// EngineType has been decoded; re-derive class from the decoded label.
		switch {
		case strings.Contains(ac.EngineType, "Turbo-fan"), strings.Contains(ac.EngineType, "Turbo-jet"), strings.Contains(ac.EngineType, "Ramjet"):
			rep.EngineClasses["jet"]++
		case strings.Contains(ac.EngineType, "Turbo-prop"):
			rep.EngineClasses["turboprop"]++
		case strings.Contains(ac.EngineType, "Turbo-shaft"):
			rep.EngineClasses["turbine (shaft)"]++
		case strings.Contains(ac.EngineType, "Reciprocating"), strings.Contains(ac.EngineType, "Cycle"), strings.Contains(ac.EngineType, "Rotary"):
			rep.EngineClasses["piston"]++
		case strings.Contains(ac.EngineType, "Electric"):
			rep.EngineClasses["electric"]++
		case ac.EngineType == "None":
			rep.EngineClasses["none"]++
		default:
			rep.EngineClasses["other/unknown"]++
		}
		if ac.State != "" {
			rep.States[ac.State]++
		}
		if s, err := strconv.Atoi(strings.TrimSpace(ac.Seats)); err == nil && s > 0 {
			seatsSum += float64(s)
			seatsN++
		}
		if y, err := strconv.Atoi(strings.TrimSpace(ac.YearMfr)); err == nil && y > 1900 {
			yearSum += float64(y)
			yearN++
			if y < minYear {
				minYear = y
			}
			if y > maxYear {
				maxYear = y
			}
		}
	}
	for _, mc := range modelCounts {
		rep.Models = append(rep.Models, *mc)
	}
	sort.Slice(rep.Models, func(i, j int) bool {
		if rep.Models[i].Count != rep.Models[j].Count {
			return rep.Models[i].Count > rep.Models[j].Count
		}
		return rep.Models[i].Model < rep.Models[j].Model
	})
	if seatsN > 0 {
		rep.AvgSeats = round1(seatsSum / seatsN)
	}
	if yearN > 0 {
		rep.AvgYear = round1(yearSum / yearN)
		rep.Oldest = strconv.Itoa(minYear)
		rep.Newest = strconv.Itoa(maxYear)
	}
	if includeAircraft {
		rep.Aircraft = acs
	}
	return rep, nil
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// HistoryEvent is one entry in an aircraft's ownership timeline.
type HistoryEvent struct {
	Kind         string `json:"kind"` // "current" or "deregistered"
	NNumber      string `json:"n_number"`
	SerialNumber string `json:"serial_number,omitempty"`
	Owner        string `json:"owner,omitempty"`
	City         string `json:"city,omitempty"`
	State        string `json:"state,omitempty"`
	Country      string `json:"country,omitempty"`
	Status       string `json:"status,omitempty"`
	CertIssued   string `json:"certificate_issue_date,omitempty"`
	CancelDate   string `json:"cancel_date,omitempty"`
	ExportedTo   string `json:"exported_to,omitempty"`
	LastAction   string `json:"last_action_date,omitempty"`
	YearMfr      string `json:"year_mfr,omitempty"`
}

// History returns the chronological ownership timeline for a tail number:
// every deregistration record plus the current registration when one exists.
func (d *DB) History(ctx context.Context, tail string) ([]HistoryEvent, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	n := NormalizeTail(tail)
	rows, err := d.db.QueryContext(ctx, `
		SELECT n_number, serial_number, name, city_mail, state_abbrev_mail, country_mail,
			status_code, cert_issue_date, cancel_date, exp_country, last_act_date, year_mfr
		FROM faa_dereg WHERE n_number = ? ORDER BY cancel_date`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []HistoryEvent
	for rows.Next() {
		var e HistoryEvent
		var status string
		if err := rows.Scan(&e.NNumber, &e.SerialNumber, &e.Owner, &e.City, &e.State, &e.Country,
			&status, &e.CertIssued, &e.CancelDate, &e.ExportedTo, &e.LastAction, &e.YearMfr); err != nil {
			return nil, err
		}
		e.Kind = "deregistered"
		e.NNumber = "N" + e.NNumber
		e.Status = DecodeStatus(status)
		e.CertIssued = fmtFAADate(e.CertIssued)
		e.CancelDate = fmtFAADate(e.CancelDate)
		e.LastAction = fmtFAADate(e.LastAction)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	cur, err := d.LookupTail(ctx, tail)
	if err != nil {
		return nil, err
	}
	if cur != nil {
		events = append(events, HistoryEvent{
			Kind:         "current",
			NNumber:      cur.NNumber,
			SerialNumber: cur.SerialNumber,
			Owner:        cur.OwnerName,
			City:         cur.City,
			State:        cur.State,
			Country:      cur.Country,
			Status:       cur.Status,
			CertIssued:   cur.CertIssueDate,
			LastAction:   cur.LastActionDate,
			YearMfr:      cur.YearMfr,
		})
	}
	return events, nil
}

// ExpiringAircraft is a registration approaching its expiration date.
type ExpiringAircraft struct {
	NNumber        string `json:"n_number"`
	Owner          string `json:"owner,omitempty"`
	Manufacturer   string `json:"manufacturer,omitempty"`
	Model          string `json:"model,omitempty"`
	State          string `json:"state,omitempty"`
	ExpirationDate string `json:"expiration_date"`
	DaysLeft       int    `json:"days_left"`
}

// Expiring lists registrations expiring within `days` days (from today),
// optionally filtered by owner prefix and/or state.
func (d *DB) Expiring(ctx context.Context, days int, owner, state string, limit int) ([]ExpiringAircraft, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	now := time.Now()
	from := now.Format("20060102")
	to := now.AddDate(0, 0, days).Format("20060102")
	where := `WHERE m.expiration_date >= ? AND m.expiration_date <= ?`
	args := []any{from, to}
	if owner != "" {
		where += ` AND ` + ownerWhere
		args = append(args, ownerArgs(owner)...)
	}
	if state != "" {
		where += ` AND m.state = ?`
		args = append(args, strings.ToUpper(strings.TrimSpace(state)))
	}
	where += ` ORDER BY m.expiration_date`
	if limit > 0 {
		where += ` LIMIT ` + strconv.Itoa(limit)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT m.n_number, m.name, COALESCE(a.mfr,''), COALESCE(a.model,''), m.state, m.expiration_date
		FROM faa_master m LEFT JOIN faa_acftref a ON a.code = m.mfr_mdl_code `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpiringAircraft
	for rows.Next() {
		var e ExpiringAircraft
		if err := rows.Scan(&e.NNumber, &e.Owner, &e.Manufacturer, &e.Model, &e.State, &e.ExpirationDate); err != nil {
			return nil, err
		}
		e.NNumber = "N" + e.NNumber
		if t, err := time.Parse("20060102", e.ExpirationDate); err == nil {
			e.DaysLeft = int(t.Sub(now).Hours() / 24)
		}
		e.ExpirationDate = fmtFAADate(e.ExpirationDate)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SoonestExpiration returns the earliest future expiration date matching the
// optional owner/state filters ("" when none exists).
func (d *DB) SoonestExpiration(ctx context.Context, owner, state string) (string, error) {
	where := `WHERE m.expiration_date >= ?`
	args := []any{time.Now().Format("20060102")}
	if owner != "" {
		where += ` AND ` + ownerWhere
		args = append(args, ownerArgs(owner)...)
	}
	if state != "" {
		where += ` AND m.state = ?`
		args = append(args, strings.ToUpper(strings.TrimSpace(state)))
	}
	var date string
	err := d.db.QueryRowContext(ctx, `SELECT MIN(m.expiration_date) FROM faa_master m `+where, args...).Scan(&date)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmtFAADate(date), nil
}

// ModelFleetReport aggregates every registered example of a make/model.
type ModelFleetReport struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Count        int    `json:"count"`
	// MatchedModels makes the prefix match explicit: --model SR22 also
	// matches SR22T, and the per-variant counts show exactly what was
	// aggregated.
	MatchedModels   []ModelCount   `json:"matched_models,omitempty"`
	RegistrantTypes map[string]int `json:"registrant_types,omitempty"`
	States          map[string]int `json:"states,omitempty"`
	YearRange       string         `json:"year_range,omitempty"`
}

// ModelFleet aggregates registrations for models matching the manufacturer
// (and optional model) prefix.
func (d *DB) ModelFleet(ctx context.Context, manufacturer, model string) (*ModelFleetReport, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	where := `WHERE a.mfr LIKE ?`
	args := []any{strings.ToUpper(strings.TrimSpace(manufacturer)) + "%"}
	if model != "" {
		where += ` AND a.model LIKE ?`
		args = append(args, strings.ToUpper(strings.TrimSpace(model))+"%")
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT m.type_registrant, m.state, m.year_mfr, a.mfr, a.model
		FROM faa_master m JOIN faa_acftref a ON a.code = m.mfr_mdl_code `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rep := &ModelFleetReport{
		Manufacturer:    strings.ToUpper(strings.TrimSpace(manufacturer)),
		Model:           strings.ToUpper(strings.TrimSpace(model)),
		RegistrantTypes: map[string]int{},
		States:          map[string]int{},
	}
	minYear, maxYear := 9999, 0
	variantCounts := map[string]*ModelCount{}
	for rows.Next() {
		var tr, st, yr, vmfr, vmodel string
		if err := rows.Scan(&tr, &st, &yr, &vmfr, &vmodel); err != nil {
			return nil, err
		}
		rep.Count++
		vkey := vmfr + "\x00" + vmodel
		if vc, ok := variantCounts[vkey]; ok {
			vc.Count++
		} else {
			variantCounts[vkey] = &ModelCount{Manufacturer: vmfr, Model: vmodel, Count: 1}
		}
		label := DecodeRegistrantType(tr)
		if label == "" {
			label = "Unknown"
		}
		rep.RegistrantTypes[label]++
		if st != "" {
			rep.States[st]++
		}
		if y, err := strconv.Atoi(strings.TrimSpace(yr)); err == nil && y > 1900 {
			if y < minYear {
				minYear = y
			}
			if y > maxYear {
				maxYear = y
			}
		}
	}
	if maxYear > 0 {
		rep.YearRange = fmt.Sprintf("%d-%d", minYear, maxYear)
	}
	for _, vc := range variantCounts {
		rep.MatchedModels = append(rep.MatchedModels, *vc)
	}
	sort.Slice(rep.MatchedModels, func(i, j int) bool {
		if rep.MatchedModels[i].Count != rep.MatchedModels[j].Count {
			return rep.MatchedModels[i].Count > rep.MatchedModels[j].Count
		}
		return rep.MatchedModels[i].Model < rep.MatchedModels[j].Model
	})
	return rep, rows.Err()
}

// Availability describes an N-number's assignment state.
// No omitempty on the payload fields: batch checks mix assigned/reserved/free
// rows and --compact's 80% rule would prune the sparse keys.
type Availability struct {
	NNumber   string `json:"n_number"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
	Owner     string `json:"owner"`
	Status    string `json:"status"`
	PurgeDate string `json:"purge_date"`
}

// Available reports whether an N-number is currently assignable: not in the
// active registry, not reserved, and not pending cancellation.
func (d *DB) Available(ctx context.Context, tail string) (*Availability, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	n := NormalizeTail(tail)
	av := &Availability{NNumber: "N" + n}

	if err := ValidTail(tail); err != nil {
		av.Available = false
		av.Reason = "invalid format: " + err.Error()
		return av, nil
	}

	var owner, status string
	err := d.db.QueryRowContext(ctx, `SELECT name, status_code FROM faa_master WHERE n_number = ?`, n).Scan(&owner, &status)
	switch {
	case err == nil:
		av.Available = false
		av.Owner = owner
		av.Status = DecodeStatus(status)
		av.Reason = "assigned in the active registry"
		return av, nil
	case err != sql.ErrNoRows:
		return nil, err
	}

	var registrant, purge string
	err = d.db.QueryRowContext(ctx, `SELECT registrant, purge_date FROM faa_reserved WHERE n_number = ?`, n).Scan(&registrant, &purge)
	switch {
	case err == nil:
		av.Available = false
		av.Owner = registrant
		av.PurgeDate = fmtFAADate(purge)
		av.Reason = "reserved"
		return av, nil
	case err != sql.ErrNoRows:
		return nil, err
	}

	av.Available = true
	av.Reason = "not assigned or reserved as of the last sync"
	return av, nil
}

// SearchResult is one FTS hit.
type SearchResult struct {
	NNumber string `json:"n_number"`
	Name    string `json:"name,omitempty"`
	Mfr     string `json:"manufacturer,omitempty"`
	Model   string `json:"model,omitempty"`
}

// Search runs a full-text query over owner names, co-owner names,
// manufacturers, and models.
func (d *DB) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT n_number, name, mfr, model FROM faa_master_fts
		WHERE faa_master_fts MATCH ? ORDER BY rank LIMIT ?`, ftsQuote(query), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.NNumber, &r.Name, &r.Mfr, &r.Model); err != nil {
			return nil, err
		}
		r.NNumber = "N" + r.NNumber
		out = append(out, r)
	}
	return out, rows.Err()
}

// ftsQuote turns free text into a safe FTS5 prefix query.
func ftsQuote(q string) string {
	words := strings.Fields(q)
	for i, w := range words {
		words[i] = `"` + strings.ReplaceAll(w, `"`, ``) + `"*`
	}
	return strings.Join(words, " ")
}
