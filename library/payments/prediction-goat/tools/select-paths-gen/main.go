// Command select-paths-gen emits internal/cli/select_paths.go: a
// per-command cheatsheet of valid dotted field paths agents can pass to
// --select. The cheatsheet is derived at build time from JSON-tagged Go
// structs so it cannot drift from the actual response shape.
//
// Why this exists (cross-link: docs/plans/2026-05-23-002, section U7):
// Agents that pass --select today guess dotted paths against shapes
// they have not introspected. A wrong path returns {} (not an error),
// forcing a re-fetch and burning roundtrips. Two of three failure
// traces in the smart-learning plan burned ~25% of their wall-clock on
// silent --select errors. The CLI already knows its response shape; it
// should expose that to agents up front via agent-context.
//
// Design choices documented inline so the next agent extending this
// tool can pick up the heuristic ceiling without re-deriving it:
//
//   1. Roots are explicit, not fuzzy-matched. A small `roots` map at
//      the top of buildCheatsheet() pins each command name to its
//      top-level response struct. Adding a new command means adding
//      one line here — that is the price of refusing to invent a
//      command-name→struct-name guesser that will eventually mis-tag.
//
//   2. Two source dirs are walked: internal/cli (for command-local
//      response types like topicResult, compareResult) and
//      internal/types (for the upstream-shaped types like
//      types.Market that the raw-pass-through commands such as
//      `markets get-by-slug` return).
//
//   3. Path expansion descends into:
//        - direct struct fields (Ident → resolves in same registry)
//        - pointer-to-struct (*X)
//        - slice-of-struct ([]X — paths use bare field names, no
//          indices, matching the response key format an agent would
//          pass to --select)
//      json.RawMessage and other opaque payloads stop recursion (we
//      cannot introspect arbitrary JSON; agents must pass the bare
//      field name and unpack downstream).
//
//   4. Cycle guard: recursion tracks the set of struct names already
//      expanded on the current path and bails when a cycle would loop.
//      Embedded structs (anonymous fields like
//      `kalshiEventWithMarketsItem.kalshiEventItem`) are inlined so
//      their fields appear at the parent's level — matching how
//      encoding/json marshals them.
//
//   5. Fields with json:"-" or unexported names are skipped. Fields
//      with a JSON tag of "" inherit the lowercase field name (Go
//      convention; encoding/json does the same).
//
// Heuristic ceiling: this tool only sees Go types. Commands whose
// response is `data []byte` from an HTTP passthrough have no Go shape
// to introspect; those route through types.Market (or similar)
// declared in internal/types. If a future command starts emitting a
// shape that is not represented as a named struct anywhere in this
// repo, the cheatsheet entry will be missing — add the struct (even a
// thin projection type) and pin it in roots.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// structInfo holds the parsed JSON-tag schema for a named struct so we
// can resolve nested type references without a second AST walk.
type structInfo struct {
	Name   string
	Fields []fieldInfo
}

// fieldInfo captures one field's JSON name and its referenced type
// info. TypeRef is the bare type name when the field points at another
// named struct (Ident, *Ident, []Ident, []*Ident). It is empty for
// leaf types (string, int, etc.) and for opaque types like
// json.RawMessage. IsSlice/IsPointer are informational — they do not
// change the dotted-path output (slices use bare field names, not
// indices).
type fieldInfo struct {
	JSONName  string
	TypeRef   string
	IsSlice   bool
	IsPointer bool
	Embedded  bool
}

func main() {
	// The generator resolves its source dirs and output path
	// relative to a single "CLI root" anchor. The anchor is picked
	// using the cheapest signal we have:
	//
	//   1. SELECT_PATHS_GEN_ROOT env (escape hatch for unusual layouts)
	//   2. cwd is the CLI root (the canonical `go generate ./...`
	//      invocation from the CLI root)
	//   3. cwd is a child of the CLI root that has internal/cli as a
	//      sibling (the `//go:generate` directive in select_paths.go
	//      runs from internal/cli/ — walk up to find a sibling
	//      tools/select-paths-gen so we anchor on the CLI root)
	//
	// Falling through all of these is fatal; we'd rather error than
	// silently write to the wrong place.
	cliRoot, err := resolveCLIRoot()
	if err != nil {
		fail(err)
	}
	cliDir := filepath.Join(cliRoot, "internal", "cli")
	typesDir := filepath.Join(cliRoot, "internal", "types")
	// Allow the test suite (and curious operators) to override the
	// output path via SELECT_PATHS_GEN_OUT so a drift check can run
	// the generator in dry-run / compare-only mode without mutating
	// the worktree. Default writes to <cliRoot>/internal/cli/select_paths.go.
	outPath := os.Getenv("SELECT_PATHS_GEN_OUT")
	if outPath == "" {
		outPath = filepath.Join(cliDir, "select_paths.go")
	}

	registry := map[string]*structInfo{}
	if err := collectStructs(cliDir, registry); err != nil {
		fail(err)
	}
	if err := collectStructs(typesDir, registry); err != nil {
		// internal/types may not exist in every CLI's tree; treat its
		// absence as informational, not fatal.
		if !os.IsNotExist(err) {
			fail(err)
		}
	}

	cheatsheet := buildCheatsheet(registry)

	src, err := renderFile(cheatsheet)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(outPath, src, 0o644); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "select-paths-gen: wrote %s (%d commands)\n", outPath, len(cheatsheet))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "select-paths-gen:", err)
	os.Exit(1)
}

// resolveCLIRoot returns the directory containing tools/select-paths-gen
// and internal/cli. See the main() doc comment for the lookup order.
func resolveCLIRoot() (string, error) {
	if v := os.Getenv("SELECT_PATHS_GEN_ROOT"); v != "" {
		return v, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Walk from cwd upward looking for a directory that has both
	// internal/cli and tools/select-paths-gen as children. Most
	// invocations either run from the CLI root (one stop) or from
	// internal/cli (the go:generate directive — two stops up).
	dir := cwd
	for i := 0; i < 6; i++ {
		if hasCLIRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate CLI root from cwd %q; pass SELECT_PATHS_GEN_ROOT or run from the CLI root", cwd)
}

func hasCLIRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "internal", "cli")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "tools", "select-paths-gen")); err != nil {
		return false
	}
	return true
}

// collectStructs parses every .go file in dir (skipping _test.go) and
// indexes every named struct type into registry by its bare name. Test
// files are skipped because they declare fixture structs that should
// not surface as cheatsheet roots.
func collectStructs(dir string, registry map[string]*structInfo) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				registry[ts.Name.Name] = &structInfo{
					Name:   ts.Name.Name,
					Fields: extractFields(st),
				}
			}
		}
	}
	return nil
}

// extractFields walks the AST struct body and returns the JSON-tagged
// field set with type references for recursion. The output mirrors
// encoding/json's marshalling rules:
//
//   - `json:"-"` drops the field
//   - missing or empty tag defaults to the lowercase field name (Go
//     convention; encoding/json does the same when a tag is absent)
//   - `,omitempty` and other tag options are stripped
//   - unexported (lowercase) fields are skipped (encoding/json ignores
//     them too)
//   - anonymous embedded fields are recorded with Embedded=true so the
//     expander can inline their fields at the parent's level
func extractFields(st *ast.StructType) []fieldInfo {
	var out []fieldInfo
	for _, f := range st.Fields.List {
		jsonName, skip := readJSONTag(f.Tag)
		if skip {
			continue
		}
		typeRef, isSlice, isPointer := resolveTypeRef(f.Type)

		if len(f.Names) == 0 {
			// Anonymous embedded field. Use the type name as the
			// embedded marker; the expander pulls its fields up.
			if typeRef == "" {
				continue
			}
			out = append(out, fieldInfo{
				JSONName: "",
				TypeRef:  typeRef,
				Embedded: true,
			})
			continue
		}
		for _, n := range f.Names {
			if !n.IsExported() {
				continue
			}
			fname := jsonName
			if fname == "" {
				// encoding/json defaults to the lowercased Go field
				// name when no tag is present. We mirror that so
				// untagged response structs still produce useful
				// paths.
				fname = lowerFirst(n.Name)
			}
			out = append(out, fieldInfo{
				JSONName:  fname,
				TypeRef:   typeRef,
				IsSlice:   isSlice,
				IsPointer: isPointer,
			})
		}
	}
	return out
}

// readJSONTag returns the field's JSON name and a skip flag. A nil tag
// produces ("", false) which signals "use the Go field name". A tag of
// `json:"-"` returns ("-", true).
func readJSONTag(tag *ast.BasicLit) (name string, skip bool) {
	if tag == nil {
		return "", false
	}
	raw := strings.Trim(tag.Value, "`")
	// Walk space-separated key:"value" pairs.
	for raw != "" {
		raw = strings.TrimSpace(raw)
		eq := strings.Index(raw, ":")
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(raw[:eq])
		raw = raw[eq+1:]
		if !strings.HasPrefix(raw, "\"") {
			break
		}
		end := strings.Index(raw[1:], "\"")
		if end < 0 {
			break
		}
		val := raw[1 : 1+end]
		raw = raw[1+end+1:]
		if key != "json" {
			continue
		}
		if val == "-" {
			return "", true
		}
		parts := strings.Split(val, ",")
		return parts[0], false
	}
	return "", false
}

// resolveTypeRef inspects a field type expression and returns the bare
// referenced type name plus the structural modifiers we care about.
// json.RawMessage and other selector-shaped types return an empty
// TypeRef so expansion stops at the leaf — we cannot introspect
// arbitrary JSON.
func resolveTypeRef(expr ast.Expr) (name string, isSlice, isPointer bool) {
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			isPointer = true
			expr = t.X
			continue
		case *ast.ArrayType:
			isSlice = true
			expr = t.Elt
			continue
		case *ast.Ident:
			return t.Name, isSlice, isPointer
		case *ast.SelectorExpr:
			// e.g., json.RawMessage, time.Time — opaque to us.
			return "", isSlice, isPointer
		case *ast.MapType, *ast.InterfaceType, *ast.FuncType, *ast.ChanType:
			return "", isSlice, isPointer
		default:
			return "", isSlice, isPointer
		}
	}
}

// buildCheatsheet assembles the command→paths map. Roots are pinned
// explicitly (see package doc comment for rationale).
func buildCheatsheet(reg map[string]*structInfo) map[string][]string {
	// roots maps a command name (as it appears in cobra's tree, joined
	// with spaces for subcommands) to its top-level response struct
	// name. Order does not matter — output is sorted by command name.
	//
	// Adding a new command: pin it here. If the response shape is
	// already covered by an existing struct (because two commands
	// return the same shape) reuse the struct name; the generator
	// emits the same paths under both keys.
	roots := map[string]string{
		"topic":    "topicResult",
		"compare":  "compareResult",
		"recall":   "recallEnvelope",
		"markets get-by-slug": "Market",
	}

	out := make(map[string][]string, len(roots))
	for cmd, root := range roots {
		paths := expandPaths(reg, root)
		sort.Strings(paths)
		out[cmd] = paths
	}
	return out
}

// expandPaths returns the dotted JSON paths reachable from a root
// struct, in arbitrary order. The caller sorts. A missing root in the
// registry returns nil — the caller decides whether that is fatal (it
// usually is during generation but the test suite asserts each pinned
// root resolves so the failure surfaces with a clear message).
func expandPaths(reg map[string]*structInfo, root string) []string {
	if reg[root] == nil {
		return nil
	}
	var paths []string
	visited := map[string]bool{}
	walkStruct(reg, root, "", visited, &paths)
	return uniqueStrings(paths)
}

// walkStruct recursively expands a struct's fields into dotted paths.
// The visited map guards against cycles (a struct that transitively
// references itself).
func walkStruct(reg map[string]*structInfo, name, prefix string, visited map[string]bool, out *[]string) {
	if visited[name] {
		return
	}
	info := reg[name]
	if info == nil {
		return
	}
	visited[name] = true
	defer delete(visited, name)

	for _, f := range info.Fields {
		if f.Embedded {
			// Inline embedded struct fields at the parent's level.
			walkStruct(reg, f.TypeRef, prefix, visited, out)
			continue
		}
		full := f.JSONName
		if prefix != "" {
			full = prefix + "." + f.JSONName
		}
		*out = append(*out, full)
		// Recurse if the field references another named struct in
		// the registry. Leaf types (string, int, json.RawMessage,
		// time.Time) have an empty TypeRef or no entry in reg.
		if f.TypeRef != "" {
			if _, ok := reg[f.TypeRef]; ok {
				walkStruct(reg, f.TypeRef, full, visited, out)
			}
		}
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

const fileHeader = `// Code generated by tools/select-paths-gen; DO NOT EDIT.
//
// Per-command --select cheatsheet. This file is regenerated by running
// 'go generate ./internal/cli/...' from the CLI root, and CI verifies
// the committed file matches regeneration. See
// docs/plans/2026-05-23-002 section U7 for the design rationale.
//
//go:generate go run ../../tools/select-paths-gen

package cli

// commandSelectPaths is the source-of-truth map exposed by
// agent-context under commands.<name>.select_paths. Each value is the
// sorted list of dotted field paths valid for --select on that
// command. Slice paths use bare field names (no [0]/[*] notation) —
// that is what an agent passes to --select.
var commandSelectPaths = map[string][]string{
`

const fileFooter = `}
`

func renderFile(cheat map[string][]string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(fileHeader)
	keys := make([]string, 0, len(cheat))
	for k := range cheat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		paths := cheat[k]
		if len(paths) == 0 {
			// Emit empty slice rather than dropping the key so the
			// agent-context surface still advertises the command and
			// the missing-paths case is auditable.
			fmt.Fprintf(&buf, "\t%q: {},\n", k)
			continue
		}
		fmt.Fprintf(&buf, "\t%q: {\n", k)
		for _, p := range paths {
			fmt.Fprintf(&buf, "\t\t%q,\n", p)
		}
		buf.WriteString("\t},\n")
	}
	buf.WriteString(fileFooter)

	// gofmt for stable diffs across machines / Go versions.
	return format.Source(buf.Bytes())
}
