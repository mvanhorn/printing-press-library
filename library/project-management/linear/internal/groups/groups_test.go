// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package groups

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fixtureTOML is the user file the resolution tests load. It is deliberately
// adversarial: it shadows a builtin group (active), shadows a builtin raw type
// with a name-only group of the same name (started), declares a names-only
// group and a mixed group, and gives one team its own active plus a team-only
// group. Every precedence rule in Registry.Resolve is observable against it.
const fixtureTOML = `
schema_version = 1

[state_groups.active]
description = "only what is genuinely in flight"
types = ["started"]

[state_groups.started]
names = ["Doing"]

[state_groups.review]
names = ["In Review", "Code Review"]

[state_groups.mixed]
types = ["started"]
names = ["Needs QA"]

[team_state_groups.ENG.active]
names = ["ENG Doing"]

[team_state_groups.ENG.shipped]
types = ["completed"]
`

// newRegistry writes contents as the groups file in a fresh temp dir and loads
// it through the public loader, returning the registry and the directory. A
// fresh directory per call gives every test its own key in the load memo, and
// clearing LINEAR_GROUPS stops an override in the developer's shell from
// redirecting the load somewhere else.
func newRegistry(t *testing.T, contents string) (*Registry, string) {
	t.Helper()
	t.Setenv(EnvGroupsPath, "")
	dir := t.TempDir()
	if contents != "" {
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(contents), 0o600); err != nil {
			t.Fatalf("writing %s: %v", FileName, err)
		}
	}
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir(%s): %v", dir, err)
	}
	return reg, dir
}

// Resolution precedence is the whole point of this package: every --state,
// --completed-group and --candidate-group token lands here, so a regression in
// the order below silently changes what "open" means across the binary.
func TestRegistryResolve_PrecedenceAndNormalization(t *testing.T) {
	reg, _ := newRegistry(t, fixtureTOML)

	tests := []struct {
		name       string
		teamKey    string
		token      string
		wantGroup  string
		wantSource string
		wantAs     string
		wantToken  string
		wantTypes  []string
		wantNames  []string
		unfiltered bool
	}{
		{
			name: "empty token carries no predicate", token: "",
			wantGroup: ReservedAll, wantSource: SourceBuiltin, wantAs: ResolvedUnfiltered,
			wantToken: "", wantTypes: []string{}, wantNames: []string{}, unfiltered: true,
		},
		{
			name: "all is reserved and carries no predicate", token: "all",
			wantGroup: ReservedAll, wantSource: SourceBuiltin, wantAs: ResolvedUnfiltered,
			wantToken: "all", wantTypes: []string{}, wantNames: []string{}, unfiltered: true,
		},
		{
			name: "all is trimmed and lowercased", token: "  ALL  ",
			wantGroup: ReservedAll, wantSource: SourceBuiltin, wantAs: ResolvedUnfiltered,
			wantToken: "all", wantTypes: []string{}, wantNames: []string{}, unfiltered: true,
		},
		{
			name: "workspace group beats the builtin of the same name", token: "active",
			wantGroup: "active", wantSource: SourceWorkspace, wantAs: ResolvedGroup,
			wantToken: "active", wantTypes: []string{"started"}, wantNames: []string{},
		},
		{
			name: "token is trimmed and lowercased before lookup", token: "  ACTIVE  ",
			wantGroup: "active", wantSource: SourceWorkspace, wantAs: ResolvedGroup,
			wantToken: "active", wantTypes: []string{"started"}, wantNames: []string{},
		},
		{
			// The team declaration carries no types. If team ever merged
			// into workspace instead of replacing it, "started" would leak
			// back in here.
			name:    "team group replaces the workspace group instead of merging",
			teamKey: "ENG", token: "active",
			wantGroup: "active", wantSource: sourceTeamPfx + "ENG", wantAs: ResolvedGroup,
			wantToken: "active", wantTypes: []string{}, wantNames: []string{"ENG Doing"},
		},
		{
			name: "team key is case-insensitive", teamKey: "eng", token: "active",
			wantGroup: "active", wantSource: sourceTeamPfx + "ENG", wantAs: ResolvedGroup,
			wantToken: "active", wantTypes: []string{}, wantNames: []string{"ENG Doing"},
		},
		{
			name: "another team does not see ENG's declaration", teamKey: "OPS", token: "active",
			wantGroup: "active", wantSource: SourceWorkspace, wantAs: ResolvedGroup,
			wantToken: "active", wantTypes: []string{"started"}, wantNames: []string{},
		},
		{
			name:    "team scope falls through to workspace for names it does not declare",
			teamKey: "ENG", token: "review",
			wantGroup: "review", wantSource: SourceWorkspace, wantAs: ResolvedGroup,
			wantToken: "review", wantTypes: []string{}, wantNames: []string{"In Review", "Code Review"},
		},
		{
			name: "team-only group resolves with the team key", teamKey: "ENG", token: "shipped",
			wantGroup: "shipped", wantSource: sourceTeamPfx + "ENG", wantAs: ResolvedGroup,
			wantToken: "shipped", wantTypes: []string{"completed"}, wantNames: []string{},
		},
		{
			name: "unshadowed builtin still resolves", token: "backlog",
			wantGroup: "backlog", wantSource: SourceBuiltin, wantAs: ResolvedGroup,
			wantToken: "backlog", wantTypes: []string{"backlog"}, wantNames: []string{},
		},
		{
			// The fixture redefines "started" as a name-only group, so a
			// bare token must return the group and the escape hatch must
			// still reach the raw API type.
			name: "a group shadows the raw type of the same name", token: "started",
			wantGroup: "started", wantSource: SourceWorkspace, wantAs: ResolvedGroup,
			wantToken: "started", wantTypes: []string{}, wantNames: []string{"Doing"},
		},
		{
			name: "type: prefix bypasses the shadowing group", token: "type:started",
			wantGroup: "started", wantSource: SourceBuiltin, wantAs: ResolvedRawType,
			wantToken: "type:started", wantTypes: []string{"started"}, wantNames: []string{},
		},
		{
			name: "type: prefix and its value are case-insensitive", token: " TYPE:Started ",
			wantGroup: "started", wantSource: SourceBuiltin, wantAs: ResolvedRawType,
			wantToken: "type:started", wantTypes: []string{"started"}, wantNames: []string{},
		},
		{
			// The token is lowercased for lookup, but a workflow state name
			// is workspace data and must survive verbatim.
			name: "name: prefix keeps the literal state name", token: "name:In Review",
			wantGroup: "In Review", wantSource: SourceBuiltin, wantAs: ResolvedLiteralName,
			wantToken: "name:in review", wantTypes: []string{}, wantNames: []string{"In Review"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reg.Resolve(tc.teamKey, tc.token)
			if err != nil {
				t.Fatalf("Resolve(%q, %q) returned error: %v", tc.teamKey, tc.token, err)
			}
			if got.Group.Name != tc.wantGroup {
				t.Errorf("group name = %q, want %q", got.Group.Name, tc.wantGroup)
			}
			if got.Source() != tc.wantSource {
				t.Errorf("source = %q, want %q", got.Source(), tc.wantSource)
			}
			if got.ResolvedAs != tc.wantAs {
				t.Errorf("resolved_as = %q, want %q", got.ResolvedAs, tc.wantAs)
			}
			if got.Token != tc.wantToken {
				t.Errorf("token = %q, want %q", got.Token, tc.wantToken)
			}
			if got.Unfiltered != tc.unfiltered {
				t.Errorf("unfiltered = %v, want %v", got.Unfiltered, tc.unfiltered)
			}
			if !slices.Equal(got.Group.Types, tc.wantTypes) {
				t.Errorf("types = %v, want %v", got.Group.Types, tc.wantTypes)
			}
			if !slices.Equal(got.Group.Names, tc.wantNames) {
				t.Errorf("names = %v, want %v", got.Group.Names, tc.wantNames)
			}
			if tc.unfiltered && !got.Group.Reserved {
				t.Errorf("unfiltered set should carry the reserved %q group", ReservedAll)
			}
		})
	}
}

// Unknown tokens are the user's typo, not a broken config file, so they must
// come back as ErrUnknownToken (exit 2) and name what was available instead.
func TestRegistryResolve_UnknownTokens(t *testing.T) {
	reg, _ := newRegistry(t, fixtureTOML)

	tests := []struct {
		name     string
		teamKey  string
		token    string
		contains []string
	}{
		{
			name: "unknown group lists the alternatives", token: "nope",
			contains: []string{`"nope"`, "active", "review", "started"},
		},
		{
			name: "team-only group is invisible without the team key", token: "shipped",
			contains: []string{`"shipped"`},
		},
		{
			name: "type: prefix rejects a non-API type", token: "type:done",
			contains: []string{"not a valid Linear workflow state type", "triage"},
		},
		{
			name: "name: prefix rejects an empty name", token: "name:   ",
			contains: []string{"names no state"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := reg.Resolve(tc.teamKey, tc.token)
			if err == nil {
				t.Fatalf("Resolve(%q, %q) = nil error, want ErrUnknownToken", tc.teamKey, tc.token)
			}
			if !errors.Is(err, ErrUnknownToken) {
				t.Errorf("error %v is not ErrUnknownToken; callers would map it to exit 10 instead of exit 2", err)
			}
			for _, want := range tc.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err.Error(), want)
				}
			}
		})
	}
}

// The shipped defaults decide what "open" means for a workspace that never
// writes a groups.toml, which is most of them.
func TestLoadDir_BuiltinDefaults(t *testing.T) {
	reg, dir := newRegistry(t, "")

	if reg.Present() {
		t.Errorf("Present() = true with no %s on disk", FileName)
	}
	if want := filepath.Join(dir, FileName); reg.Path() != want {
		t.Errorf("Path() = %q, want %q", reg.Path(), want)
	}

	active, err := reg.Resolve("", "active")
	if err != nil {
		t.Fatalf("resolving builtin active: %v", err)
	}
	if active.Source() != SourceBuiltin {
		t.Errorf("builtin active source = %q, want %q", active.Source(), SourceBuiltin)
	}
	wantActive := []string{"triage", "backlog", "unstarted", "started"}
	if !slices.Equal(active.Group.Types, wantActive) {
		t.Errorf("builtin active types = %v, want %v", active.Group.Types, wantActive)
	}
	for _, closed := range []string{"completed", "canceled", "duplicate"} {
		if active.MatchesType(closed) {
			t.Errorf("builtin active must not count %q as active", closed)
		}
	}

	// Every documented API type is reachable as a group of the same name,
	// which is what makes the raw-type fallback in Resolve unreachable.
	for _, ty := range APITypes() {
		set, err := reg.Resolve("", ty)
		if err != nil {
			t.Fatalf("resolving builtin type group %q: %v", ty, err)
		}
		if set.ResolvedAs != ResolvedGroup {
			t.Errorf("%q resolved_as = %q, want %q", ty, set.ResolvedAs, ResolvedGroup)
		}
		if !slices.Equal(set.Group.Types, []string{ty}) {
			t.Errorf("%q types = %v, want [%s]", ty, set.Group.Types, ty)
		}
	}
}

// A malformed or dishonest declaration must fail at load, loudly, rather than
// resolving to a predicate that quietly matches the wrong states.
func TestLoadDir_RejectsInvalidDeclarations(t *testing.T) {
	t.Setenv(EnvGroupsPath, "")

	tests := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name:     "all is reserved",
			contents: "[state_groups.all]\ntypes = [\"started\"]\n",
			wantErr:  "reserved",
		},
		{
			name:     "group name must be lowercase",
			contents: "[state_groups.Active]\ntypes = [\"started\"]\n",
			wantErr:  "must match",
		},
		{
			name:     "group name must not start with a dash",
			contents: "[state_groups.\"-x\"]\ntypes = [\"started\"]\n",
			wantErr:  "must match",
		},
		{
			name:     "types must be documented API types",
			contents: "[state_groups.shipped]\ntypes = [\"done\"]\n",
			wantErr:  "not a valid Linear workflow state type",
		},
		{
			name:     "a group must declare something",
			contents: "[state_groups.empty]\ndescription = \"nothing at all\"\n",
			wantErr:  "declares no types and no names",
		},
		{
			name:     "unsupported schema version",
			contents: "schema_version = 2\n[state_groups.shipped]\ntypes = [\"completed\"]\n",
			wantErr:  "unsupported groups schema_version",
		},
		{
			name:     "malformed toml",
			contents: "[state_groups.shipped\ntypes = [\"completed\"]\n",
			wantErr:  "parsing",
		},
		{
			name:     "team key must not be blank",
			contents: "[team_state_groups.\"  \".shipped]\ntypes = [\"completed\"]\n",
			wantErr:  "empty team key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(tc.contents), 0o600); err != nil {
				t.Fatalf("writing %s: %v", FileName, err)
			}
			_, err := LoadDir(dir)
			if err == nil {
				t.Fatalf("LoadDir accepted an invalid %s", FileName)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantErr)
			}
			if errors.Is(err, ErrUnknownToken) {
				t.Errorf("a bad file is a config error, not a bad token: %v", err)
			}
		})
	}
}

// schema_version is optional. A user file that omits it must still load, or
// every hand-written groups.toml in the wild breaks.
func TestLoadDir_SchemaVersionIsOptional(t *testing.T) {
	reg, _ := newRegistry(t, "[state_groups.shipped]\ntypes = [\"completed\"]\n")
	if !reg.Present() {
		t.Errorf("Present() = false after writing %s", FileName)
	}
	set, err := reg.Resolve("", "shipped")
	if err != nil {
		t.Fatalf("resolving shipped: %v", err)
	}
	if set.Source() != SourceWorkspace {
		t.Errorf("source = %q, want %q", set.Source(), SourceWorkspace)
	}
}

// `groups list` renders Effective, and its whole job is explaining why a token
// resolves the way it does. Shadowed rows must stay visible and must name the
// declarations that outrank them.
func TestRegistryEffective_ShadowingAndOrder(t *testing.T) {
	reg, _ := newRegistry(t, fixtureTOML)

	rows := reg.Effective("")
	bySourceForName := func(rows []Group, name string) []string {
		var out []string
		for _, g := range rows {
			if g.Name == name {
				out = append(out, g.Source)
			}
		}
		return out
	}

	wantOrder := []string{SourceBuiltin, SourceWorkspace, sourceTeamPfx + "ENG"}
	if got := bySourceForName(rows, "active"); !slices.Equal(got, wantOrder) {
		t.Errorf("active rows = %v, want %v (builtin, then workspace, then team)", got, wantOrder)
	}

	// A lowercase team key must select the same scope Resolve selects.
	if got := bySourceForName(reg.Effective("eng"), "shipped"); !slices.Equal(got, []string{sourceTeamPfx + "ENG"}) {
		t.Errorf("Effective(eng) shipped rows = %v, want the ENG declaration", got)
	}

	var reserved int
	for _, g := range rows {
		switch {
		case g.Name == ReservedAll:
			reserved++
			if !g.Reserved {
				t.Errorf("%q row is not marked reserved", ReservedAll)
			}
		case g.Name == "active" && g.Source == SourceBuiltin:
			want := []string{SourceWorkspace, sourceTeamPfx + "ENG"}
			if !slices.Equal(g.ShadowedBy, want) {
				t.Errorf("builtin active shadowed_by = %v, want %v", g.ShadowedBy, want)
			}
		case g.Name == "active" && g.Source == SourceWorkspace:
			want := []string{sourceTeamPfx + "ENG"}
			if !slices.Equal(g.ShadowedBy, want) {
				t.Errorf("workspace active shadowed_by = %v, want %v", g.ShadowedBy, want)
			}
		case g.Name == "review" && g.Source == SourceWorkspace:
			if len(g.ShadowedBy) != 0 {
				t.Errorf("unshadowed review reports shadowed_by = %v", g.ShadowedBy)
			}
		}
	}
	if reserved != 1 {
		t.Errorf("%q appears %d times, want exactly once", ReservedAll, reserved)
	}

	// A team scope must only report its own declarations plus the shared
	// ones, never another team's.
	for _, g := range reg.Effective("OPS") {
		if strings.HasPrefix(g.Source, sourceTeamPfx) {
			t.Errorf("Effective(OPS) leaked %s row %q", g.Source, g.Name)
		}
		if g.Name == "active" && g.Source == SourceBuiltin {
			want := []string{SourceWorkspace}
			if !slices.Equal(g.ShadowedBy, want) {
				t.Errorf("Effective(OPS) builtin active shadowed_by = %v, want %v", g.ShadowedBy, want)
			}
		}
	}
}

// Set is the predicate every local (non-GraphQL) filter evaluates, so its
// matching rules are the offline half of this package's contract.
func TestSetMatches(t *testing.T) {
	t.Parallel()

	set := Set{Group: Group{
		Name:  "mixed",
		Types: []string{"started"},
		Names: []string{"In Review"},
	}}

	tests := []struct {
		name      string
		stateType string
		stateName string
		wantType  bool
		wantName  bool
	}{
		{name: "exact type", stateType: "started", stateName: "Doing", wantType: true},
		{name: "type comparison ignores case", stateType: "STARTED", stateName: "Doing", wantType: true},
		{name: "type comparison trims", stateType: "  started  ", stateName: "Doing", wantType: true},
		{name: "unlisted type", stateType: "completed", stateName: "Done"},
		{name: "empty type matches nothing", stateName: "Done"},
		{name: "exact name", stateType: "completed", stateName: "In Review", wantName: true},
		{name: "name comparison ignores case", stateType: "completed", stateName: "in review", wantName: true},
		{name: "name comparison trims", stateType: "completed", stateName: "  In Review  ", wantName: true},
		{name: "empty name matches nothing", stateType: "completed"},
		{name: "a name is not a type", stateType: "In Review", stateName: "Done"},
		{name: "a type is not a name", stateType: "completed", stateName: "started"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := set.MatchesType(tc.stateType); got != tc.wantType {
				t.Errorf("MatchesType(%q) = %v, want %v", tc.stateType, got, tc.wantType)
			}
			if got := set.MatchesName(tc.stateName); got != tc.wantName {
				t.Errorf("MatchesName(%q) = %v, want %v", tc.stateName, got, tc.wantName)
			}
			wantUnion := tc.wantType || tc.wantName
			if got := set.Matches(tc.stateType, tc.stateName); got != wantUnion {
				t.Errorf("Matches(%q, %q) = %v, want %v", tc.stateType, tc.stateName, got, wantUnion)
			}
			// Match is the cross-package alias other packages call.
			if got := set.Match(tc.stateType, tc.stateName); got != wantUnion {
				t.Errorf("Match(%q, %q) = %v, want %v", tc.stateType, tc.stateName, got, wantUnion)
			}
		})
	}
}

// "all" means no predicate at all, not "every type I happen to know about".
func TestSetUnfiltered_MatchesEverythingAndEmitsNoFilter(t *testing.T) {
	t.Parallel()

	set := Set{Unfiltered: true}
	for _, tc := range [][2]string{{"started", "Doing"}, {"anything", "at all"}, {"", ""}} {
		if !set.Matches(tc[0], tc[1]) {
			t.Errorf("unfiltered Matches(%q, %q) = false", tc[0], tc[1])
		}
	}
	if !set.MatchesType("") || !set.MatchesName("") {
		t.Errorf("unfiltered set rejected an empty state")
	}
	if got := set.GraphQLFilter(); got != nil {
		t.Errorf("GraphQLFilter() = %v, want nil so the state key is omitted entirely", got)
	}
}

// The live half of the contract. These shapes are handed straight to Linear as
// a WorkflowStateFilter, and they must agree with what Matches does locally.
func TestSetGraphQLFilter_Shapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group Group
		want  map[string]any
	}{
		{
			name:  "types only use a single in comparator",
			group: Group{Types: []string{"triage", "started"}, Names: []string{}},
			want:  map[string]any{"type": map[string]any{"in": []string{"triage", "started"}}},
		},
		{
			// StringComparator has no inIgnoreCase, so a lone name is an
			// eqIgnoreCase rather than a case-sensitive in.
			name:  "a single name uses eqIgnoreCase",
			group: Group{Types: []string{}, Names: []string{"In Review"}},
			want:  map[string]any{"name": map[string]any{"eqIgnoreCase": "In Review"}},
		},
		{
			name:  "several names become an or of eqIgnoreCase clauses",
			group: Group{Types: []string{}, Names: []string{"In Review", "Code Review"}},
			want: map[string]any{"or": []any{
				map[string]any{"name": map[string]any{"eqIgnoreCase": "In Review"}},
				map[string]any{"name": map[string]any{"eqIgnoreCase": "Code Review"}},
			}},
		},
		{
			name:  "types and names or together with the type clause first",
			group: Group{Types: []string{"started"}, Names: []string{"Needs QA"}},
			want: map[string]any{"or": []any{
				map[string]any{"type": map[string]any{"in": []string{"started"}}},
				map[string]any{"name": map[string]any{"eqIgnoreCase": "Needs QA"}},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := Set{Group: tc.group}
			got := set.GraphQLFilter()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("GraphQLFilter() = %#v, want %#v", got, tc.want)
			}
			// LiveFilter is the package-level form other packages call.
			if other := LiveFilter(set); !reflect.DeepEqual(other, got) {
				t.Errorf("LiveFilter() = %#v, want %#v", other, got)
			}
		})
	}
}

// The emitted filter must not alias the group's own slice: a caller that sorts
// or appends to the request payload would otherwise corrupt the registry entry
// every later Matches call reads.
func TestSetGraphQLFilter_DoesNotAliasGroupTypes(t *testing.T) {
	t.Parallel()

	set := Set{Group: Group{Types: []string{"started", "backlog"}, Names: []string{}}}
	filter := set.GraphQLFilter()
	emitted := filter["type"].(map[string]any)["in"].([]string)
	emitted[0] = "canceled"

	if !slices.Equal(set.Group.Types, []string{"started", "backlog"}) {
		t.Errorf("mutating the emitted filter changed the group to %v", set.Group.Types)
	}
	if set.MatchesType("canceled") {
		t.Errorf("group started matching canceled after the emitted filter was mutated")
	}
}

func TestIsAPIType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{in: "started", want: true},
		{in: "  STARTED  ", want: true},
		{in: "triage", want: true},
		{in: "duplicate", want: true},
		{in: "done", want: false},
		{in: "in progress", want: false},
		{in: "", want: false},
	}
	for _, tc := range tests {
		if got := IsAPIType(tc.in); got != tc.want {
			t.Errorf("IsAPIType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// APITypes hands out the filter vocabulary to error messages and to callers
// that enumerate states. It must hand out a copy, not the package's own slice.
func TestAPITypes_ReturnsACopy(t *testing.T) {
	t.Parallel()

	first := APITypes()
	if len(first) != 7 {
		t.Fatalf("APITypes() returned %d types, want the 7 documented values", len(first))
	}
	for _, ty := range first {
		if !IsAPIType(ty) {
			t.Errorf("APITypes() returned %q, which IsAPIType rejects", ty)
		}
		if !strings.Contains(APITypeList, ty) {
			t.Errorf("APITypeList %q omits %q", APITypeList, ty)
		}
	}

	first[0] = "clobbered"
	if second := APITypes(); second[0] == "clobbered" {
		t.Errorf("APITypes() aliases the package slice; a caller corrupted the vocabulary")
	}
}

// Group declarations left in config.toml are silently deleted the next time
// config.save rewrites that file, so detecting them is a real user warning.
func TestMisplacedInConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name, contents string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
		return path
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "workspace groups in config", path: write("a.toml", "[state_groups.active]\ntypes = [\"started\"]\n"), want: true},
		{name: "team groups in config", path: write("b.toml", "[team_state_groups.ENG.active]\ntypes = [\"started\"]\n"), want: true},
		{name: "ordinary config", path: write("c.toml", "api_key = \"secret\"\nbase_url = \"https://api.linear.app\"\n")},
		{name: "malformed config is not a claim either way", path: write("d.toml", "[state_groups.active\n")},
		{name: "missing file", path: filepath.Join(dir, "absent.toml")},
		{name: "empty path"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MisplacedInConfig(tc.path); got != tc.want {
				t.Errorf("MisplacedInConfig(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// PathForDir and PathForConfig decide which file the declarations come from.
// $LINEAR_GROUPS is the documented escape hatch and must win verbatim.
func TestPathResolution(t *testing.T) {
	t.Setenv(EnvGroupsPath, "")

	if got, want := PathForDir("/tmp/cfg"), filepath.Join("/tmp/cfg", FileName); got != want {
		t.Errorf("PathForDir = %q, want %q", got, want)
	}
	if got := PathForDir("  "); got != "" {
		t.Errorf("PathForDir(blank) = %q, want \"\" (builtins only)", got)
	}
	if got, want := PathForConfig("/tmp/cfg/config.toml"), filepath.Join("/tmp/cfg", FileName); got != want {
		t.Errorf("PathForConfig = %q, want %q", got, want)
	}
	if got := PathForConfig(""); got != "" {
		t.Errorf("PathForConfig(blank) = %q, want \"\"", got)
	}

	t.Setenv(EnvGroupsPath, "  /elsewhere/custom.toml  ")
	for _, got := range []string{
		PathForDir("/tmp/cfg"),
		PathForDir(""),
		PathForConfig("/tmp/cfg/config.toml"),
		PathForConfig(""),
	} {
		if got != "/elsewhere/custom.toml" {
			t.Errorf("path = %q, want the %s override", got, EnvGroupsPath)
		}
	}
}

// The package-level Resolve and List are what callers outside this package
// actually import; they must agree with the Registry methods for the same dir.
func TestPackageLevelResolveAndList(t *testing.T) {
	reg, dir := newRegistry(t, fixtureTOML)

	got, err := Resolve(dir, "ENG", "active")
	if err != nil {
		t.Fatalf("Resolve(%s, ENG, active): %v", dir, err)
	}
	want, err := reg.Resolve("ENG", "active")
	if err != nil {
		t.Fatalf("Registry.Resolve(ENG, active): %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %#v, want %#v", got, want)
	}

	if _, err := Resolve(dir, "", "nope"); !errors.Is(err, ErrUnknownToken) {
		t.Errorf("Resolve error = %v, want ErrUnknownToken", err)
	}

	list, err := List(dir, "ENG")
	if err != nil {
		t.Fatalf("List(%s, ENG): %v", dir, err)
	}
	if !reflect.DeepEqual(list, reg.Effective("ENG")) {
		t.Errorf("List did not match Registry.Effective")
	}
}
