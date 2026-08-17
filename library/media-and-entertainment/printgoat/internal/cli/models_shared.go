// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
)

// printablesGraphQLURL and cults3dGraphQLURL are the absolute endpoint URLs
// for the two GraphQL-backed sources. internal/client.Client's Get/Post
// accept absolute URLs directly (see internal/client/host_auth.go), so
// every call site here passes one of these rather than a relative path.
const (
	printablesGraphQLURL = "https://api.printables.com/graphql/"
	cults3dGraphQLURL    = "https://cults3d.com/graphql"
)

// unifiedModel is the common shape search and the novel cross-site commands
// (duplicates, similar, formats gaps) render a design listing as, regardless
// of which of the three sites it came from.
type unifiedModel struct {
	Source        string `json:"source"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Designer      string `json:"designer,omitempty"`
	LikesCount    int    `json:"likes_count,omitempty"`
	DownloadCount int    `json:"download_count,omitempty"`
	IsPaid        bool   `json:"is_paid,omitempty"`
	PublishedAt   string `json:"published_at,omitempty"`
}

// graphQLErrors is the standard top-level "errors" envelope shared by
// Printables and Cults3D; both can return HTTP 200 with an errors array
// instead of (or alongside) data. Embed it in a response struct to get
// firstError() for free.
type graphQLErrors struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (g graphQLErrors) firstError() string {
	if len(g.Errors) == 0 {
		return ""
	}
	return g.Errors[0].Message
}

// rawIDString normalizes a JSON "id" field to a plain string regardless of
// whether the API encoded it as a JSON string or a JSON number — GraphQL's
// ID scalar is serialized either way depending on the API, and Printables,
// Thingiverse, and Cults3D don't all agree.
func rawIDString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return strings.Trim(string(raw), `"`)
}

// anyToIDString is rawIDString's counterpart for values already decoded into
// a map[string]any (json.Unmarshal decodes JSON numbers as float64).
func anyToIDString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// intFromAny coerces a decoded JSON numeric field (float64 from
// map[string]any, or occasionally json.Number) to int, defaulting to 0 for
// anything else so a missing/mistyped field never panics a search mapper.
func intFromAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return 0
	}
}

// extractDesignerName looks through the given nested-object keys (in order)
// for the first plausible display-name field. Used defensively for
// Thingiverse's creator/author summary and Cults3D's creator object, whose
// exact shape ground truth could not fully confirm ahead of live testing.
func extractDesignerName(obj map[string]any, nestedKeys ...string) string {
	for _, key := range nestedKeys {
		nested, ok := obj[key].(map[string]any)
		if !ok {
			continue
		}
		for _, nameKey := range []string{"name", "username", "handle", "public_username", "publicUsername", "nick", "first_name"} {
			if v, ok := nested[nameKey]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
	}
	return ""
}

// extractSearchResults unwraps an API search response by checking, in
// order: a bare top-level array, each of responsePaths (dotted JSON paths
// resolved via responsePayloadAtPath in helpers.go), then a handful of
// common wrapper keys ("data", "results", "items", "records", "entries").
// Falls back to treating the whole payload as a single-item result set
// rather than returning nothing, so a shape this helper doesn't recognize
// still surfaces (visibly, as one odd-looking row) instead of vanishing.
//
// Lives here rather than in any one command file because it is a generic
// JSON-shape helper shared across hand-written command files in this
// package (novel_shared.go's Thingiverse search parsing, in particular).
func extractSearchResults(data json.RawMessage, responsePaths ...string) []json.RawMessage {
	var items []json.RawMessage
	if json.Unmarshal(data, &items) == nil {
		return items
	}
	for _, responsePath := range responsePaths {
		if pathData, ok := responsePayloadAtPath(data, responsePath); ok {
			if json.Unmarshal(pathData, &items) == nil {
				return items
			}
		}
	}
	var wrapped map[string]json.RawMessage
	if json.Unmarshal(data, &wrapped) == nil {
		for _, key := range []string{"data", "results", "items", "records", "entries"} {
			if inner, ok := wrapped[key]; ok {
				if json.Unmarshal(inner, &items) == nil {
					return items
				}
			}
		}
	}
	return []json.RawMessage{data}
}

// asJSONItems returns the JSON array found in data: either data itself, if
// it is a bare array, or the first of keys whose value in a wrapping object
// unmarshals to an array. Used for Thingiverse and Cults3D search parsing,
// both of which are expected to return a bare array but are handled
// defensively in case the live API's shape drifts from what research found.
func asJSONItems(data json.RawMessage, keys ...string) ([]json.RawMessage, bool) {
	var items []json.RawMessage
	if json.Unmarshal(data, &items) == nil {
		return items, true
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return nil, false
	}
	for _, k := range keys {
		if raw, ok := obj[k]; ok {
			if json.Unmarshal(raw, &items) == nil {
				return items, true
			}
		}
	}
	return nil, false
}

// inferFormatFromName returns the uppercased file extension of name (e.g.
// "widget.3mf" -> "3MF"), or "OTHER" when name has no extension.
func inferFormatFromName(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 || i == len(name)-1 {
		return "OTHER"
	}
	return strings.ToUpper(name[i+1:])
}

// ---------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------

// searchPrintables runs Printables' searchPrints2 GraphQL query and maps the
// result into unifiedModel rows. ordering maps the CLI's --sort value onto
// Printables' SearchChoicesEnum ("best_match", "popular", ...); pass "" for
// the API's default.
func searchPrintables(ctx context.Context, c *client.Client, query string, limit int, ordering string) ([]unifiedModel, error) {
	if limit <= 0 {
		limit = 20
	}
	if ordering == "" {
		ordering = "best_match"
	}
	body := map[string]any{
		"query": `query SearchModels($query: String!, $limit: Int, $offset: Int, $ordering: SearchChoicesEnum) { result: searchPrints2(query: $query, limit: $limit, offset: $offset, ordering: $ordering, printType: print) { totalCount items { id name slug ratingAvg likesCount downloadCount datePublished user { id handle publicUsername } image { filePath } } } }`,
		"variables": map[string]any{
			"query":    query,
			"limit":    limit,
			"offset":   0,
			"ordering": ordering,
		},
	}
	data, _, err := c.PostQueryWithParams(ctx, printablesGraphQLURL, nil, body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		graphQLErrors
		Data struct {
			Result struct {
				Items []struct {
					ID            json.RawMessage `json:"id"`
					Name          string          `json:"name"`
					Slug          string          `json:"slug"`
					LikesCount    int             `json:"likesCount"`
					DownloadCount int             `json:"downloadCount"`
					DatePublished string          `json:"datePublished"`
					User          struct {
						Handle         string `json:"handle"`
						PublicUsername string `json:"publicUsername"`
					} `json:"user"`
				} `json:"items"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing printables search response: %w", err)
	}
	if msg := resp.firstError(); msg != "" {
		return nil, fmt.Errorf("printables search: %s", msg)
	}
	out := make([]unifiedModel, 0, len(resp.Data.Result.Items))
	for _, it := range resp.Data.Result.Items {
		id := rawIDString(it.ID)
		designer := it.User.PublicUsername
		if designer == "" {
			designer = it.User.Handle
		}
		out = append(out, unifiedModel{
			Source:        "printables",
			ID:            id,
			Name:          it.Name,
			URL:           fmt.Sprintf("https://www.printables.com/model/%s-%s", id, it.Slug),
			Designer:      designer,
			LikesCount:    it.LikesCount,
			DownloadCount: it.DownloadCount,
			PublishedAt:   it.DatePublished,
		})
	}
	return out, nil
}

// searchThingiverse runs GET /search/<term> and maps the result into
// unifiedModel rows. Thingiverse's search response is a bare JSON array of
// Thing objects; asJSONItems tolerates a wrapped shape defensively.
func searchThingiverse(ctx context.Context, c *client.Client, query string, limit int) ([]unifiedModel, error) {
	if limit <= 0 {
		limit = 20
	}
	target := "https://api.thingiverse.com/search/" + url.PathEscape(query)
	params := map[string]string{
		"type":     "things",
		"per_page": strconv.Itoa(limit),
		"page":     "1",
	}
	data, err := c.Get(ctx, target, params)
	if err != nil {
		return nil, err
	}
	items, ok := asJSONItems(data, "hits", "things", "results")
	if !ok {
		fmt.Fprintln(os.Stderr, "warning: unexpected thingiverse search response shape (expected a bare JSON array)")
		return nil, nil
	}
	out := make([]unifiedModel, 0, len(items))
	for _, raw := range items {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		name, _ := obj["name"].(string)
		publicURL, _ := obj["public_url"].(string)
		out = append(out, unifiedModel{
			Source:        "thingiverse",
			ID:            anyToIDString(obj["id"]),
			Name:          name,
			URL:           publicURL,
			Designer:      extractDesignerName(obj, "creator", "author", "user"),
			LikesCount:    intFromAny(obj["like_count"]),
			DownloadCount: intFromAny(obj["download_count"]),
		})
	}
	return out, nil
}

// searchCults3D runs the creationsSearchBatch GraphQL query and maps the
// result into unifiedModel rows. creationsSearchBatch returns a CreationBatch
// envelope ({results: [Creation!]!, total: Int!}), confirmed via live GraphQL
// introspection against cults3d.com/graphql — not a bare array, and not the
// {"items":[...]} shape originally guessed at research time without a live
// account. The "results" wrapper key below is what actually matches;
// "items"/"creations" are kept as defensive fallbacks only.
func searchCults3D(ctx context.Context, c *client.Client, query string, limit int) ([]unifiedModel, error) {
	if limit <= 0 {
		limit = 20
	}
	body := map[string]any{
		// Selects "url" (the full slugged page URL), not "shortUrl": Cults3D's
		// Creation.shortUrl field returns a bare numeric redirect form
		// (e.g. "https://cults3d.com/:2034048") that 403s when fetched
		// directly rather than redirecting — confirmed live. "url" is the
		// real working link (e.g. ".../en/3d-model/home/shark-cookie-cutter-...").
		"query": `query SearchCreations($query: String!, $limit: Int, $offset: Int) { result: creationsSearchBatch(query: $query, limit: $limit, offset: $offset) { total results { identifier name url likesCount downloadsCount publishedAt creator { nick shortUrl } } } }`,
		"variables": map[string]any{
			"query":  query,
			"limit":  limit,
			"offset": 0,
		},
	}
	data, _, err := c.PostQueryWithParams(ctx, cults3dGraphQLURL, nil, body)
	if err != nil {
		return nil, err
	}
	var top struct {
		graphQLErrors
		Data struct {
			Result json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing cults3d search response: %w", err)
	}
	if msg := top.firstError(); msg != "" {
		return nil, fmt.Errorf("cults3d search: %s", msg)
	}
	if len(top.Data.Result) == 0 {
		return nil, nil
	}
	items, ok := asJSONItems(top.Data.Result, "items", "results", "creations")
	if !ok {
		fmt.Fprintln(os.Stderr, "warning: unexpected cults3d search response shape (data.result is neither an array nor an {items:[...]}-shaped object)")
		return nil, nil
	}
	out := make([]unifiedModel, 0, len(items))
	for _, raw := range items {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		name, _ := obj["name"].(string)
		pageURL, _ := obj["url"].(string)
		published, _ := obj["publishedAt"].(string)
		out = append(out, unifiedModel{
			Source:        "cults3d",
			ID:            anyToIDString(obj["identifier"]),
			Name:          name,
			URL:           pageURL,
			Designer:      extractDesignerName(obj, "creator"),
			LikesCount:    intFromAny(obj["likesCount"]),
			DownloadCount: intFromAny(obj["downloadsCount"]),
			PublishedAt:   published,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------
// File listing
// ---------------------------------------------------------------------

// printablesModelFile is one file entry from a Printables model-detail
// response. Format is inferred from the filename extension (useful for
// --formats filtering, since "otherFiles" can hold any extension);
// APICategory is which of the four typed lists (stls/gcodes/slas/otherFiles)
// it came from, which is what getDownloadLink's fileType argument expects —
// the two are not always equal (a ".3mf" file lives in otherFiles but has
// Format "3MF", not "OTHER").
type printablesModelFile struct {
	ID          string
	Name        string
	Format      string
	APICategory string
	SizeBytes   int64
}

type printablesFileEntry struct {
	ID       json.RawMessage `json:"id"`
	Name     string          `json:"name"`
	FileSize int64           `json:"fileSize"`
}

// printablesFiles fetches a Printables model's detail and flattens its
// stls/gcodes/slas/otherFiles lists into printablesModelFile rows.
func printablesFiles(ctx context.Context, c *client.Client, id string) ([]printablesModelFile, error) {
	body := map[string]any{
		"query": `query ModelDetail($id: ID!) { model: print(id: $id) { id name slug filesType ratingAvg likesCount downloadCount datePublished user { id handle publicUsername } stls { id name fileSize } gcodes { id name fileSize } slas { id name fileSize } otherFiles { id name fileSize } } }`,
		"variables": map[string]any{
			"id": id,
		},
	}
	data, _, err := c.PostQueryWithParams(ctx, printablesGraphQLURL, nil, body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		graphQLErrors
		Data struct {
			Model *struct {
				STLs       []printablesFileEntry `json:"stls"`
				Gcodes     []printablesFileEntry `json:"gcodes"`
				Slas       []printablesFileEntry `json:"slas"`
				OtherFiles []printablesFileEntry `json:"otherFiles"`
			} `json:"model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing printables model detail response: %w", err)
	}
	if msg := resp.firstError(); msg != "" {
		return nil, fmt.Errorf("printables model detail: %s", msg)
	}
	if resp.Data.Model == nil {
		return nil, fmt.Errorf("printables model %q not found", id)
	}
	var out []printablesModelFile
	appendGroup := func(category string, entries []printablesFileEntry) {
		for _, e := range entries {
			out = append(out, printablesModelFile{
				ID:          rawIDString(e.ID),
				Name:        e.Name,
				Format:      inferFormatFromName(e.Name),
				APICategory: category,
				SizeBytes:   e.FileSize,
			})
		}
	}
	appendGroup("STL", resp.Data.Model.STLs)
	appendGroup("GCODE", resp.Data.Model.Gcodes)
	appendGroup("SLA", resp.Data.Model.Slas)
	appendGroup("OTHER", resp.Data.Model.OtherFiles)
	return out, nil
}

// thingiverseFileEntry is one file entry from GET /things/{id}/files.
// DirectURL is a pointer because the API returns null for non-printable
// formats (e.g. PDF instructions) that have no raw CDN link.
type thingiverseFileEntry struct {
	ID            json.RawMessage `json:"id"`
	Name          string          `json:"name"`
	Size          int64           `json:"size"`
	DownloadURL   string          `json:"download_url"`
	DirectURL     *string         `json:"direct_url"`
	DownloadCount int             `json:"download_count"`
}

// thingiverseFiles fetches the file list for a Thingiverse thing.
func thingiverseFiles(ctx context.Context, c *client.Client, id string) ([]thingiverseFileEntry, error) {
	target := fmt.Sprintf("https://api.thingiverse.com/things/%s/files", url.PathEscape(id))
	data, err := c.Get(ctx, target, nil)
	if err != nil {
		return nil, err
	}
	var entries []thingiverseFileEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing thingiverse files response: %w", err)
	}
	return entries, nil
}

// thingiverseFileURL prefers the raw CDN direct_url (no auth needed) and
// falls back to the proxied download_url (which goes through
// api.thingiverse.com and so picks up the Bearer token via host_auth.go).
func thingiverseFileURL(e thingiverseFileEntry) string {
	if e.DirectURL != nil && strings.TrimSpace(*e.DirectURL) != "" {
		return *e.DirectURL
	}
	return e.DownloadURL
}

// ---------------------------------------------------------------------
// Printables download-link resolution
// ---------------------------------------------------------------------

// printablesDownloadFileRequest is one entry of getDownloadLink's $files
// argument. Confirmed against the live API (research's DownloadFileInput
// shape — a singular "id" field with an uppercase fileType — was rejected
// with a GraphQL validation error): the real input type takes a plural
// "ids" array grouped per file type, and fileType is lowercase ("stl",
// "gcode", "sla", "other"), not the uppercase category names used
// elsewhere in this API's schema (stls/gcodes/slas/otherFiles).
type printablesDownloadFileRequest struct {
	IDs      []string `json:"ids"`
	FileType string   `json:"fileType"`
}

type printablesDownloadLinkFile struct {
	ID       string
	Link     string
	FileType string
}

type printablesDownloadLinkResult struct {
	Link  string
	TTL   int
	Files []printablesDownloadLinkFile
}

// groupPrintablesFilesByType builds getDownloadLink's $files argument from a
// set of selected files, grouping IDs by their lowercased APICategory (the
// live API rejects a per-file "id" entry — see printablesDownloadFileRequest
// — and expects one {ids: [...], fileType} entry per file type instead).
// Iterates in a fixed category order so requests are deterministic and
// easy to diff/log.
func groupPrintablesFilesByType(files []printablesModelFile) []printablesDownloadFileRequest {
	grouped := map[string][]string{}
	for _, f := range files {
		ft := strings.ToLower(f.APICategory)
		grouped[ft] = append(grouped[ft], f.ID)
	}
	var out []printablesDownloadFileRequest
	for _, ft := range []string{"stl", "gcode", "sla", "other"} {
		if ids, ok := grouped[ft]; ok {
			out = append(out, printablesDownloadFileRequest{IDs: ids, FileType: ft})
			delete(grouped, ft)
		}
	}
	// Any category outside the four known lists (shouldn't happen given
	// printablesFiles only ever tags STL/GCODE/SLA/OTHER) still gets sent
	// rather than silently dropped.
	for ft, ids := range grouped {
		out = append(out, printablesDownloadFileRequest{IDs: ids, FileType: ft})
	}
	return out
}

// printablesGetDownloadLink resolves signed, unauthenticated, time-limited
// download links for the given files via Printables' getDownloadLink
// mutation. Despite being a GraphQL "mutation" it does not alter any
// user-visible remote state (it only mints a link), but the wire verb is a
// real POST that mutates in the schema's own terms, so this uses c.Post
// (not the read-only PostQueryWithParams path) — matching Printables'
// getDownloadLink naming and letting the transport's verify-mode gate
// treat it like any other mutating call.
func printablesGetDownloadLink(ctx context.Context, c *client.Client, printID string, files []printablesDownloadFileRequest) (*printablesDownloadLinkResult, error) {
	body := map[string]any{
		"query": `mutation GetDownloadLink($printId: ID!, $files: [DownloadFileInput], $source: DownloadSourceEnum!) { getDownloadLink(printId: $printId, files: $files, source: $source) { ok errors { field messages } output { link ttl count files { id link fileType } } } }`,
		"variables": map[string]any{
			"printId": printID,
			"files":   files,
			"source":  "model_detail",
		},
	}
	data, _, err := c.Post(ctx, printablesGraphQLURL, body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		graphQLErrors
		Data struct {
			GetDownloadLink struct {
				OK     bool `json:"ok"`
				Errors []struct {
					Field    string   `json:"field"`
					Messages []string `json:"messages"`
				} `json:"errors"`
				Output *struct {
					Link  string `json:"link"`
					TTL   int    `json:"ttl"`
					Count int    `json:"count"`
					Files []struct {
						ID       json.RawMessage `json:"id"`
						Link     string          `json:"link"`
						FileType string          `json:"fileType"`
					} `json:"files"`
				} `json:"output"`
			} `json:"getDownloadLink"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing printables getDownloadLink response: %w", err)
	}
	if msg := resp.firstError(); msg != "" {
		return nil, fmt.Errorf("printables getDownloadLink: %s", msg)
	}
	gdl := resp.Data.GetDownloadLink
	if !gdl.OK {
		var msgs []string
		for _, e := range gdl.Errors {
			msgs = append(msgs, fmt.Sprintf("%s: %s", e.Field, strings.Join(e.Messages, "; ")))
		}
		if len(msgs) == 0 {
			msgs = append(msgs, "request rejected")
		}
		return nil, fmt.Errorf("printables getDownloadLink: %s", strings.Join(msgs, "; "))
	}
	if gdl.Output == nil {
		return nil, fmt.Errorf("printables getDownloadLink returned no output")
	}
	result := &printablesDownloadLinkResult{Link: gdl.Output.Link, TTL: gdl.Output.TTL}
	for _, f := range gdl.Output.Files {
		result.Files = append(result.Files, printablesDownloadLinkFile{
			ID:       rawIDString(f.ID),
			Link:     f.Link,
			FileType: f.FileType,
		})
	}
	return result, nil
}

// findPrintablesFileLink returns the resolved link for fileID from a
// getDownloadLink result, falling back to the top-level (bundle) link when
// there is exactly one file in the result set.
func findPrintablesFileLink(link *printablesDownloadLinkResult, fileID string) string {
	if link == nil {
		return ""
	}
	for _, f := range link.Files {
		if f.ID == fileID {
			return f.Link
		}
	}
	if len(link.Files) <= 1 {
		return link.Link
	}
	return ""
}
