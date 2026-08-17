// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written shared helpers for the novel (hand-authored) command family:
// duplicates, license audit, history diff, similar, job download/resume,
// feed, follow, snapshot create/verify, library doctor, formats gaps, and
// designer stats. Deliberately independent of models_shared.go (owned by a
// parallel task implementing search/download/files) so the two efforts never
// collide on the same file. Every source integration here is best-effort:
// fields not confirmed by research degrade to zero-values instead of
// panicking, since Printables/Thingiverse/Cults3D response shapes are only
// partially documented.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
)

// flexNumber unmarshals a JSON number OR a numeric string into a float64.
// Printables' live GraphQL API is inconsistent about this: ratingAvg comes
// back as a JSON string (e.g. "4.9394667307") despite being declared a
// Float in their schema, confirmed against the live API during research.
// Using a plain float64 field there fails to parse and breaks every command
// that fetches model detail; this type absorbs the inconsistency instead.
type flexNumber float64

func (f *flexNumber) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" {
		*f = 0
		return nil
	}
	var asFloat float64
	if err := json.Unmarshal(data, &asFloat); err == nil {
		*f = flexNumber(asFloat)
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err != nil {
		return fmt.Errorf("flexNumber: %w", err)
	}
	if strings.TrimSpace(asString) == "" {
		*f = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(asString, 64)
	if err != nil {
		return fmt.Errorf("flexNumber: cannot parse %q as a number", asString)
	}
	*f = flexNumber(parsed)
	return nil
}

// --- model key / reference parsing ---

// parseModelKey splits a "<source>:<id>" model key, e.g. "printables:3161".
func parseModelKey(key string) (source, id string, err error) {
	source, id, ok := strings.Cut(strings.TrimSpace(key), ":")
	if !ok || source == "" || id == "" {
		return "", "", fmt.Errorf("invalid model key %q: expected <source>:<id> (e.g. printables:12345)", key)
	}
	return source, id, nil
}

func modelKey(source, id string) string { return source + ":" + id }

var (
	printablesURLRe  = regexp.MustCompile(`printables\.com/(?:[a-z]{2}/)?model/(\d+)`)
	thingiverseURLRe = regexp.MustCompile(`thingiverse\.com/thing:(\d+)`)
)

// parseModelRef accepts either a "<source>:<id>" model key or a full model
// URL from one of the three sites and returns the normalized source/id pair.
// Best-effort: an unrecognized URL shape returns an error rather than a
// guess, since a wrong id would silently target the wrong model.
func parseModelRef(ref string) (source, id string, err error) {
	ref = strings.TrimSpace(ref)
	if !strings.Contains(ref, "://") {
		return parseModelKey(ref)
	}
	switch {
	case strings.Contains(ref, "printables.com"):
		if m := printablesURLRe.FindStringSubmatch(ref); m != nil {
			return "printables", m[1], nil
		}
	case strings.Contains(ref, "thingiverse.com"):
		if m := thingiverseURLRe.FindStringSubmatch(ref); m != nil {
			return "thingiverse", m[1], nil
		}
	case strings.Contains(ref, "cults3d.com"):
		if u, uerr := url.Parse(ref); uerr == nil {
			segs := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(segs) > 0 && segs[len(segs)-1] != "" {
				return "cults3d", segs[len(segs)-1], nil
			}
		}
	}
	return "", "", fmt.Errorf("could not parse a model reference (source:id or known site URL) from %q", ref)
}

func validSource(s string) bool {
	switch s {
	case "printables", "thingiverse", "cults3d":
		return true
	}
	return false
}

// --- normalization / fuzzy matching (used by duplicates & formats gaps) ---

var nonWordRE = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// normalizeName lowercases and strips punctuation down to whitespace-joined
// word tokens, so "3D Benchy!" and "3d benchy" compare equal.
func normalizeName(s string) string {
	s = nonWordRE.ReplaceAllString(strings.ToLower(s), " ")
	return strings.Join(strings.Fields(s), " ")
}

func tokenSet(s string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, tok := range strings.Fields(normalizeName(s)) {
		set[tok] = struct{}{}
	}
	return set
}

// jaccardSimilarity returns the intersection-over-union of two token sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// --- cross-source search ---

// novelResult is a normalized search hit shared by duplicates, similar, and
// formats gaps.
type novelResult struct {
	Source   string `json:"source"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Designer string `json:"designer,omitempty"`
	Paid     bool   `json:"paid"`
	Likes    int    `json:"likes"`
}

// sourceStatus reports whether a live per-source call in a fan-out succeeded,
// so partial failures (e.g. missing Thingiverse/Cults3D credentials) are
// visible to the caller instead of silently dropping that source's results.
type sourceStatus struct {
	Source string `json:"source"`
	OK     bool   `json:"ok"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

const printablesSearchQuery = `query Search($q: String!, $limit: Int) {
  searchPrints2(query: $q, limit: $limit) {
    totalCount
    items {
      id
      name
      slug
      price
      premium
      likesCount
      user { handle publicUsername }
    }
  }
}`

func searchPrintablesLive(ctx context.Context, c *client.Client, query string, limit int) ([]novelResult, error) {
	if limit <= 0 {
		limit = 20
	}
	data, _, err := c.Post(ctx, "https://api.printables.com/graphql/", map[string]any{
		"query":     printablesSearchQuery,
		"variables": map[string]any{"q": query, "limit": limit},
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			SearchPrints2 struct {
				Items []struct {
					ID      string      `json:"id"`
					Name    string      `json:"name"`
					Slug    string      `json:"slug"`
					Price   *flexNumber `json:"price"`
					Premium bool        `json:"premium"`
					User    struct {
						Handle         string `json:"handle"`
						PublicUsername string `json:"publicUsername"`
					} `json:"user"`
					LikesCount int `json:"likesCount"`
				} `json:"items"`
			} `json:"searchPrints2"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return nil, fmt.Errorf("parsing printables search response: %w", uerr)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("printables search: %s", env.Errors[0].Message)
	}
	out := make([]novelResult, 0, len(env.Data.SearchPrints2.Items))
	for _, it := range env.Data.SearchPrints2.Items {
		designer := it.User.PublicUsername
		if designer == "" {
			designer = it.User.Handle
		}
		out = append(out, novelResult{
			Source:   "printables",
			ID:       it.ID,
			Name:     it.Name,
			URL:      fmt.Sprintf("https://www.printables.com/model/%s-%s", it.ID, it.Slug),
			Designer: designer,
			Paid:     it.Premium || it.Price != nil,
			Likes:    it.LikesCount,
		})
	}
	return out, nil
}

func searchThingiverseLive(ctx context.Context, c *client.Client, query string, limit int) ([]novelResult, error) {
	if limit <= 0 {
		limit = 20
	}
	params := map[string]string{
		"type":     "things",
		"per_page": strconv.Itoa(limit),
		"page":     "1",
	}
	path := "https://api.thingiverse.com/search/" + url.PathEscape(query)
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	items := extractSearchResults(data, "hits", "data.hits")
	out := make([]novelResult, 0, len(items))
	for _, raw := range items {
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		id := fmt.Sprintf("%v", m["id"])
		if id == "" || id == "<nil>" {
			continue
		}
		out = append(out, novelResult{
			Source:   "thingiverse",
			ID:       id,
			Name:     getString(m, "name"),
			URL:      getString(m, "public_url"),
			Designer: guessThingiverseDesigner(m),
			Paid:     false, // Thingiverse things are always free downloads
			Likes:    getInt(m, "like_count"),
		})
	}
	return out, nil
}

// cults3DSearchQuery: creationsSearchBatch returns a CreationBatch envelope
// ({results: [Creation!]!, total: Int!}), not a bare array of Creation —
// confirmed via live GraphQL introspection against cults3d.com/graphql
// (__type(name: "CreationBatch")) after research-time guessing without a
// live account got this wrong. The "Batch" naming convention does NOT mean
// "returns a bare array" the way it does for e.g. Printables' items lists.
// Selects "url" (the full slugged page URL), not "shortUrl": Cults3D's
// Creation.shortUrl field returns a bare numeric redirect form (e.g.
// "https://cults3d.com/:2034048") that 403s when fetched directly rather
// than redirecting — confirmed live. "url" is the real working link.
const cults3DSearchQuery = `query SearchCreations($query: String!, $limit: Int, $offset: Int) {
  result: creationsSearchBatch(query: $query, limit: $limit, offset: $offset) {
    total
    results {
      identifier
      name
      url
      likesCount
      downloadsCount
      publishedAt
      creator { nick shortUrl }
    }
  }
}`

func searchCults3DLive(ctx context.Context, c *client.Client, query string, limit int) ([]novelResult, error) {
	if limit <= 0 {
		limit = 20
	}
	data, _, err := c.Post(ctx, "https://cults3d.com/graphql", map[string]any{
		"query":     cults3DSearchQuery,
		"variables": map[string]any{"query": query, "limit": limit, "offset": 0},
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Result struct {
				Total   int `json:"total"`
				Results []struct {
					Identifier  string `json:"identifier"`
					Name        string `json:"name"`
					URL         string `json:"url"`
					LikesCount  int    `json:"likesCount"`
					Downloads   int    `json:"downloadsCount"`
					PublishedAt string `json:"publishedAt"`
					Creator     struct {
						Nick     string `json:"nick"`
						ShortURL string `json:"shortUrl"`
					} `json:"creator"`
				} `json:"results"`
			} `json:"result"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return nil, fmt.Errorf("parsing cults3d search response: %w", uerr)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("cults3d search: %s", env.Errors[0].Message)
	}
	out := make([]novelResult, 0, len(env.Data.Result.Results))
	for _, it := range env.Data.Result.Results {
		out = append(out, novelResult{
			Source:   "cults3d",
			ID:       it.Identifier,
			Name:     it.Name,
			URL:      it.URL,
			Designer: it.Creator.Nick,
			Paid:     false, // paid/free is not exposed by creationsSearchBatch; unknown, not asserted
			Likes:    it.LikesCount,
		})
	}
	return out, nil
}

// searchAllSourcesLive fans out the same query to all three sources. Each
// source's failure (missing credentials, network error, API error) is
// reported in the returned status slice rather than aborting the whole
// search — a combo CLI across 3 independently-owned APIs must degrade one
// source at a time.
func searchAllSourcesLive(ctx context.Context, c *client.Client, query string, limitPerSource int) ([]novelResult, []sourceStatus) {
	var all []novelResult
	var statuses []sourceStatus

	if res, err := searchPrintablesLive(ctx, c, query, limitPerSource); err != nil {
		statuses = append(statuses, sourceStatus{Source: "printables", OK: false, Error: err.Error()})
	} else {
		all = append(all, res...)
		statuses = append(statuses, sourceStatus{Source: "printables", OK: true, Count: len(res)})
	}

	if res, err := searchThingiverseLive(ctx, c, query, limitPerSource); err != nil {
		statuses = append(statuses, sourceStatus{Source: "thingiverse", OK: false, Error: err.Error()})
	} else {
		all = append(all, res...)
		statuses = append(statuses, sourceStatus{Source: "thingiverse", OK: true, Count: len(res)})
	}

	if res, err := searchCults3DLive(ctx, c, query, limitPerSource); err != nil {
		statuses = append(statuses, sourceStatus{Source: "cults3d", OK: false, Error: err.Error()})
	} else {
		all = append(all, res...)
		statuses = append(statuses, sourceStatus{Source: "cults3d", OK: true, Count: len(res)})
	}

	return all, statuses
}

// --- model detail (get) ---

type modelFileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size,omitempty"`
	// URL is a direct, unauthenticated download URL, when the source's
	// researched API surface confirms one (Thingiverse's ThingiverseFile
	// carries download_url/direct_url). Printables and Cults3D expose no
	// such field via the endpoints this CLI is built against, so URL stays
	// empty for their files rather than guessing at one.
	URL    string `json:"url,omitempty"`
	Format string `json:"format"`
}

type modelDetail struct {
	Source      string          `json:"source"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	URL         string          `json:"url"`
	Designer    string          `json:"designer,omitempty"`
	DesignerID  string          `json:"designer_id,omitempty"`
	Likes       int             `json:"likes"`
	Downloads   int             `json:"downloads"`
	Rating      float64         `json:"rating,omitempty"`
	License     string          `json:"license"`
	PublishedAt string          `json:"published_at,omitempty"`
	Files       []modelFileInfo `json:"files"`
	// Found is false when the source confirmed the model no longer exists
	// (e.g. Printables returned a null model, or the API 404'd). A nil error
	// with Found=false means "confirmed gone"; a non-nil error means "could
	// not check" (auth/network failure), which callers must not conflate
	// with delisted.
	Found bool `json:"found"`
}

// guessFormat maps a filename's extension to one of the CLI's tracked
// formats {stl, 3mf, step, gcode}, falling back to "other".
func guessFormat(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".stl"):
		return "stl"
	case strings.HasSuffix(lower, ".3mf"):
		return "3mf"
	case strings.HasSuffix(lower, ".step"), strings.HasSuffix(lower, ".stp"):
		return "step"
	case strings.HasSuffix(lower, ".gcode"), strings.HasSuffix(lower, ".g"):
		return "gcode"
	default:
		return "other"
	}
}

const printablesModelDetailQuery = `query ModelDetail($id: ID!) {
  model: print(id: $id) {
    id
    name
    slug
    filesType
    ratingAvg
    likesCount
    downloadCount
    datePublished
    user { id handle publicUsername }
    stls { id name fileSize }
    gcodes { id name fileSize }
    slas { id name fileSize }
    otherFiles { id name fileSize }
    license { name }
  }
}`

func fetchPrintablesDetail(ctx context.Context, c *client.Client, id string) (*modelDetail, error) {
	data, _, err := c.Post(ctx, "https://api.printables.com/graphql/", map[string]any{
		"query":     printablesModelDetailQuery,
		"variables": map[string]any{"id": id},
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Model *struct {
				ID            string     `json:"id"`
				Name          string     `json:"name"`
				Slug          string     `json:"slug"`
				RatingAvg     flexNumber `json:"ratingAvg"`
				LikesCount    int        `json:"likesCount"`
				DownloadCount int        `json:"downloadCount"`
				DatePublished string     `json:"datePublished"`
				User          struct {
					ID             string `json:"id"`
					Handle         string `json:"handle"`
					PublicUsername string `json:"publicUsername"`
				} `json:"user"`
				STLs []struct {
					Name     string `json:"name"`
					FileSize int64  `json:"fileSize"`
				} `json:"stls"`
				Gcodes []struct {
					Name     string `json:"name"`
					FileSize int64  `json:"fileSize"`
				} `json:"gcodes"`
				SLAs []struct {
					Name     string `json:"name"`
					FileSize int64  `json:"fileSize"`
				} `json:"slas"`
				OtherFiles []struct {
					Name     string `json:"name"`
					FileSize int64  `json:"fileSize"`
				} `json:"otherFiles"`
				License *struct {
					Name string `json:"name"`
				} `json:"license"`
			} `json:"model"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return nil, fmt.Errorf("parsing printables model response: %w", uerr)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("printables model detail: %s", env.Errors[0].Message)
	}
	if env.Data.Model == nil {
		return &modelDetail{Source: "printables", ID: id, Found: false}, nil
	}
	m := env.Data.Model
	designer := m.User.PublicUsername
	if designer == "" {
		designer = m.User.Handle
	}
	license := "unknown"
	if m.License != nil && m.License.Name != "" {
		license = m.License.Name
	}
	var files []modelFileInfo
	for _, f := range m.STLs {
		files = append(files, modelFileInfo{Name: f.Name, Size: f.FileSize, Format: "stl"})
	}
	for _, f := range m.Gcodes {
		files = append(files, modelFileInfo{Name: f.Name, Size: f.FileSize, Format: "gcode"})
	}
	for _, f := range m.SLAs {
		files = append(files, modelFileInfo{Name: f.Name, Size: f.FileSize, Format: "sla"})
	}
	for _, f := range m.OtherFiles {
		files = append(files, modelFileInfo{Name: f.Name, Size: f.FileSize, Format: guessFormat(f.Name)})
	}
	return &modelDetail{
		Source:      "printables",
		ID:          m.ID,
		Name:        m.Name,
		URL:         fmt.Sprintf("https://www.printables.com/model/%s-%s", m.ID, m.Slug),
		Designer:    designer,
		DesignerID:  m.User.ID,
		Likes:       m.LikesCount,
		Downloads:   m.DownloadCount,
		Rating:      float64(m.RatingAvg),
		License:     license,
		PublishedAt: m.DatePublished,
		Files:       files,
		Found:       true,
	}, nil
}

func fetchThingiverseDetail(ctx context.Context, c *client.Client, id string) (*modelDetail, error) {
	data, err := c.Get(ctx, "https://api.thingiverse.com/things/"+url.PathEscape(id), nil)
	if err != nil {
		if apiErr, ok := asAPIError(err); ok && apiErr.StatusCode == 404 {
			return &modelDetail{Source: "thingiverse", ID: id, Found: false}, nil
		}
		return nil, err
	}
	var m map[string]any
	if uerr := json.Unmarshal(data, &m); uerr != nil {
		return nil, fmt.Errorf("parsing thingiverse thing response: %w", uerr)
	}
	license := getString(m, "license")
	if license == "" {
		license = "unknown"
	}
	detail := &modelDetail{
		Source:      "thingiverse",
		ID:          id,
		Name:        getString(m, "name"),
		URL:         getString(m, "public_url"),
		Designer:    guessThingiverseDesigner(m),
		Likes:       getInt(m, "like_count"),
		Downloads:   getInt(m, "download_count"),
		License:     license,
		PublishedAt: getString(m, "modified"),
		Found:       true,
	}
	// Best-effort file listing via the confirmed files sub-endpoint. A
	// failure here (e.g. missing token) must not fail the whole detail
	// fetch — files just come back empty.
	if filesData, ferr := c.Get(ctx, "https://api.thingiverse.com/things/"+url.PathEscape(id)+"/files", nil); ferr == nil {
		var items []map[string]any
		if json.Unmarshal(filesData, &items) == nil {
			for _, it := range items {
				name := getString(it, "name")
				if name == "" {
					continue
				}
				// Prefer the raw CDN direct_url (no auth needed, more
				// reliable) over the proxied download_url, matching
				// models_shared.go's thingiverseFileURL. Both this fetcher
				// and that one feed download-capable commands, so they must
				// agree on which URL wins or the same file downloads
				// differently (and needs different auth) depending on which
				// command path a user takes.
				fileURL := getString(it, "direct_url")
				if fileURL == "" {
					fileURL = getString(it, "download_url")
				}
				detail.Files = append(detail.Files, modelFileInfo{
					Name:   name,
					Size:   int64(getFloat(it, "size")),
					URL:    fileURL,
					Format: guessFormat(name),
				})
			}
		}
	}
	return detail, nil
}

// Selects "url" (the full slugged page URL), not "shortUrl": Cults3D's
// Creation.shortUrl field returns a bare numeric redirect form that 403s
// when fetched directly rather than redirecting — confirmed live.
const cults3DDetailQuery = `query GetCreation($slug: String!) {
  model: creation(slug: $slug) {
    identifier
    name
    url
    likesCount
    downloadsCount
    viewsCount
    publishedAt
    category { name }
    license { code name }
    creator { nick shortUrl }
  }
}`

func fetchCults3DDetail(ctx context.Context, c *client.Client, slug string) (*modelDetail, error) {
	data, _, err := c.Post(ctx, "https://cults3d.com/graphql", map[string]any{
		"query":     cults3DDetailQuery,
		"variables": map[string]any{"slug": slug},
	})
	if err != nil {
		if apiErr, ok := asAPIError(err); ok && apiErr.StatusCode == 404 {
			return &modelDetail{Source: "cults3d", ID: slug, Found: false}, nil
		}
		return nil, err
	}
	var env struct {
		Data struct {
			Model *struct {
				Identifier  string `json:"identifier"`
				Name        string `json:"name"`
				URL         string `json:"url"`
				LikesCount  int    `json:"likesCount"`
				Downloads   int    `json:"downloadsCount"`
				PublishedAt string `json:"publishedAt"`
				License     *struct {
					Code string `json:"code"`
					Name string `json:"name"`
				} `json:"license"`
				Creator struct {
					Nick     string `json:"nick"`
					ShortURL string `json:"shortUrl"`
				} `json:"creator"`
			} `json:"model"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return nil, fmt.Errorf("parsing cults3d creation response: %w", uerr)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("cults3d creation detail: %s", env.Errors[0].Message)
	}
	if env.Data.Model == nil {
		return &modelDetail{Source: "cults3d", ID: slug, Found: false}, nil
	}
	m := env.Data.Model
	license := "unknown"
	if m.License != nil {
		if m.License.Name != "" {
			license = m.License.Name
		} else if m.License.Code != "" {
			license = m.License.Code
		}
	}
	return &modelDetail{
		Source:      "cults3d",
		ID:          m.Identifier,
		Name:        m.Name,
		URL:         m.URL,
		Designer:    m.Creator.Nick,
		Likes:       m.LikesCount,
		Downloads:   m.Downloads,
		License:     license,
		PublishedAt: m.PublishedAt,
		// Cults3D's API does not expose a file listing (by design — Cults3D
		// search/metadata only, no file downloads via the API).
		Files: nil,
		Found: true,
	}, nil
}

// fetchModelDetail dispatches to the correct per-source fetcher. A non-nil
// error means the fetch could not be completed (auth/network/API error); a
// nil error with Found=false means the source confirmed the model is gone.
func fetchModelDetail(ctx context.Context, c *client.Client, source, id string) (*modelDetail, error) {
	switch source {
	case "printables":
		return fetchPrintablesDetail(ctx, c, id)
	case "thingiverse":
		return fetchThingiverseDetail(ctx, c, id)
	case "cults3d":
		return fetchCults3DDetail(ctx, c, id)
	default:
		return nil, fmt.Errorf("unknown source %q: expected printables, thingiverse, or cults3d", source)
	}
}

// isNoSuchTable reports whether err is SQLite's "no such table" error, used
// throughout the novel command family to degrade gracefully when reading a
// table owned by the parallel search/download task that hasn't run yet.
func isNoSuchTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}

// isNoSuchColumn reports whether err is SQLite's "no such column" error,
// used when reading printgoat_downloads (owned by the parallel task) with an
// assumed column list that may not match what that task actually created.
func isNoSuchColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such column")
}

// asAPIError unwraps a *client.APIError from err, if present.
func asAPIError(err error) (*client.APIError, bool) {
	var apiErr *client.APIError
	if As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// guessThingiverseDesigner best-effort extracts a designer/creator display
// name from a raw Thingiverse thing object. The confirmed field set for
// ThingiverseThing does not include a creator sub-object, but the live API
// commonly nests one; this degrades to "" (unknown) rather than panicking
// when it is absent or differently shaped.
func guessThingiverseDesigner(m map[string]any) string {
	for _, key := range []string{"creator", "author", "designer"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if name := getString(obj, "name"); name != "" {
			return name
		}
		first := getString(obj, "first_name")
		last := getString(obj, "last_name")
		if first != "" || last != "" {
			return strings.TrimSpace(first + " " + last)
		}
	}
	return ""
}

// --- generic JSON map accessors (defensive best-effort field reads) ---

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func getFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

func getInt(m map[string]any, key string) int {
	return int(getFloat(m, key))
}
