// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package tock

// PATCH: cross-network-source-clients (calendar) — see .printing-press-patches.json.
// Tock's per-(business, date, party) availability is not in SSR or any JSON
// REST endpoint. The site's runtime XHR is POST /api/consumer/calendar/full/v2
// with an empty protobuf body and an X-Tock-Scope header carrying the
// businessId. The response is a protobuf MessageSet wrapping
// ConsumerFullCalendarV2 → map<businessDay,TicketGroupByDate> →
// map<date,TicketGroupList> → repeated CalendarTicketGroup. Each
// CalendarTicketGroup carries date, time, availableTickets, and
// minPurchaseSize/maxPurchaseSize. Schema extracted from the live JS bundle
// at exploretock.com/static/<build>/explore.js (proto2 JSON descriptors are
// embedded inline in the bundle).
//
// We hand-roll a wire-format walker rather than pulling protoreflect because
// the schema we need is small (5 message types, ~15 fields), the printed CLI
// must stay self-contained, and we don't depend on .proto compilation.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CalendarSlot is a single (business, date, time) slot from
// /api/consumer/calendar/full/v2. A slot is bookable for a party of N when
// MinPurchaseSize <= N <= MaxPurchaseSize and AvailableTickets >= N.
type CalendarSlot struct {
	Date             string // "2026-05-13"
	BusinessDay      string // "2026-05-13" (may differ from Date for late-night slots)
	Time             string // "20:00"
	NumTickets       int32
	AvailableTickets int32
	HeldTickets      int32
	SoldTickets      int32
	LockedTickets    int32
	MinPurchaseSize  int32
	MaxPurchaseSize  int32
	// ExperienceIDs lists the ticketTypeIds attached to this slot. Tock encodes
	// price-per-experience inside a slot, so a single slot can serve multiple
	// experience IDs (e.g., a Dining Room slot serving the public-menu and the
	// chef's-counter experience). Caller filters by experience when relevant.
	ExperienceIDs []uint64
}

// CalendarResponse is a per-business view across all surfaced dates.
type CalendarResponse struct {
	BusinessID int
	Slots      []CalendarSlot
	OpenDates  []string // dates Tock surfaces as eligible (slots may still be sold out)
}

// AvailableForParty returns the subset of slots whose purchase-size window
// includes party AND availableTickets >= party. Caller still filters by date
// or experience.
func (r *CalendarResponse) AvailableForParty(party int) []CalendarSlot {
	if r == nil || party <= 0 {
		return nil
	}
	out := make([]CalendarSlot, 0, len(r.Slots))
	for _, s := range r.Slots {
		if s.MinPurchaseSize > 0 && int32(party) < s.MinPurchaseSize {
			continue
		}
		if s.MaxPurchaseSize > 0 && int32(party) > s.MaxPurchaseSize {
			continue
		}
		if s.AvailableTickets < int32(party) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// EarliestAvailable returns the earliest slot that fits party between
// dateFrom and dateTo (inclusive, "YYYY-MM-DD"). Returns nil when nothing
// matches. Caller may pass an experienceID > 0 to filter; 0 means any.
func (r *CalendarResponse) EarliestAvailable(party int, dateFrom, dateTo string, experienceID uint64) *CalendarSlot {
	if r == nil {
		return nil
	}
	var best *CalendarSlot
	for i := range r.Slots {
		s := r.Slots[i]
		if s.Date < dateFrom || s.Date > dateTo {
			continue
		}
		if s.MinPurchaseSize > 0 && int32(party) < s.MinPurchaseSize {
			continue
		}
		if s.MaxPurchaseSize > 0 && int32(party) > s.MaxPurchaseSize {
			continue
		}
		if s.AvailableTickets < int32(party) {
			continue
		}
		if experienceID != 0 {
			matched := false
			for _, id := range s.ExperienceIDs {
				if id == experienceID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		// "Earliest" = lowest (Date, Time) pair. Both are stable lex-sortable.
		if best == nil {
			b := s
			best = &b
			continue
		}
		if s.Date < best.Date || (s.Date == best.Date && s.Time < best.Time) {
			b := s
			best = &b
		}
	}
	return best
}

// CalendarBootstrap captures the per-venue values needed to call
// /api/consumer/calendar/full/v2: numeric business id (sent in X-Tock-Scope),
// anonymous JWT (sent as X-Tock-Authorization), and the SPA build number
// (sent as X-Tock-Build-Number). The bootstrap fetches `/<slug>` once and
// reads what the SSR Redux state carries.
type CalendarBootstrap struct {
	BusinessID  int
	JWT         string
	BuildNumber string
}

// staticBuildRE matches script src URLs like "/static/2026-05-05RC06-00/..."
// the SPA emits — that's the public form of __BUILD_NUMBER__.
var staticBuildRE = regexp.MustCompile(`/static/([A-Za-z0-9._-]+)/`)

// CalendarBootstrap fetches `/<slug>` and harvests the values needed for the
// runtime calendar XHR. Slug is the venue path component (e.g., "canlis").
func (c *Client) CalendarBootstrap(ctx context.Context, slug string) (*CalendarBootstrap, error) {
	path := "/" + strings.TrimPrefix(slug, "/")
	url := Origin + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building tock SSR request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.do429Aware(req)
	if err != nil {
		return nil, fmt.Errorf("fetching tock %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tock %s returned HTTP %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tock %s body: %w", path, err)
	}

	bs := &CalendarBootstrap{}
	if m := staticBuildRE.FindSubmatch(body); m != nil {
		bs.BuildNumber = string(m[1])
	}

	// Parse the Redux state to get app.config.business.id and app.jwtToken.
	state, err := c.parseReduxBytes(body)
	if err != nil {
		return nil, fmt.Errorf("tock: parse $REDUX_STATE for bootstrap: %w", err)
	}
	if app, ok := state["app"].(map[string]any); ok {
		if jwt, ok := app["jwtToken"].(string); ok {
			bs.JWT = jwt
		}
		if cfg, ok := app["config"].(map[string]any); ok {
			if biz, ok := cfg["business"].(map[string]any); ok {
				if idF, ok := biz["id"].(float64); ok && idF > 0 {
					bs.BusinessID = int(idF)
				}
			}
		}
	}
	if bs.BusinessID == 0 {
		return nil, fmt.Errorf("tock: business id missing from %s SSR; venue may not exist", path)
	}
	return bs, nil
}

// Calendar fetches the full calendar (all dates Tock surfaces) for a venue
// and returns a flat slice of slots. Internally calls CalendarBootstrap to
// get the business id + JWT + build number, then POSTs an empty
// ConsumerCalendarRequest body to /api/consumer/calendar/full/v2 with the
// correct X-Tock-Scope header. Empty body returns slots for all
// experiences/ticket-types the venue exposes; caller filters by date,
// party, or experience.
func (c *Client) Calendar(ctx context.Context, slug string) (*CalendarResponse, error) {
	bs, err := c.CalendarBootstrap(ctx, slug)
	if err != nil {
		return nil, err
	}
	return c.calendarFromBootstrap(ctx, slug, bs)
}

// CalendarWithBootstrap lets callers reuse a bootstrap they already have
// (e.g., earliest already fetched the SSR for VenueDetail).
func (c *Client) CalendarWithBootstrap(ctx context.Context, slug string, bs *CalendarBootstrap) (*CalendarResponse, error) {
	if bs == nil {
		return c.Calendar(ctx, slug)
	}
	return c.calendarFromBootstrap(ctx, slug, bs)
}

func (c *Client) calendarFromBootstrap(ctx context.Context, slug string, bs *CalendarBootstrap) (*CalendarResponse, error) {
	url := Origin + "/api/consumer/calendar/full/v2"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(""))
	if err != nil {
		return nil, fmt.Errorf("building tock calendar request: %w", err)
	}
	scope := fmt.Sprintf(`{"businessId":"%d"}`, bs.BusinessID)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("X-Tock-Stream-Format", "proto2")
	req.Header.Set("X-Tock-Scope", scope)
	req.Header.Set("X-Tock-Path", "/"+strings.TrimPrefix(slug, "/"))
	if bs.BuildNumber != "" {
		req.Header.Set("X-Tock-Build-Number", bs.BuildNumber)
	}
	if bs.JWT != "" {
		req.Header.Set("X-Tock-Authorization", bs.JWT)
	}
	req.Header.Set("Origin", Origin)
	req.Header.Set("Referer", Origin+"/"+strings.TrimPrefix(slug, "/"))

	resp, err := c.do429Aware(req)
	if err != nil {
		return nil, fmt.Errorf("calling tock calendar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tock: calendar endpoint returned 404 for businessId=%d (venue may not be using Tock for online booking)", bs.BusinessID)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tock calendar returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tock calendar body: %w", err)
	}
	out, err := decodeCalendarResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decoding tock calendar protobuf: %w", err)
	}
	out.BusinessID = bs.BusinessID
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// parseReduxBytes is a thin wrapper around the FetchReduxState parsing path
// that lets us reuse already-fetched HTML bytes.
func (c *Client) parseReduxBytes(body []byte) (map[string]any, error) {
	// Reuse the same primary path as FetchReduxState.
	// We import here rather than duplicate to keep the parsing surface single-sourced.
	state, err := parseReduxStateBody(body)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// --- Protobuf wire-format decoder (just what we need) -----------------------

// readVarint reads a base-128 varint at pos, returning the value and the new
// position. Returns (0, len, true) on truncation.
func readVarint(buf []byte, pos int) (uint64, int, bool) {
	var v uint64
	var shift uint
	for pos < len(buf) {
		b := buf[pos]
		pos++
		v |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return v, pos, false
		}
		shift += 7
		if shift > 63 {
			return 0, pos, true
		}
	}
	return 0, pos, true
}

// readTag reads a protobuf tag varint and returns (fieldNum, wireType, newPos, eof).
func readTag(buf []byte, pos int) (int, int, int, bool) {
	v, p, eof := readVarint(buf, pos)
	if eof {
		return 0, 0, p, true
	}
	return int(v >> 3), int(v & 7), p, false
}

// each visits each (field, wireType, valueBytes/varint) tuple in [pos, end).
// For wire-type 0 (varint), the visitor receives the varint as v and len(v)==0.
// For wire-type 2 (length-delimited), v is the slice. Wire types 1 and 5
// (fixed64/fixed32) are skipped without invoking the visitor.
func each(buf []byte, pos, end int, visit func(field int, wire int, v uint64, lv []byte)) {
	for pos < end {
		f, w, np, eof := readTag(buf, pos)
		if eof {
			return
		}
		pos = np
		switch w {
		case 0: // varint
			v, p, e := readVarint(buf, pos)
			if e {
				return
			}
			pos = p
			visit(f, w, v, nil)
		case 2: // length-delimited
			l, p, e := readVarint(buf, pos)
			if e {
				return
			}
			pos = p
			n := int(l)
			if n < 0 || pos+n > end {
				return
			}
			visit(f, w, 0, buf[pos:pos+n])
			pos += n
		case 1: // fixed64
			pos += 8
		case 5: // fixed32
			pos += 4
		default:
			return
		}
	}
}

// decodeCalendarResponse walks the protobuf bytes returned by
// /api/consumer/calendar/full/v2 and produces a flat slice of CalendarSlot.
//
// The wire envelope is:
//
//	field 1 (len) → field 1 (len) → field 60686 (len) → ConsumerFullCalendarV2
//
// where 60686 is the messageSetExtension id of ConsumerFullCalendarV2.
//
// ConsumerFullCalendarV2:
//
//	field 1 = repeated map<string, TicketGroupByDate> ticketGroupByBusinessDay
//	field 7 = repeated string openDate
//
// Each map entry is itself a length-delimited message with field 1=key and
// field 2=value; that's the standard proto map encoding.
//
// TicketGroupByDate:
//
//	field 1 = repeated map<string, TicketGroupList> ticketGroupByDate
//
// TicketGroupList:
//
//	field 2 = repeated CalendarTicketGroup
//
// CalendarTicketGroup:
//
//	1 date, 2 businessDay, 3 time, 4 numTickets, 5 availableTickets,
//	6 heldTickets, 7 soldTickets, 8 lockedTickets, 9 minPurchaseSize,
//	13 ticketTypePrice (repeated CalendarTicketTypePrice with field 1=ticketTypeId),
//	19 maxPurchaseSize.
func decodeCalendarResponse(body []byte) (*CalendarResponse, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty calendar body")
	}
	// Strip envelope: locate field 60686 inside the (field 1 → field 1) wrapper.
	var inner []byte
	each(body, 0, len(body), func(f int, _ int, _ uint64, lv []byte) {
		if f == 1 && lv != nil && inner == nil {
			each(lv, 0, len(lv), func(f2 int, _ int, _ uint64, lv2 []byte) {
				if f2 == 1 && lv2 != nil && inner == nil {
					each(lv2, 0, len(lv2), func(f3 int, _ int, _ uint64, lv3 []byte) {
						if f3 == 60686 && lv3 != nil && inner == nil {
							inner = lv3
						}
					})
				}
			})
		}
	})
	if inner == nil {
		// Some Tock builds skip a wrapper layer; try one level shallower.
		each(body, 0, len(body), func(f int, _ int, _ uint64, lv []byte) {
			if f == 1 && lv != nil && inner == nil {
				each(lv, 0, len(lv), func(f2 int, _ int, _ uint64, lv2 []byte) {
					if f2 == 60686 && lv2 != nil && inner == nil {
						inner = lv2
					}
				})
			}
		})
	}
	if inner == nil {
		return nil, fmt.Errorf("calendar response: ConsumerFullCalendarV2 extension (60686) not found in envelope")
	}

	out := &CalendarResponse{}
	each(inner, 0, len(inner), func(f int, _ int, _ uint64, lv []byte) {
		if f == 1 && lv != nil {
			// outer map entry: {1:businessDay,2:TicketGroupByDate}
			var bizDay string
			var tgbd []byte
			each(lv, 0, len(lv), func(f2 int, _ int, _ uint64, lv2 []byte) {
				if f2 == 1 && lv2 != nil {
					bizDay = string(lv2)
				} else if f2 == 2 && lv2 != nil {
					tgbd = lv2
				}
			})
			if tgbd == nil {
				return
			}
			each(tgbd, 0, len(tgbd), func(f3 int, _ int, _ uint64, lv3 []byte) {
				if f3 != 1 || lv3 == nil {
					return
				}
				// inner map entry: {1:date,2:TicketGroupList}
				var date string
				var tgl []byte
				each(lv3, 0, len(lv3), func(f4 int, _ int, _ uint64, lv4 []byte) {
					if f4 == 1 && lv4 != nil {
						date = string(lv4)
					} else if f4 == 2 && lv4 != nil {
						tgl = lv4
					}
				})
				if tgl == nil {
					return
				}
				each(tgl, 0, len(tgl), func(f5 int, _ int, _ uint64, lv5 []byte) {
					if f5 != 2 || lv5 == nil {
						return
					}
					slot := CalendarSlot{Date: date, BusinessDay: bizDay}
					each(lv5, 0, len(lv5), func(f6 int, _ int, vi uint64, lv6 []byte) {
						switch f6 {
						case 1:
							if lv6 != nil {
								slot.Date = string(lv6)
							}
						case 2:
							if lv6 != nil {
								slot.BusinessDay = string(lv6)
							}
						case 3:
							if lv6 != nil {
								slot.Time = string(lv6)
							}
						case 4:
							slot.NumTickets = int32(int64(vi))
						case 5:
							slot.AvailableTickets = int32(int64(vi))
						case 6:
							slot.HeldTickets = int32(int64(vi))
						case 7:
							slot.SoldTickets = int32(int64(vi))
						case 8:
							slot.LockedTickets = int32(int64(vi))
						case 9:
							slot.MinPurchaseSize = int32(int64(vi))
						case 19:
							slot.MaxPurchaseSize = int32(int64(vi))
						case 13:
							if lv6 == nil {
								return
							}
							each(lv6, 0, len(lv6), func(f7 int, _ int, vi2 uint64, _ []byte) {
								if f7 == 1 {
									slot.ExperienceIDs = append(slot.ExperienceIDs, vi2)
								}
							})
						}
					})
					if slot.Date == "" {
						return
					}
					out.Slots = append(out.Slots, slot)
				})
			})
		} else if f == 7 && lv != nil {
			out.OpenDates = append(out.OpenDates, string(lv))
		}
	})
	return out, nil
}

// --- Date helpers ------------------------------------------------------------

// AddDays returns a "YYYY-MM-DD" date string offset from in by days.
func AddDays(date string, days int) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, days).Format("2006-01-02"), nil
}

// ParseDate is exposed so callers don't need to import time directly.
func ParseDate(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }

// IntFromBizID is a tiny helper for callers that have a string-ish id.
func IntFromBizID(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
