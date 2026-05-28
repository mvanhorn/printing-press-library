// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// blueprintDoc is a permissive shape for a Render render.yaml blueprint.
// We intentionally model only the fields drift compares: name, type, and a
// small set of declared spec fields. Everything else is preserved as raw
// JSON in case a future caller wants it.
type blueprintDoc struct {
	Services  []blueprintEntity `yaml:"services"`
	EnvGroups []blueprintEntity `yaml:"envVarGroups"`
	Databases []blueprintEntity `yaml:"databases"`
}

type blueprintEntity struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Plan    string `yaml:"plan"`
	Region  string `yaml:"region"`
	Branch  string `yaml:"branch"`
	EnvVars []bpKV `yaml:"envVars"`
}

type bpKV struct {
	Key string `yaml:"key"`
}

// driftEntity is a normalized live-state record used by both sides of the
// comparison; loaded from the local store. Type is kept open since blueprints
// and live state name service types differently in some Render tiers.
type driftEntity struct {
	Kind    string // service | env-group | database
	Name    string
	Type    string
	Plan    string
	Region  string
	Branch  string
	EnvKeys map[string]bool
}

// driftReport is the JSON output shape — also drives the text renderer.
type driftReport struct {
	Added    []driftAddedEntity    `json:"added"`
	Removed  []driftRemovedEntity  `json:"removed"`
	Modified []driftModifiedEntity `json:"modified"`
}

type driftAddedEntity struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Plan   string `json:"plan,omitempty"`
	Region string `json:"region,omitempty"`
}

type driftRemovedEntity struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Plan string `json:"plan,omitempty"`
}

type driftModifiedEntity struct {
	Kind    string        `json:"kind"`
	Name    string        `json:"name"`
	Changes []driftChange `json:"changes"`
}

type driftChange struct {
	Field   string   `json:"field"`
	From    string   `json:"from,omitempty"`
	To      string   `json:"to,omitempty"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		blueprintPath string
		dbPath        string
	)
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare a checked-in render.yaml against live cached state and report added/removed/modified entities.",
		Long:  `Reads render.yaml (default ./render.yaml) and the local cache populated by 'render-pp-cli sync'. Reports services, env-groups, and databases declared in the blueprint vs. live state. Exits 0 on no drift, 2 when drift is detected.`,
		Example: strings.Trim(`
  render-pp-cli drift
  render-pp-cli drift --blueprint config/render.yaml
  render-pp-cli drift --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "drift"}`)
				return nil
			}
			path := blueprintPath
			if path == "" {
				path = "render.yaml"
			}
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no blueprint found at %s — pass --blueprint or run from a directory containing render.yaml", path)
				}
				return fmt.Errorf("reading blueprint %s: %w", path, err)
			}
			bp, err := parseBlueprint(data)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			live, err := loadLiveDriftState(db)
			if err != nil {
				return err
			}
			if len(live) == 0 {
				return fmt.Errorf("local cache empty — run 'render-pp-cli sync' first")
			}
			declared := blueprintToEntities(bp)
			report := computeDrift(declared, live)

			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
					return err
				}
			} else {
				renderDriftText(cmd.OutOrStdout(), report)
			}
			if hasDrift(report) {
				return &cliError{code: 2, err: fmt.Errorf("drift detected")}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&blueprintPath, "blueprint", "", "Path to the Render blueprint (default: ./render.yaml)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// parseBlueprint parses the minimal subset of render.yaml drift needs without
// requiring a yaml library: it walks lines and extracts service/env-group/
// database blocks by indent level. This is intentional — adding a yaml
// dependency for one consumer is a heavy lift the generator would have to
// also wire. Indent tracking lets us distinguish a top-level entity ("- ")
// at indent 2 from a nested envVars entry ("- key: X") at indent 6.
func parseBlueprint(data []byte) (*blueprintDoc, error) {
	out := &blueprintDoc{}
	lines := strings.Split(string(data), "\n")
	section := "" // services | envVarGroups | databases
	current := blueprintEntity{}
	entityIndent := -1 // column where the entity-level "- " sits
	inEnvVars := false
	flush := func() {
		if current.Name == "" && len(current.EnvVars) == 0 {
			return
		}
		switch section {
		case "services":
			out.Services = append(out.Services, current)
		case "envVarGroups":
			out.EnvGroups = append(out.EnvGroups, current)
		case "databases":
			out.Databases = append(out.Databases, current)
		}
		current = blueprintEntity{}
		inEnvVars = false
	}
	leadSpaces := func(s string) int {
		n := 0
		for _, r := range s {
			if r == ' ' {
				n++
				continue
			}
			if r == '\t' {
				// expand tab to 4 spaces for indent accounting; render.yaml
				// is conventionally space-indented but tab-tolerant parsing
				// is cheap insurance.
				n += 4
				continue
			}
			break
		}
		return n
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadSpaces(line)
		// top-level section start (indent 0, ends with :)
		if indent == 0 {
			flush()
			head := strings.TrimSuffix(trimmed, ":")
			switch strings.TrimSpace(head) {
			case "services":
				section = "services"
			case "envVarGroups":
				section = "envVarGroups"
			case "databases":
				section = "databases"
			default:
				section = ""
			}
			entityIndent = -1
			continue
		}
		// "- " starts either a new top-level entity (when indent matches the
		// entity column) or a list item under a nested key like envVars
		// (deeper indent).
		stripped := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(stripped, "- ") {
			rest := strings.TrimSpace(strings.TrimPrefix(stripped, "-"))
			if entityIndent < 0 {
				entityIndent = indent
			}
			if indent == entityIndent {
				flush()
				if k, v, ok := splitYAMLPair(rest); ok {
					assignBlueprintField(&current, k, v)
				}
				inEnvVars = false
				continue
			}
			// nested list item (e.g. - key: STRIPE_KEY under envVars)
			if inEnvVars {
				if k, v, ok := splitYAMLPair(rest); ok && k == "key" {
					current.EnvVars = append(current.EnvVars, bpKV{Key: v})
				}
			}
			continue
		}
		// "key: value" inside an entity
		k, v, ok := splitYAMLPair(stripped)
		if !ok {
			continue
		}
		if k == "envVars" {
			inEnvVars = true
			continue
		}
		if inEnvVars && k == "key" {
			current.EnvVars = append(current.EnvVars, bpKV{Key: v})
			continue
		}
		assignBlueprintField(&current, k, v)
	}
	flush()
	return out, nil
}

// splitYAMLPair handles a single "key: value" line. Returns ok=false on
// blank/list-only lines so the parser can skip them.
func splitYAMLPair(s string) (string, string, bool) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(s[:idx])
	v := strings.TrimSpace(s[idx+1:])
	v = strings.Trim(v, "\"'")
	if k == "" {
		return "", "", false
	}
	return k, v, true
}

func assignBlueprintField(e *blueprintEntity, k, v string) {
	switch k {
	case "name":
		e.Name = v
	case "type":
		e.Type = v
	case "plan":
		e.Plan = v
	case "region":
		e.Region = v
	case "branch":
		e.Branch = v
	}
}

// blueprintToEntities flattens the parsed doc into kind-tagged entities.
func blueprintToEntities(bp *blueprintDoc) map[string]driftEntity {
	out := map[string]driftEntity{}
	add := func(kind string, items []blueprintEntity) {
		for _, e := range items {
			if e.Name == "" {
				continue
			}
			ent := driftEntity{
				Kind:    kind,
				Name:    e.Name,
				Type:    e.Type,
				Plan:    e.Plan,
				Region:  e.Region,
				Branch:  e.Branch,
				EnvKeys: map[string]bool{},
			}
			for _, kv := range e.EnvVars {
				if kv.Key != "" {
					ent.EnvKeys[kv.Key] = true
				}
			}
			out[kind+"/"+e.Name] = ent
		}
	}
	add("service", bp.Services)
	add("env-group", bp.EnvGroups)
	add("database", bp.Databases)
	return out
}

// loadLiveDriftState reads services, env-groups, and databases from the
// local store and converts them to the same driftEntity shape as the
// blueprint side. Env-key sets for env-groups come from env_groups_env_vars.
func loadLiveDriftState(db *store.Store) (map[string]driftEntity, error) {
	out := map[string]driftEntity{}

	loadResources := func(kind, resourceType string) error {
		rows, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type = ?`, resourceType)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				return err
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			name := strFromAny(obj["name"])
			if name == "" {
				continue
			}
			plan := strFromAny(obj["plan"])
			region := strFromAny(obj["region"])
			typ := strFromAny(obj["type"])
			branch := strFromAny(obj["branch"])
			out[kind+"/"+name] = driftEntity{
				Kind:    kind,
				Name:    name,
				Type:    typ,
				Plan:    plan,
				Region:  region,
				Branch:  branch,
				EnvKeys: map[string]bool{},
			}
		}
		return rows.Err()
	}
	if err := loadResources("service", "services"); err != nil {
		return nil, err
	}
	if err := loadResources("env-group", "env-groups"); err != nil {
		return nil, err
	}
	if err := loadResources("database", "postgres"); err != nil {
		return nil, err
	}

	// Env-group env keys live in env_groups_env_vars; index by group id then
	// by name via the resources table.
	groupNameByID := map[string]string{}
	for k, e := range out {
		_ = k
		if e.Kind != "env-group" {
			continue
		}
	}
	groupRows, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type = 'env-groups'`)
	if err == nil {
		defer groupRows.Close()
		for groupRows.Next() {
			var id string
			var raw []byte
			if err := groupRows.Scan(&id, &raw); err != nil {
				continue
			}
			var obj map[string]any
			if json.Unmarshal(raw, &obj) == nil {
				groupNameByID[id] = strFromAny(obj["name"])
			}
		}
	}
	envRows, err := db.DB().Query(`SELECT env_groups_id, data FROM env_groups_env_vars`)
	if err == nil {
		defer envRows.Close()
		for envRows.Next() {
			var groupID string
			var raw []byte
			if err := envRows.Scan(&groupID, &raw); err != nil {
				continue
			}
			var obj map[string]any
			if json.Unmarshal(raw, &obj) != nil {
				continue
			}
			key := strFromAny(obj["key"])
			name := groupNameByID[groupID]
			if key == "" || name == "" {
				continue
			}
			full := "env-group/" + name
			ent, ok := out[full]
			if !ok {
				continue
			}
			if ent.EnvKeys == nil {
				ent.EnvKeys = map[string]bool{}
			}
			ent.EnvKeys[key] = true
			out[full] = ent
		}
	}

	return out, nil
}

// computeDrift returns the canonical drift report for declared vs. live.
// Output ordering is alphabetical for deterministic diffs.
func computeDrift(declared, live map[string]driftEntity) driftReport {
	rep := driftReport{Added: []driftAddedEntity{}, Removed: []driftRemovedEntity{}, Modified: []driftModifiedEntity{}}
	declaredKeys := sortedMapKeys(declared)
	liveKeys := sortedMapKeys(live)
	for _, k := range declaredKeys {
		d := declared[k]
		l, ok := live[k]
		if !ok {
			rep.Added = append(rep.Added, driftAddedEntity{Kind: d.Kind, Name: d.Name, Type: d.Type, Plan: d.Plan, Region: d.Region})
			continue
		}
		var changes []driftChange
		if d.Plan != "" && l.Plan != "" && d.Plan != l.Plan {
			changes = append(changes, driftChange{Field: "plan", From: l.Plan, To: d.Plan})
		}
		if d.Region != "" && l.Region != "" && d.Region != l.Region {
			changes = append(changes, driftChange{Field: "region", From: l.Region, To: d.Region})
		}
		if d.Branch != "" && l.Branch != "" && d.Branch != l.Branch {
			changes = append(changes, driftChange{Field: "branch", From: l.Branch, To: d.Branch})
		}
		if len(d.EnvKeys) > 0 || len(l.EnvKeys) > 0 {
			added, removed := diffStringSets(d.EnvKeys, l.EnvKeys)
			if len(added) > 0 || len(removed) > 0 {
				changes = append(changes, driftChange{Field: "env-keys", Added: added, Removed: removed})
			}
		}
		if len(changes) > 0 {
			rep.Modified = append(rep.Modified, driftModifiedEntity{Kind: d.Kind, Name: d.Name, Changes: changes})
		}
	}
	for _, k := range liveKeys {
		if _, ok := declared[k]; ok {
			continue
		}
		l := live[k]
		rep.Removed = append(rep.Removed, driftRemovedEntity{Kind: l.Kind, Name: l.Name, Type: l.Type, Plan: l.Plan})
	}
	return rep
}

func sortedMapKeys(m map[string]driftEntity) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// diffStringSets returns (added=in a, not b), (removed=in b, not a). The
// declared side is "a" — added means the blueprint declares it but live
// doesn't have it.
func diffStringSets(a, b map[string]bool) ([]string, []string) {
	var added, removed []string
	for k := range a {
		if !b[k] {
			added = append(added, k)
		}
	}
	for k := range b {
		if !a[k] {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func hasDrift(rep driftReport) bool {
	return len(rep.Added) > 0 || len(rep.Removed) > 0 || len(rep.Modified) > 0
}

func renderDriftText(w interface {
	Write([]byte) (int, error)
}, rep driftReport) {
	if len(rep.Added) > 0 {
		fmt.Fprintln(w, "ADDED (in blueprint, not live):")
		for _, e := range rep.Added {
			fmt.Fprintf(w, "  + %s %q (%s, plan=%s, region=%s)\n", e.Kind, e.Name, e.Type, e.Plan, e.Region)
		}
	}
	if len(rep.Removed) > 0 {
		fmt.Fprintln(w, "REMOVED (live, not in blueprint):")
		for _, e := range rep.Removed {
			fmt.Fprintf(w, "  - %s %q (%s, plan=%s)\n", e.Kind, e.Name, e.Type, e.Plan)
		}
	}
	if len(rep.Modified) > 0 {
		fmt.Fprintln(w, "MODIFIED:")
		for _, e := range rep.Modified {
			parts := []string{}
			for _, c := range e.Changes {
				if c.Field == "env-keys" {
					seg := ""
					if len(c.Added) > 0 {
						seg += "added " + strings.Join(c.Added, ", ")
					}
					if len(c.Removed) > 0 {
						if seg != "" {
							seg += "; "
						}
						seg += "removed " + strings.Join(c.Removed, ", ")
					}
					parts = append(parts, seg)
					continue
				}
				parts = append(parts, fmt.Sprintf("%s: %s -> %s", c.Field, c.From, c.To))
			}
			fmt.Fprintf(w, "  ~ %s %q: %s\n", e.Kind, e.Name, strings.Join(parts, "; "))
		}
	}
	if !hasDrift(rep) {
		fmt.Fprintln(w, "No drift detected.")
	}
}
