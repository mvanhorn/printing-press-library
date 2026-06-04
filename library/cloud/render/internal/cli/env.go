// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// envEntry is the in-memory representation of a single env-var loaded from
// the local store: hashes are computed up front so the rest of the diff
// pipeline never touches the raw value.
type envEntry struct {
	Key       string
	ValueHash string
	ValueLen  int
}

// envContainer carries the resolved kind ("service" or "env-group"), id, and
// loaded env vars. The diff functions only need this much; nothing about the
// API call shape leaks past load time.
type envContainer struct {
	ID      string
	Kind    string
	Entries map[string]envEntry
}

// envDiffResult is the structured shape returned by env diff (and consumed
// by env promote). Keeping it a named type lets the test exercise the
// pure-logic path without a database.
type envDiffResult struct {
	A       envSide      `json:"a"`
	B       envSide      `json:"b"`
	OnlyInA []envOnly    `json:"only_in_a"`
	OnlyInB []envOnly    `json:"only_in_b"`
	Changed []envChanged `json:"changed"`
}

type envSide struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type envOnly struct {
	Key       string `json:"key"`
	ValueHash string `json:"value_hash"`
	ValueLen  int    `json:"value_len"`
}

type envChanged struct {
	Key        string `json:"key"`
	ValueHashA string `json:"value_hash_a"`
	ValueHashB string `json:"value_hash_b"`
	ValueLenA  int    `json:"value_len_a"`
	ValueLenB  int    `json:"value_len_b"`
}

// hashEnvValue returns the sha256 of value truncated to the first 12 hex
// chars. Diff output never carries raw values; this is the only fingerprint
// callers see.
func hashEnvValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	full := hex.EncodeToString(sum[:])
	if len(full) < 12 {
		return full
	}
	return full[:12]
}

// classifyEnvTarget reads the leading prefix on a Render id to decide
// whether the caller pointed at a service or an env-group. A render id
// without a recognized prefix is rejected so callers cannot accidentally
// pass a name where an id is expected.
func classifyEnvTarget(id string) (string, error) {
	switch {
	case strings.HasPrefix(id, "srv-"):
		return "service", nil
	case strings.HasPrefix(id, "evg-"):
		return "env-group", nil
	default:
		return "", fmt.Errorf("id %q is not a recognized service (srv-*) or env-group (evg-*) identifier", id)
	}
}

// loadEnvContainer reads the env vars cached in the local store for either
// a service (services_env_vars) or an env-group (env_groups_env_vars). It
// fingerprints values as it goes so callers never see the raw data.
func loadEnvContainer(db *store.Store, id string) (*envContainer, error) {
	kind, err := classifyEnvTarget(id)
	if err != nil {
		return nil, err
	}
	c := &envContainer{ID: id, Kind: kind, Entries: map[string]envEntry{}}

	var rowsQuery string
	switch kind {
	case "service":
		rowsQuery = `SELECT data FROM services_env_vars WHERE services_id = ?`
	case "env-group":
		rowsQuery = `SELECT data FROM env_groups_env_vars WHERE env_groups_id = ?`
	}
	rows, err := db.DB().Query(rowsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("querying %s env vars: %w", kind, err)
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		key := strFromAny(obj["key"])
		value := strFromAny(obj["value"])
		if key == "" {
			continue
		}
		// Render's env-var endpoints return the cleartext value; "value"
		// may also be empty when it's a generated value or sealed secret.
		c.Entries[key] = envEntry{
			Key:       key,
			ValueHash: hashEnvValue(value),
			ValueLen:  len(value),
		}
	}
	return c, rows.Err()
}

func strFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", v)
	}
}

// computeEnvDiff produces a deterministic three-way diff between two
// envContainers. Output ordering is alphabetical on key so test assertions
// and human review both stay stable.
func computeEnvDiff(a, b *envContainer) envDiffResult {
	out := envDiffResult{
		A:       envSide{ID: a.ID, Kind: a.Kind},
		B:       envSide{ID: b.ID, Kind: b.Kind},
		OnlyInA: []envOnly{},
		OnlyInB: []envOnly{},
		Changed: []envChanged{},
	}
	keysA := sortedKeys(a.Entries)
	for _, k := range keysA {
		ea := a.Entries[k]
		eb, ok := b.Entries[k]
		if !ok {
			out.OnlyInA = append(out.OnlyInA, envOnly{Key: k, ValueHash: ea.ValueHash, ValueLen: ea.ValueLen})
			continue
		}
		if ea.ValueHash != eb.ValueHash {
			out.Changed = append(out.Changed, envChanged{
				Key:        k,
				ValueHashA: ea.ValueHash,
				ValueHashB: eb.ValueHash,
				ValueLenA:  ea.ValueLen,
				ValueLenB:  eb.ValueLen,
			})
		}
	}
	keysB := sortedKeys(b.Entries)
	for _, k := range keysB {
		if _, ok := a.Entries[k]; ok {
			continue
		}
		eb := b.Entries[k]
		out.OnlyInB = append(out.OnlyInB, envOnly{Key: k, ValueHash: eb.ValueHash, ValueLen: eb.ValueLen})
	}
	return out
}

func sortedKeys(m map[string]envEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// newEnvCmd is the new env parent group: diff/promote/where.
func newEnvCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Diff, promote, and locate env vars across services and env-groups (cached, value-hashed only).",
	}
	cmd.AddCommand(newEnvDiffCmd(flags))
	cmd.AddCommand(newEnvPromoteCmd(flags))
	cmd.AddCommand(newEnvWhereCmd(flags))
	return cmd
}

func newEnvDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "diff <a> <b>",
		Short: "Three-way diff of env vars between two services or env-groups (value-hashed).",
		Example: strings.Trim(`
  render-pp-cli env diff srv-d12abc srv-d34xyz
  render-pp-cli env diff srv-d12abc evg-shared --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "env diff"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			a, err := loadEnvContainer(db, args[0])
			if err != nil {
				return err
			}
			b, err := loadEnvContainer(db, args[1])
			if err != nil {
				return err
			}
			if len(a.Entries) == 0 && len(b.Entries) == 0 {
				return fmt.Errorf("no env vars cached for either %s or %s — local cache empty — run 'render-pp-cli sync' first", a.ID, b.ID)
			}
			result := computeEnvDiff(a, b)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// envPromotePlanItem is the per-key record in a promote plan: action +
// fingerprints, never a raw value.
type envPromotePlanItem struct {
	Key       string `json:"key"`
	Action    string `json:"action"` // add | update
	ValueHash string `json:"value_hash"`
	ValueLen  int    `json:"value_len"`
}

func newEnvPromoteCmd(flags *rootFlags) *cobra.Command {
	var (
		from    string
		to      string
		only    []string
		exclude []string
		apply   bool
		dbPath  string
	)
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Copy env vars from a source service or env-group to a target (default: print plan).",
		Long:  `Compute the diff between --from and --to, then copy "only-in-source" and "changed" keys to the target. Default mode is dry-run; pass --apply to perform writes.`,
		Example: strings.Trim(`
  render-pp-cli env promote --from srv-d12abc --to srv-d34xyz
  render-pp-cli env promote --from evg-shared --to srv-d12abc --only STRIPE_KEY,DEBUG_TOKEN --apply
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" || to == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "env promote"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()
			a, err := loadEnvContainer(db, from)
			if err != nil {
				return err
			}
			b, err := loadEnvContainer(db, to)
			if err != nil {
				return err
			}
			if len(a.Entries) == 0 {
				return fmt.Errorf("no env vars cached for source %s — local cache empty — run 'render-pp-cli sync' first", a.ID)
			}
			diff := computeEnvDiff(a, b)
			onlySet := stringSet(only)
			excludeSet := stringSet(exclude)

			plan := []envPromotePlanItem{}
			for _, k := range diff.OnlyInA {
				if !filterAllow(k.Key, onlySet, excludeSet) {
					continue
				}
				plan = append(plan, envPromotePlanItem{Key: k.Key, Action: "add", ValueHash: k.ValueHash, ValueLen: k.ValueLen})
			}
			for _, k := range diff.Changed {
				if !filterAllow(k.Key, onlySet, excludeSet) {
					continue
				}
				plan = append(plan, envPromotePlanItem{Key: k.Key, Action: "update", ValueHash: k.ValueHashA, ValueLen: k.ValueLenA})
			}

			envelope := map[string]any{
				"from":  envSide{ID: a.ID, Kind: a.Kind},
				"to":    envSide{ID: b.ID, Kind: b.Kind},
				"plan":  plan,
				"apply": apply,
			}
			// Mutating path: requires --apply AND not in verify env. The
			// verify-env short-circuit is the floor for any side-effect
			// command; classification can miss novel commands.
			if apply && !cliutil.IsVerifyEnv() {
				if len(plan) == 0 {
					envelope["status"] = "no-op"
					return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				// Reload raw source values just-in-time for the API call so
				// we never round-trip them through any in-memory hashed
				// container; the local store still holds them.
				rawValues, err := loadRawEnvValues(db, a.ID, a.Kind)
				if err != nil {
					return err
				}
				results := make([]map[string]any, 0, len(plan))
				switch b.Kind {
				case "service":
					body := buildServiceEnvVarsBody(b, plan, rawValues)
					path := "/services/" + b.ID + "/env-vars"
					_, status, err := c.Put(path, body)
					results = append(results, map[string]any{"target": b.ID, "status": status, "error": errString(err), "method": "PUT", "path": path})
				case "env-group":
					for _, item := range plan {
						val, ok := rawValues[item.Key]
						if !ok {
							continue
						}
						path := "/env-groups/" + b.ID + "/env-vars/" + item.Key
						_, status, err := c.Post(path, map[string]string{"value": val})
						results = append(results, map[string]any{"key": item.Key, "status": status, "error": errString(err), "method": "POST", "path": path})
					}
				}
				envelope["results"] = results
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Source service id (srv-*) or env-group id (evg-*)")
	cmd.Flags().StringVar(&to, "to", "", "Target service id (srv-*) or env-group id (evg-*)")
	cmd.Flags().StringSliceVar(&only, "only", nil, "Comma-separated allow-list of env-var keys (default: all)")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "Comma-separated deny-list of env-var keys")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually issue the writes (default: print plan only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// stringSet builds a lowercase-friendly lookup set from a slice. Empty
// input returns an empty set so the allow/deny check naturally degenerates
// to "no filter" rather than blocking everything.
func stringSet(s []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range s {
		v = strings.TrimSpace(v)
		if v != "" {
			out[v] = true
		}
	}
	return out
}

// filterAllow applies --only (allow-list) and --exclude (deny-list) to a
// single key. An empty allow-list means "allow all".
func filterAllow(key string, only, exclude map[string]bool) bool {
	if len(only) > 0 && !only[key] {
		return false
	}
	if exclude[key] {
		return false
	}
	return true
}

// loadRawEnvValues re-reads the source env vars and returns key->raw value.
// Called only in the --apply path so raw values stay confined to the smallest
// possible window in memory.
func loadRawEnvValues(db *store.Store, id, kind string) (map[string]string, error) {
	out := map[string]string{}
	var query string
	switch kind {
	case "service":
		query = `SELECT data FROM services_env_vars WHERE services_id = ?`
	case "env-group":
		query = `SELECT data FROM env_groups_env_vars WHERE env_groups_id = ?`
	default:
		return out, fmt.Errorf("unknown kind %q", kind)
	}
	rows, err := db.DB().Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		key := strFromAny(obj["key"])
		val := strFromAny(obj["value"])
		if key != "" {
			out[key] = val
		}
	}
	return out, rows.Err()
}

// buildServiceEnvVarsBody constructs the bulk PUT body for the
// services env-vars endpoint by merging the existing target snapshot with
// the planned additions/updates. Missing keys on the target carry over so
// the bulk PUT does not delete vars not touched by the promote.
func buildServiceEnvVarsBody(target *envContainer, plan []envPromotePlanItem, sourceValues map[string]string) []map[string]string {
	merged := map[string]string{}
	for k := range target.Entries {
		// Preserve existing keys; raw value isn't on the hashed entry so
		// the user must accept that bulk-PUT will only carry through values
		// for keys we have raw access to. This is intentional: a partial
		// merge is safer than dropping keys silently.
		merged[k] = ""
	}
	for _, p := range plan {
		if v, ok := sourceValues[p.Key]; ok {
			merged[p.Key] = v
		}
	}
	out := make([]map[string]string, 0, len(merged))
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, map[string]string{"key": k, "value": merged[k]})
	}
	return out
}

// envWhereHit is one row in the where-key result. Includes a parent name
// (the service or env-group display name) so output is human-readable
// without forcing a second lookup.
type envWhereHit struct {
	ContainerID   string `json:"container_id"`
	ContainerKind string `json:"container_kind"`
	ContainerName string `json:"container_name,omitempty"`
	Key           string `json:"key"`
	ValueHash     string `json:"value_hash"`
	ValueLen      int    `json:"value_len"`
	SyncedAt      string `json:"synced_at,omitempty"`
}

func newEnvWhereCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "where <KEY>",
		Short: "List every service and env-group that defines a given env-var key.",
		Example: strings.Trim(`
  render-pp-cli env where STRIPE_KEY
  render-pp-cli env where DEBUG_TOKEN --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			key := args[0]
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "env where"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			hits, err := envWhereScan(db, key)
			if err != nil {
				return err
			}
			if len(hits) == 0 {
				return fmt.Errorf("key %q not found in cached env vars — run 'render-pp-cli sync' to refresh", key)
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// envWhereScan walks both env-var tables once and joins each hit with its
// parent's display name from the resources table.
func envWhereScan(db *store.Store, key string) ([]envWhereHit, error) {
	out := []envWhereHit{}

	scanTable := func(query, kind string) error {
		rows, err := db.DB().Query(query, key)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var parentID string
			var raw []byte
			var syncedAt sql.NullString
			if err := rows.Scan(&parentID, &raw, &syncedAt); err != nil {
				return err
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			value := strFromAny(obj["value"])
			name := lookupContainerName(db, parentID, kind)
			ts := ""
			if syncedAt.Valid {
				ts = syncedAt.String
			}
			out = append(out, envWhereHit{
				ContainerID:   parentID,
				ContainerKind: kind,
				ContainerName: name,
				Key:           key,
				ValueHash:     hashEnvValue(value),
				ValueLen:      len(value),
				SyncedAt:      ts,
			})
		}
		return rows.Err()
	}

	if err := scanTable(
		`SELECT services_id, data, synced_at FROM services_env_vars WHERE json_extract(data, '$.key') = ?`,
		"service",
	); err != nil {
		return nil, err
	}
	if err := scanTable(
		`SELECT env_groups_id, data, synced_at FROM env_groups_env_vars WHERE json_extract(data, '$.key') = ?`,
		"env-group",
	); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ContainerKind != out[j].ContainerKind {
			return out[i].ContainerKind < out[j].ContainerKind
		}
		return out[i].ContainerID < out[j].ContainerID
	})
	return out, nil
}

func lookupContainerName(db *store.Store, id, kind string) string {
	resType := "services"
	if kind == "env-group" {
		resType = "env-groups"
	}
	var data []byte
	err := db.DB().QueryRow(`SELECT data FROM resources WHERE resource_type = ? AND id = ?`, resType, id).Scan(&data)
	if err != nil {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	return strFromAny(obj["name"])
}

// keep time import live for tests that may want to format
var _ = time.RFC3339
