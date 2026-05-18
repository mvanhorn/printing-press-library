package pricebook

import (
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// dupCandidate is the minimal projection of a SKU used for duplicate
// detection — kept tiny so the O(n^2) pairwise scan stays cheap on a
// few-thousand-SKU pricebook.
type dupCandidate struct {
	kind       SKUKind
	id         int64
	code       string
	name       string
	vendorPart string
}

// DuplicateMember is one SKU inside a duplicate cluster.
type DuplicateMember struct {
	Kind        SKUKind `json:"kind"`
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	DisplayName string  `json:"display_name"`
	VendorPart  string  `json:"vendor_part"`
}

// DuplicateCluster is a group of SKUs that look like the same part. Score is
// the lowest pairwise similarity inside the cluster — the conservative
// "these are at least this similar" floor.
type DuplicateCluster struct {
	Score   float64           `json:"score"`
	Members []DuplicateMember `json:"members"`
}

// Dedupe clusters near-duplicate materials and equipment so excess pricebook
// growth can be collapsed. Two SKUs are linked when their pairwise
// similarity is >= minScore; similarity is the max of name similarity, code
// similarity, and an exact vendor-part match (which alone is a strong
// signal). Clusters form by transitive union-find. kind filters to one SKU
// type; empty kind dedupes materials and equipment together (the same part
// can be miscategorised across both). The ServiceTitan API has no
// "find SKUs like this" query — this is a pure local scan.
func Dedupe(db *store.Store, kind SKUKind, minScore float64) ([]DuplicateCluster, error) {
	if minScore <= 0 {
		minScore = 0.85
	}
	var cands []dupCandidate
	if kind == "" || kind == KindMaterial {
		mats, err := LoadMaterials(db)
		if err != nil {
			return nil, err
		}
		for _, m := range mats {
			if !m.Active {
				continue
			}
			vp := ""
			if m.PrimaryVendor != nil {
				vp = m.PrimaryVendor.VendorPart
			}
			cands = append(cands, dupCandidate{KindMaterial, m.ID, m.Code, m.DisplayName, vp})
		}
	}
	if kind == "" || kind == KindEquipment {
		eqs, err := LoadEquipment(db)
		if err != nil {
			return nil, err
		}
		for _, e := range eqs {
			if !e.Active {
				continue
			}
			vp := ""
			if e.PrimaryVendor != nil {
				vp = e.PrimaryVendor.VendorPart
			}
			cands = append(cands, dupCandidate{KindEquipment, e.ID, e.Code, e.DisplayName, vp})
		}
	}

	n := len(cands)
	uf := newUnionFind(n)
	// pairScore is symmetric; cache the score that linked each pair so the
	// cluster floor can be computed without re-scoring.
	type pair struct{ a, b int }
	linkScore := make(map[pair]float64)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			s := pairSimilarity(cands[i], cands[j])
			if s >= minScore {
				uf.union(i, j)
				linkScore[pair{i, j}] = s
			}
		}
	}

	// Gather members per root.
	groups := make(map[int][]int)
	for i := 0; i < n; i++ {
		r := uf.find(i)
		groups[r] = append(groups[r], i)
	}

	var clusters []DuplicateCluster
	for _, idxs := range groups {
		if len(idxs) < 2 {
			continue
		}
		// Cluster floor: lowest linking score among member pairs that were
		// actually linked. Pairs not directly linked (transitive) don't
		// lower the floor below minScore by construction.
		floor := 1.0
		for a := 0; a < len(idxs); a++ {
			for b := a + 1; b < len(idxs); b++ {
				i, j := idxs[a], idxs[b]
				if i > j {
					i, j = j, i
				}
				if s, ok := linkScore[pair{i, j}]; ok && s < floor {
					floor = s
				}
			}
		}
		if floor == 1.0 {
			floor = minScore // all links were transitive
		}
		var members []DuplicateMember
		for _, idx := range idxs {
			c := cands[idx]
			members = append(members, DuplicateMember{
				Kind: c.kind, ID: c.id, Code: c.code, DisplayName: c.name, VendorPart: c.vendorPart,
			})
		}
		sort.SliceStable(members, func(a, b int) bool {
			if members[a].Kind != members[b].Kind {
				return members[a].Kind < members[b].Kind
			}
			return members[a].Code < members[b].Code
		})
		clusters = append(clusters, DuplicateCluster{Score: round2(floor), Members: members})
	}
	// Most-similar clusters first.
	sort.SliceStable(clusters, func(a, b int) bool {
		return clusters[a].Score > clusters[b].Score
	})
	return clusters, nil
}

// pairSimilarity scores two SKU candidates: an exact vendor-part match is a
// strong duplicate signal on its own; otherwise take the better of name and
// code similarity.
func pairSimilarity(a, b dupCandidate) float64 {
	if PartMatch(a.vendorPart, b.vendorPart) {
		return 1.0
	}
	nameScore := Similarity(a.name, b.name)
	codeScore := Similarity(a.code, b.code)
	if codeScore > nameScore {
		return codeScore
	}
	return nameScore
}

// ----- union-find -------------------------------------------------------

type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	uf := &unionFind{parent: make([]int, n), rank: make([]int, n)}
	for i := range uf.parent {
		uf.parent[i] = i
	}
	return uf
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]] // path halving
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	if u.rank[ra] < u.rank[rb] {
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	if u.rank[ra] == u.rank[rb] {
		u.rank[ra]++
	}
}
