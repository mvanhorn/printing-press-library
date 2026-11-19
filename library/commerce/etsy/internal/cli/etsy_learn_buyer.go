package cli

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"strings"
)

func init() {
	learn.EtsyDropResourceEntities = func(t string) bool {
		return cliutil.IsMCPChildEnv() && !etsyMCPBuyerPIIOptIn() && etsyBuyerBearingType(t)
	}
	learn.EtsyCandidateFilter = func(ctx context.Context, db *sql.DB) func(string) bool {
		if !etsyHideBuyerLearningsActive() {
			return nil
		}
		idx, err := etsyLoadBuyerIndex(ctx, db)
		if err != nil {
			return func(string) bool { return true } // fail closed: hide all
		}
		return idx.familyBuyerBearing
	}
}

func etsyRefuseBuyerTeach(resourceType string) error {
	if etsyBuyerBearingType(resourceType) {
		return etsyRefuseBuyerDataOverMCP("teaching a " + strings.TrimSpace(resourceType) + " learning")
	}
	return nil
}

func etsyHideBuyerLearningsActive() bool {
	return cliutil.IsMCPChildEnv() && !etsyMCPBuyerPIIOptIn()
}

func etsyBuyerHiddenNote(n int, noun string) string {
	return fmt.Sprintf("%d buyer %s hidden; set ETSY_MCP_ALLOW_BUYER_PII=1", n, noun)
}

// etsyHiddenEnvelope keeps the output shape independent of the data: in MCP restricted mode it
// is always the object form (buyer_hidden may be 0, the note appears only when something was
// hidden); outside restricted mode the rows stay a bare array.
func etsyHiddenEnvelope(key string, rows any, hidden int, noun string) any {
	if !etsyHideBuyerLearningsActive() && hidden == 0 {
		return rows
	}
	out := map[string]any{key: rows, "buyer_hidden": hidden}
	if hidden > 0 {
		out["note"] = etsyBuyerHiddenNote(hidden, noun)
	}
	return out
}

func etsyLearningsFetchLimit(limit int) int {
	if etsyHideBuyerLearningsActive() {
		return 100000
	}
	return limit
}

type etsyBuyerIndex struct {
	tokens   map[string]struct{}
	buyerIDs map[string]struct{}
	otherIDs map[string]struct{}
}

func etsyLoadBuyerIndex(ctx context.Context, db *sql.DB) (*etsyBuyerIndex, error) {
	return etsyLoadBuyerIndexForErasure(ctx, db, false)
}

func etsyLoadBuyerIndexForErasure(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, force bool) (*etsyBuyerIndex, error) {
	idx := &etsyBuyerIndex{tokens: map[string]struct{}{}, buyerIDs: map[string]struct{}{}, otherIDs: map[string]struct{}{}}
	if !force && !etsyHideBuyerLearningsActive() {
		return idx, nil
	}
	if force {
		rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='search_learnings'`)
		if err != nil {
			return nil, err
		}
		exists := rows.Next()
		scanErr := rows.Err()
		rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		if !exists {
			return idx, nil
		}
	}
	rows, err := db.QueryContext(ctx, "SELECT query_pattern, COALESCE(resource_type,''), resource_id FROM search_learnings")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return idx, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var etsyQueryPattern, etsyResourceType, etsyResourceID string
		if err := rows.Scan(&etsyQueryPattern, &etsyResourceType, &etsyResourceID); err != nil {
			return nil, err
		}
		if etsyBuyerBearingType(etsyResourceType) {
			idx.buyerIDs[etsyResourceID] = struct{}{}
			for _, token := range strings.Fields(etsyQueryPattern) {
				idx.tokens[token] = struct{}{}
			}
		} else {
			idx.otherIDs[etsyResourceID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return idx, nil
}

func (i *etsyBuyerIndex) familyBuyerBearing(family string) bool {
	for _, token := range strings.Fields(family) {
		if _, ok := i.tokens[token]; ok {
			return true
		}
	}
	return false
}

func etsyDropBuyerLearnings(rows []learningRow, limit int) ([]learningRow, int) {
	kept := make([]learningRow, 0, len(rows))
	hidden := 0
	for _, row := range rows {
		if etsyBuyerBearingType(row.ResourceType) {
			hidden++
			continue
		}
		kept = append(kept, row)
	}
	if limit > 0 && len(kept) > limit {
		kept = kept[:limit]
	}
	return kept, hidden
}

func etsyDropBuyerCandidates(idx *etsyBuyerIndex, rows []candidateRow, limit int) ([]candidateRow, int) {
	kept := make([]candidateRow, 0, len(rows))
	hidden := 0
	for _, row := range rows {
		if idx.familyBuyerBearing(row.QueryFamily) {
			hidden++
			continue
		}
		kept = append(kept, row)
	}
	if limit > 0 && len(kept) > limit {
		kept = kept[:limit]
	}
	return kept, hidden
}

func etsyDropBuyerPlaybooks(idx *etsyBuyerIndex, rows []store.PlaybookRow) ([]store.PlaybookRow, int) {
	kept := make([]store.PlaybookRow, 0, len(rows))
	hidden := 0
	for _, row := range rows {
		if idx.familyBuyerBearing(row.QueryFamily) {
			hidden++
			continue
		}
		kept = append(kept, row)
	}
	return kept, hidden
}

func etsyDropBuyerWarnings(idx *etsyBuyerIndex, entries []learn.TeachLogEntry) ([]learn.TeachLogEntry, int) {
	kept := make([]learn.TeachLogEntry, 0, len(entries))
	hidden := 0
	for _, entry := range entries {
		_, other := idx.otherIDs[entry.Resource]
		_, buyer := idx.buyerIDs[entry.Resource]
		if !other || buyer {
			hidden++
			continue
		}
		kept = append(kept, entry)
	}
	return kept, hidden
}

func etsyRefuseBuyerCandidate(ctx context.Context, s *store.Store, family, what string) error {
	if !etsyHideBuyerLearningsActive() {
		return nil
	}
	idx, err := etsyLoadBuyerIndex(ctx, s.DB())
	if err != nil {
		return err
	}
	if idx.familyBuyerBearing(family) {
		return etsyRefuseBuyerDataOverMCP(what)
	}
	return nil
}
