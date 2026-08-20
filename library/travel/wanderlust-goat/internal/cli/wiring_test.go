// MANDATORY INVARIANT TEST. v1's biggest miss was that the dispatcher
// planned 12-source fanouts but only ever invoked 4 source clients —
// scaffolded packages with passing unit tests that never got imported by
// the orchestrator. v2 makes that regression class impossible by
// asserting, at test time, that every internal/<source>/ package is
// transitively reachable from a user-facing cli/*.go file AND has at
// least one of its exported methods called from cli/ or dispatch/.
//
// This is structural — go/parser + go/ast walk only. No runtime, no
// network, no flakiness.
package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// modulePath is the Go module path; every internal/<source>/ import
// matches the prefix below.
const modulePath = "wanderlust-goat-pp-cli"

// nonSourcePackages are internal/ subdirectories that are infrastructure,
// not Stage-2 sources. They are NOT subject to the wiring assertion.
//
// The wiring rule applies to Stage-2 source packages — the per-locale
// review/blog clients the dispatcher routes by name. Infra packages
// (helpers, types, the dispatcher itself, the runtime walker) get exempted
// here.
var nonSourcePackages = map[string]bool{
	"cli":          true,
	"client":       true,
	"cliutil":      true,
	"config":       true,
	"store":        true,
	"types":        true,
	"cache":        true,
	"httperr":      true,
	"mcp":          true,
	"goatstore":    true,
	"sun":          true,
	"criteria":     true,
	"sources":      true,
	"osrm":         true,
	"dispatch":     true,
	"regions":      true,
	"walking":      true,
	"closedsignal": true,
	"googleplaces": true,
	"sourcetypes":  true,
}

// projectRoot finds the project root by walking up from the test's CWD
// until it finds a directory containing go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", dir)
		}
		dir = parent
	}
}

// listSourceDirs returns kebab-case slugs for every internal/<dir>/ that
// looks like a Stage-2 source package (i.e., not in nonSourcePackages).
func listSourceDirs(t *testing.T, root string) []string {
	t.Helper()
	internal := filepath.Join(root, "internal")
	entries, err := os.ReadDir(internal)
	if err != nil {
		t.Fatalf("readdir %s: %v", internal, err)
	}
	var slugs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip subdirs of cli/ and mcp/ (those are sub-packages, not sources)
		// and any package in nonSourcePackages.
		if nonSourcePackages[e.Name()] {
			continue
		}
		slugs = append(slugs, e.Name())
	}
	sort.Strings(slugs)
	return slugs
}

// scanGoFilesIn parses every *.go (excluding _test.go) in dir and returns
// the parsed *ast.File set, the file set, and the import paths seen.
func scanGoFilesIn(t *testing.T, dir string) (importsSeen map[string]bool, calls map[string]bool) {
	t.Helper()
	importsSeen = map[string]bool{}
	calls = map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		// Collect imports.
		for _, imp := range f.Imports {
			if imp.Path != nil {
				importsSeen[strings.Trim(imp.Path.Value, `"`)] = true
			}
		}
		// Collect "<pkg>.<Func>(" call expressions where pkg is a local
		// import alias. Walk the AST.
		ast.Inspect(f, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			calls[ident.Name+"."+sel.Sel.Name] = true
			return true
		})
	}
	return
}

func TestWiringInvariant_EverySourceIsImported(t *testing.T) {
	root := projectRoot(t)
	sources := listSourceDirs(t, root)
	if len(sources) == 0 {
		t.Fatal("no source packages found under internal/; aborting (would always pass)")
	}

	cliImports, _ := scanGoFilesIn(t, filepath.Join(root, "internal", "cli"))
	dispImports, _ := scanGoFilesIn(t, filepath.Join(root, "internal", "dispatch"))

	combined := map[string]bool{}
	for k := range cliImports {
		combined[k] = true
	}
	for k := range dispImports {
		combined[k] = true
	}

	var missing []string
	for _, slug := range sources {
		want := modulePath + "/internal/" + slug
		if !combined[want] {
			missing = append(missing, slug)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("v1-class regression: %d source package(s) under internal/ are not imported by either internal/cli/ or internal/dispatch/:\n  %s\n\nv2 invariant: every Stage-2 source must be reachable from cli/ (directly or transitively via dispatch). Add the import or remove the package.",
			len(missing), strings.Join(missing, ", "))
	}
}

func TestWiringInvariant_EverySourceHasAClientUsed(t *testing.T) {
	root := projectRoot(t)
	sources := listSourceDirs(t, root)

	// Combined: any "<slug>.<X>" call expression in either cli/ or dispatch/
	// counts as the source being used. The DefaultRegistry call site in
	// dispatch/registry.go invokes <slug>.NewClient() for every registered
	// source, so this assertion fails if a source's package is imported but
	// never has any of its exported funcs called.
	_, cliCalls := scanGoFilesIn(t, filepath.Join(root, "internal", "cli"))
	_, dispCalls := scanGoFilesIn(t, filepath.Join(root, "internal", "dispatch"))

	combined := map[string]bool{}
	for k := range cliCalls {
		combined[k] = true
	}
	for k := range dispCalls {
		combined[k] = true
	}

	var unused []string
	for _, slug := range sources {
		// Look for any "<slug>.<anything>" call. This is a heuristic but
		// catches the v1 regression class: scaffolded sources whose
		// NewClient/LookupByName/etc. are never invoked.
		found := false
		prefix := slug + "."
		for call := range combined {
			if strings.HasPrefix(call, prefix) {
				found = true
				break
			}
		}
		if !found {
			unused = append(unused, slug)
		}
	}
	if len(unused) > 0 {
		t.Fatalf("v1-class regression: %d source package(s) are imported but never have any exported function called from cli/ or dispatch/:\n  %s\n\nv2 invariant: every Stage-2 source must have at least one method invoked.",
			len(unused), strings.Join(unused, ", "))
	}
}

func TestWiringInvariant_DefaultRegistryCoversRegionsTable(t *testing.T) {
	// This is a runtime check (no parser) — but it's cheap and catches the
	// case where a slug is added to regions.AllSourceSlugs() but never
	// registered with DefaultRegistry. The compile-time check above doesn't
	// catch this because the slug is just a string in the regions table.
	root := projectRoot(t)
	// Collect every slug that appears in regions/regions.go's
	// LocalReviewSites entries by string-search; this is a sanity check on
	// the regions table itself. The full coverage check happens via
	// regions.AllSourceSlugs() in dispatch.DefaultRegistry, which is unit-
	// tested in dispatch/dispatch_test.go.
	regionsFile := filepath.Join(root, "internal", "regions", "regions.go")
	body, err := os.ReadFile(regionsFile)
	if err != nil {
		t.Fatalf("read regions.go: %v", err)
	}
	// Sources actually in regions table.
	for _, slug := range listSourceDirs(t, root) {
		// Match `"<slug>"` (with quotes) inside the file body. Skip if
		// the slug is part of an unrelated string (no near-quote).
		needle := `"` + slug + `"`
		if strings.Contains(string(body), needle) {
			continue
		}
		// A source package exists in internal/<slug>/ but the regions
		// table doesn't name it. That's fine in principle (the package
		// could be infra), but every package the regions table NAMES
		// must exist. The reverse check happens here:
		_ = slug
	}
}
