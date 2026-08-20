package pricebook

import (
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// FindResult is one ranked part-finder hit, shaped for a field tech: the
// fields needed to pick a part, plus the relevance score and which field
// matched best.
type FindResult struct {
	Kind        SKUKind `json:"kind"`
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	DisplayName string  `json:"display_name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Cost        float64 `json:"cost"`
	VendorPart  string  `json:"vendor_part"`
	VendorName  string  `json:"vendor_name"`
	Active      bool    `json:"active"`
	Score       float64 `json:"score"`
	MatchedOn   string  `json:"matched_on"` // "code" | "name" | "description" | "vendor-part"
}

// Find runs a forgiving ranked search over every synced material, equipment,
// and service for a natural-language query — "describe the part, I don't
// know the code". Each SKU is scored against the query on code, display
// name, description, and primary vendor part; the best field wins and names
// MatchedOn. Only SKUs scoring at or above minScore are returned, so a
// nonsense query yields an empty result rather than weak junk; pass
// minScore <= 0 to keep every positive-scoring hit. Results are sorted by
// score descending and capped at limit (default 15). This is domain-tuned
// beyond the framework `search`: it ranks across SKU-specific fields and
// returns the tech-facing fields needed to actually pick a part.
func Find(db *store.Store, query string, minScore float64, limit int) ([]FindResult, error) {
	if limit <= 0 {
		limit = 15
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	var results []FindResult

	score := func(kind SKUKind, id int64, code, name, desc string, price, cost float64, active bool, pv *SkuVendor) {
		vendorPart, vendorName := "", ""
		if pv != nil {
			vendorPart, vendorName = pv.VendorPart, pv.VendorName
		}
		best, matched := 0.0, ""
		consider := func(field, value string) {
			if value == "" {
				return
			}
			// Take the best of: symmetric Similarity (typo/reorder tolerance)
			// and asymmetric TokenCoverage (a short query whose words all
			// appear in a longer SKU name — the common "describe the part"
			// case). A normalized-substring hit is also a strong signal.
			s := Similarity(q, value)
			if cov := TokenCoverage(q, value); cov > s {
				s = cov
			}
			if nq, nv := Normalize(q), Normalize(value); nq != "" && strings.Contains(nv, nq) && s < 0.85 {
				s = 0.85
			}
			if s > best {
				best, matched = s, field
			}
		}
		consider("code", code)
		consider("name", name)
		consider("description", desc)
		consider("vendor-part", vendorPart)
		if best <= 0 || best < minScore {
			return
		}
		results = append(results, FindResult{
			Kind: kind, ID: id, Code: code, DisplayName: name, Description: desc,
			Price: price, Cost: cost, VendorPart: vendorPart, VendorName: vendorName,
			Active: active, Score: round2(best), MatchedOn: matched,
		})
	}

	mats, err := LoadMaterials(db)
	if err != nil {
		return nil, err
	}
	for _, m := range mats {
		score(KindMaterial, m.ID, m.Code, m.DisplayName, m.Description, m.Price, m.Cost, m.Active, m.PrimaryVendor)
	}
	eqs, err := LoadEquipment(db)
	if err != nil {
		return nil, err
	}
	for _, e := range eqs {
		score(KindEquipment, e.ID, e.Code, e.DisplayName, e.Description, e.Price, e.Cost, e.Active, e.PrimaryVendor)
	}
	svcs, err := LoadServices(db)
	if err != nil {
		return nil, err
	}
	for _, s := range svcs {
		score(KindService, s.ID, s.Code, s.DisplayName, s.Description, s.Price, 0, s.Active, nil)
	}

	// Highest score first; active SKUs win ties (a tech wants a part they can
	// actually use); then code for determinism.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Active != results[j].Active {
			return results[i].Active
		}
		return results[i].Code < results[j].Code
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
