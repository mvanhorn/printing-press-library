package mcp

import (
	"net/url"
	"strings"
	"testing"
)

// Duplicate proposals look (board #95): the executor checks the list's paging params and the import rows' id and paging as the
// CLI does (hand-written; a reprint must keep it), and sends valid ones as query parameters.
const dpImportID = "7a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"

func TestCodeOrchProposalListPagingIsCheckedThenSent(t *testing.T) {
	const list = "contacts.duplicate-proposals.list-contact"
	for name, extra := range map[string]map[string]any{
		"limit 0":           {"limit": float64(0)},
		"limit 101":         {"limit": float64(101)},
		"limit fraction":    {"limit": 2.5},
		"limit string":      {"limit": "10"},
		"held_limit 101":    {"held_limit": float64(101)},
		"blank cursor":      {"cursor": " "},
		"held_cursor num":   {"held_cursor": float64(1)},
		"empty held_cursor": {"held_cursor": ""},
	} {
		params := map[string]any{"id": dpContactID}
		for k, v := range extra {
			params[k] = v
		}
		hits, _, _, res := execGuard(t, list, params)
		if hits != 0 || !res.IsError {
			t.Errorf("%s: hits=%d isError=%v", name, hits, res.IsError)
		}
	}
	hits, _, rawQuery, res := execGuard(t, list, map[string]any{"id": dpContactID, "cursor": "c1", "limit": float64(10), "held_cursor": "h0", "held_limit": float64(7)})
	if hits != 1 || res.IsError {
		t.Fatalf("valid paging: hits=%d isError=%v", hits, res.IsError)
	}
	q, _ := url.ParseQuery(rawQuery)
	if q.Get("cursor") != "c1" || q.Get("limit") != "10" || q.Get("held_cursor") != "h0" || q.Get("held_limit") != "7" {
		t.Fatalf("query = %s", rawQuery)
	}
}

func TestCodeOrchImportRowsAreCheckedThenSent(t *testing.T) {
	const rows = "imports.duplicate-proposals.get-imports-id"
	for name, params := range map[string]map[string]any{
		"bad id":       {"id": "nope"},
		"no id":        {},
		"page 0":       {"id": dpImportID, "page": float64(0)},
		"page 100001":  {"id": dpImportID, "page": float64(100001)},
		"per_page 101": {"id": dpImportID, "per_page": float64(101)},
		"per_page 1.5": {"id": dpImportID, "per_page": 1.5},
	} {
		hits, _, _, res := execGuard(t, rows, params)
		if hits != 0 || !res.IsError {
			t.Errorf("%s: hits=%d isError=%v", name, hits, res.IsError)
		}
	}
	hits, _, rawQuery, res := execGuard(t, rows, map[string]any{"id": dpImportID, "page": float64(2), "per_page": float64(100)})
	if hits != 1 || res.IsError {
		t.Fatalf("valid: hits=%d isError=%v", hits, res.IsError)
	}
	q, _ := url.ParseQuery(rawQuery)
	if q.Get("page") != "2" || q.Get("per_page") != "100" {
		t.Fatalf("query = %s", rawQuery)
	}
}

// Parity r2 (Codex P1): the two renamed ids keep answering for existing callers, through the same guards and the same request.
func TestCodeOrchLegacyProposalEndpointIdsStillWork(t *testing.T) {
	const legacyList = "contacts.duplicate-proposals.get-contacts-id"
	const legacyDecide = "contacts.duplicate-proposals.post-contacts-id-proposal-id"

	hits, _, rawQuery, res := execGuard(t, legacyList, map[string]any{"id": dpContactID, "limit": float64(10)})
	if hits != 1 || res.IsError {
		t.Fatalf("legacy list: hits=%d isError=%v", hits, res.IsError)
	}
	if q, _ := url.ParseQuery(rawQuery); q.Get("limit") != "10" {
		t.Fatalf("legacy list query = %s", rawQuery)
	}
	// the new guards apply to the old id: a bad page size and a blank cursor never reach the network
	for name, extra := range map[string]map[string]any{"limit 101": {"limit": float64(101)}, "blank cursor": {"cursor": " "}} {
		params := map[string]any{"id": dpContactID}
		for k, v := range extra {
			params[k] = v
		}
		if hits, _, _, res := execGuard(t, legacyList, params); hits != 0 || !res.IsError {
			t.Errorf("legacy list %s: hits=%d isError=%v", name, hits, res.IsError)
		}
	}

	hits, body, _, res := execGuard(t, legacyDecide, map[string]any{"id": dpContactID, "proposalId": dpProposalID, "decision": "not_sure"})
	if hits != 1 || res.IsError || !strings.Contains(body, "not_sure") {
		t.Fatalf("legacy decide: hits=%d isError=%v body=%s", hits, res.IsError, body)
	}
	for name, params := range map[string]map[string]any{
		"bad decision": {"id": dpContactID, "proposalId": dpProposalID, "decision": "maybe"},
		"bad uuid":     {"id": "nope", "proposalId": dpProposalID, "decision": "not_sure"},
		"extra body":   {"id": dpContactID, "proposalId": dpProposalID, "decision": "not_sure", "x": 1},
	} {
		if hits, _, _, res := execGuard(t, legacyDecide, params); hits != 0 || !res.IsError {
			t.Errorf("legacy decide %s: hits=%d isError=%v", name, hits, res.IsError)
		}
	}
}
