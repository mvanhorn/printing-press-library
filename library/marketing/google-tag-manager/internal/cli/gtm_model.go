// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// GTM domain model: the local snapshot/entity store, the container "pull"
// walker, the tag/trigger/variable reference index, and the shared audit /
// diff / fleet logic that the read-only transcendence commands build on.
// Hand-authored (no generator header) so it survives regeneration.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/internal/store"
)

// allPagesTriggerID is GTM's reserved built-in "All Pages" trigger id. Tags
// whose firingTriggerId includes it fire on every page load.
const allPagesTriggerID = "2147479553"

// gtmEntityKinds are the resource kinds a container pull mirrors. Order is the
// natural read order (containers down to leaf config).
var gtmEntityKinds = []string{
	"tag", "trigger", "variable", "builtInVariable",
	"folder", "template", "client", "zone", "gtagConfig",
}

// gtmContainerVersion mirrors the subset of the GTM ContainerVersion /
// workspace responses we persist. Every leaf is also kept as raw JSON.
var measurementIDRe = regexp.MustCompile(`\b(G-[A-Z0-9]{6,}|AW-[0-9]{6,}|GT-[A-Z0-9]{6,}|UA-[0-9]{4,}-[0-9]+|DC-[0-9]{6,})\b`)
var variableRefRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

type gtmSnapshot struct {
	ID            int64
	AccountID     string
	AccountName   string
	ContainerID   string
	ContainerName string
	PublicID      string
	Source        string
	VersionID     string
	PulledAt      string
	Label         string
	EntityCount   int
}

type gtmEntity struct {
	SnapshotID int64
	Kind       string
	EntityID   string
	Name       string
	Type       string
	Data       json.RawMessage
}

// ---------- schema ----------

func ensureGTMSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS gtm_snapshot (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL,
			account_name TEXT,
			container_id TEXT NOT NULL,
			container_name TEXT,
			public_id TEXT,
			source TEXT NOT NULL,
			version_id TEXT,
			pulled_at TEXT NOT NULL,
			label TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gtm_snapshot_container ON gtm_snapshot(container_id, source, pulled_at)`,
		`CREATE TABLE IF NOT EXISTS gtm_entity (
			snapshot_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			entity_id TEXT,
			name TEXT,
			type TEXT,
			data TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gtm_entity_snap_kind ON gtm_entity(snapshot_id, kind)`,
		`CREATE INDEX IF NOT EXISTS idx_gtm_entity_name ON gtm_entity(snapshot_id, name)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("gtm schema: %w", err)
		}
	}
	return nil
}

// openGTMStore opens the local store and guarantees the GTM tables exist.
func openGTMStore(ctx context.Context, dbPath string) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := ensureGTMSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// ---------- pull ----------

// pullContainer fetches a container's configuration from GTM and writes one
// snapshot plus its entities. source is "live" or "workspace:<id>".
func pullContainer(ctx context.Context, c gtmGetter, db *sql.DB, accountID, containerID, source string, nowFn func() time.Time) (gtmSnapshot, error) {
	snap := gtmSnapshot{
		AccountID:   accountID,
		ContainerID: containerID,
		Source:      source,
		PulledAt:    nowFn().UTC().Format(time.RFC3339),
	}
	base := fmt.Sprintf("/tagmanager/v2/accounts/%s/containers/%s", accountID, containerID)

	// Best-effort friendly names from the container record.
	if raw, err := c.Get(ctx, base, nil); err == nil {
		var cont struct {
			Name     string `json:"name"`
			PublicID string `json:"publicId"`
		}
		if json.Unmarshal(raw, &cont) == nil {
			snap.ContainerName = cont.Name
			snap.PublicID = cont.PublicID
		}
	}
	if raw, err := c.Get(ctx, fmt.Sprintf("/tagmanager/v2/accounts/%s", accountID), nil); err == nil {
		var acct struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &acct) == nil {
			snap.AccountName = acct.Name
		}
	}

	var entities []gtmEntity
	if source == "live" {
		raw, err := c.Get(ctx, base+"/versions:live", nil)
		if err != nil {
			return snap, fmt.Errorf("fetching live version: %w", err)
		}
		var ver map[string]json.RawMessage
		if err := json.Unmarshal(raw, &ver); err != nil {
			return snap, fmt.Errorf("parsing live version: %w", err)
		}
		if id, ok := ver["containerVersionId"]; ok {
			snap.VersionID = strings.Trim(string(id), `"`)
		}
		for _, kind := range gtmEntityKinds {
			arr := versionArrayKey(kind)
			if arr == "" {
				continue
			}
			entities = append(entities, parseEntityArray(ver[arr], kind)...)
		}
	} else if strings.HasPrefix(source, "workspace:") {
		wsID := strings.TrimPrefix(source, "workspace:")
		parent := fmt.Sprintf("%s/workspaces/%s", base, wsID)
		for _, kind := range gtmEntityKinds {
			path, key := workspaceListEndpoint(kind)
			if path == "" {
				continue
			}
			items, err := listAll(ctx, c, parent+path, key)
			if err != nil {
				return snap, fmt.Errorf("listing %s: %w", kind, err)
			}
			for _, it := range items {
				entities = append(entities, parseEntity(it, kind))
			}
		}
	} else {
		return snap, fmt.Errorf("unknown pull source %q (use live or workspace:<id>)", source)
	}

	snap.EntityCount = len(entities)
	if snap.Label == "" {
		name := snap.ContainerName
		if name == "" {
			name = containerID
		}
		snap.Label = fmt.Sprintf("%s@%s", name, source)
	}
	id, err := writeSnapshot(ctx, db, snap, entities)
	if err != nil {
		return snap, err
	}
	snap.ID = id
	return snap, nil
}

// gtmGetter is the read-only subset of the generated client the GTM layer
// needs. Keeping it small makes pullContainer unit-testable with a fake.
type gtmGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

func versionArrayKey(kind string) string {
	switch kind {
	case "tag":
		return "tag"
	case "trigger":
		return "trigger"
	case "variable":
		return "variable"
	case "builtInVariable":
		return "builtInVariable"
	case "folder":
		return "folder"
	case "template":
		return "customTemplate"
	case "client":
		return "client"
	case "zone":
		return "zone"
	case "gtagConfig":
		return "gtagConfig"
	}
	return ""
}

func workspaceListEndpoint(kind string) (path, key string) {
	switch kind {
	case "tag":
		return "/tags", "tag"
	case "trigger":
		return "/triggers", "trigger"
	case "variable":
		return "/variables", "variable"
	case "builtInVariable":
		return "/built_in_variables", "builtInVariable"
	case "folder":
		return "/folders", "folder"
	case "template":
		return "/templates", "template"
	case "client":
		return "/clients", "client"
	case "zone":
		return "/zones", "zone"
	case "gtagConfig":
		return "/gtag_config", "gtagConfig"
	}
	return "", ""
}

// listAll pages a GTM list endpoint, following nextPageToken. If the page cap
// is reached with more pages still pending, it warns on stderr rather than
// silently truncating, so a caller can tell a complete pull from a partial one.
func listAll(ctx context.Context, c gtmGetter, path, key string) ([]json.RawMessage, error) {
	const maxPages = 50
	var out []json.RawMessage
	params := map[string]string{}
	for page := 0; page < maxPages; page++ {
		raw, err := c.Get(ctx, path, params)
		if err != nil {
			return nil, err
		}
		var resp map[string]json.RawMessage
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		out = append(out, parseEntityArrayRaw(resp[key])...)
		tok := strings.Trim(string(resp["nextPageToken"]), `"`)
		if tok == "" {
			return out, nil // all pages consumed
		}
		params["pageToken"] = tok
	}
	fmt.Fprintf(os.Stderr, "warning: %s pagination stopped at the %d-page cap; some entities may be missing from this pull\n", path, maxPages)
	return out, nil
}

func parseEntityArrayRaw(arr json.RawMessage) []json.RawMessage {
	if len(arr) == 0 {
		return nil
	}
	var items []json.RawMessage
	_ = json.Unmarshal(arr, &items)
	return items
}

func parseEntityArray(arr json.RawMessage, kind string) []gtmEntity {
	var out []gtmEntity
	for _, it := range parseEntityArrayRaw(arr) {
		out = append(out, parseEntity(it, kind))
	}
	return out
}

func parseEntity(raw json.RawMessage, kind string) gtmEntity {
	var fields struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		TagID      string `json:"tagId"`
		TriggerID  string `json:"triggerId"`
		VariableID string `json:"variableId"`
		TemplateID string `json:"templateId"`
		ClientID   string `json:"clientId"`
		ZoneID     string `json:"zoneId"`
		FolderID   string `json:"folderId"`
		GtagID     string `json:"gtagConfigId"`
	}
	_ = json.Unmarshal(raw, &fields)
	id := firstNonEmpty(fields.TagID, fields.TriggerID, fields.VariableID, fields.TemplateID, fields.ClientID, fields.ZoneID, fields.FolderID, fields.GtagID)
	return gtmEntity{Kind: kind, EntityID: id, Name: fields.Name, Type: fields.Type, Data: raw}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeSnapshot(ctx context.Context, db *sql.DB, snap gtmSnapshot, entities []gtmEntity) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO gtm_snapshot (account_id, account_name, container_id, container_name, public_id, source, version_id, pulled_at, label)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.AccountID, snap.AccountName, snap.ContainerID, snap.ContainerName, snap.PublicID, snap.Source, snap.VersionID, snap.PulledAt, snap.Label)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	for _, e := range entities {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO gtm_entity (snapshot_id, kind, entity_id, name, type, data) VALUES (?, ?, ?, ?, ?, ?)`,
			id, e.Kind, e.EntityID, e.Name, e.Type, string(e.Data)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// ---------- snapshot resolution & reads ----------

func listSnapshots(ctx context.Context, db *sql.DB) ([]gtmSnapshot, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT s.id, s.account_id, s.account_name, s.container_id, s.container_name, s.public_id, s.source, s.version_id, s.pulled_at, s.label,
		        (SELECT COUNT(*) FROM gtm_entity e WHERE e.snapshot_id = s.id)
		 FROM gtm_snapshot s ORDER BY s.pulled_at DESC, s.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

func scanSnapshots(rows *sql.Rows) ([]gtmSnapshot, error) {
	var out []gtmSnapshot
	for rows.Next() {
		var s gtmSnapshot
		var accName, contName, pub, ver sql.NullString
		if err := rows.Scan(&s.ID, &s.AccountID, &accName, &s.ContainerID, &contName, &pub, &s.Source, &ver, &s.PulledAt, &s.Label, &s.EntityCount); err != nil {
			return nil, err
		}
		s.AccountName, s.ContainerName, s.PublicID, s.VersionID = accName.String, contName.String, pub.String, ver.String
		out = append(out, s)
	}
	return out, rows.Err()
}

// resolveSnapshot maps a ref to a concrete snapshot, scoped to the active
// container (the container of the most recent snapshot). snaps must be ordered
// newest-first. See resolveSnapshotScoped for the ref grammar.
func resolveSnapshot(snaps []gtmSnapshot, ref string) (gtmSnapshot, error) {
	return resolveSnapshotScoped(snaps, ref, "")
}

// resolveSnapshotScoped resolves a ref:
//
//	""/"latest"        most recent snapshot in the scoped container
//	"live"             newest live snapshot in the scoped container
//	"workspace:<id>"   newest matching-source snapshot in the scoped container
//	"container:<id>"   newest snapshot for that container/public id (switches container)
//	"version:<id>"     snapshot with that container version id
//	"snapshot:<id>"/N  snapshot by internal id
//
// Bare source/empty refs are scoped to preferContainer; when preferContainer is
// "" they scope to the active container (newest snapshot overall) and fall back
// to any container if no match is found there. This keeps single-container use
// trivial while preventing bare refs from silently crossing containers.
func resolveSnapshotScoped(snaps []gtmSnapshot, ref, preferContainer string) (gtmSnapshot, error) {
	if len(snaps) == 0 {
		return gtmSnapshot{}, fmt.Errorf("no snapshots in the local mirror — run 'pull' first")
	}
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "container:"):
		want := strings.TrimPrefix(ref, "container:")
		for _, s := range snaps {
			if s.ContainerID == want || s.PublicID == want {
				return s, nil
			}
		}
		return gtmSnapshot{}, fmt.Errorf("no snapshot for container %q (pull it first)", want)
	case strings.HasPrefix(ref, "version:"):
		want := strings.TrimPrefix(ref, "version:")
		for _, s := range snaps {
			if s.VersionID == want {
				return s, nil
			}
		}
		return gtmSnapshot{}, fmt.Errorf("no snapshot with version id %q", want)
	case strings.HasPrefix(ref, "snapshot:") || isAllDigits(ref):
		want := strings.TrimPrefix(ref, "snapshot:")
		for _, s := range snaps {
			if fmt.Sprintf("%d", s.ID) == want {
				return s, nil
			}
		}
		return gtmSnapshot{}, fmt.Errorf("no snapshot with id %q", want)
	default:
		// ""/"latest"/"live"/"workspace:<id>" — scope to a container.
		scope := preferContainer
		if scope == "" {
			scope = snaps[0].ContainerID // active container = newest overall
		}
		match := func(s gtmSnapshot) bool {
			if ref == "" || ref == "latest" {
				return true
			}
			return s.Source == ref
		}
		for _, s := range snaps {
			if s.ContainerID == scope && match(s) {
				return s, nil
			}
		}
		// No explicit scope requested: tolerate a single-container mirror whose
		// active container differs, so a bare "live" still resolves.
		if preferContainer == "" {
			for _, s := range snaps {
				if match(s) {
					return s, nil
				}
			}
		}
		return gtmSnapshot{}, fmt.Errorf("no snapshot matches ref %q in the active container (try 'container:<id>' to pick a different container)", ref)
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func snapshotEntities(ctx context.Context, db *sql.DB, snapshotID int64, kind string) ([]gtmEntity, error) {
	q := `SELECT snapshot_id, kind, entity_id, name, type, data FROM gtm_entity WHERE snapshot_id = ?`
	args := []any{snapshotID}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	q += ` ORDER BY kind, name`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []gtmEntity
	for rows.Next() {
		var e gtmEntity
		var eid, name, typ sql.NullString
		var data string
		if err := rows.Scan(&e.SnapshotID, &e.Kind, &eid, &name, &typ, &data); err != nil {
			return nil, err
		}
		e.EntityID, e.Name, e.Type, e.Data = eid.String, name.String, typ.String, json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------- reference index ----------

type refIndex struct {
	tags        []gtmEntity
	triggers    []gtmEntity
	variables   []gtmEntity
	trigByID    map[string]gtmEntity
	varByName   map[string]gtmEntity
	tagTriggers map[string][]string // tagId -> firing+blocking trigger ids
	varUsedBy   map[string][]string // variable name -> ["tag:Name", "trigger:Name", ...]
	trigUsedBy  map[string][]string // triggerId -> ["tag:Name", ...]
}

func buildRefIndex(entities []gtmEntity) *refIndex {
	idx := &refIndex{
		trigByID:    map[string]gtmEntity{},
		varByName:   map[string]gtmEntity{},
		tagTriggers: map[string][]string{},
		varUsedBy:   map[string][]string{},
		trigUsedBy:  map[string][]string{},
	}
	for _, e := range entities {
		switch e.Kind {
		case "tag":
			idx.tags = append(idx.tags, e)
		case "trigger":
			idx.triggers = append(idx.triggers, e)
			if e.EntityID != "" {
				idx.trigByID[e.EntityID] = e
			}
		case "variable":
			idx.variables = append(idx.variables, e)
			if e.Name != "" {
				idx.varByName[e.Name] = e
			}
		}
	}
	// Edges: tag -> triggers, and variable-name references anywhere.
	for _, t := range idx.tags {
		var tag struct {
			FiringTriggerID   []string `json:"firingTriggerId"`
			BlockingTriggerID []string `json:"blockingTriggerId"`
		}
		_ = json.Unmarshal(t.Data, &tag)
		ids := append(append([]string{}, tag.FiringTriggerID...), tag.BlockingTriggerID...)
		idx.tagTriggers[t.EntityID] = ids
		for _, id := range ids {
			idx.trigUsedBy[id] = appendUnique(idx.trigUsedBy[id], "tag:"+t.Name)
		}
	}
	// Variable references via {{name}} tokens across tags, triggers, variables.
	record := func(kind string, e gtmEntity) {
		for _, name := range variableRefsIn(e.Data) {
			idx.varUsedBy[name] = appendUnique(idx.varUsedBy[name], kind+":"+e.Name)
		}
	}
	for _, e := range idx.tags {
		record("tag", e)
	}
	for _, e := range idx.triggers {
		record("trigger", e)
	}
	for _, e := range idx.variables {
		record("variable", e)
	}
	return idx
}

func variableRefsIn(data json.RawMessage) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range variableRefRe.FindAllStringSubmatch(string(data), -1) {
		name := strings.TrimSpace(m[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// ---------- audit ----------

type auditFinding struct {
	Severity string `json:"severity"` // high | warning | info
	Check    string `json:"check"`
	Kind     string `json:"kind"`
	Entity   string `json:"entity"`
	Message  string `json:"message"`
}

func runAudit(entities []gtmEntity) []auditFinding {
	idx := buildRefIndex(entities)
	var findings []auditFinding
	add := func(sev, check, kind, entity, msg string) {
		findings = append(findings, auditFinding{Severity: sev, Check: check, Kind: kind, Entity: entity, Message: msg})
	}

	for _, t := range idx.tags {
		var tag struct {
			FiringTriggerID []string `json:"firingTriggerId"`
			Paused          bool     `json:"paused"`
			ConsentSettings struct {
				ConsentStatus string `json:"consentStatus"`
			} `json:"consentSettings"`
		}
		_ = json.Unmarshal(t.Data, &tag)
		if len(tag.FiringTriggerID) == 0 {
			add("warning", "dead-tag", "tag", t.Name, "tag has no firing trigger and can never fire")
		}
		if tag.ConsentSettings.ConsentStatus == "" || tag.ConsentSettings.ConsentStatus == "notSet" {
			if isTrackingTagType(t.Type) {
				add("high", "missing-consent", "tag", t.Name, "tracking tag has no Consent Mode settings (consentStatus notSet)")
			}
		}
		if tag.Paused {
			add("info", "paused-tag", "tag", t.Name, "tag is paused")
		}
		if t.Type == "html" {
			add("info", "custom-html", "tag", t.Name, "custom HTML tag — review for injected/3rd-party script")
		}
		for _, id := range idx.tagTriggers[t.EntityID] {
			if id == allPagesTriggerID {
				add("info", "all-pages", "tag", t.Name, "fires on the built-in All Pages trigger")
			}
		}
	}
	for _, tr := range idx.triggers {
		if tr.EntityID == "" {
			continue
		}
		if len(idx.trigUsedBy[tr.EntityID]) == 0 {
			add("warning", "orphan-trigger", "trigger", tr.Name, "trigger is referenced by no tag")
		}
	}
	for _, v := range idx.variables {
		if len(idx.varUsedBy[v.Name]) == 0 {
			add("warning", "unused-variable", "variable", v.Name, "variable is referenced nowhere ({{"+v.Name+"}} unused)")
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})
	return findings
}

func severityRank(s string) int {
	switch s {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

// isTrackingTagType reports whether a tag type sends data to an ads/analytics
// vendor and therefore should declare Consent Mode settings.
func isTrackingTagType(t string) bool {
	switch t {
	case "html": // Custom HTML can do anything; treat as tracking-capable.
		return true
	}
	tracking := []string{"gaaw", "gaawe", "googtag", "ua", "awct", "sp", "flc", "fls", "gclidw", "baut", "twitter_website_tag", "pntr"}
	for _, p := range tracking {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// ---------- consent report ----------

type consentRow struct {
	Tag           string   `json:"tag"`
	Type          string   `json:"type"`
	Vendor        string   `json:"vendor"`
	ConsentStatus string   `json:"consentStatus"`
	ConsentTypes  []string `json:"consentTypes,omitempty"`
	Gated         bool     `json:"gated"`
}

func runConsentReport(entities []gtmEntity) []consentRow {
	var rows []consentRow
	for _, e := range entities {
		if e.Kind != "tag" {
			continue
		}
		var tag struct {
			ConsentSettings struct {
				ConsentStatus string `json:"consentStatus"`
				ConsentType   struct {
					List []struct {
						Value string `json:"value"`
					} `json:"list"`
				} `json:"consentType"`
			} `json:"consentSettings"`
		}
		_ = json.Unmarshal(e.Data, &tag)
		status := tag.ConsentSettings.ConsentStatus
		if status == "" {
			status = "notSet"
		}
		var types []string
		for _, t := range tag.ConsentSettings.ConsentType.List {
			if t.Value != "" {
				types = append(types, t.Value)
			}
		}
		rows = append(rows, consentRow{
			Tag:           e.Name,
			Type:          e.Type,
			Vendor:        vendorForTagType(e.Type),
			ConsentStatus: status,
			ConsentTypes:  types,
			Gated:         status == "needed",
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Gated != rows[j].Gated {
			return !rows[i].Gated // ungated first — they're the risk
		}
		return rows[i].Tag < rows[j].Tag
	})
	return rows
}

func vendorForTagType(t string) string {
	switch {
	case strings.HasPrefix(t, "gaaw"), strings.HasPrefix(t, "ua"), strings.HasPrefix(t, "googtag"):
		return "analytics"
	case strings.HasPrefix(t, "awct"), strings.HasPrefix(t, "sp"), strings.HasPrefix(t, "gclidw"), strings.HasPrefix(t, "flc"), strings.HasPrefix(t, "fls"):
		return "google-ads"
	case strings.HasPrefix(t, "baut"):
		return "microsoft-ads"
	case t == "html":
		return "custom"
	}
	return "other"
}

// ---------- diff ----------

type diffChange struct {
	Op            string   `json:"op"` // added | removed | changed
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	ChangedFields []string `json:"changedFields,omitempty"`
}

// volatileFields are metadata that change on every save and would create noise
// in a config diff.
var volatileFields = map[string]bool{
	"fingerprint": true, "path": true, "tagManagerUrl": true,
	"workspaceId": true, "accountId": true, "containerId": true,
}

func diffSnapshots(a, b []gtmEntity) []diffChange {
	keyOf := func(e gtmEntity) string { return e.Kind + "\x00" + e.Name }
	am := map[string]gtmEntity{}
	bm := map[string]gtmEntity{}
	for _, e := range a {
		am[keyOf(e)] = e
	}
	for _, e := range b {
		bm[keyOf(e)] = e
	}
	var out []diffChange
	for k, ea := range am {
		eb, ok := bm[k]
		if !ok {
			out = append(out, diffChange{Op: "removed", Kind: ea.Kind, Name: ea.Name})
			continue
		}
		if changed := changedFields(ea.Data, eb.Data); len(changed) > 0 {
			out = append(out, diffChange{Op: "changed", Kind: ea.Kind, Name: ea.Name, ChangedFields: changed})
		}
	}
	for k, eb := range bm {
		if _, ok := am[k]; !ok {
			out = append(out, diffChange{Op: "added", Kind: eb.Kind, Name: eb.Name})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Op != out[j].Op {
			return out[i].Op < out[j].Op
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func changedFields(a, b json.RawMessage) []string {
	var ma, mb map[string]json.RawMessage
	_ = json.Unmarshal(a, &ma)
	_ = json.Unmarshal(b, &mb)
	seen := map[string]bool{}
	var fields []string
	consider := func(k string) {
		if volatileFields[k] || seen[k] {
			return
		}
		seen[k] = true
		if !jsonEqual(ma[k], mb[k]) {
			fields = append(fields, k)
		}
	}
	for k := range ma {
		consider(k)
	}
	for k := range mb {
		consider(k)
	}
	sort.Strings(fields)
	return fields
}

func jsonEqual(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return string(a) == string(b)
	}
	na, _ := json.Marshal(va)
	nb, _ := json.Marshal(vb)
	return string(na) == string(nb)
}

// ---------- fleet ----------

type fleetRow struct {
	Container      string   `json:"container"`
	PublicID       string   `json:"publicId,omitempty"`
	Source         string   `json:"source"`
	Tags           int      `json:"tags"`
	Triggers       int      `json:"triggers"`
	Variables      int      `json:"variables"`
	CustomHTML     int      `json:"customHtml"`
	MeasurementIDs []string `json:"measurementIds,omitempty"`
	ConsentPct     int      `json:"consentCoveragePct"`
}

func fleetRowFor(snap gtmSnapshot, entities []gtmEntity) fleetRow {
	row := fleetRow{Container: displayName(snap), PublicID: snap.PublicID, Source: snap.Source}
	ids := map[string]bool{}
	gated, tracking := 0, 0
	for _, e := range entities {
		switch e.Kind {
		case "tag":
			row.Tags++
			if e.Type == "html" {
				row.CustomHTML++
			}
			if isTrackingTagType(e.Type) {
				tracking++
				var tag struct {
					ConsentSettings struct {
						ConsentStatus string `json:"consentStatus"`
					} `json:"consentSettings"`
				}
				_ = json.Unmarshal(e.Data, &tag)
				if tag.ConsentSettings.ConsentStatus == "needed" {
					gated++
				}
			}
		case "trigger":
			row.Triggers++
		case "variable":
			row.Variables++
		}
		for _, id := range measurementIDRe.FindAllString(string(e.Data), -1) {
			ids[id] = true
		}
	}
	for id := range ids {
		row.MeasurementIDs = append(row.MeasurementIDs, id)
	}
	sort.Strings(row.MeasurementIDs)
	if tracking > 0 {
		row.ConsentPct = gated * 100 / tracking
	} else {
		row.ConsentPct = 100
	}
	return row
}

func displayName(s gtmSnapshot) string {
	if s.ContainerName != "" {
		return s.ContainerName
	}
	if s.PublicID != "" {
		return s.PublicID
	}
	return s.ContainerID
}

// latestPerContainer keeps the newest snapshot for each container id.
func latestPerContainer(snaps []gtmSnapshot) []gtmSnapshot {
	seen := map[string]bool{}
	var out []gtmSnapshot
	for _, s := range snaps { // already newest-first
		if seen[s.ContainerID] {
			continue
		}
		seen[s.ContainerID] = true
		out = append(out, s)
	}
	return out
}
