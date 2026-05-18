// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

// Package granola provides typed readers for Granola.ai's local cache file
// and clients for Granola's internal API. The cache lives at
// ~/Library/Application Support/Granola/cache-v6.json (override via
// GRANOLA_CACHE_PATH); see https://github.com/getprobo/getprobo and the
// granola.py community client for the originating reverse-engineering.
package granola

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola/safestorage"
)

// supportDir returns the Granola support directory, honoring
// GRANOLA_SUPPORT_DIR for tests. PATCH(encrypted-cache): shared between
// the plaintext and encrypted path resolvers so tests can point both at
// the same tmp directory with a single env var.
func supportDir() string {
	if v := os.Getenv("GRANOLA_SUPPORT_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Granola")
}

// DefaultCachePath returns the macOS-default plaintext cache path,
// honoring the GRANOLA_CACHE_PATH override used by tests. The plaintext
// path is what callers default to when no override is set; LoadCache
// internally prefers the encrypted sibling (`cache-v6.json.enc`) when
// it exists. See DefaultEncryptedCachePath.
func DefaultCachePath() string {
	if v := os.Getenv("GRANOLA_CACHE_PATH"); v != "" {
		return v
	}
	return filepath.Join(supportDir(), "cache-v6.json")
}

// PATCH(encrypted-cache): Granola desktop began writing cache-v6.json.enc
// alongside the plaintext cache around May 2026 and stopped updating the
// plaintext copy. This helper returns the .enc path so LoadCache can
// prefer it. See safestorage/testdata/scheme.md for the empirical
// encryption scheme.
func DefaultEncryptedCachePath() string {
	return filepath.Join(supportDir(), "cache-v6.json.enc")
}

// ResolveCachePath picks the cache file LoadCache will read. Order:
//
//  1. GRANOLA_CACHE_PATH (explicit test/user override, treated as plaintext)
//  2. cache-v6.json.enc next to the default support dir, when present
//  3. plaintext cache-v6.json (legacy fallback for pre-encryption Granola)
//
// Returns the resolved path and whether decryption is needed.
func ResolveCachePath() (path string, encrypted bool) {
	if v := os.Getenv("GRANOLA_CACHE_PATH"); v != "" {
		return v, false
	}
	enc := DefaultEncryptedCachePath()
	if _, err := os.Stat(enc); err == nil {
		return enc, true
	}
	return DefaultCachePath(), false
}

// Document is a Granola meeting note. The notes field is left as
// json.RawMessage so callers can either decode it themselves or hand it
// straight to the TipTap renderer in tiptap.go.
type Document struct {
	ID                    string               `json:"id"`
	CreatedAt             string               `json:"created_at,omitempty"`
	UpdatedAt             string               `json:"updated_at,omitempty"`
	DeletedAt             *string              `json:"deleted_at,omitempty"`
	Title                 string               `json:"title,omitempty"`
	Notes                 json.RawMessage      `json:"notes,omitempty"`
	NotesPlain            string               `json:"notes_plain,omitempty"`
	NotesMarkdown         string               `json:"notes_markdown,omitempty"`
	Transcribe            bool                 `json:"transcribe,omitempty"`
	Type                  string               `json:"type,omitempty"`
	ValidMeeting          bool                 `json:"valid_meeting,omitempty"`
	WorkspaceID           string               `json:"workspace_id,omitempty"`
	LastIndexedAt         string               `json:"last_indexed_at,omitempty"`
	HasShareableLink      bool                 `json:"has_shareable_link,omitempty"`
	SharingLinkVisibility string               `json:"sharing_link_visibility,omitempty"`
	CreationSource        string               `json:"creation_source,omitempty"`
	PrivacyModeEnabled    bool                 `json:"privacy_mode_enabled,omitempty"`
	GoogleCalendarEvent   *GoogleCalendarEvent `json:"google_calendar_event,omitempty"`
	People                *DocPeople           `json:"people,omitempty"`
	UserID                string               `json:"user_id,omitempty"`
	Summary               json.RawMessage      `json:"summary,omitempty"`
	Overview              json.RawMessage      `json:"overview,omitempty"`
	SelectedTemplate      json.RawMessage      `json:"selected_template,omitempty"`
}

// DocPeople carries the people block from a document. Granola stores both
// the attendee list (who was invited) and creator (who recorded).
type DocPeople struct {
	Attendees []DocPerson `json:"attendees,omitempty"`
	Creator   *DocPerson  `json:"creator,omitempty"`
}

// DocPerson is the lightweight person record inside documents.people.
type DocPerson struct {
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	ResponseStatus string `json:"response_status,omitempty"`
	Avatar         string `json:"avatar,omitempty"`
}

// TranscriptSegment matches the cache shape under state.transcripts[doc_id][].
type TranscriptSegment struct {
	ID                string  `json:"id,omitempty"`
	DocumentID        string  `json:"document_id,omitempty"`
	Source            string  `json:"source,omitempty"` // microphone | system | mixed
	Text              string  `json:"text,omitempty"`
	StartTimestamp    string  `json:"start_timestamp,omitempty"` // ISO-8601
	EndTimestamp      string  `json:"end_timestamp,omitempty"`
	Confidence        float64 `json:"confidence,omitempty"`
	IsFinal           bool    `json:"is_final,omitempty"`
	TranscriberUserID *string `json:"transcriber_user_id,omitempty"`
}

// MeetingMetadata is keyed by document_id in state.meetingsMetadata.
type MeetingMetadata struct {
	Creator      *CalendarInvitee  `json:"creator,omitempty"`
	Attendees    []CalendarInvitee `json:"attendees,omitempty"`
	Conferencing json.RawMessage   `json:"conferencing,omitempty"`
	URL          string            `json:"url,omitempty"`
}

// CalendarInvitee is one attendee on the calendar event.
type CalendarInvitee struct {
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	ResponseStatus string `json:"response_status,omitempty"`
}

// GoogleCalendarEvent is the raw calendar event attached to a doc.
type GoogleCalendarEvent struct {
	ID          string            `json:"id,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Location    string            `json:"location,omitempty"`
	Start       json.RawMessage   `json:"start,omitempty"`
	End         json.RawMessage   `json:"end,omitempty"`
	Attendees   []CalendarInvitee `json:"attendees,omitempty"`
	HtmlLink    string            `json:"htmlLink,omitempty"`
}

// DocumentListMetadata is the metadata for a Granola folder (also called a
// "documentList" in the cache).
type DocumentListMetadata struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title,omitempty"`
	Description          string          `json:"description,omitempty"`
	ParentDocumentListID string          `json:"parent_document_list_id,omitempty"`
	WorkspaceID          string          `json:"workspace_id,omitempty"`
	UserRole             string          `json:"user_role,omitempty"`
	Preset               string          `json:"preset,omitempty"`
	CreatedAt            string          `json:"created_at,omitempty"`
	UpdatedAt            string          `json:"updated_at,omitempty"`
	DeletedAt            *string         `json:"deleted_at,omitempty"`
	Visibility           string          `json:"visibility,omitempty"`
	Members              []DocPerson     `json:"members,omitempty"`
	Rules                json.RawMessage `json:"rules,omitempty"`
	IsFavourited         bool            `json:"is_favourited,omitempty"`
}

// ListRule is one rule in the listRules engine. The cache stores them as
// either an array of rules or an empty list per folder.
type ListRule struct {
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Field    string          `json:"field,omitempty"`
	Operator string          `json:"operator,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
	ListID   string          `json:"list_id,omitempty"`
}

// PanelTemplate is an AI panel template ("recipe-like" structured prompt).
type PanelTemplate struct {
	ID          string          `json:"id,omitempty"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Category    string          `json:"category,omitempty"`
	Slug        string          `json:"slug,omitempty"`
	IsGranola   bool            `json:"is_granola,omitempty"`
	Sections    json.RawMessage `json:"sections,omitempty"`
}

// Recipe is one entry in publicRecipes / userRecipes / sharedRecipes. The
// cache shapes the description+instructions under config{}, which we
// flatten on the typed surface via the Description/Instructions accessors.
type Recipe struct {
	ID            string          `json:"id,omitempty"`
	Slug          string          `json:"slug,omitempty"`
	UserID        string          `json:"user_id,omitempty"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	Visibility    string          `json:"visibility,omitempty"`
	Config        RecipeConfig    `json:"config,omitempty"`
	CreatedAt     string          `json:"created_at,omitempty"`
	UpdatedAt     string          `json:"updated_at,omitempty"`
	DeletedAt     *string         `json:"deleted_at,omitempty"`
	PublisherSlug string          `json:"publisher_slug,omitempty"`
	CreatorInfo   json.RawMessage `json:"creator_info,omitempty"`
	// Source is set by LoadCache to "public", "user", or "shared".
	Source string `json:"-"`
	// Name is derived from slug when the inner Config has no name. Most
	// publicRecipes use slug as the canonical identifier.
	Name string `json:"-"`
	// Category is filled from the parent panelTemplate if linked, otherwise
	// from the recipe's allowed_views.
	Category string `json:"-"`
}

// RecipeConfig is the inner config block on a Recipe.
type RecipeConfig struct {
	Description        string          `json:"description,omitempty"`
	Instructions       string          `json:"instructions,omitempty"`
	Examples           json.RawMessage `json:"examples,omitempty"`
	AllowedViews       []string        `json:"allowed_views,omitempty"`
	AllowedRecipeViews []string        `json:"allowed_recipe_views,omitempty"`
	GenerateArtifact   bool            `json:"generate_artifact,omitempty"`
	ShowInSharedTabs   bool            `json:"show_in_shared_tabs,omitempty"`
}

// RecipeUsage is one entry under state.recipesUsage keyed by recipe_id.
type RecipeUsage struct {
	RecipeID   string `json:"recipe_id,omitempty"`
	TotalCount string `json:"total_count,omitempty"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

// ChatThread is one entry under state.entities.chat_thread.
type ChatThread struct {
	ID          string         `json:"id,omitempty"`
	Type        string         `json:"type,omitempty"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	CreatedAt   string         `json:"created_at,omitempty"`
	UpdatedAt   string         `json:"updated_at,omitempty"`
	DeletedAt   *string        `json:"deleted_at,omitempty"`
	Data        ChatThreadData `json:"data,omitempty"`
}

// ChatThreadData is the inner data block on a ChatThread.
type ChatThreadData struct {
	UserID      string `json:"user_id,omitempty"`
	Title       string `json:"title,omitempty"`
	GroupingKey string `json:"grouping_key,omitempty"`
	DocumentID  string `json:"document_id,omitempty"`
}

// ChatMessage is one entry under state.entities.chat_message.
type ChatMessage struct {
	ID          string          `json:"id,omitempty"`
	Type        string          `json:"type,omitempty"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	CreatedAt   string          `json:"created_at,omitempty"`
	UpdatedAt   string          `json:"updated_at,omitempty"`
	Data        ChatMessageData `json:"data,omitempty"`
}

// ChatMessageData is the inner data block on a ChatMessage.
type ChatMessageData struct {
	ThreadID  string `json:"thread_id,omitempty"`
	Role      string `json:"role,omitempty"` // user | assistant | system
	TurnIndex int    `json:"turn_index,omitempty"`
	RawText   string `json:"raw_text,omitempty"`
}

// Workspace is one workspace under state.workspaceData.workspaces[].
type Workspace struct {
	PlanType  string          `json:"plan_type,omitempty"`
	Role      string          `json:"role,omitempty"`
	Workspace json.RawMessage `json:"workspace,omitempty"`
}

// Cache is the parsed cache-v6.json file.
type Cache struct {
	Version               int
	Documents             map[string]Document
	Transcripts           map[string][]TranscriptSegment
	MeetingsMetadata      map[string]MeetingMetadata
	DocumentLists         map[string][]string // list_id -> [meeting_ids]
	DocumentListsMetadata map[string]DocumentListMetadata
	ListRules             map[string][]ListRule
	PanelTemplates        []PanelTemplate
	PublicRecipes         []Recipe
	UserRecipes           []Recipe
	SharedRecipes         []Recipe
	RecipesUsage          map[string]RecipeUsage
	ChatThreads           map[string]ChatThread
	ChatMessages          map[string]ChatMessage
	Workspaces            []Workspace
	rawState              map[string]json.RawMessage
}

// LoadCache reads and parses the cache file at path. Honors v3 (cache key
// is a stringified JSON blob) through v6 (top-level cache.state dict). The
// machine the press runs on uses v6; older versions are kept so a stale
// cache from a paused machine still loads.
func LoadCache(path string) (*Cache, error) {
	var encrypted bool
	if path == "" {
		path, encrypted = ResolveCachePath()
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache %s: %w", path, err)
	}

	// PATCH(encrypted-cache): when the resolver picked the .enc sibling,
	// decrypt through the safestorage package before parsing. D2 in the
	// plan: a decrypt failure is fatal rather than silently falling back
	// to the stale plaintext (which would mask the real problem).
	if encrypted {
		plain, err := safestorage.Decrypt(raw)
		if err != nil {
			if errors.Is(err, safestorage.ErrKeyUnavailable) {
				return nil, fmt.Errorf("reading encrypted cache %s: Keychain access unavailable (sign into Granola desktop or run `granola-pp-cli sync` to authorize Keychain access): %w", path, err)
			}
			return nil, fmt.Errorf("reading encrypted cache %s: %w", path, err)
		}
		defer safestorage.ZeroBytes(plain)
		raw = plain
	}

	// Phase 1: top-level container — either {"cache": {...}} (v4+) or
	// {"cache": "json-string"} (v3). We probe by trying to decode into a
	// map first; if cache is a string, we re-decode it.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parsing cache top-level: %w", err)
	}
	cacheRaw, ok := top["cache"]
	if !ok {
		return nil, fmt.Errorf("cache file missing top-level `cache` key")
	}

	// v3: cache is a JSON-stringified object.
	cacheRaw = unwrapStringifiedJSON(cacheRaw)

	var container struct {
		State   map[string]json.RawMessage `json:"state"`
		Version int                        `json:"version"`
	}
	if err := json.Unmarshal(cacheRaw, &container); err != nil {
		return nil, fmt.Errorf("parsing cache container: %w", err)
	}
	if container.State == nil {
		return nil, fmt.Errorf("cache file missing `state` block")
	}

	c := &Cache{
		Version:               container.Version,
		Documents:             map[string]Document{},
		Transcripts:           map[string][]TranscriptSegment{},
		MeetingsMetadata:      map[string]MeetingMetadata{},
		DocumentLists:         map[string][]string{},
		DocumentListsMetadata: map[string]DocumentListMetadata{},
		ListRules:             map[string][]ListRule{},
		RecipesUsage:          map[string]RecipeUsage{},
		ChatThreads:           map[string]ChatThread{},
		ChatMessages:          map[string]ChatMessage{},
		rawState:              container.State,
	}

	if v, ok := container.State["documents"]; ok {
		_ = json.Unmarshal(v, &c.Documents)
	}
	if v, ok := container.State["transcripts"]; ok {
		_ = json.Unmarshal(v, &c.Transcripts)
	}
	if v, ok := container.State["meetingsMetadata"]; ok {
		_ = json.Unmarshal(v, &c.MeetingsMetadata)
	}
	if v, ok := container.State["documentLists"]; ok {
		_ = json.Unmarshal(v, &c.DocumentLists)
	}
	if v, ok := container.State["documentListsMetadata"]; ok {
		_ = json.Unmarshal(v, &c.DocumentListsMetadata)
	}
	if v, ok := container.State["listRules"]; ok {
		// listRules per-folder is either [] or array of rule objects
		var tmp map[string]json.RawMessage
		if err := json.Unmarshal(v, &tmp); err == nil {
			for k, raw := range tmp {
				var rules []ListRule
				if err := json.Unmarshal(raw, &rules); err == nil {
					c.ListRules[k] = rules
				}
			}
		}
	}
	if v, ok := container.State["panelTemplates"]; ok {
		_ = json.Unmarshal(v, &c.PanelTemplates)
		// Derive slug from title when missing.
		for i, t := range c.PanelTemplates {
			if t.Slug == "" {
				c.PanelTemplates[i].Slug = slugify(t.Title)
			}
		}
	}
	if v, ok := container.State["publicRecipes"]; ok {
		_ = json.Unmarshal(v, &c.PublicRecipes)
		for i := range c.PublicRecipes {
			c.PublicRecipes[i].Source = "public"
			if c.PublicRecipes[i].Name == "" {
				c.PublicRecipes[i].Name = c.PublicRecipes[i].Slug
			}
		}
	}
	if v, ok := container.State["userRecipes"]; ok {
		_ = json.Unmarshal(v, &c.UserRecipes)
		for i := range c.UserRecipes {
			c.UserRecipes[i].Source = "user"
			if c.UserRecipes[i].Name == "" {
				c.UserRecipes[i].Name = c.UserRecipes[i].Slug
			}
		}
	}
	if v, ok := container.State["sharedRecipes"]; ok {
		_ = json.Unmarshal(v, &c.SharedRecipes)
		for i := range c.SharedRecipes {
			c.SharedRecipes[i].Source = "shared"
			if c.SharedRecipes[i].Name == "" {
				c.SharedRecipes[i].Name = c.SharedRecipes[i].Slug
			}
		}
	}
	if v, ok := container.State["recipesUsage"]; ok {
		_ = json.Unmarshal(v, &c.RecipesUsage)
	}
	if v, ok := container.State["entities"]; ok {
		var ent struct {
			ChatThread  map[string]ChatThread  `json:"chat_thread"`
			ChatMessage map[string]ChatMessage `json:"chat_message"`
		}
		if err := json.Unmarshal(v, &ent); err == nil {
			if ent.ChatThread != nil {
				c.ChatThreads = ent.ChatThread
			}
			if ent.ChatMessage != nil {
				c.ChatMessages = ent.ChatMessage
			}
		}
	}
	if v, ok := container.State["workspaceData"]; ok {
		var ws struct {
			Workspaces []Workspace `json:"workspaces"`
		}
		if err := json.Unmarshal(v, &ws); err == nil {
			c.Workspaces = ws.Workspaces
		}
	}

	return c, nil
}

// unwrapStringifiedJSON re-parses a json.RawMessage when its contents are
// a JSON-encoded string of more JSON (the v3 cache shape).
func unwrapStringifiedJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return json.RawMessage(s)
		}
	}
	return raw
}

// DocumentByID returns the document with the given id, or nil.
func (c *Cache) DocumentByID(id string) *Document {
	if c == nil {
		return nil
	}
	d, ok := c.Documents[id]
	if !ok {
		return nil
	}
	return &d
}

// TranscriptByID returns the cached transcript segments for the given doc id.
func (c *Cache) TranscriptByID(id string) []TranscriptSegment {
	if c == nil {
		return nil
	}
	return c.Transcripts[id]
}

// MeetingMetadataByID returns the metadata for the given doc id, or nil.
func (c *Cache) MeetingMetadataByID(id string) *MeetingMetadata {
	if c == nil {
		return nil
	}
	m, ok := c.MeetingsMetadata[id]
	if !ok {
		return nil
	}
	return &m
}

// FolderByName returns the documentListMetadata whose title equals name
// (case-insensitive) or whose id equals name, or nil.
func (c *Cache) FolderByName(name string) *DocumentListMetadata {
	if c == nil {
		return nil
	}
	lname := strings.ToLower(name)
	for _, dl := range c.DocumentListsMetadata {
		if dl.ID == name || strings.EqualFold(dl.Title, lname) || strings.ToLower(dl.Title) == lname {
			res := dl
			return &res
		}
	}
	return nil
}

// FolderMeetings returns the meeting IDs that belong to the named folder,
// resolving via DocumentLists (the direct membership map). Auto-rule
// membership is not re-evaluated here; the cache's DocumentLists block
// reflects Granola's already-applied rules.
func (c *Cache) FolderMeetings(folderID string) []string {
	if c == nil {
		return nil
	}
	out := append([]string(nil), c.DocumentLists[folderID]...)
	return out
}

// SortedDocumentIDs returns doc IDs sorted by descending created_at.
func (c *Cache) SortedDocumentIDs() []string {
	if c == nil {
		return nil
	}
	ids := make([]string, 0, len(c.Documents))
	for id := range c.Documents {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return c.Documents[ids[i]].CreatedAt > c.Documents[ids[j]].CreatedAt
	})
	return ids
}

// RecipesAll returns the merged list of recipes (public + user + shared).
func (c *Cache) RecipesAll() []Recipe {
	if c == nil {
		return nil
	}
	out := make([]Recipe, 0, len(c.PublicRecipes)+len(c.UserRecipes)+len(c.SharedRecipes))
	out = append(out, c.PublicRecipes...)
	out = append(out, c.UserRecipes...)
	out = append(out, c.SharedRecipes...)
	return out
}

// RawState exposes the raw per-key state JSON for debugging / extras.
func (c *Cache) RawState() map[string]json.RawMessage {
	return c.rawState
}

// slugify produces a lowercase, dash-separated slug from a title. Used
// only when a panelTemplate has no slug field — most ship with one.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '_' || r == '-' || r == '/':
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// ParseISO is a small helper for parsing Granola's ISO-8601 timestamps.
func ParseISO(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", s)
}
