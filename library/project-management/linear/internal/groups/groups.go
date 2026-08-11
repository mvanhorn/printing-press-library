// Package groups resolves user-declarable workflow-state groups.
//
// A group names a set of Linear workflow states. It is the single place in
// this CLI that is allowed to express a state predicate: every --state token,
// every "is this issue still open" check and every live WorkflowStateFilter
// is produced here. Nothing else may hand-write a state comparison.
//
// Declarations live in a groups.toml sibling of the CLI config file, never in
// config.toml itself (config.save rewrites that file wholesale from a fixed
// struct and would silently delete them). Built-in defaults are embedded from
// defaults.toml and parsed by the same code path as the user file, so there is
// exactly one representation of a group in the binary.
//
// The only Linear vocabulary compiled in here is the seven API-documented
// WorkflowState.type values. Workspace-specific state names, team keys and
// project names never appear: they belong in the user's own groups.toml.
package groups

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/config"

	"github.com/pelletier/go-toml/v2"
)

//go:embed defaults.toml
var defaultsTOML []byte

// FileName is the sibling file the declarations are read from.
const FileName = "groups.toml"

// EnvGroupsPath overrides the resolved groups file path verbatim.
const EnvGroupsPath = "LINEAR_GROUPS"

// ReservedAll is the token that means "no state predicate at all". It is not
// a set, so it is never encoded as a list of types: if Linear ever adds an
// eighth WorkflowState.type, an enumerated "all" would start silently
// excluding it while an absent predicate cannot.
const ReservedAll = "all"

// Group source discriminators, reported by `groups list` and by Set.Source.
const (
	SourceBuiltin   = "builtin"
	SourceWorkspace = "config:workspace"
	sourceTeamPfx   = "config:team:"
)

// Scope labels paired with the sources above.
const (
	scopeWorkspace = "workspace"
	scopeTeamPfx   = "team:"
)

// How a token was resolved, reported by `groups check`.
const (
	ResolvedGroup       = "group"
	ResolvedRawType     = "raw_type"
	ResolvedLiteralName = "literal_name"
	ResolvedUnfiltered  = "unfiltered"
)

// Escape-hatch prefixes that bypass group lookup entirely, so a user who
// shadows a raw type with a group of the same name can still reach the type.
const (
	prefixType = "type:"
	prefixName = "name:"
)

// GraphQL field and comparator names on WorkflowStateFilter / StringComparator.
// StringComparator has no inIgnoreCase, which is why names are emitted as an
// or-list of eqIgnoreCase rather than a single in: an in would be
// case-sensitive live while the local path compares case-insensitively, which
// is exactly the live-versus-local divergence this package exists to remove.
const (
	fieldType       = "type"
	fieldName       = "name"
	fieldOr         = "or"
	cmpIn           = "in"
	cmpEqIgnoreCase = "eqIgnoreCase"
)

// apiStateTypes is the complete read/filter vocabulary of
// WorkflowState.type. There is no WorkflowStateType enum in the live schema.
// The field is a bare String and these seven values come from its own
// description. TIER 2 API data, identical for every Linear workspace.
var apiStateTypes = []string{
	"triage",
	"backlog",
	"unstarted",
	"started",
	"completed",
	"canceled",
	"duplicate",
}

var apiStateTypeSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(apiStateTypes))
	for _, t := range apiStateTypes {
		m[t] = struct{}{}
	}
	return m
}()

// APITypeList renders the seven valid types for error messages.
var APITypeList = strings.Join(apiStateTypes, ", ")

// IsAPIType reports whether s is one of the seven documented type values.
// The input is trimmed and lowercased first.
func IsAPIType(s string) bool {
	_, ok := apiStateTypeSet[strings.ToLower(strings.TrimSpace(s))]
	return ok
}

// APITypes returns a copy of the seven documented type values.
func APITypes() []string { return append([]string(nil), apiStateTypes...) }

var groupNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ErrUnknownToken marks a resolution failure caused by this invocation's
// token rather than by the file on disk. Callers map it to a usage error
// (exit 2). Every other error from this package is a config error (exit 10).
var ErrUnknownToken = errors.New("unknown state group or type")

// Group is one declared set of workflow states.
type Group struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Scope       string   `json:"scope"`
	Reserved    bool     `json:"reserved"`
	ShadowedBy  []string `json:"shadowed_by"`
	Description string   `json:"description"`
	Types       []string `json:"types"`
	Names       []string `json:"names"`
}

// GroupInfo is the name the cross-package implementation contract uses for
// the rows `groups list` returns.
type GroupInfo = Group

// Set is a resolved predicate. Unfiltered means "no state predicate at all",
// which is what the empty token and the reserved "all" produce.
type Set struct {
	Group      Group  `json:"group"`
	Unfiltered bool   `json:"unfiltered"`
	ResolvedAs string `json:"resolved_as"`
	Token      string `json:"token"`
}

// Predicate is the cross-package name for a resolved Set. It is a concrete
// value type, not an interface, so callers can declare and copy it freely.
type Predicate = Set

// Source reports where the resolved group came from: builtin,
// config:workspace, or config:team:<KEY>.
func (s Set) Source() string { return s.Group.Source }

// MatchesType reports whether a WorkflowState.type belongs to the set.
func (s Set) MatchesType(stateType string) bool {
	if s.Unfiltered {
		return true
	}
	t := strings.ToLower(strings.TrimSpace(stateType))
	if t == "" {
		return false
	}
	for _, want := range s.Group.Types {
		if want == t {
			return true
		}
	}
	return false
}

// MatchesName reports whether a WorkflowState.name belongs to the set.
// Comparison is case-insensitive after trimming, matching the live path's
// eqIgnoreCase emission.
func (s Set) MatchesName(stateName string) bool {
	if s.Unfiltered {
		return true
	}
	n := strings.TrimSpace(stateName)
	if n == "" {
		return false
	}
	for _, want := range s.Group.Names {
		if strings.EqualFold(strings.TrimSpace(want), n) {
			return true
		}
	}
	return false
}

// Matches is the local-path predicate: membership is the union of the type
// set and the name set, evaluated against the same state.
func (s Set) Matches(stateType, stateName string) bool {
	if s.Unfiltered {
		return true
	}
	return s.MatchesType(stateType) || s.MatchesName(stateName)
}

// Match is the cross-package alias of Matches.
func (s Set) Match(stateType, stateName string) bool { return s.Matches(stateType, stateName) }

// GraphQLFilter is the live-path predicate. It returns the value for the
// "state" key of an IssueFilter (a WorkflowStateFilter), or nil when the set
// is unfiltered and the key must be omitted entirely.
func (s Set) GraphQLFilter() map[string]any {
	if s.Unfiltered {
		return nil
	}
	typeClause := func() map[string]any {
		return map[string]any{fieldType: map[string]any{cmpIn: append([]string(nil), s.Group.Types...)}}
	}
	nameClause := func(n string) map[string]any {
		return map[string]any{fieldName: map[string]any{cmpEqIgnoreCase: n}}
	}
	hasTypes := len(s.Group.Types) > 0
	hasNames := len(s.Group.Names) > 0
	switch {
	case hasTypes && !hasNames:
		return typeClause()
	case !hasTypes && len(s.Group.Names) == 1:
		return nameClause(s.Group.Names[0])
	case !hasTypes && hasNames:
		or := make([]any, 0, len(s.Group.Names))
		for _, n := range s.Group.Names {
			or = append(or, nameClause(n))
		}
		return map[string]any{fieldOr: or}
	case hasTypes && hasNames:
		or := make([]any, 0, len(s.Group.Names)+1)
		or = append(or, typeClause())
		for _, n := range s.Group.Names {
			or = append(or, nameClause(n))
		}
		return map[string]any{fieldOr: or}
	}
	return nil
}

// LiveFilter is the package-level form of Set.GraphQLFilter, named by the
// cross-package implementation contract.
func LiveFilter(p Predicate) map[string]any { return p.GraphQLFilter() }

// fileDoc is the on-disk grammar, shared by defaults.toml and the user file.
type fileDoc struct {
	SchemaVersion   int                              `toml:"schema_version"`
	StateGroups     map[string]groupTable            `toml:"state_groups"`
	TeamStateGroups map[string]map[string]groupTable `toml:"team_state_groups"`
}

type groupTable struct {
	Types       []string `toml:"types"`
	Names       []string `toml:"names"`
	Description string   `toml:"description"`
}

const supportedSchemaVersion = 1

// Registry holds the merged built-in and user-declared groups.
type Registry struct {
	path      string
	present   bool
	builtin   map[string]Group
	workspace map[string]Group
	team      map[string]map[string]Group // upper-cased team key -> group name -> group
}

// Path reports where the user file was looked for, for meta.groups_path.
func (r *Registry) Path() string { return r.path }

// Present reports whether that file exists. A missing file is not an error.
func (r *Registry) Present() bool { return r.present }

// DirForConfigPath maps a config FILE path to the directory the groups file
// lives in. An empty configPath resolves through the normal config chain, so
// callers can pass the raw --config flag value straight through.
func DirForConfigPath(configPath string) string {
	cfg, err := config.Load(configPath)
	if err != nil || cfg == nil || cfg.Path == "" {
		return ""
	}
	return filepath.Dir(cfg.Path)
}

// PathForDir resolves the groups file inside a directory. $LINEAR_GROUPS
// wins verbatim. An empty directory with no override means built-ins only.
func PathForDir(dir string) string {
	if v := strings.TrimSpace(os.Getenv(EnvGroupsPath)); v != "" {
		return v
	}
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	return filepath.Join(dir, FileName)
}

// PathForConfig resolves the groups file that sits next to a config file.
func PathForConfig(configPath string) string {
	if v := strings.TrimSpace(os.Getenv(EnvGroupsPath)); v != "" {
		return v
	}
	if strings.TrimSpace(configPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), FileName)
}

type memo struct {
	reg *Registry
	err error
}

var (
	memoMu sync.Mutex
	memos  = map[string]memo{}
)

// Load parses defaults.toml plus the user file that sits next to configPath.
// Memoized per resolved path so a malformed file is reported once and a valid
// one is parsed once per process.
func Load(configPath string) (*Registry, error) { return loadPath(PathForConfig(configPath)) }

// LoadDir parses defaults.toml plus the user file inside dir.
func LoadDir(dir string) (*Registry, error) { return loadPath(PathForDir(dir)) }

func loadPath(path string) (*Registry, error) {
	memoMu.Lock()
	defer memoMu.Unlock()
	if m, ok := memos[path]; ok {
		return m.reg, m.err
	}
	reg, err := build(path)
	memos[path] = memo{reg: reg, err: err}
	return reg, err
}

func build(path string) (*Registry, error) {
	r := &Registry{
		path:      path,
		builtin:   map[string]Group{},
		workspace: map[string]Group{},
		team:      map[string]map[string]Group{},
	}

	var defaults fileDoc
	if err := toml.Unmarshal(defaultsTOML, &defaults); err != nil {
		return nil, fmt.Errorf("parsing embedded %s: %w", FileName, err)
	}
	if err := r.absorb(defaults, "", SourceBuiltin); err != nil {
		return nil, err
	}

	if path == "" {
		return r, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	r.present = true

	var user fileDoc
	if err := toml.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if user.SchemaVersion != 0 && user.SchemaVersion != supportedSchemaVersion {
		return nil, fmt.Errorf("%s: unsupported groups schema_version %d", path, user.SchemaVersion)
	}
	if err := r.absorb(user, path, SourceWorkspace); err != nil {
		return nil, err
	}
	return r, nil
}

// absorb validates and installs one parsed document. origin is "" for the
// embedded defaults, which are correct by construction but validated anyway
// so a defaults typo fails loudly at load rather than silently at use.
func (r *Registry) absorb(doc fileDoc, origin, workspaceSource string) error {
	dest := r.builtin
	if workspaceSource == SourceWorkspace {
		dest = r.workspace
	}
	names := make([]string, 0, len(doc.StateGroups))
	for name := range doc.StateGroups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		g, err := normalizeGroup(origin, name, doc.StateGroups[name], workspaceSource, scopeWorkspace)
		if err != nil {
			return err
		}
		dest[g.Name] = g
	}

	teamKeys := make([]string, 0, len(doc.TeamStateGroups))
	for key := range doc.TeamStateGroups {
		teamKeys = append(teamKeys, key)
	}
	sort.Strings(teamKeys)
	for _, key := range teamKeys {
		upper := strings.ToUpper(strings.TrimSpace(key))
		if upper == "" {
			return fmt.Errorf("%s: team_state_groups has an empty team key", describeOrigin(origin))
		}
		tableNames := make([]string, 0, len(doc.TeamStateGroups[key]))
		for name := range doc.TeamStateGroups[key] {
			tableNames = append(tableNames, name)
		}
		sort.Strings(tableNames)
		for _, name := range tableNames {
			g, err := normalizeGroup(origin, name, doc.TeamStateGroups[key][name], sourceTeamPfx+upper, scopeTeamPfx+upper)
			if err != nil {
				return err
			}
			if r.team[upper] == nil {
				r.team[upper] = map[string]Group{}
			}
			r.team[upper][g.Name] = g
		}
	}
	return nil
}

func describeOrigin(origin string) string {
	if origin == "" {
		return FileName
	}
	return origin
}

func normalizeGroup(origin, rawName string, t groupTable, source, scope string) (Group, error) {
	where := describeOrigin(origin)
	name := strings.TrimSpace(rawName)
	if !groupNamePattern.MatchString(name) {
		return Group{}, fmt.Errorf("%s: group name %q must match %s", where, rawName, groupNamePattern.String())
	}
	if name == ReservedAll {
		return Group{}, fmt.Errorf("%s: %q is reserved and always means no state filter", where, ReservedAll)
	}
	types := make([]string, 0, len(t.Types))
	for _, raw := range t.Types {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if !IsAPIType(v) {
			return Group{}, fmt.Errorf("%s: group %q lists %q, which is not a valid Linear workflow state type. Valid types: %s", where, name, raw, APITypeList)
		}
		types = append(types, v)
	}
	names := make([]string, 0, len(t.Names))
	for _, raw := range t.Names {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		names = append(names, v)
	}
	if len(types) == 0 && len(names) == 0 {
		return Group{}, fmt.Errorf("%s: group %q declares no types and no names", where, name)
	}
	return Group{
		Name:        name,
		Source:      source,
		Scope:       scope,
		ShadowedBy:  []string{},
		Description: strings.TrimSpace(t.Description),
		Types:       types,
		Names:       names,
	}, nil
}

func reservedAllGroup() Group {
	return Group{
		Name:        ReservedAll,
		Source:      SourceBuiltin,
		Scope:       scopeWorkspace,
		Reserved:    true,
		ShadowedBy:  []string{},
		Description: "No state predicate. Matches every workflow state.",
		Types:       []string{},
		Names:       []string{},
	}
}

func unfilteredSet(token, resolvedAs string) Set {
	return Set{Group: reservedAllGroup(), Unfiltered: true, ResolvedAs: resolvedAs, Token: token}
}

// Resolve is THE entry point. Every state filter in the CLI goes through it.
// teamKey may be "". Resolution order, first match wins, after an
// unconditional trim-plus-lowercase of the token:
//
//	1 empty token          no predicate
//	2 "all"                no predicate, reserved
//	3 "type:<t>"           the raw API type, bypassing every group
//	4 "name:<n>"           one literal state name, bypassing every group
//	5 team-scoped group    replaces, never merges with, the workspace group
//	6 workspace group      user config beats built-in of the same name
//	7 built-in group       covers "active" and the seven trivial type groups
//	8 raw API type         redundancy guard, unreachable in a correct build
//	9 otherwise            ErrUnknownToken, listing everything available
func (r *Registry) Resolve(teamKey, token string) (Set, error) {
	trimmed := strings.TrimSpace(token)
	lower := strings.ToLower(trimmed)

	if lower == "" {
		return unfilteredSet(lower, ResolvedUnfiltered), nil
	}
	if lower == ReservedAll {
		return unfilteredSet(lower, ResolvedUnfiltered), nil
	}

	if strings.HasPrefix(lower, prefixType) {
		want := strings.ToLower(strings.TrimSpace(trimmed[len(prefixType):]))
		if !IsAPIType(want) {
			return Set{}, fmt.Errorf("%w: %q is not a valid Linear workflow state type. Valid types: %s", ErrUnknownToken, want, APITypeList)
		}
		return Set{
			Group: Group{
				Name:       want,
				Source:     SourceBuiltin,
				Scope:      scopeWorkspace,
				ShadowedBy: []string{},
				Types:      []string{want},
				Names:      []string{},
			},
			ResolvedAs: ResolvedRawType,
			Token:      lower,
		}, nil
	}

	if strings.HasPrefix(lower, prefixName) {
		want := strings.TrimSpace(trimmed[len(prefixName):])
		if want == "" {
			return Set{}, fmt.Errorf("%w: %q names no state", ErrUnknownToken, trimmed)
		}
		return Set{
			Group: Group{
				Name:       want,
				Source:     SourceBuiltin,
				Scope:      scopeWorkspace,
				ShadowedBy: []string{},
				Types:      []string{},
				Names:      []string{want},
			},
			ResolvedAs: ResolvedLiteralName,
			Token:      lower,
		}, nil
	}

	if key := strings.ToUpper(strings.TrimSpace(teamKey)); key != "" {
		if g, ok := r.team[key][lower]; ok {
			return Set{Group: g, ResolvedAs: ResolvedGroup, Token: lower}, nil
		}
	}
	if g, ok := r.workspace[lower]; ok {
		return Set{Group: g, ResolvedAs: ResolvedGroup, Token: lower}, nil
	}
	if g, ok := r.builtin[lower]; ok {
		return Set{Group: g, ResolvedAs: ResolvedGroup, Token: lower}, nil
	}
	if IsAPIType(lower) {
		return Set{
			Group: Group{
				Name:       lower,
				Source:     SourceBuiltin,
				Scope:      scopeWorkspace,
				ShadowedBy: []string{},
				Types:      []string{lower},
				Names:      []string{},
			},
			ResolvedAs: ResolvedRawType,
			Token:      lower,
		}, nil
	}
	return Set{}, fmt.Errorf("%w: %q. Available groups: %s. Raw types: %s. Run 'groups list' to see where each one is declared",
		ErrUnknownToken, trimmed, strings.Join(r.availableNames(teamKey), ", "), APITypeList)
}

func (r *Registry) availableNames(teamKey string) []string {
	seen := map[string]struct{}{ReservedAll: {}}
	for name := range r.builtin {
		seen[name] = struct{}{}
	}
	for name := range r.workspace {
		seen[name] = struct{}{}
	}
	if key := strings.ToUpper(strings.TrimSpace(teamKey)); key != "" {
		for name := range r.team[key] {
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Effective enumerates every group visible at a scope, for `groups list`.
// Shadowed entries are listed too, each naming the higher-precedence entries
// that win over it, so the user can see why a token resolves the way it does.
// An empty teamKey lists every team's declarations. A non-empty one lists
// only that team's.
func (r *Registry) Effective(teamKey string) []Group {
	want := strings.ToUpper(strings.TrimSpace(teamKey))
	teamKeys := make([]string, 0, len(r.team))
	for key := range r.team {
		if want != "" && key != want {
			continue
		}
		teamKeys = append(teamKeys, key)
	}
	sort.Strings(teamKeys)

	shadowsOf := func(name string, includeWorkspace bool) []string {
		out := []string{}
		if includeWorkspace {
			if _, ok := r.workspace[name]; ok {
				out = append(out, SourceWorkspace)
			}
		}
		for _, key := range teamKeys {
			if _, ok := r.team[key][name]; ok {
				out = append(out, sourceTeamPfx+key)
			}
		}
		return out
	}

	out := make([]Group, 0, len(r.builtin)+len(r.workspace)+1)
	all := reservedAllGroup()
	out = append(out, all)
	for _, g := range r.builtin {
		g.ShadowedBy = shadowsOf(g.Name, true)
		out = append(out, g)
	}
	for _, g := range r.workspace {
		g.ShadowedBy = shadowsOf(g.Name, false)
		out = append(out, g)
	}
	for _, key := range teamKeys {
		for _, g := range r.team[key] {
			g.ShadowedBy = []string{}
			out = append(out, g)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return sourceRank(out[i].Source) < sourceRank(out[j].Source)
	})
	return out
}

func sourceRank(source string) int {
	switch {
	case source == SourceBuiltin:
		return 0
	case source == SourceWorkspace:
		return 1
	default:
		return 2
	}
}

// Resolve is the package-level form named by the cross-package implementation
// contract. cfgDir is a DIRECTORY. Passing "" uses the default config chain,
// which honors $LINEAR_CONFIG. $LINEAR_GROUPS still overrides the resolved
// file either way.
func Resolve(cfgDir, teamKey, name string) (Predicate, error) {
	reg, err := LoadDir(resolveDir(cfgDir))
	if err != nil {
		return Predicate{}, err
	}
	return reg.Resolve(teamKey, name)
}

// List is the package-level form of Registry.Effective.
func List(cfgDir, teamKey string) ([]GroupInfo, error) {
	reg, err := LoadDir(resolveDir(cfgDir))
	if err != nil {
		return nil, err
	}
	return reg.Effective(teamKey), nil
}

func resolveDir(cfgDir string) string {
	if strings.TrimSpace(cfgDir) != "" {
		return cfgDir
	}
	return DirForConfigPath("")
}

// MisplacedInConfig reports whether the config file carries group tables that
// this package deliberately does not honor. Callers warn about it: config.save
// rewrites config.toml wholesale from a fixed struct, so declarations left
// there are deleted the first time the user rotates a token.
func MisplacedInConfig(configPath string) bool {
	path := strings.TrimSpace(configPath)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return false
	}
	_, hasWorkspace := doc["state_groups"]
	_, hasTeam := doc["team_state_groups"]
	return hasWorkspace || hasTeam
}
