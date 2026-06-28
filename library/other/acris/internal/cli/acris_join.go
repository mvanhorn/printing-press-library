// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the cross-dataset ACRIS novel commands
// (bbl, debt, document, party-search). ACRIS publishes Master, Legals, Parties,
// and Document Control Codes as four separate NYC Open Data (Socrata) datasets;
// these helpers chain them by document_id the way a title search needs.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/acris/internal/client"
)

// ACRIS dataset paths on the NYC Open Data Socrata endpoint.
const (
	acrisMasterPath   = "/bnx9-e6tj.json" // Real Property Master (one row per recorded document)
	acrisLegalsPath   = "/8h5j-fqxa.json" // Real Property Legals (document <-> BBL)
	acrisPartiesPath  = "/636b-3b5g.json" // Real Property Parties (names on documents)
	acrisDocCodesPath = "/7isb-wh4c.json" // Document Control Codes (doc_type lookup)

	// acrisMaxInClause bounds how many document IDs we fan out in a single SoQL
	// in(...) filter, keeping the request URL within Socrata's limits.
	acrisMaxInClause = 200
)

// boroughNames maps ACRIS borough codes to their human names.
var boroughNames = map[string]string{
	"1": "Manhattan",
	"2": "Bronx",
	"3": "Brooklyn",
	"4": "Queens",
	"5": "Staten Island",
}

// soqlQuote escapes a value for inclusion in a single-quoted SoQL string literal
// by doubling embedded single quotes, the SoQL/SQL escaping convention.
func soqlQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// soqlInClause builds a SoQL `field in('a','b',...)` predicate from values.
func soqlInClause(field string, values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, "'"+soqlQuote(v)+"'")
	}
	return field + " in(" + strings.Join(quoted, ",") + ")"
}

// fetchACRISRows runs a GET against an ACRIS dataset and decodes the JSON array
// of records. Socrata always returns a top-level JSON array of objects.
func fetchACRISRows(ctx context.Context, c *client.Client, path string, params map[string]string) ([]map[string]any, error) {
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", path, err)
	}
	return rows, nil
}

// strField returns the string form of a record field. Socrata serializes every
// value as a JSON string, but this guards against numbers and nulls too.
func strField(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// distinctDocumentIDs collects unique, non-empty document_id values from rows,
// preserving first-seen order and capping at the given limit (<=0 means no cap).
func distinctDocumentIDs(rows []map[string]any, limit int) []string {
	seen := make(map[string]struct{}, len(rows))
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		id := strField(r, "document_id")
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		if limit > 0 && len(ids) >= limit {
			break
		}
	}
	return ids
}

// fetchMastersByID fetches Master records for the given document IDs, chunking
// the in(...) clause to stay within URL limits. Returns a map keyed by
// document_id for joining.
func fetchMastersByID(ctx context.Context, c *client.Client, ids []string) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(ids))
	for start := 0; start < len(ids); start += acrisMaxInClause {
		end := start + acrisMaxInClause
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		rows, err := fetchACRISRows(ctx, c, acrisMasterPath, map[string]string{
			"$where": soqlInClause("document_id", chunk),
			"$limit": strconv.Itoa(len(chunk)),
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if id := strField(r, "document_id"); id != "" {
				out[id] = r
			}
		}
	}
	return out, nil
}

// mortgageClassCodes is the curated set of ACRIS Master doc_type codes in the
// "MORTGAGES & INSTRUMENTS" class, mapped to a human description. The companion
// Document Control Codes dataset (7isb-wh4c) exposes the code→class map in its
// schema but serves the code column as null through the SODA resource API, so a
// live lookup is not reliable; this curated map (every code verified present in
// the Master dataset) keeps the `debt` command deterministic and offline-safe.
var mortgageClassCodes = map[string]string{
	"MTGE":  "Mortgage",
	"M&CON": "Mortgage & Consolidation (CEMA)",
	"SMTG":  "Spreader / Supplemental Mortgage",
	"ASPM":  "Assignment of Mortgage",
	"ASST":  "Assignment of Mortgage",
	"SAT":   "Satisfaction of Mortgage",
	"WSAT":  "Satisfaction of Mortgage",
	"PREL":  "Partial Release of Mortgage",
	"SUBM":  "Subordination of Mortgage",
	"AALR":  "Assignment of Leases & Rents",
	"AL&R":  "Assignment of Leases & Rents",
}

// originatingMortgageCodes are the doc types that record new mortgage principal.
// Only these are summed into a BBL's recorded-mortgage total; assignments,
// satisfactions, releases, and subordinations are part of the history but would
// double-count or misstate the principal if summed.
var originatingMortgageCodes = map[string]bool{
	"MTGE":  true,
	"M&CON": true,
	"SMTG":  true,
}
