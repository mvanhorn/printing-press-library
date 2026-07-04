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

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/internal/store"
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

// wzComments absorbs Workiz's Comments field. Confirmed against live data to
// take three shapes: an empty string `""` (no comments), a single non-empty
// string containing free text (the common live shape — e.g. "Left VM for
// second visit,leonid-p-and-office..."), or an array of `{Comment: "..."}`
// objects (the shape documented by community SDKs, possibly an older/other
// API version). All three are absorbed so `job search` can find text in any
// of them.
type wzComments []string

func (c *wzComments) UnmarshalJSON(b []byte) error {
	if string(b) == `""` || string(b) == "null" {
		return nil
	}
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		if single != "" {
			*c = append(*c, single)
		}
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

// flexibleMoney absorbs Workiz's stringly-inconsistent price fields.
// Confirmed against live data that JobTotalPrice/JobAmountDue arrive as JSON
// numbers (int or float), not strings as some community SDK docs suggested —
// unmarshaling straight into a Go string field fails and (before this type
// existed) silently dropped the entire job/lead row from loadJobs/loadLeads,
// since json.Unmarshal on the whole struct returns an error on any field
// type mismatch. Also accepts a JSON string in case some responses do send
// one, so either wire shape parses into the same value.
type flexibleMoney string

func (f *flexibleMoney) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" {
		*f = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err == nil {
			*f = flexibleMoney(str)
			return nil
		}
	}
	*f = flexibleMoney(s)
	return nil
}

type wzJob struct {
	UUID             string        `json:"UUID"`
	JobDateTime      string        `json:"JobDateTime"`
	JobEndDateTime   string        `json:"JobEndDateTime"`
	CreatedDate      string        `json:"CreatedDate"`
	LastStatusUpdate string        `json:"LastStatusUpdate"`
	JobTotalPrice    flexibleMoney `json:"JobTotalPrice"`
	JobAmountDue     flexibleMoney `json:"JobAmountDue"`
	Status           string        `json:"Status"`
	JobType          string        `json:"JobType"`
	JobSource        string        `json:"JobSource"`
	Phone            string        `json:"Phone"`
	Email            string        `json:"Email"`
	FirstName        string        `json:"FirstName"`
	LastName         string        `json:"LastName"`
	JobNotes         string        `json:"JobNotes"`
	Address          string        `json:"Address"`
	City             string        `json:"City"`
	State            string        `json:"State"`
	PostalCode       string        `json:"PostalCode"`
	Team             []wzTeamRef   `json:"Team"`
	Comments         wzComments    `json:"Comments"`
}

type wzLead struct {
	UUID             string        `json:"UUID"`
	LeadDateTime     string        `json:"LeadDateTime"`
	LeadEndDateTime  string        `json:"LeadEndDateTime"`
	CreatedDate      string        `json:"CreatedDate"`
	LastStatusUpdate string        `json:"LastStatusUpdate"`
	LeadTotalPrice   flexibleMoney `json:"LeadTotalPrice"`
	LeadAmountDue    flexibleMoney `json:"LeadAmountDue"`
	Status           string        `json:"Status"`
	LeadType         string        `json:"LeadType"`
	// Confirmed against live data: leads report their source under the
	// "JobSource" wire key, not "LeadSource" (which never appears in any
	// synced lead). The Go field stays LeadSource for readability in this
	// package; only the JSON tag needs to match the wire key.
	LeadSource       string     `json:"JobSource"`
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

// parseMoney parses Workiz's price fields (flexibleMoney absorbs both the
// JSON-number and JSON-string wire shapes), tolerating blank or non-numeric
// values by returning 0 rather than failing the row.
func parseMoney(f flexibleMoney) float64 {
	s := strings.TrimSpace(string(f))
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
