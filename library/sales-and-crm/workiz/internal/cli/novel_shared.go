// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/internal/store"
	"github.com/spf13/cobra"
)

// flexibleID absorbs Workiz's inconsistent Team[].id wire shape: numeric on
// Job records, string on Lead records. Both normalize to a plain string.
type flexibleID string

func (f *flexibleID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexibleID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexibleID(n.String())
		return nil
	}
	return nil
}

type wzTeamRef struct {
	ID   flexibleID `json:"id"`
	Name string     `json:"name"`
}

// wzComments absorbs Workiz's Comments field, which is an empty string
// `""` when there are none, or an array of `{Comment: "..."}` objects.
type wzComments []string

func (c *wzComments) UnmarshalJSON(b []byte) error {
	if string(b) == `""` || string(b) == "null" {
		return nil
	}
	var raw []struct {
		Comment string `json:"Comment"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil // unknown shape; treat as no comments rather than failing the row
	}
	for _, r := range raw {
		*c = append(*c, r.Comment)
	}
	return nil
}

type wzJob struct {
	UUID             string      `json:"UUID"`
	JobDateTime      string      `json:"JobDateTime"`
	JobEndDateTime   string      `json:"JobEndDateTime"`
	CreatedDate      string      `json:"CreatedDate"`
	LastStatusUpdate string      `json:"LastStatusUpdate"`
	JobTotalPrice    string      `json:"JobTotalPrice"`
	JobAmountDue     string      `json:"JobAmountDue"`
	Status           string      `json:"Status"`
	JobType          string      `json:"JobType"`
	JobSource        string      `json:"JobSource"`
	Phone            string      `json:"Phone"`
	Email            string      `json:"Email"`
	FirstName        string      `json:"FirstName"`
	LastName         string      `json:"LastName"`
	JobNotes         string      `json:"JobNotes"`
	Address          string      `json:"Address"`
	City             string      `json:"City"`
	State            string      `json:"State"`
	PostalCode       string      `json:"PostalCode"`
	Team             []wzTeamRef `json:"Team"`
	Comments         wzComments  `json:"Comments"`
}

type wzLead struct {
	UUID             string     `json:"UUID"`
	LeadDateTime     string     `json:"LeadDateTime"`
	LeadEndDateTime  string     `json:"LeadEndDateTime"`
	CreatedDate      string     `json:"CreatedDate"`
	LastStatusUpdate string     `json:"LastStatusUpdate"`
	LeadTotalPrice   string     `json:"LeadTotalPrice"`
	LeadAmountDue    string     `json:"LeadAmountDue"`
	Status           string     `json:"Status"`
	LeadType         string     `json:"LeadType"`
	LeadSource       string     `json:"LeadSource"`
	Phone            string     `json:"Phone"`
	Email            string     `json:"Email"`
	FirstName        string     `json:"FirstName"`
	LastName         string     `json:"LastName"`
	Company          string     `json:"Company"`
	LeadNotes        string     `json:"LeadNotes"`
	Address          string     `json:"Address"`
	City             string     `json:"City"`
	State            string     `json:"State"`
	PostalCode       string     `json:"PostalCode"`
	Comments         wzComments `json:"Comments"`
}

type wzTeamMember struct {
	Id        string `json:"Id"`
	Name      string `json:"Name"`
	Role      string `json:"Role"`
	Email     string `json:"Email"`
	Active    bool   `json:"Active"`
	FieldTech bool   `json:"FieldTech"`
}

type wzTimeOff struct {
	Id        string `json:"Id"`
	UserName  string `json:"UserName"`
	StartDate string `json:"StartDate"`
	EndDate   string `json:"EndDate"`
	Reason    string `json:"Reason"`
}

type wzCustomer struct {
	Id        string `json:"Id"`
	FirstName string `json:"FirstName"`
	LastName  string `json:"LastName"`
	Email     string `json:"Email"`
}

// loadRawRows drains a typed table's `data` JSON column into raw messages.
// Follows the drain-first pattern: the returned slice is fully materialized
// before the caller runs any follow-up query against the same *sql.DB.
func loadRawRows(ctx context.Context, db *sql.DB, table string) ([]json.RawMessage, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM "`+table+`"`) // #nosec G202 -- table is one of a small fixed internal set, never user input
	if err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if raw.Valid {
			out = append(out, json.RawMessage(raw.String))
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

func loadJobs(ctx context.Context, db *sql.DB) ([]wzJob, error) {
	raws, err := loadRawRows(ctx, db, "job")
	if err != nil {
		return nil, err
	}
	out := make([]wzJob, 0, len(raws))
	for _, r := range raws {
		var j wzJob
		if err := json.Unmarshal(r, &j); err == nil {
			out = append(out, j)
		}
	}
	return out, nil
}

func loadLeads(ctx context.Context, db *sql.DB) ([]wzLead, error) {
	raws, err := loadRawRows(ctx, db, "lead")
	if err != nil {
		return nil, err
	}
	out := make([]wzLead, 0, len(raws))
	for _, r := range raws {
		var l wzLead
		if err := json.Unmarshal(r, &l); err == nil {
			out = append(out, l)
		}
	}
	return out, nil
}

func loadTeamMembers(ctx context.Context, db *sql.DB) ([]wzTeamMember, error) {
	raws, err := loadRawRows(ctx, db, "team")
	if err != nil {
		return nil, err
	}
	out := make([]wzTeamMember, 0, len(raws))
	for _, r := range raws {
		var t wzTeamMember
		if err := json.Unmarshal(r, &t); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func loadTimeOff(ctx context.Context, db *sql.DB) ([]wzTimeOff, error) {
	raws, err := loadRawRows(ctx, db, "timeoff")
	if err != nil {
		return nil, err
	}
	out := make([]wzTimeOff, 0, len(raws))
	for _, r := range raws {
		var t wzTimeOff
		if err := json.Unmarshal(r, &t); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func loadCustomers(ctx context.Context, db *sql.DB) ([]wzCustomer, error) {
	raws, err := loadRawRows(ctx, db, "customer")
	if err != nil {
		return nil, err
	}
	out := make([]wzCustomer, 0, len(raws))
	for _, r := range raws {
		var c wzCustomer
		if err := json.Unmarshal(r, &c); err == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// parseWorkizTime parses Workiz's "2006-01-02 15:04:05" wire format. The
// literal string "null" (and empty string) mean no value was set.
func parseWorkizTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseMoney parses Workiz's stringly-typed price fields, tolerating blank
// or non-numeric values by returning 0 rather than failing the row.
func parseMoney(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// checkNovelMirror resolves the --db override, stats it, and on a missing
// mirror prints the standard sync hint plus (for --json/--agent callers) the
// correctly-shaped empty JSON envelope for this command — `empty` must be
// the command's real zero-value result type, not a bare `[]`, so JSON
// consumers see the same schema whether or not data exists yet. Returns the
// resolved db path and whether the caller should return nil immediately.
func checkNovelMirror(cmd *cobra.Command, flags *rootFlags, dbPath, resources string, empty any) (string, bool) {
	if dbPath == "" {
		dbPath = defaultDBPath("workiz-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: workiz-pp-cli sync --resources %s --db %s\n", dbPath, resources, dbPath)
		if flags.asJSON || flags.agent {
			_ = printJSONFiltered(cmd.OutOrStdout(), empty, flags)
		}
		return dbPath, true
	}
	return dbPath, false
}

// openNovelStore opens the local mirror read-only-in-spirit for a
// hand-written novel command, honoring the same --db override and
// missing-mirror hint every generated command uses.
func openNovelStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("workiz-pp-cli")
	}
	return store.OpenWithContext(ctx, dbPath)
}
