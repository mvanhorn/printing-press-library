package lancet

import (
	"context"
	"database/sql"
	"testing"
)

// CoAuthorMesh holds the institution's author set in a TEMP table on an
// explicit *sql.Conn. The existing TestCoAuthorMesh — one pair, one call —
// cannot see either failure mode of that: a query landing on a different
// connection than its CREATE, or a table left behind for the next call.

type meshWork struct {
	id      string
	authors []meshAuthor
}

// meshAuthor is one author on a paper. Affiliation is per work in this schema,
// not per person.
type meshAuthor struct {
	id   string
	name string
	inst string
}

// meshTestDB seeds a store whose pool is forced to CHURN connections.
//
// SetMaxIdleConns(0) parks nothing for reuse, so each acquisition opens a fresh
// connection. This is the point: an unconfigured pool always hands back the same
// idle connection, and a test built on it would pass even if CoAuthorMesh
// abandoned connection affinity entirely — the TEMP table would happen to still
// be there.
//
// It does NOT pin the pool with SetMaxOpenConns(1). One caller does that, but
// CoAuthorMesh receives a *sql.DB and cannot see it, so holding a connection has
// to be the function's own job.
//
// The DSN is shared-cache rather than plain ":memory:", which gives every new
// connection its own empty database; the keep-alive connection stops the shared
// one from being destroyed when the pool briefly drops to zero.
func meshTestDB(t *testing.T, works []meshWork) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:meshtest?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxIdleConns(0)
	keepAlive, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("keepalive: %v", err)
	}
	t.Cleanup(func() { _ = keepAlive.Close() })

	ctx := context.Background()
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	var decoded []decodedWork
	for _, w := range works {
		dw := decodedWork{ID: w.id, Title: "Work " + w.id, Year: 2024, Date: "2024-01-01"}
		for _, a := range w.authors {
			dw.Authors = append(dw.Authors, decodedAuthor{
				ID:           a.id,
				Name:         a.name,
				Institutions: []decodedInstitution{{ID: "I-" + a.inst, Name: a.inst}},
			})
		}
		decoded = append(decoded, dw)
	}
	if _, err := StoreWorks(ctx, db, decoded, "0140-6736", "The Lancet"); err != nil {
		t.Fatalf("store: %v", err)
	}
	return db
}

// meshFixture: three Oxford authors and one outsider.
//
//	W1: Alice, Bob, Carol (Oxford)        -> A-B, A-C, B-C
//	W2: Alice, Bob (Oxford)               -> A-B, so A-B totals 2
//	W3: Alice (Oxford) + Dave (Cambridge) -> no pair: Dave is not Oxford
//	W4: Dave alone (Cambridge)            -> nothing
func meshFixture() []meshWork {
	return []meshWork{
		{id: "W1", authors: []meshAuthor{
			{"A1", "Alice", "Oxford"}, {"A2", "Bob", "Oxford"}, {"A3", "Carol", "Oxford"},
		}},
		{id: "W2", authors: []meshAuthor{
			{"A1", "Alice", "Oxford"}, {"A2", "Bob", "Oxford"},
		}},
		{id: "W3", authors: []meshAuthor{
			{"A1", "Alice", "Oxford"}, {"A4", "Dave", "Cambridge"},
		}},
		{id: "W4", authors: []meshAuthor{
			{"A4", "Dave", "Cambridge"},
		}},
	}
}

// TestCoAuthorMeshRanksAndScopes pins three rules that can break independently:
// the ranking, the pair direction (no A-B plus B-A), and the institution scope
// (Dave shares W3 with Alice but is not at Oxford).
func TestCoAuthorMeshRanksAndScopes(t *testing.T) {
	db := meshTestDB(t, meshFixture())
	defer db.Close()

	edges, err := CoAuthorMesh(context.Background(), db, "Oxford", 10)
	if err != nil {
		t.Fatalf("CoAuthorMesh: %v", err)
	}
	if len(edges) != 3 {
		t.Fatalf("got %d pairs, want 3 (Alice-Bob, Alice-Carol, Bob-Carol): %+v", len(edges), edges)
	}
	if edges[0].SharedWorks != 2 || !pairIs(edges[0], "Alice", "Bob") {
		t.Errorf("top pair = %+v, want Alice-Bob with 2 shared works: %+v", edges[0], edges)
	}
	seen := map[string]bool{}
	for _, e := range edges {
		if e.AuthorA == "Dave" || e.AuthorB == "Dave" {
			t.Errorf("Dave is at Cambridge and must not appear: %+v", e)
		}
		if e.AuthorA == e.AuthorB {
			t.Errorf("author paired with themselves: %+v", e)
		}
		if seen[e.AuthorA+"|"+e.AuthorB] || seen[e.AuthorB+"|"+e.AuthorA] {
			t.Errorf("pair %s/%s appears twice; the a2>a1 guard is not deduplicating", e.AuthorA, e.AuthorB)
		}
		seen[e.AuthorA+"|"+e.AuthorB] = true
	}
}

// TestCoAuthorMeshRespectsLimit. The existing test passes limit 10 against one
// pair, so LIMIT could be dropped entirely and nothing would notice. The limit
// must also cut the tail, not the head.
func TestCoAuthorMeshRespectsLimit(t *testing.T) {
	db := meshTestDB(t, meshFixture())
	defer db.Close()

	edges, err := CoAuthorMesh(context.Background(), db, "Oxford", 2)
	if err != nil {
		t.Fatalf("CoAuthorMesh: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d pairs with limit 2, want 2: %+v", len(edges), edges)
	}
	if edges[0].SharedWorks != 2 {
		t.Errorf("top pair has %d shared works, want 2; the limit is cutting the wrong end", edges[0].SharedWorks)
	}
}

// TestCoAuthorMeshRepeatedCallsOnAChurningPool is the connection-affinity guard,
// and it is only meaningful because meshTestDB forbids idle connections. An
// unrelated query is interleaved so the pool genuinely cycles rather than
// settling on one connection by luck.
func TestCoAuthorMeshRepeatedCallsOnAChurningPool(t *testing.T) {
	db := meshTestDB(t, meshFixture())
	defer db.Close()
	ctx := context.Background()

	var first []CoAuthorEdge
	for i := 1; i <= 4; i++ {
		if _, err := RankAuthors(ctx, db, "", "", 5); err != nil {
			t.Fatalf("call %d: RankAuthors: %v", i, err)
		}
		edges, err := CoAuthorMesh(ctx, db, "Oxford", 10)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if i == 1 {
			first = edges
			continue
		}
		if len(edges) != len(first) {
			t.Fatalf("call %d returned %d pairs, call 1 returned %d: the temp table is not surviving connection churn",
				i, len(edges), len(first))
		}
		for j := range edges {
			if edges[j] != first[j] {
				t.Errorf("call %d row %d = %+v, call 1 had %+v", i, j, edges[j], first[j])
			}
		}
	}
}

// TestCoAuthorMeshDifferentInstitutionsInSequence. Repeating one institution
// could pass on a stale table holding the right authors by chance; switching
// cannot. The second call must see Cambridge's set, not Oxford's left behind.
func TestCoAuthorMeshDifferentInstitutionsInSequence(t *testing.T) {
	works := append(meshFixture(), meshWork{id: "W5", authors: []meshAuthor{
		{"A4", "Dave", "Cambridge"}, {"A5", "Erin", "Cambridge"},
	}})
	db := meshTestDB(t, works)
	defer db.Close()
	ctx := context.Background()

	if _, err := CoAuthorMesh(ctx, db, "Oxford", 10); err != nil {
		t.Fatalf("Oxford: %v", err)
	}
	edges, err := CoAuthorMesh(ctx, db, "Cambridge", 10)
	if err != nil {
		t.Fatalf("Cambridge: %v", err)
	}
	if len(edges) != 1 || !pairIs(edges[0], "Dave", "Erin") {
		t.Fatalf("got %+v for Cambridge, want one Dave-Erin pair", edges)
	}
}

// TestCoAuthorMeshEmptyInstitution: an unknown institution is an empty result,
// not an error and not everyone.
func TestCoAuthorMeshEmptyInstitution(t *testing.T) {
	db := meshTestDB(t, meshFixture())
	defer db.Close()

	edges, err := CoAuthorMesh(context.Background(), db, "Nowhere University", 10)
	if err != nil {
		t.Fatalf("CoAuthorMesh: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("got %d pairs for an unknown institution, want 0: %+v", len(edges), edges)
	}
}

// pairIs reports whether the edge joins these two names, in either order.
func pairIs(e CoAuthorEdge, x, y string) bool {
	return (e.AuthorA == x && e.AuthorB == y) || (e.AuthorA == y && e.AuthorB == x)
}
