// Package normalize unifies per-Actor dataset item shapes into one schema
// for cross-Actor search, novelty diffing, and digest rendering.
//
// Every Actor on Apify emits a different field shape — Twitter scrapers
// emit {full_text, user.screen_name}, Reddit scrapers emit {title, selftext,
// author}, news scrapers emit {title, content, source}, and so on. Without
// normalization, cross-Actor queries become per-Actor switch statements.
//
// Profiles ship embedded for the top newsletter-relevant Actors. Users can
// override or add profiles by dropping YAML files in ~/.apify-pp/profiles/.
package normalize

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed profiles/*.yaml
var embeddedProfiles embed.FS

// Item is the unified shape every normalized dataset item collapses to.
// Raw preserves the original JSON for users who need fields the profile
// didn't pull through.
type Item struct {
	URL             string          `json:"url,omitempty"`
	Title           string          `json:"title,omitempty"`
	Body            string          `json:"body,omitempty"`
	Author          string          `json:"author,omitempty"`
	PublishedAt     time.Time       `json:"published_at,omitempty"`
	EngagementScore int64           `json:"engagement_score,omitempty"`
	SourceActor     string          `json:"source_actor"`
	RunID           string          `json:"run_id,omitempty"`
	DatasetID       string          `json:"dataset_id,omitempty"`
	FetchedAt       time.Time       `json:"fetched_at"`
	Hash            string          `json:"hash"`
	Raw             json.RawMessage `json:"raw,omitempty"`
}

// Profile maps raw item fields to the unified shape. Each field accepts a
// list of dotted paths tried in order until one resolves to a non-empty value.
type Profile struct {
	Actor       string   `yaml:"actor"`
	Title       []string `yaml:"title,omitempty"`
	URL         []string `yaml:"url,omitempty"`
	Body        []string `yaml:"body,omitempty"`
	Author      []string `yaml:"author,omitempty"`
	PublishedAt []string `yaml:"published_at,omitempty"`
	Engagement  []string `yaml:"engagement,omitempty"`
}

// Registry holds embedded + user-overridden profiles, keyed by Actor name
// (lowercase username/actor-name with no version suffix).
type Registry struct {
	profiles map[string]*Profile
	fallback *Profile
}

// NewRegistry loads embedded profiles, then merges any YAML files found in
// ~/.apify-pp/profiles/ (overriding embedded keys on collision).
func NewRegistry() (*Registry, error) {
	r := &Registry{profiles: map[string]*Profile{}}

	// Load embedded
	entries, err := embeddedProfiles.ReadDir("profiles")
	if err != nil {
		return nil, fmt.Errorf("reading embedded profiles: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := embeddedProfiles.ReadFile("profiles/" + e.Name())
		if err != nil {
			continue
		}
		p := &Profile{}
		if err := yaml.Unmarshal(data, p); err != nil {
			continue
		}
		if p.Actor == "default" {
			r.fallback = p
			continue
		}
		r.profiles[strings.ToLower(p.Actor)] = p
	}

	// Load user overrides
	home, err := os.UserHomeDir()
	if err == nil {
		userDir := filepath.Join(home, ".apify-pp", "profiles")
		_ = fs.WalkDir(os.DirFS(userDir), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return nil
			}
			data, err := os.ReadFile(filepath.Join(userDir, path))
			if err != nil {
				return nil
			}
			p := &Profile{}
			if err := yaml.Unmarshal(data, p); err != nil {
				return nil
			}
			if p.Actor == "default" {
				r.fallback = p
				return nil
			}
			r.profiles[strings.ToLower(p.Actor)] = p
			return nil
		})
	}

	if r.fallback == nil {
		r.fallback = defaultFallback()
	}
	return r, nil
}

// Lookup returns the profile for an Actor name, or the fallback profile if
// no specific one is registered.
//
// The Actor key on Apify is "username/actor-name" or "username~actor-name";
// we normalize to "username/actor-name" lowercase and strip any version.
func (r *Registry) Lookup(actor string) *Profile {
	key := normalizeActorKey(actor)
	if p, ok := r.profiles[key]; ok {
		return p
	}
	return r.fallback
}

// Normalize applies a profile to a raw item map, returning a unified Item.
// Hash is computed from URL when present, otherwise from the SHA-256 of the
// serialized raw item. Hash powers --only-new dedupe and cross-run novelty.
func (r *Registry) Normalize(actor string, rawItem map[string]any) (*Item, error) {
	profile := r.Lookup(actor)
	rawBytes, err := json.Marshal(rawItem)
	if err != nil {
		return nil, err
	}
	item := &Item{
		SourceActor: actor,
		FetchedAt:   time.Now().UTC(),
		Raw:         rawBytes,
	}
	item.Title = firstString(rawItem, profile.Title)
	item.URL = firstString(rawItem, profile.URL)
	item.Body = firstString(rawItem, profile.Body)
	item.Author = firstString(rawItem, profile.Author)
	if ts := firstString(rawItem, profile.PublishedAt); ts != "" {
		item.PublishedAt = parseTime(ts)
	}
	item.EngagementScore = firstInt(rawItem, profile.Engagement)
	item.Hash = computeHash(item.URL, rawBytes)
	return item, nil
}

// NormalizeBatch normalizes a slice of raw items for the same Actor.
// Errors on individual items are skipped; the caller gets back what worked.
func (r *Registry) NormalizeBatch(actor string, raw []map[string]any) []*Item {
	out := make([]*Item, 0, len(raw))
	for _, m := range raw {
		item, err := r.Normalize(actor, m)
		if err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

// ActorNames returns the list of registered Actor profile keys (sorted),
// for doctor reporting and `apify-pp profile list`.
func (r *Registry) ActorNames() []string {
	out := make([]string, 0, len(r.profiles))
	for k := range r.profiles {
		out = append(out, k)
	}
	return out
}

// --- helpers ---

func normalizeActorKey(actor string) string {
	a := strings.ToLower(actor)
	a = strings.ReplaceAll(a, "~", "/")
	// Strip ":version" suffix if present (apidojo/twitter-scraper-lite:1.2.3)
	if idx := strings.LastIndex(a, ":"); idx > 0 && !strings.Contains(a[idx:], "/") {
		a = a[:idx]
	}
	return a
}

// firstString walks dotted paths and returns the first non-empty string.
func firstString(m map[string]any, paths []string) string {
	for _, p := range paths {
		v := walkDotted(m, p)
		if s, ok := v.(string); ok && s != "" {
			return s
		}
		// Numeric fallback: stringify ints/floats so paths like "id" work
		if v != nil {
			switch n := v.(type) {
			case float64:
				return fmt.Sprintf("%v", n)
			case int:
				return fmt.Sprintf("%d", n)
			case int64:
				return fmt.Sprintf("%d", n)
			}
		}
	}
	return ""
}

func firstInt(m map[string]any, paths []string) int64 {
	for _, p := range paths {
		v := walkDotted(m, p)
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int:
			return int64(n)
		case int64:
			return n
		case string:
			// Some Actors stringify counts ("1.2K"). Skip those for now.
			if n == "" {
				continue
			}
		}
	}
	return 0
}

func walkDotted(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var cur any = m
	for _, part := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[part]
	}
	return cur
}

func parseTime(s string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC1123,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func computeHash(url string, raw []byte) string {
	h := sha256.New()
	if url != "" {
		h.Write([]byte(url))
	} else {
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func defaultFallback() *Profile {
	return &Profile{
		Actor:       "default",
		Title:       []string{"title", "headline", "name", "subject"},
		URL:         []string{"url", "link", "permalink", "webUrl", "guid"},
		Body:        []string{"body", "text", "content", "description", "summary", "selftext"},
		Author:      []string{"author", "user.name", "user.username", "screen_name", "by"},
		PublishedAt: []string{"publishedAt", "published_at", "date", "createdAt", "created_at", "timestamp", "publishDate"},
		Engagement:  []string{"score", "favorites", "favoriteCount", "likeCount", "likes", "upvotes", "ups", "viewCount"},
	}
}
