// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Structural event-mutation safety barrier: forced sendUpdates=none, attendee refusal, etag concurrency.
// Every mutating request to the events surface passes through enforceEventSafety
// at the client choke point — generated commands, novel commands, and any future
// code path alike. Three guarantees, none of them opt-outable by flag:
//
//  1. sendUpdates is FORCED to "none" (and legacy sendNotifications stripped):
//     no mutation performed by this binary can ever email attendees.
//  2. Attendee-bearing payloads are refused, and mutations targeting an
//     existing event that HAS attendees are refused after a live pre-check —
//     attendee-bearing events are changed by humans in the Calendar UI, never
//     by this tool.
//  3. Mutations of existing events default to optimistic concurrency: when the
//     caller did not supply If-Match, the pre-check's etag is attached, so a
//     mid-flight human edit makes the write fail cleanly instead of clobbering.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/cliutil"
)

// ErrThirdPartyBarrier marks refusals by guarantee #2.
var ErrThirdPartyBarrier = errors.New("event-safety barrier")

// eventItemRe matches /calendars/{id}/events/{eventId} with an optional /move
// suffix — the "existing event" mutation targets.
var eventItemRe = regexp.MustCompile(`^(/calendars/[^/]+/events/[^/]+?)(/move)?$`)

// eventsCollectionRe matches /calendars/{id}/events — the insert target.
var eventsCollectionRe = regexp.MustCompile(`^/calendars/[^/]+/events$`)

func isMutating(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// enforceEventSafety applies the three guarantees. It may perform one read
// (the pre-check GET) for existing-event mutations; that read is skipped in
// dry-run and verify modes, where no real mutation will be sent anyway.
func (c *Client) enforceEventSafety(ctx context.Context, method, path string, params map[string]string, body any, headers map[string]string) (map[string]string, map[string]string, error) {
	if !isMutating(method) || !strings.Contains(path, "/events") {
		return params, headers, nil
	}
	itemMatch := eventItemRe.FindStringSubmatch(path)
	isInsert := method == "POST" && eventsCollectionRe.MatchString(path)
	if itemMatch == nil && !isInsert {
		return params, headers, nil // not an events mutation shape (e.g. /freeBusy)
	}

	// Guarantee 1: forced quiet.
	if params == nil {
		params = map[string]string{}
	}
	params["sendUpdates"] = "none"
	delete(params, "sendNotifications")

	// Guarantee 2a: payload may not carry attendees.
	if hasAttendees(body) {
		return nil, nil, fmt.Errorf("%w: refusing to write an attendee-bearing event payload — attendee-bearing events are managed in the Calendar UI", ErrThirdPartyBarrier)
	}

	// Existing-event mutations: pre-check attendees + default If-Match.
	if itemMatch != nil {
		if c.DryRun || cliutil.IsVerifyEnv() {
			return params, headers, nil // no real mutation will fire; skip the live pre-check
		}
		eventPath := itemMatch[1]
		pre, _, err := c.doRead(ctx, "GET", eventPath, map[string]string{}, nil, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("event-safety pre-check failed for %s (refusing to mutate blind): %w", eventPath, err)
		}
		var ev struct {
			Etag      string           `json:"etag"`
			Attendees []map[string]any `json:"attendees"`
		}
		if err := json.Unmarshal(pre, &ev); err != nil {
			return nil, nil, fmt.Errorf("event-safety pre-check: unparseable event body for %s: %w", eventPath, err)
		}
		if len(ev.Attendees) > 0 {
			return nil, nil, fmt.Errorf("%w: event %s has %d attendee(s) — this tool never mutates attendee-bearing events", ErrThirdPartyBarrier, eventPath, len(ev.Attendees))
		}
		// Guarantee 3: default optimistic concurrency.
		if headers == nil {
			headers = map[string]string{}
		}
		hasIfMatch := false
		for k := range headers {
			if strings.EqualFold(k, "If-Match") {
				hasIfMatch = true
				break
			}
		}
		if !hasIfMatch && ev.Etag != "" {
			headers["If-Match"] = ev.Etag
		}
	}
	return params, headers, nil
}

// hasAttendees probes an arbitrary request body for a non-empty attendees list.
func hasAttendees(body any) bool {
	if body == nil {
		return false
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	if a, ok := m["attendees"].([]any); ok && len(a) > 0 {
		return true
	}
	return false
}
