// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Jane patient booking is a multi-step transaction, not a single
// POST. Reverse-engineered from the booking SPA (public-*.js):
//   1. POST /api/v2/reservations {reservation:{...,browser_session_id}}  -> holds the slot
//   2. (if the reservation has no patient) PUT /api/v2/session_appointments/{id}
//      {appointment:{patient_id}}                                         -> assigns you
//   3. POST /api/v2/appointments/{reservation_id}/book                    -> confirms
// Reservations carry a short expiry, so a failed run self-heals (the hold lapses).
// All requests ride the clinic's session cookie plus the page CSRF token.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var reMetaCSRF = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)
var reBrowserSession = regexp.MustCompile(`browser_session_id\s*=\s*"([^"]+)"`)

type janeBooker struct {
	hc        *http.Client
	base      string
	csrf      string
	browserID string
	dbg       io.Writer
}

type reservationResp struct {
	Reservation struct {
		ID           int             `json:"id"`
		Patient      json.RawMessage `json:"patient"`
		TreatmentID  int             `json:"treatment_id"`
		StartAt      string          `json:"start_at"`
		SessionAppts []struct {
			ID int `json:"id"`
		} `json:"session_appointments"`
	} `json:"reservation"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// newJaneBooker builds an authenticated HTTP session for the clinic and pulls
// the CSRF token + browser_session_id from the booking page.
func newJaneBooker(ctx context.Context, clinic *Clinic, timeout time.Duration, dbg io.Writer) (*janeBooker, error) {
	if strings.TrimSpace(clinic.Session) == "" {
		return nil, fmt.Errorf("not logged in to clinic %q", clinic.Name)
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(clinic.BaseURL, "/")
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	// Seed the clinic session cookie into the jar.
	name, value, _ := strings.Cut(clinic.Session, "=")
	jar.SetCookies(u, []*http.Cookie{{Name: strings.TrimSpace(name), Value: value, Path: "/"}})
	b := &janeBooker{
		hc:   &http.Client{Timeout: timeout, Jar: jar},
		base: base,
		dbg:  dbg,
	}
	// Fetch the booking page to capture CSRF + browser_session_id.
	_, body, _, err := janeReq(ctx, b.hc, http.MethodGet, base+"/", "", nil)
	if err != nil {
		return nil, fmt.Errorf("loading booking page: %w", err)
	}
	b.csrf = firstMatch(reMetaCSRF, body)
	b.browserID = firstMatch(reBrowserSession, body)
	if b.csrf == "" {
		return nil, fmt.Errorf("could not read CSRF token from %s (session may be expired — re-run auth login)", base)
	}
	if b.browserID == "" {
		return nil, fmt.Errorf("could not read browser_session_id from %s", base)
	}
	janeTrace(dbg, "booker init", 0, "csrf="+yn(b.csrf != "")+" browser_session_id="+yn(b.browserID != ""))
	return b, nil
}

// jsonReq issues a JSON request with the session cookie + CSRF header and
// returns status + body.
func (b *janeBooker) jsonReq(ctx context.Context, method, path string, payload any) (int, string, error) {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return 0, "", err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.base+path, body)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", janeUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", b.csrf)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Referer", b.base+"/")
	resp, err := b.hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, string(buf), nil
}

// Book runs the full reserve -> (assign) -> confirm sequence and returns the
// confirmed appointment JSON.
func (b *janeBooker) Book(ctx context.Context, treatmentID, staffID, locationID int, startAt string) (string, error) {
	// Step 1: reserve the slot.
	reservePayload := map[string]any{
		"reservation": map[string]any{
			"staff_member_id":    staffID,
			"location_id":        locationID,
			"treatment_id":       treatmentID,
			"start_at":           startAt,
			"browser_session_id": b.browserID,
		},
	}
	status, body, err := b.jsonReq(ctx, http.MethodPost, "/api/v2/reservations", reservePayload)
	if err != nil {
		return "", fmt.Errorf("reserving slot: %w", err)
	}
	janeTrace(b.dbg, "POST /reservations", status, "body="+snippet(body))
	if status >= 400 {
		return "", fmt.Errorf("reservation failed (HTTP %d): %s", status, snippet(body))
	}
	var rr reservationResp
	if err := json.Unmarshal([]byte(body), &rr); err != nil {
		return "", fmt.Errorf("parsing reservation response: %w", err)
	}
	if rr.Error != "" {
		return "", fmt.Errorf("reservation rejected: %s", rr.Error)
	}
	resID := rr.Reservation.ID
	if resID == 0 {
		return "", fmt.Errorf("no reservation id returned: %s", snippet(body))
	}
	janeTrace(b.dbg, "reservation held", 0, fmt.Sprintf("id=%d patient_set=%s", resID, yn(len(rr.Reservation.Patient) > 0 && string(rr.Reservation.Patient) != "null")))

	// Step 2: assign patient if the reservation didn't auto-associate one.
	patientMissing := len(rr.Reservation.Patient) == 0 || string(rr.Reservation.Patient) == "null"
	if patientMissing {
		pid := b.resolvePatientID(ctx)
		if pid == "" {
			return "", fmt.Errorf("reservation has no patient and could not resolve your patient id; book once in the browser to initialize your profile")
		}
		for _, sa := range rr.Reservation.SessionAppts {
			ps, pb, perr := b.jsonReq(ctx, http.MethodPut, fmt.Sprintf("/api/v2/session_appointments/%d", sa.ID),
				map[string]any{"appointment": map[string]any{"patient_id": pid}})
			janeTrace(b.dbg, "PUT session_appointment", ps, "body="+snippet(pb))
			if perr != nil || ps >= 400 {
				return "", fmt.Errorf("assigning patient failed (HTTP %d): %s", ps, snippet(pb))
			}
		}
	}

	// Step 3: confirm the booking.
	cstatus, cbody, err := b.jsonReq(ctx, http.MethodPost, fmt.Sprintf("/api/v2/appointments/%d/book", resID), map[string]any{})
	if err != nil {
		return "", fmt.Errorf("confirming booking: %w", err)
	}
	janeTrace(b.dbg, "POST /appointments/{id}/book", cstatus, "body="+snippet(cbody))
	if cstatus >= 400 {
		if strings.Contains(cbody, "MISSING_PATIENT_INFO") {
			return "", fmt.Errorf("Jane needs your patient profile completed first — book once in the browser, then CLI booking will work")
		}
		return "", fmt.Errorf("booking confirmation failed (HTTP %d): %s", cstatus, snippet(cbody))
	}
	return cbody, nil
}

// parseAppointmentList decodes /api/v2/appointments, which may return a bare
// array or an object wrapping one ({"appointments":[...]}).
func parseAppointmentList(body string) []map[string]any {
	var arr []map[string]any
	if json.Unmarshal([]byte(body), &arr) == nil {
		return arr
	}
	var wrapper map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &wrapper) == nil {
		for _, key := range []string{"appointments", "data", "results"} {
			if raw, ok := wrapper[key]; ok {
				_ = json.Unmarshal(raw, &arr)
				return arr
			}
		}
	}
	return arr
}

// resolvePatientID pulls the authenticated patient's id from an existing
// appointment (the payload carries patient_id).
func (b *janeBooker) resolvePatientID(ctx context.Context) string {
	status, body, _, err := janeReqAccept(ctx, b.hc, b.base+"/api/v2/appointments")
	if err != nil || status >= 400 {
		return ""
	}
	for _, a := range parseAppointmentList(body) {
		if pid, ok := a["patient_id"]; ok {
			return fmt.Sprintf("%v", jsonNumberToInt(pid))
		}
	}
	return ""
}

func jsonNumberToInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

// Cancel cancels a booked appointment. Jane's patient portal cancels with
// DELETE /api/v2/appointments/{id} (the booking/cancel thunk); the /cancel and
// /late-cancel suffixes are the staff/admin API, not the patient one. `reason`
// is currently unused by the patient endpoint but kept for a stable signature.
func (b *janeBooker) Cancel(ctx context.Context, appointmentID int, reason string) (string, error) {
	_ = reason
	status, body, err := b.jsonReq(ctx, http.MethodDelete, fmt.Sprintf("/api/v2/appointments/%d", appointmentID), nil)
	if err != nil {
		return "", fmt.Errorf("cancelling appointment: %w", err)
	}
	janeTrace(b.dbg, "DELETE /appointments/{id}", status, "body="+snippet(body))
	if status == 404 {
		return "", fmt.Errorf("appointment %d not found (already cancelled, or wrong id)", appointmentID)
	}
	if status >= 400 {
		return "", fmt.Errorf("cancel failed (HTTP %d): %s", status, snippet(body))
	}
	return body, nil
}

// apptDetail holds the fields needed to rebook an appointment during reschedule.
type apptDetail struct {
	TreatmentID   int
	StaffMemberID int
	LocationID    int
	StartAt       string
	Found         bool
}

// appointmentByID fetches the authenticated appointment list and returns the
// treatment/staff/location for the given appointment id (needed to rebook it
// at a new time during reschedule).
func (b *janeBooker) appointmentByID(ctx context.Context, id int) (apptDetail, error) {
	status, body, _, err := janeReqAccept(ctx, b.hc, b.base+"/api/v2/appointments")
	if err != nil {
		return apptDetail{}, err
	}
	if status == 401 || status == 403 {
		return apptDetail{}, fmt.Errorf("session expired — re-run 'auth login --clinic <name> --chrome'")
	}
	if status >= 400 {
		return apptDetail{}, fmt.Errorf("fetching appointments failed (HTTP %d)", status)
	}
	for _, a := range parseAppointmentList(body) {
		if int(jsonNumberToInt(a["id"])) != id {
			continue
		}
		d := apptDetail{
			TreatmentID:   int(jsonNumberToInt(a["treatment_id"])),
			StaffMemberID: int(jsonNumberToInt(a["staff_member_id"])),
			LocationID:    int(jsonNumberToInt(a["location_id"])),
			Found:         true,
		}
		if s, ok := a["start_at"].(string); ok {
			d.StartAt = s
		}
		return d, nil
	}
	return apptDetail{}, nil
}
