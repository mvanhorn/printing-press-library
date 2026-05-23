// Hand-authored shared store readers for the Eventbrite transcendence commands.
//
// All novel analytics commands (sales-velocity, repeat-attendees,
// discount-performance, org-rollup, roster, capacity, refund-rate, top-buyers,
// fan-export) read synced Eventbrite data from the generic `resources` table,
// which every typed Upsert also populates (UpsertEvents → upsertGenericResourceTx).
//
// Eventbrite list endpoints are hierarchical, so the same logical entity lands
// under several resource_type keys (e.g. an event synced via
// /organizations/{id}/events is `organizations_events`, while a single GET is
// `events`). Each reader queries the full IN-list and de-dupes by id so a
// command sees one row per entity regardless of which sync path produced it.
//
// NULL-safety: every optional JSON field is COALESCE'd to a zero default in SQL
// so the scan targets stay bare (no per-row sql.Null* dance, no silent
// row-drops on NULL). Money lives in `costs.gross.value` as integer minor units
// (cents); callers divide by 100 for display. This file is NOT generated and
// survives `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/internal/store"
)

// ebCLIName is the binary name used to resolve the default SQLite path.
const ebCLIName = "eventbrite-pp-cli"

// resource_type IN-lists covering every sync path that can produce each entity.
const (
	ebEventTypes       = `'events','organizations_events','series_events','venues_events'`
	ebOrderTypes       = `'orders','events_orders','organizations_orders','users_orders'`
	ebAttendeeTypes    = `'events_attendees','organizations_attendees'`
	ebTicketClassTypes = `'ticket_classes'`
	ebDiscountTypes    = `'discounts','organizations_discounts'`
)

type ebEvent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	StartUTC string `json:"start_utc"`
	EndUTC   string `json:"end_utc"`
	Status   string `json:"status"`
	Currency string `json:"currency"`
	OrgID    string `json:"organization_id"`
	Capacity int64  `json:"capacity"`
	Created  string `json:"created"`
}

type ebOrder struct {
	ID         string `json:"id"`
	Created    string `json:"created"`
	Status     string `json:"status"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	EventID    string `json:"event_id"`
	GrossMinor int64  `json:"gross_minor"`
	Currency   string `json:"currency"`
}

type ebAttendee struct {
	ID          string `json:"id"`
	Created     string `json:"created"`
	CheckedIn   bool   `json:"checked_in"`
	Cancelled   bool   `json:"cancelled"`
	Refunded    bool   `json:"refunded"`
	Status      string `json:"status"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	EventID     string `json:"event_id"`
	TicketClass string `json:"ticket_class"`
	GrossMinor  int64  `json:"gross_minor"`
	Currency    string `json:"currency"`
}

type ebTicketClass struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	EventID   string `json:"event_id"`
	QtyTotal  int64  `json:"quantity_total"`
	QtySold   int64  `json:"quantity_sold"`
	CostMinor int64  `json:"cost_minor"`
}

type ebDiscount struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Type       string  `json:"type"`
	EventID    string  `json:"event_id"`
	PercentOff float64 `json:"percent_off"`
	AmountOff  int64   `json:"amount_off_minor"`
	QtyAvail   int64   `json:"quantity_available"`
	QtySold    int64   `json:"quantity_sold"`
}

// openEBStore opens the local store for reading; returns (nil, nil) when no
// database has been synced yet so callers can emit an empty result + sync hint.
func openEBStore(ctx context.Context) (*store.Store, error) {
	return openStoreForRead(ctx, ebCLIName)
}

func readEBEvents(ctx context.Context, db *sql.DB) ([]ebEvent, error) {
	q := `SELECT
		COALESCE(json_extract(data,'$.id'), id),
		COALESCE(json_extract(data,'$.name.text'), json_extract(data,'$.name'), ''),
		COALESCE(json_extract(data,'$.start.utc'), ''),
		COALESCE(json_extract(data,'$.end.utc'), ''),
		COALESCE(json_extract(data,'$.status'), ''),
		COALESCE(json_extract(data,'$.currency'), ''),
		COALESCE(json_extract(data,'$.organization_id'), ''),
		COALESCE(json_extract(data,'$.capacity'), 0),
		COALESCE(json_extract(data,'$.created'), '')
	FROM resources WHERE resource_type IN (` + ebEventTypes + `)`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading events: %w", err)
	}
	defer rows.Close()
	seen := map[string]int{}
	out := make([]ebEvent, 0)
	for rows.Next() {
		var e ebEvent
		if err := rows.Scan(&e.ID, &e.Name, &e.StartUTC, &e.EndUTC, &e.Status, &e.Currency, &e.OrgID, &e.Capacity, &e.Created); err != nil {
			continue
		}
		if i, ok := seen[e.ID]; ok {
			out[i] = e
			continue
		}
		seen[e.ID] = len(out)
		out = append(out, e)
	}
	return out, rows.Err()
}

func readEBOrders(ctx context.Context, db *sql.DB) ([]ebOrder, error) {
	q := `SELECT
		COALESCE(json_extract(data,'$.id'), id),
		COALESCE(json_extract(data,'$.created'), ''),
		COALESCE(json_extract(data,'$.status'), ''),
		COALESCE(json_extract(data,'$.email'), ''),
		COALESCE(json_extract(data,'$.name'), ''),
		COALESCE(json_extract(data,'$.event_id'), ''),
		COALESCE(json_extract(data,'$.costs.gross.value'), 0),
		COALESCE(json_extract(data,'$.costs.gross.currency'), '')
	FROM resources WHERE resource_type IN (` + ebOrderTypes + `)`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading orders: %w", err)
	}
	defer rows.Close()
	seen := map[string]int{}
	out := make([]ebOrder, 0)
	for rows.Next() {
		var o ebOrder
		if err := rows.Scan(&o.ID, &o.Created, &o.Status, &o.Email, &o.Name, &o.EventID, &o.GrossMinor, &o.Currency); err != nil {
			continue
		}
		o.Email = strings.ToLower(strings.TrimSpace(o.Email))
		if i, ok := seen[o.ID]; ok {
			out[i] = o
			continue
		}
		seen[o.ID] = len(out)
		out = append(out, o)
	}
	return out, rows.Err()
}

func readEBAttendees(ctx context.Context, db *sql.DB) ([]ebAttendee, error) {
	q := `SELECT
		COALESCE(json_extract(data,'$.id'), id),
		COALESCE(json_extract(data,'$.created'), ''),
		COALESCE(json_extract(data,'$.checked_in'), 0),
		COALESCE(json_extract(data,'$.cancelled'), 0),
		COALESCE(json_extract(data,'$.refunded'), 0),
		COALESCE(json_extract(data,'$.status'), ''),
		COALESCE(json_extract(data,'$.profile.email'), ''),
		COALESCE(json_extract(data,'$.profile.name'), ''),
		COALESCE(json_extract(data,'$.profile.first_name'), ''),
		COALESCE(json_extract(data,'$.profile.last_name'), ''),
		COALESCE(json_extract(data,'$.event_id'), ''),
		COALESCE(json_extract(data,'$.ticket_class_name'), ''),
		COALESCE(json_extract(data,'$.costs.gross.value'), 0),
		COALESCE(json_extract(data,'$.costs.gross.currency'), '')
	FROM resources WHERE resource_type IN (` + ebAttendeeTypes + `)`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading attendees: %w", err)
	}
	defer rows.Close()
	seen := map[string]int{}
	out := make([]ebAttendee, 0)
	for rows.Next() {
		var a ebAttendee
		var checkedIn, cancelled, refunded int64
		if err := rows.Scan(&a.ID, &a.Created, &checkedIn, &cancelled, &refunded, &a.Status,
			&a.Email, &a.Name, &a.FirstName, &a.LastName, &a.EventID, &a.TicketClass, &a.GrossMinor, &a.Currency); err != nil {
			continue
		}
		a.CheckedIn = checkedIn != 0
		a.Cancelled = cancelled != 0
		a.Refunded = refunded != 0
		a.Email = strings.ToLower(strings.TrimSpace(a.Email))
		if a.Name == "" {
			a.Name = strings.TrimSpace(a.FirstName + " " + a.LastName)
		}
		if i, ok := seen[a.ID]; ok {
			out[i] = a
			continue
		}
		seen[a.ID] = len(out)
		out = append(out, a)
	}
	return out, rows.Err()
}

func readEBTicketClasses(ctx context.Context, db *sql.DB) ([]ebTicketClass, error) {
	q := `SELECT
		COALESCE(json_extract(data,'$.id'), id),
		COALESCE(json_extract(data,'$.name'), ''),
		COALESCE(json_extract(data,'$.event_id'), ''),
		COALESCE(json_extract(data,'$.quantity_total'), 0),
		COALESCE(json_extract(data,'$.quantity_sold'), 0),
		COALESCE(json_extract(data,'$.cost.value'), 0)
	FROM resources WHERE resource_type IN (` + ebTicketClassTypes + `)`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading ticket classes: %w", err)
	}
	defer rows.Close()
	seen := map[string]int{}
	out := make([]ebTicketClass, 0)
	for rows.Next() {
		var t ebTicketClass
		if err := rows.Scan(&t.ID, &t.Name, &t.EventID, &t.QtyTotal, &t.QtySold, &t.CostMinor); err != nil {
			continue
		}
		if i, ok := seen[t.ID]; ok {
			out[i] = t
			continue
		}
		seen[t.ID] = len(out)
		out = append(out, t)
	}
	return out, rows.Err()
}

func readEBDiscounts(ctx context.Context, db *sql.DB) ([]ebDiscount, error) {
	q := `SELECT
		COALESCE(json_extract(data,'$.id'), id),
		COALESCE(json_extract(data,'$.code'), ''),
		COALESCE(json_extract(data,'$.type'), ''),
		COALESCE(json_extract(data,'$.event_id'), ''),
		COALESCE(json_extract(data,'$.percent_off'), 0),
		COALESCE(json_extract(data,'$.amount_off'), 0),
		COALESCE(json_extract(data,'$.quantity_available'), 0),
		COALESCE(json_extract(data,'$.quantity_sold'), 0)
	FROM resources WHERE resource_type IN (` + ebDiscountTypes + `)`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading discounts: %w", err)
	}
	defer rows.Close()
	seen := map[string]int{}
	out := make([]ebDiscount, 0)
	for rows.Next() {
		var d ebDiscount
		var percent sql.NullFloat64
		if err := rows.Scan(&d.ID, &d.Code, &d.Type, &d.EventID, &percent, &d.AmountOff, &d.QtyAvail, &d.QtySold); err != nil {
			continue
		}
		d.PercentOff = percent.Float64
		if i, ok := seen[d.ID]; ok {
			out[i] = d
			continue
		}
		seen[d.ID] = len(out)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ebMajor converts integer minor units (cents) to a major-unit float.
func ebMajor(minor int64) float64 { return float64(minor) / 100.0 }

// ebRound2 rounds to 2 decimal places for money display. Uses math.Round so
// negative values (refund adjustments, chargebacks) round symmetrically rather
// than truncating toward zero.
func ebRound2(f float64) float64 {
	return math.Round(f*100) / 100
}

// ebIsLiveEvent reports whether an event status counts as "live" for the
// live-events analytics commands (sales-velocity, capacity). Live and started
// events are on sale; an empty status (event row not synced, only its ticket
// classes) is included rather than silently dropped. Draft, ended, completed,
// and canceled events are excluded so their sell-out projections don't mislead.
func ebIsLiveEvent(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "live", "started":
		return true
	}
	return false
}

// ebOrderRefundedOrCancelled reports whether an order/attendee status means the
// money was returned or the record voided. Eventbrite keeps the original
// costs.gross.value positive on refunded orders, so revenue/spend/participation
// aggregations must exclude these rather than counting them at face value.
func ebOrderRefundedOrCancelled(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(s, "refund") || s == "cancelled" || s == "canceled" || s == "deleted" || s == "voided"
}
