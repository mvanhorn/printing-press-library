// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared parsing helpers for hand-authored Agilix Dawn novel commands.
// Kept in its own file so `generate --force` preserves it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/internal/client"
)

// dawnInstruction is one lesson/activity within a section. Interaction and
// supplemental are kept raw because only their counts matter to novel commands.
type dawnInstruction struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	Duration     float64           `json:"duration"`
	Points       int               `json:"points"`
	Interaction  []json.RawMessage `json:"interaction"`
	Supplemental []json.RawMessage `json:"supplemental"`
}

// dawnSection groups instructions.
type dawnSection struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Status      string            `json:"status"`
	Instruction []dawnInstruction `json:"instruction"`
}

// dawnConcept is a catalog item (course/content) with its full structure.
type dawnConcept struct {
	ID               string        `json:"id"`
	Title            string        `json:"title"`
	ShortDescription string        `json:"shortDescription"`
	Description      string        `json:"description"`
	Status           string        `json:"status"`
	Price            int           `json:"price"`
	FullPrice        int           `json:"fullPrice"`
	DefaultLanguage  string        `json:"defaultLanguage"`
	Created          string        `json:"created"`
	Modified         string        `json:"modified"`
	Section          []dawnSection `json:"section"`
}

// dawnEnvelope is the standard Dawn search response wrapper.
type dawnEnvelope struct {
	TotalMatches int               `json:"totalMatches"`
	Matches      []json.RawMessage `json:"matches"`
}

// fetchConcept GETs /concept/{id}, which returns the object directly (not enveloped).
func fetchConcept(ctx context.Context, c *client.Client, id string) (*dawnConcept, error) {
	data, err := c.Get(ctx, "/concept/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	var concept dawnConcept
	if err := json.Unmarshal(data, &concept); err != nil {
		return nil, fmt.Errorf("parsing concept %s: %w", id, err)
	}
	if concept.ID == "" {
		return nil, fmt.Errorf("no concept found for id %q", id)
	}
	return &concept, nil
}

// fetchSearch GETs /<resource>?search=<json> and returns the matches array.
// search is the raw Dawn DSL JSON (e.g. {"limit":100}).
func fetchSearch(ctx context.Context, c *client.Client, resource, search string) (int, []json.RawMessage, error) {
	if search == "" {
		search = `{"limit":100}`
	}
	data, err := c.Get(ctx, "/"+resource, map[string]string{"search": search})
	if err != nil {
		return 0, nil, err
	}
	var env dawnEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return 0, nil, fmt.Errorf("parsing %s response: %w", resource, err)
	}
	return env.TotalMatches, env.Matches, nil
}

// searchPageSize is the per-request limit fetchAllSearch pages with.
const searchPageSize = 1000

// searchMaxRecords caps fetchAllSearch so a runaway resource cannot exhaust
// memory. A cap hit is surfaced (total > len(all)) so callers can warn rather
// than silently truncate — the failure mode this helper exists to prevent.
const searchMaxRecords = 100000

// fetchAllSearch pages /<resource> until every match is retrieved, walking the
// Dawn search DSL's start/limit window. It returns the full match set and the
// server-reported total; when total exceeds the returned slice length the
// safety cap was hit and the caller should warn (see warnTruncated). base is
// the query body without start/limit (e.g. {"query":...,"include":[...]}); a
// nil base means an unfiltered listing.
func fetchAllSearch(ctx context.Context, c *client.Client, resource string, base map[string]any) ([]json.RawMessage, int, error) {
	var all []json.RawMessage
	total := 0
	for start := 0; ; start += searchPageSize {
		q := make(map[string]any, len(base)+2)
		for k, v := range base {
			q[k] = v
		}
		q["start"] = start
		q["limit"] = searchPageSize
		raw, err := json.Marshal(q)
		if err != nil {
			return nil, 0, fmt.Errorf("building %s search: %w", resource, err)
		}
		t, matches, err := fetchSearch(ctx, c, resource, string(raw))
		if err != nil {
			return nil, 0, err
		}
		total = t
		all = append(all, matches...)
		// Stop when the page came back short (we've reached the end), when we've
		// collected everything the server reported, or at the safety cap.
		if len(matches) < searchPageSize || len(all) >= total || len(all) >= searchMaxRecords {
			break
		}
	}
	return all, total, nil
}

// warnTruncated emits a stderr warning when a paged/limited fetch returned
// fewer records than the server's reported total, so an operator is never left
// believing an aggregation is complete when it is not.
func warnTruncated(w io.Writer, resource string, got, total int) {
	if total > got {
		fmt.Fprintf(w, "warning: %s results truncated — showing %d of %d; output is incomplete\n", resource, got, total)
	}
}

// instructionInteractions counts interactions in one instruction.
func (i dawnInstruction) interactionCount() int { return len(i.Interaction) }
