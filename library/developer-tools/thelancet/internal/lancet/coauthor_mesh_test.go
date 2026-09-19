package lancet

import (
	"context"
	"database/sql"
	"testing"
)

// CoAuthorMesh collects the institution's authors into a TEMP table on an
// explicit *sql.Conn. That is a performance change with two failure modes of
// its own, and the existing TestCoAuthorMesh — one institution, two authors,
// one pair, one call — cannot see either of them:
//
//   - the table is scoped to one connection, so if the query ever runs on a
//     different pooled connection it finds no table at all;
//   - the connection returns to the pool afterwards, so a table left behind
//     meets the NEXT call's CREATE and fails it.
//
// Neither is visible in a single call. The tests here drive repeated calls and
// a wider fixture so that the ranking, the pair direction and the limit are
// pinned too: all three were free to change silently before.

// meshWork is a compact description of one paper for meshTestDB.
type meshWork struct {
	id      string
	authors []meshAuthor
}

// meshAuthor is one author on a paper, with the institution recorded for that
// paper. Affiliation is per work in this schema, not per person.
type meshAuthor struct {
	id   string
	name string
	inst string
}

// meshTestDB seeds an in-memory store from a compact description.
//
// It does NOT call SetMaxOpenConns. That omission is the point: ensureLancetStore
// pins the pool to one connection, but CoAuthorMesh is handed a *sql.DB and
// cannot see how the caller configured it. Testing without the pin is what makes
// the connection handling the function's own responsibility rather than the
// caller's.
func meshTestDB(t *testing.T, works []meshWork) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
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

// meshFixture: three Oxford authors and one outsider across four papers.
//
//	W1: Alice, Bob, Carol (Oxford)        -> A-B, A-C, B-C
//	W2: Alice, Bob (Oxford)               -> A-B  (so A-B totals 2)
//	W3: Alice (Oxford) + Dave (Cambridge) -> no pair: Dave is not Oxford
//	W4: Dave alone (Cambridge)            -> nothing
//
// Expected for "Oxford": A-B with 2 shared works, then A-C and B-C with 1 each.
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

// TestCoAuthorMeshRanksAndScopes pins what the query returns, not just how many
// rows come back.
//
// Three separate rules are checked at once because they are free to break
// independently: the ORDER BY (A-B must lead on 2 shared works), the pair
// direction (each unordered pair appears exactly once, never as both A-B and
// B-A), and the institution scope (Dave shares W3 with Alice but is not at
// Oxford, so that pair must not appear).
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
	if edges[0].SharedWorks != 2 {
		t.Errorf("top pair has %d shared works, want 2 — ORDER BY shared DESC is not holding: %+v",
			edges[0].SharedWorks, edges)
	}
	if !pairIs(edges[0], "Alice", "Bob") {
		t.Errorf("top pair = %s/%s, want Alice and Bob", edges[0].AuthorA, edges[0].AuthorB)
	}
	for _, e := range edges {
		if e.AuthorA == "Dave" || e.AuthorB == "Dave" {
			t.Errorf("Dave is at Cambridge and must not appear in an Oxford mesh: %+v", e)
		}
		if e.AuthorA == e.AuthorB {
			t.Errorf("an author is paired with themselves: %+v", e)
		}
	}
	seen := map[string]bool{}
	for _, e := range edges {
		k := e.AuthorA + "|" + e.AuthorB
		rev := e.AuthorB + "|" + e.AuthorA
		if seen[k] || seen[rev] {
			t.Errorf("pair %s/%s appears twice; the a2>a1 guard is not deduplicating", e.AuthorA, e.AuthorB)
		}
		seen[k] = true
	}
}

// TestCoAuthorMeshRespectsLimit. The existing test passes limit 10 against a
// single pair, so the LIMIT clause could be dropped entirely and nothing would
// notice.
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
	// The limit must cut the tail, not the head: the highest-ranked pair has to
	// survive it.
	if edges[0].SharedWorks != 2 {
		t.Errorf("top pair has %d shared works, want 2; the limit is cutting the wrong end", edges[0].SharedWorks)
	}
}

// TestCoAuthorMeshRepeatedCallsOnOnePool is the one that guards the temp table.
//
// Three calls in a row on the same *sql.DB, with no SetMaxOpenConns pin. If the
// table is not dropped, the second call's CREATE fails with "table already
// exists". If it is dropped but the query runs on a different connection than
// the CREATE, the query fails with "no such table". Either way the failure is
// in the second or third call, never the first — which is why one call proves
// nothing here.
func TestCoAuthorMeshRepeatedCallsOnOnePool(t *testing.T) {
	db := meshTestDB(t, meshFixture())
	defer db.Close()

	var first []CoAuthorEdge
	for i := 1; i <= 3; i++ {
		edges, err := CoAuthorMesh(context.Background(), db, "Oxford", 10)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if i == 1 {
			first = edges
			continue
		}
		if len(edges) != len(first) {
			t.Fatalf("call %d returned %d pairs, call 1 returned %d: the temp table is leaking between calls",
				i, len(edges), len(first))
		}
		for j := range edges {
			if edges[j] != first[j] {
				t.Errorf("call %d row %d = %+v, call 1 had %+v", i, j, edges[j], first[j])
			}
		}
	}
}

// TestCoAuthorMeshDifferentInstitutionsInSequence is the other half of the same
// guard. Repeating one institution could pass on a stale table that happens to
// hold the right authors; switching institutions cannot. The second call must
// see Cambridge's author set, not Oxford's left behind.
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
	if len(edges) != 1 {
		t.Fatalf("got %d Cambridge pairs, want 1 (Dave-Erin): %+v", len(edges), edges)
	}
	if !pairIs(edges[0], "Dave", "Erin") {
		t.Errorf("Cambridge pair = %s/%s, want Dave and Erin; the previous call's author set may have survived",
			edges[0].AuthorA, edges[0].AuthorB)
	}
}

// TestCoAuthorMeshEmptyInstitution keeps the guard clause honest: an unknown
// institution is an empty result, not an error and not everyone.
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
