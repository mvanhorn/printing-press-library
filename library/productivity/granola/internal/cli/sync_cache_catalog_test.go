// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// PATCH(api-catalog-refresh): the catalog refresh exists for the run that
// cannot read the cache. On a healthy run the decrypted cache is authoritative
// for recipes, panel templates and folders, and calling the API for them would
// spend three round-trips per sync to overwrite fresher local data with the
// same rows -- while claiming API ownership of catalog rows the cache path
// created, which is the one thing ownership markers exist to prevent.
//
// These guards are structural rather than behavioural because the degraded
// branch is only reachable through a real migrated-scheme decrypt failure plus
// a live internal API, neither of which a unit test can stand up honestly.
// What is worth pinning is the wiring, which is exactly what a later edit
// could quietly drop. Same approach as
// TestOpenGranolaCacheCallSites_AllAccountedFor.

const syncCacheFile = "sync_cache.go"

func TestCatalogHydrateOnlyRunsOnTheDegradedPath(t *testing.T) {
	guards := degradedGuards(t, syncCacheFile, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "HydrateCatalogFromAPI"
	})
	if len(guards) != 1 {
		t.Fatalf("%s calls HydrateCatalogFromAPI %d times, want exactly 1", syncCacheFile, len(guards))
	}
	if !strings.Contains(guards[0], "degraded") {
		t.Errorf("the catalog hydrate is guarded by %q, which does not test `degraded`: a healthy cache sync would refresh the catalog over the API and stamp cache-owned rows as API-owned",
			guards[0])
	}
}

// The hydrate is only half the wiring. Without the ownership stamp every row
// this path creates is filed as cache-created, and nothing downstream can ever
// tell the two apart again — the marker is not recoverable after the fact.
func TestDegradedSyncClaimsCatalogOwnership(t *testing.T) {
	guards := degradedGuards(t, syncCacheFile, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 {
			return false
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "CatalogOwner"
	})
	if len(guards) != 1 {
		t.Fatalf("%s assigns SyncOptions.CatalogOwner %d times, want exactly 1", syncCacheFile, len(guards))
	}
	if !strings.Contains(guards[0], "degraded") {
		t.Errorf("CatalogOwner is assigned under %q rather than the degraded branch: a healthy cache sync would claim API ownership of cache-created catalog rows",
			guards[0])
	}
}

// degradedGuards returns, for every node in file that match reports on, the
// conjunction of the `if` conditions it is lexically nested inside.
func degradedGuards(t *testing.T, file string, match func(ast.Node) bool) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var found []string
	var walk func(n ast.Node, enclosing []string)
	walk = func(n ast.Node, enclosing []string) {
		ast.Inspect(n, func(node ast.Node) bool {
			if node == nil {
				return false
			}
			if ifStmt, ok := node.(*ast.IfStmt); ok {
				inner := append(append([]string{}, enclosing...), exprText(fset, ifStmt.Cond))
				walk(ifStmt.Body, inner)
				if ifStmt.Else != nil {
					walk(ifStmt.Else, enclosing)
				}
				return false
			}
			if match(node) {
				found = append(found, strings.Join(enclosing, " && "))
			}
			return true
		})
	}
	walk(f, nil)
	return found
}

// exprText renders an expression back to source text for the assertion message.
func exprText(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, e); err != nil {
		return ""
	}
	return b.String()
}
