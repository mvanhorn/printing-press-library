// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared local-store plumbing for the hand-authored Webflow audit commands.
// Every novel command in this CLI reads the SQLite mirror that `sync` builds,
// so the open/guard/scan boilerplate lives here once instead of seven times.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/internal/store"
)

// localQuery bundles the state every novel command needs after it has decided
// it is really going to touch the mirror.
type localQuery struct {
	db  *store.Store
	ctx context.Context
}

// emptyLocalResult is what a novel command prints when the mirror has not been
// built yet. A missing mirror is an empty local-cache state, not a usage error
// or an API failure, so callers return nil after emitting it.
func emptyLocalResult(cmd *cobra.Command, flags *rootFlags, dbPath, syncResources string) {
	if dbPath == "" {
		dbPath = defaultDBPath("webflow-pp-cli")
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"no local mirror at %s\nrun: webflow-pp-cli sync --resources %s --db %s\n",
		dbPath, syncResources, dbPath)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
}

// emitEmptyLocal prints the caller's own zero-valued view when there is nothing
// to read, so the JSON shape is identical whether or not the mirror exists.
// Emitting a bare [] here instead would break `jq` paths and silently drop
// --select / --csv for the six commands whose views are objects.
func emitEmptyLocal(cmd *cobra.Command, flags *rootFlags, dbPath, syncResources string, view any) error {
	// Callers pass the raw --db flag, which is empty when the user did not set
	// it; resolve it so the hint names a real path instead of a blank.
	if dbPath == "" {
		dbPath = defaultDBPath("webflow-pp-cli")
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"nothing to read locally (%s)\nhint: run 'webflow-pp-cli sync --resources %s' to build the mirror, or set WEBFLOW_API_TOKEN to read live\n",
		dbPath, syncResources)
	return emitLocalResult(cmd, flags, view, func() {
		fmt.Fprintln(cmd.OutOrStdout(), "nothing to report: the local mirror is empty")
	})
}

// missingTableNote distinguishes "the mirror file is absent" from "the mirror
// exists but has no rows of this kind", which the earlier message conflated.
func missingTableNote(dbPath, table string) string {
	return fmt.Sprintf("the local mirror at %s has no %s data yet", dbPath, table)
}

// openLocalMirror resolves the database path, guards the missing-mirror case,
// and opens the store read-only. ok=false means the caller should return nil
// immediately; the user-facing message has already been printed.
func openLocalMirror(cmd *cobra.Command, flags *rootFlags, dbPath, syncResources string) (lq *localQuery, cleanup func(), ok bool, err error) {
	if dbPath == "" {
		dbPath = defaultDBPath("webflow-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		emptyLocalResult(cmd, flags, dbPath, syncResources)
		return nil, func() {}, false, nil
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	db, openErr := store.OpenReadOnlyContext(ctx, dbPath)
	if openErr != nil {
		cancel()
		return nil, func() {}, false, fmt.Errorf("opening local mirror at %s: %w", dbPath, openErr)
	}
	return &localQuery{db: db, ctx: ctx}, func() {
		_ = db.Close()
		cancel()
	}, true, nil
}

// hasTable reports whether a typed sync table exists yet. Tables are created by
// the store migration, but a mirror synced by an older binary can be missing
// one, and querying it would fail the whole command rather than degrade.
func (lq *localQuery) hasTable(name string) bool {
	var found sql.NullString
	row := lq.db.DB().QueryRowContext(lq.ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name)
	if err := row.Scan(&found); err != nil {
		return false
	}
	return found.Valid
}

// rawRow is the drain-first carrier for a (id, scope, data) select. Commands
// scan every row into a slice of these, close the parent *sql.Rows, and only
// then decode or run follow-up queries. The store allows 2 open connections, so
// a nested read would not deadlock outright, but draining first keeps the read
// pool free and makes query ordering explicit rather than incidental.
type rawRow struct {
	ID    string
	Scope string
	Data  []byte
}

// selectRaw runs a SELECT returning (id, scope, data) and drains it fully.
func (lq *localQuery) selectRaw(query string, args ...any) ([]rawRow, error) {
	rows, err := lq.db.DB().QueryContext(lq.ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	out := make([]rawRow, 0, 64)
	for rows.Next() {
		var id, scope sql.NullString
		var data []byte
		if scanErr := rows.Scan(&id, &scope, &data); scanErr != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan row: %w", scanErr)
		}
		out = append(out, rawRow{ID: id.String, Scope: scope.String, Data: data})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close rows: %w", err)
	}
	return out, nil
}

// decodeRows unmarshals each row's data column into T, skipping rows whose JSON
// is unreadable rather than failing the whole audit on one bad record.
func decodeRows[T any](rows []rawRow) []T {
	out := make([]T, 0, len(rows))
	for _, r := range rows {
		var v T
		if err := json.Unmarshal(r.Data, &v); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// --- Webflow record shapes -------------------------------------------------
//
// Only the fields the audits actually read. Everything is a pointer or a
// plain string so a field the API omitted decodes to its zero value instead
// of failing the unmarshal.

type wfSite struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"displayName"`
	ShortName     string   `json:"shortName"`
	LastPublished string   `json:"lastPublished"`
	LastUpdated   string   `json:"lastUpdated"`
	PreviewURL    string   `json:"previewUrl"`
	WorkspaceID   string   `json:"workspaceId"`
	CustomDomains []wfHost `json:"customDomains"`
}

type wfHost struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type wfPageSEO struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type wfPageOG struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	TitleCopied      *bool  `json:"titleCopied"`
	DescriptionCopie *bool  `json:"descriptionCopied"`
}

type wfPage struct {
	ID            string     `json:"id"`
	SiteID        string     `json:"siteId"`
	Title         string     `json:"title"`
	Slug          string     `json:"slug"`
	CollectionID  string     `json:"collectionId"`
	ParentID      string     `json:"parentId"`
	PublishedPath string     `json:"publishedPath"`
	CreatedOn     string     `json:"createdOn"`
	LastUpdated   string     `json:"lastUpdated"`
	Draft         *bool      `json:"draft"`
	Archived      *bool      `json:"archived"`
	SEO           *wfPageSEO `json:"seo"`
	OpenGraph     *wfPageOG  `json:"openGraph"`
}

func (p wfPage) isDraft() bool    { return p.Draft != nil && *p.Draft }
func (p wfPage) isArchived() bool { return p.Archived != nil && *p.Archived }

func (p wfPage) seoTitle() string {
	if p.SEO == nil {
		return ""
	}
	return strings.TrimSpace(p.SEO.Title)
}

func (p wfPage) seoDescription() string {
	if p.SEO == nil {
		return ""
	}
	return strings.TrimSpace(p.SEO.Description)
}

func (p wfPage) ogTitle() string {
	if p.OpenGraph == nil {
		return ""
	}
	return strings.TrimSpace(p.OpenGraph.Title)
}

func (p wfPage) ogDescription() string {
	if p.OpenGraph == nil {
		return ""
	}
	return strings.TrimSpace(p.OpenGraph.Description)
}

// label is the human handle for a page in audit output: the slug when there is
// one, otherwise the title, otherwise the raw ID.
func (p wfPage) label() string {
	if s := strings.TrimSpace(p.Slug); s != "" {
		return s
	}
	if t := strings.TrimSpace(p.Title); t != "" {
		return t
	}
	return p.ID
}

type wfItem struct {
	ID            string         `json:"id"`
	CMSLocaleID   string         `json:"cmsLocaleId"`
	IsDraft       *bool          `json:"isDraft"`
	IsArchived    *bool          `json:"isArchived"`
	CreatedOn     string         `json:"createdOn"`
	LastUpdated   string         `json:"lastUpdated"`
	LastPublished string         `json:"lastPublished"`
	FieldData     map[string]any `json:"fieldData"`
}

func (i wfItem) draft() bool    { return i.IsDraft != nil && *i.IsDraft }
func (i wfItem) archived() bool { return i.IsArchived != nil && *i.IsArchived }

// name returns the item's display name from fieldData, falling back to slug
// then ID. Webflow guarantees `name` and `slug` on every collection.
func (i wfItem) name() string {
	for _, k := range []string{"name", "slug"} {
		if v, ok := i.FieldData[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return i.ID
}

func (i wfItem) slug() string {
	if v, ok := i.FieldData["slug"]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

type wfCollection struct {
	ID           string    `json:"id"`
	DisplayName  string    `json:"displayName"`
	Slug         string    `json:"slug"`
	SingularName string    `json:"singularName"`
	Fields       []wfField `json:"fields"`
}

type wfField struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
	IsRequired  *bool  `json:"isRequired"`
	IsEditable  *bool  `json:"isEditable"`
}

func (f wfField) required() bool { return f.IsRequired != nil && *f.IsRequired }

type wfRedirect struct {
	ID      string `json:"id"`
	FromURL string `json:"fromUrl"`
	ToURL   string `json:"toUrl"`
}

// --- shared helpers --------------------------------------------------------

// parseWFTime accepts the ISO-8601 timestamps Webflow returns and reports
// ok=false for empty, null, or unparseable values rather than guessing a zero
// time that would silently sort first.
func parseWFTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// hasUnpublishedChanges reports whether a CMS item has staged edits that are
// not on the live site. Never-published items count; so do items whose
// lastUpdated is strictly newer than lastPublished.
func hasUnpublishedChanges(i wfItem) bool {
	pub, pubOK := parseWFTime(i.LastPublished)
	if !pubOK {
		return true
	}
	upd, updOK := parseWFTime(i.LastUpdated)
	if !updOK {
		return false
	}
	return upd.After(pub)
}

// normalizePath makes redirect sources, page slugs, and item slugs comparable:
// lowercased, leading slash, no trailing slash, query and fragment dropped.
func normalizePath(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	for len(s) > 1 && strings.HasSuffix(s, "/") {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

// daysSince returns whole days between t and now, and ok=false when the
// timestamp could not be parsed.
func daysSince(s string) (int, bool) {
	t, ok := parseWFTime(s)
	if !ok {
		return 0, false
	}
	return int(time.Since(t).Hours() / 24), true
}

// emitLocalResult writes a novel command's view in whichever shape the caller
// asked for, honoring --json/--agent/--select/--compact/--csv, and falling
// back to the human renderer when stdout is a terminal.
func emitLocalResult(cmd *cobra.Command, flags *rootFlags, view any, human func()) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	human()
	return nil
}

// --- live fetch fallback ---------------------------------------------------
//
// Generated `sync` in this Press version cannot reach `collections`,
// `redirects`, or an unqualified `items` (verified: each returns
// "1 resource(s) failed to sync"; `items` additionally demands
// WEBFLOW_COLLECTION_ID and covers one collection per run). The audit commands
// therefore fetch those resources from the API when the mirror has nothing,
// and fall back to the mirror when it does. A regen must re-check whether the
// sync enumeration has grown before assuming the mirror is populated.
//
// Every fetch here is scoped to one site or one collection, so the cost is a
// small bounded number of calls even on the 60 req/min floor. Cross-site
// fan-out is deliberately NOT done — see `overview`, which stays local-only.

// fetchPaged walks an offset-paginated Webflow list endpoint and returns one
// rawRow per element of the named array field, tagged with the given scope so
// the local-store code paths can consume it unchanged.
func (lq *localQuery) fetchPaged(c liveFetcher, path, arrayField, scope string, maxPages int) ([]rawRow, error) {
	out := make([]rawRow, 0, 64)
	const pageSize = 100
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"limit":  strconv.Itoa(pageSize),
			"offset": strconv.Itoa(page * pageSize),
		}
		body, err := c.Get(lq.ctx, path, params)
		if err != nil {
			return nil, err
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		raw, ok := env[arrayField]
		if !ok {
			return out, nil
		}
		var elems []json.RawMessage
		if err := json.Unmarshal(raw, &elems); err != nil {
			return nil, fmt.Errorf("decoding %s.%s: %w", path, arrayField, err)
		}
		for _, e := range elems {
			var idHolder struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(e, &idHolder)
			out = append(out, rawRow{ID: idHolder.ID, Scope: scope, Data: e})
		}
		if len(elems) < pageSize {
			break
		}
	}
	return out, nil
}

// fetchOne retrieves a single object and returns it as one rawRow.
func (lq *localQuery) fetchOne(c liveFetcher, path, scope string) ([]rawRow, error) {
	body, err := c.Get(lq.ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var idHolder struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &idHolder)
	id := idHolder.ID
	if id == "" {
		id = scope
	}
	return []rawRow{{ID: id, Scope: scope, Data: body}}, nil
}

// liveFetcher is the slice of the generated client these audits need. Taking an
// interface keeps the fetch helpers testable without a live endpoint.
type liveFetcher interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// escapeSeg makes a user-supplied id safe to concatenate into a request path,
// matching what the generated endpoint commands do via replacePathParam.
func escapeSeg(s string) string { return cliutil.EscapePathParam(s) }

// classifyWriteError maps an API failure to the CLI's typed exit code so
// callers get 4 for auth, 7 for rate limiting, and 5 for everything else,
// instead of a blanket 5.
func classifyWriteError(err error) error {
	var ae *client.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.StatusCode == 401 || ae.StatusCode == 403:
			return authErr(err)
		case ae.StatusCode == 429:
			return rateLimitErr(err)
		}
	}
	return apiErr(err)
}

// isRateLimited reports whether an API failure was a genuine 429. The
// generated client surfaces these as *client.APIError; cliutil.RateLimitError
// is declared but never constructed, so matching on it would never fire.
func isRateLimited(err error) bool {
	var ae *client.APIError
	if errors.As(err, &ae) {
		return ae.StatusCode == 429
	}
	return false
}

// fetchOneList retrieves a non-paginated list endpoint (for example a site's
// redirect table) and returns one rawRow per element of the named array field.
func (lq *localQuery) fetchOneList(c liveFetcher, path, arrayField, scope string) ([]rawRow, error) {
	body, err := c.Get(lq.ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	raw, ok := env[arrayField]
	if !ok {
		return []rawRow{}, nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil, fmt.Errorf("decoding %s.%s: %w", path, arrayField, err)
	}
	out := make([]rawRow, 0, len(elems))
	for _, e := range elems {
		var idHolder struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(e, &idHolder)
		out = append(out, rawRow{ID: idHolder.ID, Scope: scope, Data: e})
	}
	return out, nil
}

// liveFallbackErr converts an API failure during a read-only audit's live
// fallback into the right user-facing outcome.
//
// A 401/403 here almost always means no credential is configured, or one
// without the needed scope. That is a configuration state, not a crash, and
// the audit genuinely has nothing to report — so callers degrade to their
// zero-valued view with an actionable note rather than exiting non-zero on a
// command the user may simply not have set up yet. Everything else (5xx,
// transport failures, malformed bodies) is a real error and propagates.
func liveFallbackErr(err error, what string) (note string, degrade bool) {
	var ae *client.APIError
	if errors.As(err, &ae) && (ae.StatusCode == 401 || ae.StatusCode == 403) {
		// Surface the API's own reason. A 403 here is not always a scope
		// problem: Webflow also returns it for plan-gated endpoints (redirects
		// are Enterprise-only), and telling the user to add a scope they cannot
		// add is worse than telling them nothing.
		if reason := apiErrorReason(ae); reason != "" {
			return fmt.Sprintf("could not read %s: %s (HTTP %d)", what, reason, ae.StatusCode), true
		}
		return fmt.Sprintf(
			"could not read %s from the API (HTTP %d). Check that WEBFLOW_API_TOKEN is set and carries the required scope; 'webflow-pp-cli doctor' shows what is currently loaded.",
			what, ae.StatusCode), true
	}
	return "", false
}

// apiErrorReason pulls Webflow's human-readable message out of an error body so
// the caller can repeat the provider's own explanation instead of guessing.
func apiErrorReason(ae *client.APIError) string {
	if ae == nil || strings.TrimSpace(ae.Body) == "" {
		return ""
	}
	var body struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal([]byte(ae.Body), &body); err != nil {
		return ""
	}
	msg := strings.TrimSpace(body.Message)
	if msg == "" {
		return ""
	}
	if code := strings.TrimSpace(body.Code); code != "" {
		return fmt.Sprintf("%s [%s]", msg, code)
	}
	return msg
}

// emitDryRun prints the dry-run explanation in whichever format the caller
// asked for. Printing bare prose here breaks `--dry-run --json`, which agents
// and the dogfood JSON-fidelity check both parse.
func emitDryRun(cmd *cobra.Command, flags *rootFlags, action string) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"dryRun": true,
			"action": action,
		}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), action)
	return nil
}

// resolveCollectionID prefers the positional argument and falls back to
// WEBFLOW_COLLECTION_ID, matching the generated endpoint commands' fixture
// convention so a bare invocation works in a configured environment.
func resolveCollectionID(args []string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0])
	}
	return strings.TrimSpace(os.Getenv("WEBFLOW_COLLECTION_ID"))
}

// resolveSiteID is the same fallback for site-scoped commands.
func resolveSiteID(args []string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0])
	}
	return strings.TrimSpace(os.Getenv("WEBFLOW_SITE_ID"))
}
