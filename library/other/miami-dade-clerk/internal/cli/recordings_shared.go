// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

// Helpers shared across the 7 transcendence commands (lien-chain,
// surviving-liens, chain-of-title, owner-portfolio, case-arc, enrich,
// ftl-scan). Keeps the per-command files focused on their domain logic.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

const cliName = "miami-dade-clerk-pp-cli"

// docTypeLabels maps the 3-letter linK_DOCTYPE codes to human-readable
// labels for output. Only the codes our commands key on are listed; the
// fallback in docTypeLabel returns the raw code so unknown types still
// render readably.
var docTypeLabels = map[string]string{
	"DEE": "Deed",
	"QCD": "Quit Claim Deed",
	"ODE": "Owner-Drafted Deed",
	"DAM": "Deed Amendment",
	"DM":  "Deed Memo",
	"MOR": "Mortgage",
	"AMO": "Assignment of Mortgage",
	"SMO": "Subordination of Mortgage",
	"SAT": "Satisfaction of Mortgage",
	"REL": "Release of Lien",
	"LIS": "Lis Pendens",
	"CLP": "Cancellation of Lis Pendens",
	"JUD": "Judgment",
	"SJU": "Satisfaction of Judgment",
	"FTL": "Federal Tax Lien",
	"LIE": "State Lien",
	"ASG": "Assignment",
	"CTI": "Certificate of Title",
	"DIS": "Dismissal",
}

// deedDocTypes is the set of linK_DOCTYPE codes considered to be deeds
// (i.e. transfers of property ownership). Property/Condo searches at the
// portal return only these; chain-of-title walks rely on this set.
var deedDocTypes = []string{"DEE", "QCD", "ODE", "DAM", "DM"}

// docTypeLabel returns a human-readable name for a 3-letter linK_DOCTYPE
// code. Returns the input unchanged for unknown codes so output stays
// debuggable on schema drift.
func docTypeLabel(code string) string {
	if v, ok := docTypeLabels[code]; ok {
		return v
	}
	return code
}

// NormalizeFolio strips dashes from a clerk-style folio
// (30-2232-066-1610 → 3022320661610) and parses it to int64. Returns 0
// on malformed input rather than an error so callers can decide whether
// to fail loudly or treat the folio as absent.
func NormalizeFolio(s string) int64 {
	clean := strings.ReplaceAll(s, "-", "")
	clean = strings.TrimSpace(clean)
	var n int64
	_, err := fmt.Sscanf(clean, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}

// viewerURL builds the public viewer URL for a recording given its
// per-record qs token. The clerk portal exposes individual records at
// /standardsearchResults/ViewDoc?qs=<token>. Documented here so the URL
// shape is centralized — if the portal changes the path, one edit
// updates every command's output.
func viewerURL(qs string) string {
	if qs == "" {
		return ""
	}
	return "https://onlineservices.miamidadeclerk.gov/officialrecords/StandardSearchResults/ViewDoc?qs=" + qs
}

// openStoreOrFail opens the local SQLite store and returns nil-store
// when no DB exists yet (so callers can produce a structured error
// rather than panicking). The transcendence commands all need local
// data — they don't currently fall through to live API for reads.
func openStoreOrFail(ctx context.Context) (*store.Store, error) {
	s, err := openStoreForRead(ctx, cliName)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if s == nil {
		return nil, fmt.Errorf("no local data — run `%s sync recordings` first or use `--data-source live`", cliName)
	}
	return s, nil
}
