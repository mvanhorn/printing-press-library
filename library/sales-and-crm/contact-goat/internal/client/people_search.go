// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package client

// People-search client for the Happenstance web-app graph. Wraps the
// POST /api/search + GET /api/dynamo?requestId=... flow the web UI uses
// to answer "who in my network knows about X". Every field, path, and
// shape here is derived from a live sniff; see
// library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SearchPeopleOptions controls the three network tiers Happenstance
// exposes on a people-search: 1st-degree (your synced connections),
// 2nd-degree (your friends' networks), and 3rd-degree / public
// (searchEveryone).
//
// Defaults match the web UI: 1st + 2nd degree, no public fallback.
type SearchPeopleOptions struct {
	// IncludeMyConnections toggles the "your-connections" tier. 1st-degree.
	// Zero value (false) is used only when the caller explicitly opts out;
	// SearchPeopleByQuery treats a nil *SearchPeopleOptions as "defaults".
	IncludeMyConnections bool

	// IncludeMyFriends toggles "your-friends" — 2nd-degree via your
	// Happenstance-friend graph.
	IncludeMyFriends bool

	// SearchEveryone flips the search to the public / 3rd-degree surface.
	// Expensive; off by default.
	SearchEveryone bool

	// ParentRequestID refines a prior search (equivalent of "find more"
	// in the web UI). Empty string creates a fresh search.
	ParentRequestID string

	// ExcludePersonUUIDs is the list of person_uuid values to omit from
	// results. Used by "find more" after the caller has already seen
	// some results.
	ExcludePersonUUIDs []string

	// PollTimeout bounds the total wall-clock the client will spend
	// polling /api/dynamo before giving up. Default: 60 seconds.
	PollTimeout time.Duration

	// PollInterval is the delay between dynamo polls. Default: 1 second.
	// The web UI polls about every 1-2 seconds.
	PollInterval time.Duration
}

// DefaultPollTimeout is the Happenstance graph-search poll ceiling
// exported so CLI flag help text can reference the exact default.
// Bumped from 60s to 180s on 2026-04-19 after real-session evidence
// that Happenstance routinely takes 2-5 minutes. 60s was causing
// frequent false failures on legitimate queries.
const DefaultPollTimeout = 180 * time.Second

// defaultSearchOptions returns the zero-config behavior: 1st + 2nd
// degree, no public, 180-second timeout, 1-second poll.
func defaultSearchOptions() SearchPeopleOptions {
	return SearchPeopleOptions{
		IncludeMyConnections: true,
		IncludeMyFriends:     true,
		SearchEveryone:       false,
		PollTimeout:          DefaultPollTimeout,
		PollInterval:         1 * time.Second,
	}
}

// PeopleSearchResult is the normalized output of a people-search.
type PeopleSearchResult struct {
	// RequestID is the uuid Happenstance assigned to the search. Useful
	// for refining ("find more") and for surfacing in logs.
	RequestID string `json:"request_id"`
	// Query is the natural-language query that was submitted.
	Query string `json:"query"`
	// Status is Happenstance's human-readable status line, e.g.
	// "Found 19 people".
	Status string `json:"status"`
	// Completed is true when the search finished; false if the poll
	// timeout fired before the server finished.
	Completed bool `json:"completed"`
	// Logs is the structured log stream Happenstance emits while
	// running the search (INFO, FILTER, EMBEDDING_SEARCH, ...).
	Logs []SearchLog `json:"logs"`
	// People is the result list, most-relevant first per Happenstance's
	// scoring.
	People []Person `json:"people"`
}

// SearchLog mirrors one entry in the dynamo response's `logs` array.
// `Message` is intentionally json.RawMessage because Happenstance sends
// strings, arrays, and objects there depending on the log type.
type SearchLog struct {
	Type      string          `json:"type"`
	Title     string          `json:"title,omitempty"`
	Message   json.RawMessage `json:"message"`
	Timestamp string          `json:"timestamp"`
}

// Person is one result from a people-search. Field names mirror the
// dynamo response verbatim so the struct can be decoded directly.
type Person struct {
	Name           string     `json:"author_name"`
	PersonUUID     string     `json:"person_uuid"`
	Score          float64    `json:"score"`
	LinkedInURL    string     `json:"linkedin_url"`
	TwitterURL     string     `json:"twitter_url"`
	InstagramURL   string     `json:"instagram_url"`
	Quotes         string     `json:"quotes"`
	QuotesCited    []Citation `json:"quotes_cited"`
	CurrentTitle   string     `json:"current_title"`
	CurrentCompany string     `json:"current_company"`
	Summary        string     `json:"summary"`
	Referrers      Referrers  `json:"referrers"`
}

// Citation is one (text, url) pair under quotes_cited.
type Citation struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Referrers wraps the chain that tells callers how a Person is
// connected to the current user. The inner slice is ordered: the first
// entry is the closest hop.
type Referrers struct {
	IsYC      bool       `json:"is_yc"`
	Referrers []Referrer `json:"referrers"`
}

// Referrer is one node in the connection chain. When Referrer.ID
// matches the current user's uuid, the Person is 1st-degree. Otherwise
// the Person is 2nd-degree via that referrer (or 3rd-degree when
// SearchEveryone is true and no referrer is the current user).
type Referrer struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Source          []string `json:"source"`
	ImageURL        string   `json:"image_url"`
	AffinityScore   float64  `json:"affinity_score"`
	AffinityLevel   string   `json:"affinity_level"`
	IsDirectoryUser bool     `json:"is_directory_user"`
}

// RelationshipTier names the degree of connection between the current
// user and a Person in the result set.
type RelationshipTier string

const (
	TierFirstDegree  RelationshipTier = "1st_degree"
	TierSecondDegree RelationshipTier = "2nd_degree"
	TierThirdDegree  RelationshipTier = "3rd_degree"
	TierUnknown      RelationshipTier = "unknown"
)

// Tier returns which degree this Person sits at relative to
// currentUserUUID. Callers get this from /api/user and pass it in; the
// Person struct itself doesn't carry it because the server treats tier
// as derivable from referrers + the caller's identity.
func (p Person) Tier(currentUserUUID string) RelationshipTier {
	if currentUserUUID == "" {
		return TierUnknown
	}
	if len(p.Referrers.Referrers) == 0 {
		// No referrer chain usually means searchEveryone=true matched.
		return TierThirdDegree
	}
	for _, r := range p.Referrers.Referrers {
		if r.ID == currentUserUUID {
			return TierFirstDegree
		}
	}
	return TierSecondDegree
}

// createSearchRequest is the POST /api/search body. Field names are
// camelCase; the server rejects snake_case silently with 500.
type createSearchRequest struct {
	RequestText        string        `json:"requestText"`
	RequestContent     []slateBlock  `json:"requestContent"`
	RequestGroups      []string      `json:"requestGroups"`
	ParentRequestID    *string       `json:"parentRequestId"`
	ExcludePersonUUIDs []string      `json:"excludePersonUUIDs"`
	SearchEveryone     bool          `json:"searchEveryone"`
	CreditID           *string       `json:"creditId"`
}

type slateBlock struct {
	Type     string         `json:"type"`
	Children []slateInline  `json:"children"`
}

type slateInline struct {
	Text string `json:"text"`
}

type createSearchResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

// dynamoEntry is the wrapper for one polled search record. The
// /api/dynamo?requestId= endpoint returns an array with exactly one
// element in practice.
type dynamoEntry struct {
	RequestID    string          `json:"request_id"`
	RequestText  string          `json:"request_text"`
	RequestStatus string         `json:"request_status"`
	Completed    bool            `json:"completed"`
	Logs         []SearchLog     `json:"logs"`
	Results      []Person        `json:"results"`
}

// SearchPeopleByQuery runs a Happenstance natural-language people-search
// with configurable tier filtering and blocks until results are ready
// or PollTimeout fires. When opts is nil, default tiers apply
// (1st + 2nd degree, no public).
func (c *Client) SearchPeopleByQuery(query string, opts *SearchPeopleOptions) (*PeopleSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("happenstance people-search: empty query")
	}
	if c.cookieAuth == nil {
		return nil, errors.New("happenstance people-search: cookie auth not configured (run `contact-goat-pp-cli auth login`)")
	}

	o := defaultSearchOptions()
	if opts != nil {
		// Respect explicit tier choices. When the caller passes
		// opts with all three tiers false, we raise — that is always a
		// bug because Happenstance needs at least one tier to search.
		o = *opts
		if o.PollTimeout == 0 {
			o.PollTimeout = DefaultPollTimeout
		}
		if o.PollInterval == 0 {
			o.PollInterval = 1 * time.Second
		}
	}
	if !o.IncludeMyConnections && !o.IncludeMyFriends && !o.SearchEveryone {
		return nil, errors.New("happenstance people-search: at least one tier must be enabled (include_my_connections, include_my_friends, or search_everyone)")
	}

	// Refresh the session proactively; a sub-60-second search that
	// spans a JWT expiry would otherwise fail mid-poll.
	if err := c.MaybeRefreshSession(); err != nil {
		return nil, fmt.Errorf("happenstance people-search: refresh session: %w", err)
	}

	reqID, err := c.createSearch(query, o)
	if err != nil {
		return nil, err
	}

	final, err := c.pollSearch(reqID, o.PollTimeout, o.PollInterval)
	if err != nil {
		return nil, err
	}

	return &PeopleSearchResult{
		RequestID: final.RequestID,
		Query:     final.RequestText,
		Status:    final.RequestStatus,
		Completed: final.Completed,
		Logs:      final.Logs,
		People:    final.Results,
	}, nil
}

// SearchPeopleByCompany is a convenience wrapper that phrases the query
// as "people at <company>" with default tier settings (1st + 2nd
// degree). It is the direct replacement for contact-goat's coverage
// command's narrow friends/list-only source.
func (c *Client) SearchPeopleByCompany(company string) (*PeopleSearchResult, error) {
	return c.SearchPeopleByCompanyWithOptions(company, nil)
}

// SearchPeopleByCompanyWithOptions lets callers override tier and
// poll-timeout settings per-call. Passing nil is identical to
// SearchPeopleByCompany. A zero PollTimeout on opts falls back to
// DefaultPollTimeout.
func (c *Client) SearchPeopleByCompanyWithOptions(company string, opts *SearchPeopleOptions) (*PeopleSearchResult, error) {
	return c.SearchPeopleByQuery(fmt.Sprintf("people at %s", company), opts)
}

// createSearch posts the request body and returns the request uuid the
// server assigns. It does not wait for results — see pollSearch.
func (c *Client) createSearch(query string, o SearchPeopleOptions) (string, error) {
	groups := make([]string, 0, 2)
	if o.IncludeMyConnections {
		groups = append(groups, "your-connections")
	}
	if o.IncludeMyFriends {
		groups = append(groups, "your-friends")
	}

	var parent *string
	if o.ParentRequestID != "" {
		parent = &o.ParentRequestID
	}
	excludes := o.ExcludePersonUUIDs
	if excludes == nil {
		excludes = []string{}
	}

	body := createSearchRequest{
		RequestText: query,
		RequestContent: []slateBlock{
			{Type: "p", Children: []slateInline{{Text: query}}},
		},
		RequestGroups:      groups,
		ParentRequestID:    parent,
		ExcludePersonUUIDs: excludes,
		SearchEveryone:     o.SearchEveryone,
		CreditID:           nil,
	}

	raw, status, err := c.Post("/api/search", body)
	if err != nil {
		return "", fmt.Errorf("happenstance POST /api/search: %w", err)
	}
	if status >= 400 {
		return "", fmt.Errorf("happenstance POST /api/search: HTTP %d (body: %s)", status, truncateForError(string(raw), 300))
	}
	if status == 204 || len(raw) == 0 {
		// 204 is the Clerk-signed-out path: the request landed on a
		// signed-out session. Force a refresh and retry once.
		if refErr := c.refreshClerkSession(); refErr != nil {
			return "", fmt.Errorf("happenstance POST /api/search: HTTP %d empty body (likely signed-out). Refresh also failed: %w", status, refErr)
		}
		raw, status, err = c.Post("/api/search", body)
		if err != nil {
			return "", fmt.Errorf("happenstance POST /api/search (retry): %w", err)
		}
		if status >= 400 {
			return "", fmt.Errorf("happenstance POST /api/search (retry): HTTP %d (body: %s)", status, truncateForError(string(raw), 300))
		}
		if status == 204 || len(raw) == 0 {
			return "", fmt.Errorf("happenstance POST /api/search: HTTP %d empty body after refresh (session may be revoked - re-run `auth login --chrome`)", status)
		}
	}

	var resp createSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("happenstance POST /api/search: decode response: %w (status=%d body: %s)", err, status, truncateForError(string(raw), 200))
	}
	if resp.ID == "" {
		return "", fmt.Errorf("happenstance POST /api/search: server returned empty id (body: %s)", truncateForError(string(raw), 200))
	}
	return resp.ID, nil
}

// pollSearch polls /api/dynamo?requestId= until the search completes or
// timeout fires. Returns the final dynamo entry even if the timeout
// fired, so callers can see partial progress (with completed=false).
func (c *Client) pollSearch(requestID string, timeout, interval time.Duration) (*dynamoEntry, error) {
	deadline := time.Now().Add(timeout)

	var last dynamoEntry
	for {
		entries, err := c.fetchDynamo(requestID)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			return nil, fmt.Errorf("happenstance poll: empty dynamo response for requestId %s", requestID)
		}
		last = entries[0]
		if last.Completed {
			return &last, nil
		}

		if time.Now().After(deadline) {
			// Return partial result rather than the bare timeout error:
			// callers may want to render what the server streamed so far.
			return &last, fmt.Errorf("happenstance poll: timeout after %s waiting for requestId %s (last status: %q)",
				timeout, requestID, last.RequestStatus)
		}
		time.Sleep(interval)
	}
}

func (c *Client) fetchDynamo(requestID string) ([]dynamoEntry, error) {
	raw, err := c.Get("/api/dynamo", map[string]string{"requestId": requestID})
	if err != nil {
		return nil, fmt.Errorf("happenstance GET /api/dynamo: %w", err)
	}
	var entries []dynamoEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("happenstance GET /api/dynamo: decode: %w (body: %s)", err, truncateForError(string(raw), 300))
	}
	return entries, nil
}

func truncateForError(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}
