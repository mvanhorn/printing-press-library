// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package taskrabbit

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/client"
)

const maxTRPCErrorMessageRunes = 300

// The checkout commit uses the captured REST endpoint behind the funnel
// "Confirm and chat" button.

// Client wraps the generated TaskRabbit client with tRPC/BFF helpers.
type Client struct {
	c *client.Client
}

// New returns a TaskRabbit tRPC/BFF source adapter using the generated client.
func New(c *client.Client) *Client {
	return &Client{c: c}
}

// TRPCError is a typed tRPC envelope error.
type TRPCError struct {
	Proc    string
	Code    string
	Message string
}

func (e *TRPCError) Error() string {
	if e == nil {
		return "taskrabbit: <nil>"
	}
	return fmt.Sprintf("taskrabbit %s: %s: %s", e.Proc, e.Code, truncateRunes(e.Message, maxTRPCErrorMessageRunes))
}

// Booking is a permissive TaskRabbit task-list item.
type Booking struct {
	ID                 string          `json:"id"`
	JobID              int             `json:"job_id"`    // details.id — the cancelTask jobId
	RabbitID           int             `json:"rabbit_id"` // taskers[0].id — the cancelTask rabbitId
	Appointment        string          `json:"appointment,omitempty"`
	TaskerName         string          `json:"tasker_name,omitempty"`
	Status             string          `json:"status"`
	Taskers            json.RawMessage `json:"taskers"`
	FutureAppointments json.RawMessage `json:"future_appointments"`
	Raw                json.RawMessage `json:"-"`
}

// RecommendationsInput is the page.book.recommendations BFF input.
// RecommendationsInput is the full input page.book.recommendations requires.
// Verified live 2026-07-03: the endpoint is stateful but does NOT need a real
// job-draft — it needs bootstrap+isRecosPage+funnelId+categoryId+top-level
// lat/lng + a location object + a schedule. The funnelId is NOT validated
// server-side (a fabricated "<uuid>_<ms>" returns full results), so the CLI
// synthesizes one instead of driving the funnel UI.
type RecommendationsInput struct {
	Bootstrap      bool           `json:"bootstrap"`
	CategoryID     int            `json:"categoryId"`
	FunnelID       string         `json:"funnelId"`
	IsRecosPage    bool           `json:"isRecosPage"`
	Lat            float64        `json:"lat"`
	Lng            float64        `json:"lng"`
	Locale         string         `json:"locale"`
	Location       map[string]any `json:"location"`
	Schedule       ScheduleInput  `json:"schedule"`
	TaskTemplateID any            `json:"taskTemplateId,omitempty"`
}

// BuildRecommendationsInput assembles the full recommendations input from the
// essentials a CLI command has: category, template, coordinates, and the dates
// to search. It fabricates a funnelId and fills the required funnel flags.
func BuildRecommendationsInput(categoryID, templateID int, lat, lng float64, dates []string) RecommendationsInput {
	specs := make([]DateSpec, 0, len(dates))
	for _, d := range dates {
		specs = append(specs, NewDateSpec(d))
	}
	return RecommendationsInput{
		Bootstrap:      true,
		CategoryID:     categoryID,
		FunnelID:       synthFunnelID(),
		IsRecosPage:    true,
		Lat:            lat,
		Lng:            lng,
		Locale:         "en-US",
		Location:       map[string]any{"lat": lat, "lng": lng},
		Schedule:       ScheduleInput{Dates: specs, DayTimeRanges: []any{}},
		TaskTemplateID: templateID,
	}
}

// SynthFunnelID returns a shape-valid funnel id for TaskRabbit book flows.
func SynthFunnelID() string {
	return synthFunnelID()
}

// synthFunnelID returns a "<uuid-ish>_<unix-ms>" funnel id. The server does not
// validate it; it only needs the shape.
func synthFunnelID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// deterministic fallback; still shape-valid
		copy(b, []byte("humangoatfunnel!"))
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return fmt.Sprintf("%s_%d", uuid, time.Now().UnixMilli())
}

// SynthSessionID returns a shape-valid session id for TaskRabbit checkout.
func SynthSessionID() string {
	return synthSessionID()
}

// synthSessionID returns a 52-character lower-case hex-ish session token.
func synthSessionID() string {
	b := make([]byte, 26)
	if _, err := rand.Read(b); err != nil {
		copy(b, []byte("humangoattaskrabbitcheckout"))
	}
	return hex.EncodeToString(b)
}

// ScheduleInput is the availability window sent to recommendations.
type ScheduleInput struct {
	Dates         []DateSpec `json:"dates"`
	DayTimeRanges []any      `json:"dayTimeRanges"`
}

// DateSpec is one element of schedule.dates. Verified live 2026-07-03: the
// recommendations endpoint requires date + duration_seconds + offset_seconds
// (all three), not a bare date string.
type DateSpec struct {
	Date            string `json:"date"`             // YYYY-MM-DD
	DurationSeconds int    `json:"duration_seconds"` // job length estimate, e.g. 3600 (1h)
	OffsetSeconds   int    `json:"offset_seconds"`   // start offset within the day; 0 = any time
}

// NewDateSpec builds a DateSpec with sensible defaults (1-hour job, any start time).
func NewDateSpec(date string) DateSpec {
	return DateSpec{Date: date, DurationSeconds: 86400, OffsetSeconds: 0}
}

type Address struct {
	Address1         string  `json:"address1"`
	Address2         string  `json:"address2"`
	Country          string  `json:"country"`
	FormattedAddress string  `json:"formatted_address"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	Locality         string  `json:"locality"`
	MetroID          int     `json:"metro_id"`
	MetroName        string  `json:"metro_name"`
	PostalCode       string  `json:"postal_code"`
	Region           string  `json:"region"`
}

type HireInput struct {
	Source                  string   `json:"source"`
	JobType                 string   `json:"job_type"`
	FixedRate               bool     `json:"fixed_rate"`
	SecondsBetween          string   `json:"seconds_between"`
	ShownCancellationPolicy bool     `json:"shown_cancellation_policy"`
	TaskTemplateID          int      `json:"task_template_id"`
	CategoryID              int      `json:"category_id"`
	CategoryName            string   `json:"category_name"`
	Title                   string   `json:"title"`
	MarketingGroupID        int      `json:"marketing_group_id"`
	FunnelID                string   `json:"funnel_id"`
	SessionID               string   `json:"session_id"`
	RecommendationID        string   `json:"recommendation_id"`
	InviteeID               int      `json:"invitee_id"`
	RabbitID                int      `json:"rabbit_id"`
	PosterHourlyRateCents   int      `json:"poster_hourly_rate_cents"`
	JobDraftGUID            string   `json:"job_draft_guid"`
	FormReferrer            string   `json:"form_referrer"`
	JobSize                 string   `json:"job_size"`
	Description             string   `json:"description"`
	Schedule                DateSpec `json:"schedule"`
	Address                 Address  `json:"address"`
	SecondaryLocation       *Address `json:"secondary_location,omitempty"`
}

// Tasker is a permissive recommendation item.
type Tasker struct {
	ID                    string          `json:"id"`
	UserID                any             `json:"user_id"`
	UserIDInt             int             `json:"user_id_int"`
	FirstName             string          `json:"first_name"`
	DisplayName           string          `json:"display_name"`
	PosterHourlyRateCents int             `json:"poster_hourly_rate_cents"`
	PosterRateCurrency    string          `json:"poster_hourly_rate_currency"`
	RabbitRating          float64         `json:"rabbit_rating"`
	RabbitReviews         int             `json:"rabbit_number_of_reviews"`
	CategoryInvoicesCount int             `json:"category_invoices_count"`
	HoursWorked           float64         `json:"hours_worked"`
	Elite                 bool            `json:"elite"`
	ReliabilityRate       any             `json:"reliability_rate"`
	NextAvailableAt       string          `json:"next_available_at"`
	IsFavorite            bool            `json:"is_favorite"`
	PastTasker            bool            `json:"past_tasker"`
	TwoHourMinimum        any             `json:"two_hour_minimum_required_display"`
	Raw                   json.RawMessage `json:"-"`
}

// flexFloat/flexInt/flexBool tolerate TaskRabbit encoding numerics and booleans
// as JSON strings (e.g. "5.0", "932", "true") instead of native types.
type flexFloat float64
type flexInt int
type flexBool bool

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexFloat(v)
	return nil
}

func (i *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*i = 0
		return nil
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		*i = flexInt(v)
		return nil
	}
	// tolerate "3300.0"
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*i = flexInt(int64(v))
	}
	return nil
}

func (bl *flexBool) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	*bl = flexBool(s == "true" || s == "1")
	return nil
}

// UnmarshalJSON tolerates string-encoded numerics/booleans in the Tasker payload.
func (t *Tasker) UnmarshalJSON(b []byte) error {
	type shadow struct {
		ID                    string          `json:"id"`
		UserID                json.RawMessage `json:"user_id"`
		FirstName             string          `json:"first_name"`
		DisplayName           string          `json:"display_name"`
		PosterHourlyRateCents flexInt         `json:"poster_hourly_rate_cents"`
		PosterRateCurrency    string          `json:"poster_hourly_rate_currency"`
		// The 0-5 star rating the app shows is category_family_average_star_rating;
		// rabbit_rating is a "100%" positive-review string, not a star rating.
		RabbitRating          flexFloat `json:"category_family_average_star_rating"`
		RabbitReviews         flexInt   `json:"category_family_review_count"`
		CategoryInvoicesCount flexInt   `json:"category_invoices_count"`
		HoursWorked           flexFloat `json:"hours_worked"`
		Elite                 flexBool  `json:"elite"`
		ReliabilityRate       any       `json:"reliability_rate"`
		NextAvailableAt       string    `json:"next_available_at"`
		IsFavorite            flexBool  `json:"is_favorite"`
		PastTasker            flexBool  `json:"past_tasker"`
		TwoHourMinimum        any       `json:"two_hour_minimum_required_display"`
	}
	var s shadow
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	var userID any
	if len(s.UserID) > 0 {
		_ = json.Unmarshal(s.UserID, &userID)
	}
	t.ID = s.ID
	t.UserID = userID
	t.UserIDInt = intFromRaw(s.UserID)
	t.FirstName = s.FirstName
	t.DisplayName = s.DisplayName
	t.PosterHourlyRateCents = int(s.PosterHourlyRateCents)
	t.PosterRateCurrency = s.PosterRateCurrency
	t.RabbitRating = float64(s.RabbitRating)
	t.RabbitReviews = int(s.RabbitReviews)
	t.CategoryInvoicesCount = int(s.CategoryInvoicesCount)
	t.HoursWorked = float64(s.HoursWorked)
	t.Elite = bool(s.Elite)
	t.ReliabilityRate = s.ReliabilityRate
	t.NextAvailableAt = s.NextAvailableAt
	t.IsFavorite = bool(s.IsFavorite)
	t.PastTasker = bool(s.PastTasker)
	t.TwoHourMinimum = s.TwoHourMinimum
	t.Raw = append(json.RawMessage(nil), b...)
	return nil
}

// Histogram describes recommendation price distribution.
type Histogram struct {
	MinimumPriceCents int    `json:"minimum_price_cents"`
	MedianPriceCents  int    `json:"median_price_cents"`
	MaximumPriceCents int    `json:"maximum_price_cents"`
	CurrencyCode      string `json:"currency_code"`
}

// AvailableDate is one day of TaskRabbit schedule availability.
type AvailableDate struct {
	Date    string `json:"date"`
	Sameday bool   `json:"sameday"`
	Slots   []Slot `json:"slots"`
}

// Slot is one available appointment slot.
type Slot struct {
	DurationSeconds int    `json:"durationSeconds"`
	OffsetSeconds   int    `json:"offsetSeconds"`
	SelectLabel     string `json:"selectLabel"`
}

// Query calls a TaskRabbit tRPC query procedure and returns result.data.json.
func (t *Client) Query(ctx context.Context, proc string, input any) (json.RawMessage, error) {
	encoded, err := json.Marshal(trpcInputEnvelope(input))
	if err != nil {
		return nil, fmt.Errorf("taskrabbit %s: encode tRPC input: %w", proc, err)
	}
	body, err := t.c.Get(ctx, "/next-api/trpc/"+proc, map[string]string{
		"batch": "1",
		"input": string(encoded),
	})
	if err != nil {
		return nil, err
	}
	return unwrapTRPC(proc, body)
}

// Mutation calls a TaskRabbit tRPC mutation procedure and returns result.data.json.
func (t *Client) Mutation(ctx context.Context, proc string, input any, csrfToken string) (json.RawMessage, error) {
	body, _, err := t.c.PostWithHeaders(ctx, "/next-api/trpc/"+proc+"?batch=1", trpcInputEnvelope(input), map[string]string{
		"X-CSRF-Token": csrfToken,
	})
	if err != nil {
		return nil, err
	}
	return unwrapTRPC(proc, body)
}

// Hire posts the captured REST checkout commit. The caller supplies the
// metro_id-bearing address and accepts that SynthFunnelID/SynthSessionID match
// the captured shapes but are not re-verified against a fresh booking here.
func (t *Client) Hire(ctx context.Context, in HireInput, csrfToken string) (json.RawMessage, error) {
	body, status, err := t.c.PostWithHeaders(ctx, "/api/v3/jobs/post/hire.json", in, map[string]string{
		"X-CSRF-Token": csrfToken,
	})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("taskrabbit hire: HTTP %d: %s", apiErr.StatusCode, truncateRunes(apiErr.Body, maxTRPCErrorMessageRunes))
		}
		if status >= 300 {
			return nil, fmt.Errorf("taskrabbit hire: HTTP %d: %s", status, truncateRunes(err.Error(), maxTRPCErrorMessageRunes))
		}
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("taskrabbit hire: HTTP %d: %s", status, truncateRunes(string(body), maxTRPCErrorMessageRunes))
	}
	return body, nil
}

// ListTasks reads the TaskRabbit task-list BFF.
func (t *Client) ListTasks(ctx context.Context, page, perPage int, filters map[string]any, locale string) ([]Booking, error) {
	if filters == nil {
		filters = make(map[string]any)
	}
	// page.tasks.list requires filters.status; it is a Zod enum ("active" | "completed").
	// Default to "active" so a bare `tasks list` returns current bookings instead of 400.
	if _, ok := filters["status"]; !ok {
		filters["status"] = "active"
	}
	payload, err := t.Query(ctx, "page.tasks.list", struct {
		Page    int            `json:"page"`
		PerPage int            `json:"perPage"`
		Filters map[string]any `json:"filters"`
		Locale  string         `json:"locale"`
	}{
		Page:    page,
		PerPage: perPage,
		Filters: filters,
		Locale:  locale,
	})
	if err != nil {
		return nil, err
	}
	return parseListTasksPayload(payload)
}

// Recommendations reads recommended TaskRabbit taskers.
func (t *Client) Recommendations(ctx context.Context, input RecommendationsInput) ([]Tasker, Histogram, string, error) {
	payload, err := t.Query(ctx, "page.book.recommendations", input)
	if err != nil {
		return nil, Histogram{}, "", err
	}
	// The endpoint returns HTTP 200 with a domain-level {"error":{...}} body when
	// called without a valid funnel job-draft (recommendations is stateful: the
	// funnel `details` step must create a job draft first). Surface it loudly
	// instead of returning an empty list that looks like "no Taskers available".
	var envelope struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &envelope) == nil && envelope.Error != nil {
		return nil, Histogram{}, "", fmt.Errorf("taskrabbit recommendations: %s: %s (recommendations requires a funnel job-draft; the details step must run first)", envelope.Error.Code, envelope.Error.Message)
	}
	return parseRecommendationsPayloadWithID(payload)
}

// Schedule reads TaskRabbit availability for a tasker/category.
func (t *Client) Schedule(ctx context.Context, categoryID, inviteeID int, locale string, lat, lng float64, taskTemplateID int) ([]AvailableDate, error) {
	payload, err := t.Query(ctx, "page.book.schedule", struct {
		CategoryID int    `json:"categoryId"`
		InviteeID  int    `json:"inviteeId"`
		Locale     string `json:"locale"`
		Location   struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
		TaskTemplateID int `json:"taskTemplateId"`
	}{
		CategoryID: categoryID,
		InviteeID:  inviteeID,
		Locale:     locale,
		Location: struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		}{
			Lat: lat,
			Lng: lng,
		},
		TaskTemplateID: taskTemplateID,
	})
	if err != nil {
		return nil, err
	}
	return parseSchedulePayload(payload)
}

// Exact single-branch member fields are finalized during the authorized real hire+cancel round-trip; taskId is the best-effort guess.
// CancelTask cancels a single TaskRabbit booking. Verified live 2026-07-03:
// page.tasks.cancelTask with type "single" requires jobId (details.id),
// rabbitId (taskers[0].id), and reason.
func (t *Client) CancelTask(ctx context.Context, jobID, rabbitID int, reason string, csrfToken string) (json.RawMessage, error) {
	if reason == "" {
		reason = "Plans changed, no longer need the help"
	}
	return t.Mutation(ctx, "page.tasks.cancelTask", map[string]any{
		"type":     "single",
		"jobId":    jobID,
		"rabbitId": rabbitID,
		"reason":   reason,
	}, csrfToken)
}

func unwrapTRPC(proc string, body []byte) (json.RawMessage, error) {
	var batch []json.RawMessage
	if err := json.Unmarshal(body, &batch); err != nil {
		return nil, fmt.Errorf("taskrabbit %s: decode tRPC batch: %w", proc, err)
	}
	if len(batch) == 0 {
		return nil, fmt.Errorf("taskrabbit %s: empty tRPC response", proc)
	}

	var item struct {
		Result *struct {
			Data struct {
				JSON json.RawMessage `json:"json"`
			} `json:"data"`
		} `json:"result"`
		Error *struct {
			JSON struct {
				Message string `json:"message"`
				Data    struct {
					Code string `json:"code"`
				} `json:"data"`
			} `json:"json"`
		} `json:"error"`
	}
	if err := json.Unmarshal(batch[0], &item); err != nil {
		return nil, fmt.Errorf("taskrabbit %s: decode tRPC item: %w", proc, err)
	}
	if item.Error != nil {
		return nil, &TRPCError{
			Proc:    proc,
			Code:    item.Error.JSON.Data.Code,
			Message: item.Error.JSON.Message,
		}
	}
	if item.Result == nil || len(item.Result.Data.JSON) == 0 {
		return nil, fmt.Errorf("taskrabbit %s: missing tRPC result.data.json", proc)
	}
	return cloneRaw(item.Result.Data.JSON), nil
}

func parseListTasksPayload(payload json.RawMessage) ([]Booking, error) {
	var decoded struct {
		BFF struct {
			Items []json.RawMessage `json:"items"`
		} `json:"bff"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("taskrabbit page.tasks.list: decode payload: %w", err)
	}
	bookings := make([]Booking, 0, len(decoded.BFF.Items))
	for _, raw := range decoded.BFF.Items {
		booking, err := parseBooking(raw)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}
	return bookings, nil
}

func parseBooking(raw json.RawMessage) (Booking, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Booking{}, fmt.Errorf("taskrabbit page.tasks.list: decode item: %w", err)
	}
	booking := Booking{
		ID:                 stringFromRaw(fields["id"]),
		Status:             stringFromRaw(fields["status"]),
		Taskers:            firstRaw(fields, "taskers", "tasker", "rabbits"),
		FutureAppointments: firstRaw(fields, "futureAppointments", "future_appointments"),
		Raw:                cloneRaw(raw),
	}
	if booking.ID == "" {
		booking.ID = idFromDetails(fields["details"])
	}
	// jobId (details.id) and rabbitId (taskers[0].id) power cancelTask.
	if d := fields["details"]; len(d) > 0 {
		var det struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(d, &det) == nil {
			booking.JobID = det.ID
		}
	}
	if booking.JobID == 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(booking.ID)); err == nil {
			booking.JobID = n
		}
	}
	if t := booking.Taskers; len(t) > 0 {
		var arr []struct {
			ID        int    `json:"id"`
			FirstName string `json:"firstName"`
			Name      string `json:"name"`
		}
		if json.Unmarshal(t, &arr) == nil && len(arr) > 0 {
			booking.RabbitID = arr[0].ID
			if arr[0].FirstName != "" {
				booking.TaskerName = arr[0].FirstName
			} else {
				booking.TaskerName = arr[0].Name
			}
		}
	}
	if a := fields["appointment"]; len(a) > 0 {
		var appt struct {
			LongDateText string `json:"longDateText"`
			TimeText     string `json:"timeText"`
		}
		if json.Unmarshal(a, &appt) == nil {
			booking.Appointment = strings.TrimSpace(appt.LongDateText + " " + appt.TimeText)
		}
	}
	return booking, nil
}

func parseRecommendationsPayload(payload json.RawMessage) ([]Tasker, Histogram, error) {
	taskers, histogram, _, err := parseRecommendationsPayloadWithID(payload)
	return taskers, histogram, err
}

func parseRecommendationsPayloadWithID(payload json.RawMessage) ([]Tasker, Histogram, string, error) {
	var decoded struct {
		BFF struct {
			Recommendations  []json.RawMessage `json:"recommendations"`
			Histogram        Histogram         `json:"histogram"`
			RecommendationID string            `json:"recommendation_id"`
		} `json:"bff"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, Histogram{}, "", fmt.Errorf("taskrabbit page.book.recommendations: decode payload: %w", err)
	}
	taskers := make([]Tasker, 0, len(decoded.BFF.Recommendations))
	for _, raw := range decoded.BFF.Recommendations {
		var tasker Tasker
		if err := json.Unmarshal(raw, &tasker); err != nil {
			return nil, Histogram{}, "", fmt.Errorf("taskrabbit page.book.recommendations: decode tasker: %w", err)
		}
		tasker.Raw = cloneRaw(raw)
		taskers = append(taskers, tasker)
	}
	return taskers, decoded.BFF.Histogram, decoded.BFF.RecommendationID, nil
}

func parseSchedulePayload(payload json.RawMessage) ([]AvailableDate, error) {
	var decoded struct {
		BFF struct {
			AvailableDates []AvailableDate `json:"availableDates"`
		} `json:"bff"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("taskrabbit page.book.schedule: decode payload: %w", err)
	}
	if decoded.BFF.AvailableDates == nil {
		return make([]AvailableDate, 0), nil
	}
	return decoded.BFF.AvailableDates, nil
}

func trpcInputEnvelope(input any) map[string]map[string]any {
	return map[string]map[string]any{
		"0": {
			"json": input,
		},
	}
}

func idFromDetails(raw json.RawMessage) string {
	if id := stringFromRaw(raw); id != "" {
		return id
	}
	var details map[string]json.RawMessage
	if err := json.Unmarshal(raw, &details); err != nil {
		return ""
	}
	for _, key := range []string{"id", "taskId", "task_id", "guid", "taskGuid", "task_guid"} {
		if id := stringFromRaw(details[key]); id != "" {
			return id
		}
	}
	return ""
}

func stringFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return ""
	}
	switch v := value.(type) {
	case json.Number:
		return v.String()
	case bool:
		return fmt.Sprint(v)
	default:
		return ""
	}
}

func intFromRaw(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0
	}
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
		if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return int(f)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	case float64:
		return int(v)
	}
	return 0
}

func firstRaw(fields map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, key := range keys {
		if raw, ok := fields[key]; ok {
			return cloneRaw(raw)
		}
	}
	return nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}
