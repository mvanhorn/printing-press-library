// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared parsing helpers for hand-authored Agilix Dawn novel commands.
// Kept in its own file so `generate --force` preserves it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
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

// instructionInteractions counts interactions in one instruction.
func (i dawnInstruction) interactionCount() int { return len(i.Interaction) }
