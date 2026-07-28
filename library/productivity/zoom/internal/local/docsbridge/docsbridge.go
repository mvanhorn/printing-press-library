// Package docsbridge reaches the Zoom Docs surface that backs the "My Notes"
// feature. Zoom publishes no REST endpoint for My Notes, and the documented
// /docs/archives API that does carry note transcripts requires
// docs:read:archive:admin plus account-level Docs archiving. This package uses
// the same web-session calls the docs.zoom.us SPA makes, so an ordinary user
// reaches their own notes without an account admin installing anything.
//
// Auth is two-legged: browser cookies mint a short-lived ES256 bearer (Zoom
// calls it a "nak") which every bridge call then carries.
package docsbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
)

const (
	// CookieDomain is what press-auth stores captured zoom.us cookies under.
	CookieDomain = "zoom.us"
	// nakURL mints the bearer. The pms list is the permission set the SPA asks
	// for; AICW covers AI Companion surfaces, which is where notes live.
	nakURL = "https://docs.zoom.us/nws/common/2.0/nak?pms=AICW%2CUser%3ABase&sfr=5"
	// defaultAPIBase is only the starting host. Every file row carries its own
	// fileClusterApiPrefix, and the client adopts it for subsequent calls.
	defaultAPIBase = "https://us01docs.zoom.us"
	// notesFolderTitle is the system folder the SPA shows as the My Notes tab.
	notesFolderTitle = "my notes"
	rootFolderID     = "my-docs"
	// nakSkew retires a bearer early so a call can't start on a token that
	// expires mid-flight.
	nakSkew = 2 * time.Minute
)

// Client talks to the Zoom Docs bridge with a cookie-derived bearer.
type Client struct {
	HTTP    *http.Client
	APIBase string
	limiter *cliutil.AdaptiveLimiter

	cookie       string
	CookieSource string

	nak       string
	nakExpiry time.Time
	accountID string
	clusterID string
}

// New builds a client from whatever cookie source is available.
func New(timeout time.Duration) (*Client, error) {
	cookie, source, err := CookieHeader()
	if err != nil {
		return nil, err
	}
	base := defaultAPIBase
	if v := strings.TrimSpace(os.Getenv("ZOOM_DOCS_API_BASE")); v != "" {
		base = strings.TrimRight(v, "/")
	}
	return &Client{
		HTTP:         &http.Client{Timeout: timeout},
		APIBase:      base,
		limiter:      cliutil.NewAdaptiveLimiterAuto(2.0),
		cookie:       cookie,
		CookieSource: source,
		clusterID:    "aw1",
	}, nil
}

// CookieHeader resolves a zoom.us cookie header, preferring press-auth. It
// returns the header value and a short label naming where it came from, so
// commands can tell the user which source answered.
func CookieHeader() (string, string, error) {
	if env := strings.TrimSpace(os.Getenv("ZOOM_WEB_COOKIE")); env != "" {
		return env, "env:ZOOM_WEB_COOKIE", nil
	}
	if bin, err := exec.LookPath("press-auth"); err == nil {
		out, err := exec.Command(bin, "cookies", CookieDomain).Output() // #nosec G204 -- bin resolved via LookPath("press-auth"); args are package constants
		if cookie := strings.TrimSpace(string(out)); err == nil && cookie != "" {
			return cookie, "press-auth", nil
		}
	}
	path := CookieFilePath()
	if raw, err := os.ReadFile(path); err == nil { // #nosec G304 -- fixed cookie-file path under the user's own config dir
		if cookie := normalizeCookieFile(string(raw)); cookie != "" {
			return cookie, "file:" + path, nil
		}
	}
	return "", "", &ErrNoSession{CookieFile: path}
}

// ErrNoSession reports that no Zoom web session is reachable by any of the
// three supported routes. Typed so callers can tell "you never logged in"
// apart from "the session exists but the request failed", which read commands
// treat very differently.
type ErrNoSession struct{ CookieFile string }

func (e *ErrNoSession) Error() string {
	return fmt.Sprintf("no Zoom web session found. Capture one with `press-auth login %s --login-url https://zoom.us/signin`, "+
		"or write a Cookie header to %s, or export ZOOM_WEB_COOKIE", CookieDomain, e.CookieFile)
}

// CookieFilePath is the fallback cookie location for users without press-auth.
func CookieFilePath() string {
	if v := strings.TrimSpace(os.Getenv("ZOOM_WEB_COOKIE_FILE")); v != "" {
		return v
	}
	// Sits beside config.toml and token.json rather than under
	// os.UserConfigDir, so every zoom-pp-cli credential lives in one directory.
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zoom-pp-cli", "web-cookies.txt")
}

// normalizeCookieFile accepts either a single Cookie header line or one
// name=value pair per line, which is what users hand-assemble.
func normalizeCookieFile(raw string) string {
	var pairs []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "Cookie:")
		pairs = append(pairs, strings.TrimSpace(line))
	}
	return strings.Join(pairs, "; ")
}

// AccountID is the account the minted bearer belongs to. Empty until the first
// call mints a token.
func (c *Client) AccountID() string { return c.accountID }

// ensureNak mints a bearer when the cached one is missing or near expiry. The
// account id and cluster ride along in the token's claims, so minting also
// supplies the request-body and header values later calls need.
func (c *Client) ensureNak(ctx context.Context) error {
	if c.nak != "" && time.Until(c.nakExpiry) > nakSkew {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nakURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Referer", "https://docs.zoom.us/")
	c.limiter.Wait()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("minting Zoom docs bearer: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	token := strings.TrimSpace(string(body))
	if resp.StatusCode >= 400 || token == "" || strings.Count(token, ".") != 2 {
		return fmt.Errorf("minting Zoom docs bearer: HTTP %d: the Zoom web session is missing or expired "+
			"(re-capture with `press-auth login %s`)", resp.StatusCode, CookieDomain)
	}
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return err
	}
	c.nak = token
	c.nakExpiry = claims.expiry
	if claims.accountID != "" {
		c.accountID = claims.accountID
	}
	return nil
}

type nakClaims struct {
	expiry    time.Time
	accountID string
}

func decodeJWTClaims(token string) (nakClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nakClaims{}, errors.New("Zoom docs bearer is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nakClaims{}, fmt.Errorf("decoding Zoom docs bearer: %w", err)
	}
	var body struct {
		Exp int64  `json:"exp"`
		AID string `json:"aid"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return nakClaims{}, fmt.Errorf("decoding Zoom docs bearer: %w", err)
	}
	if body.Exp == 0 {
		return nakClaims{}, errors.New("Zoom docs bearer carries no exp claim")
	}
	return nakClaims{expiry: time.Unix(body.Exp, 0), accountID: body.AID}, nil
}

// do issues one bridge call, minting or refreshing the bearer first. A nil body
// sends GET; a non-nil body sends POST with JSON.
func (c *Client) do(ctx context.Context, path string, body any, out any) error {
	if err := c.ensureNak(ctx); err != nil {
		return err
	}
	method := http.MethodGet
	var reader io.Reader
	if body != nil {
		method = http.MethodPost
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	endpoint := path
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = strings.TrimRight(c.APIBase, "/") + path
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Authorization", "Bearer "+c.nak)
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Origin", "https://docs.zoom.us")
	req.Header.Set("Referer", "https://docs.zoom.us/")
	req.Header.Set("X-Zm-Cluster-Id", c.clusterID)
	req.Header.Set("X-Zm-Docs-Container", "docs/browser")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.limiter.Wait()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("zoom docs bridge %s: %w", path, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("zoom docs bridge %s: %w", path, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
		hint := "retry shortly"
		if retryAfter != "" {
			hint = "retry after " + retryAfter + "s"
		}
		return fmt.Errorf("zoom docs bridge %s: HTTP 429 rate limited (%s): %s", path, hint, truncate(strings.TrimSpace(string(payload)), 200))
	}
	c.limiter.OnSuccess()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("zoom docs bridge %s: HTTP %d: %s", path, resp.StatusCode, truncate(strings.TrimSpace(string(payload)), 200))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decoding zoom docs bridge %s: %w", path, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

// Note is one My Notes document plus the meeting it was taken in.
type Note struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	MeetingID     string    `json:"meeting_id,omitempty"`
	MainMeetingID string    `json:"main_meeting_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	Link          string    `json:"link,omitempty"`
	HasRecording  bool      `json:"has_recording"`
}

type fileRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	FileType    string `json:"fileType"`
	IsDeleted   bool   `json:"isDeleted"`
	FileLink    string `json:"fileLink"`
	APIPrefix   string `json:"fileClusterApiPrefix"`
	ClusterID   string `json:"clusterId"`
	CreatedInfo struct {
		Time time.Time `json:"time"`
	} `json:"createdInfo"`
	UpdatedInfo struct {
		Time time.Time `json:"time"`
	} `json:"updatedInfo"`
	MeetingNotes struct {
		MeetingID     string `json:"meetingId"`
		MainMeetingID string `json:"mainMeetingId"`
		HasRecording  bool   `json:"hasRecording"`
	} `json:"meetingNotes"`
}

type childrenResponse struct {
	SuccessItems []struct {
		ParentID string    `json:"parentId"`
		Children []fileRow `json:"children"`
	} `json:"successItems"`
	FailureItems []struct {
		FileID        string `json:"fileId"`
		FailureReason string `json:"failureReason"`
	} `json:"failureItems"`
}

func (c *Client) children(ctx context.Context, parentID string) ([]fileRow, error) {
	if err := c.ensureNak(ctx); err != nil {
		return nil, err
	}
	var resp childrenResponse
	body := map[string]any{"parentIds": []string{parentID}, "accountId": c.accountID}
	if err := c.do(ctx, "/api/file/files/action/batch_get_children", body, &resp); err != nil {
		return nil, err
	}
	for _, item := range resp.SuccessItems {
		if item.ParentID == parentID {
			return item.Children, nil
		}
	}
	for _, item := range resp.FailureItems {
		if item.FileID == parentID {
			return nil, fmt.Errorf("zoom docs folder %s: %s", parentID, item.FailureReason)
		}
	}
	return nil, nil
}

// NotesFolderID finds the My Notes system folder under the user's docs root.
// The id is per-account, so it is resolved at runtime rather than baked in.
func (c *Client) NotesFolderID(ctx context.Context) (string, error) {
	rows, err := c.children(ctx, rootFolderID)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.FileType == "folder" && strings.EqualFold(strings.TrimSpace(row.Title), notesFolderTitle) {
			if row.APIPrefix != "" {
				c.APIBase = strings.TrimRight(row.APIPrefix, "/")
			}
			if row.ClusterID != "" {
				c.clusterID = row.ClusterID
			}
			return row.ID, nil
		}
	}
	return "", errors.New("no \"My notes\" folder in this Zoom account's docs root")
}

// Notes lists My Notes documents, newest first. A zero since or limit means no
// filter.
func (c *Client) Notes(ctx context.Context, since time.Time, limit int) ([]Note, error) {
	folderID, err := c.NotesFolderID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := c.children(ctx, folderID)
	if err != nil {
		return nil, err
	}
	notes := make([]Note, 0, len(rows))
	for _, row := range rows {
		if row.IsDeleted || row.FileType == "folder" {
			continue
		}
		updated := row.UpdatedInfo.Time
		if !since.IsZero() && updated.Before(since) {
			continue
		}
		notes = append(notes, Note{
			ID:            row.ID,
			Title:         row.Title,
			MeetingID:     row.MeetingNotes.MeetingID,
			MainMeetingID: row.MeetingNotes.MainMeetingID,
			UpdatedAt:     updated,
			CreatedAt:     row.CreatedInfo.Time,
			Link:          row.FileLink,
			HasRecording:  row.MeetingNotes.HasRecording,
		})
	}
	sortNotesNewestFirst(notes)
	if limit > 0 && len(notes) > limit {
		notes = notes[:limit]
	}
	return notes, nil
}

func sortNotesNewestFirst(notes []Note) {
	for i := 1; i < len(notes); i++ {
		for j := i; j > 0 && notes[j].UpdatedAt.After(notes[j-1].UpdatedAt); j-- {
			notes[j], notes[j-1] = notes[j-1], notes[j]
		}
	}
}

// TranscriptItem is one utterance.
type TranscriptItem struct {
	Text          string `json:"text"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	UserID        string `json:"userId"`
	HighlightFlag string `json:"highlightFlag"`
}

// TranscriptSpeaker maps a numeric userId to a display name.
type TranscriptSpeaker struct {
	UserID      string `json:"userId"`
	ZoomUserID  string `json:"zoomUserId"`
	Username    string `json:"username"`
	SpeakerName string `json:"speakerName"`
	IsUnknown   bool   `json:"isUnknown"`
}

// Transcript is the AI Companion transcript attached to a meeting's note.
type Transcript struct {
	MeetingID        string              `json:"meetingId"`
	MeetingStartTime string              `json:"meetingStartTime"`
	MeetingEndTime   string              `json:"meetingEndTime"`
	Items            []TranscriptItem    `json:"items"`
	Speakers         []TranscriptSpeaker `json:"speakers"`
	Uncompleted      bool                `json:"uncompleted"`
}

// SpeakerName resolves an item's userId to the best available display name.
func (t *Transcript) SpeakerName(userID string) string {
	for _, s := range t.Speakers {
		if s.UserID != userID {
			continue
		}
		if s.SpeakerName != "" {
			return s.SpeakerName
		}
		if s.Username != "" {
			return s.Username
		}
		break
	}
	return "Unknown speaker"
}

// StartedAt parses the epoch-millis start time the bridge returns as a string.
func (t *Transcript) StartedAt() time.Time {
	ms, err := strconv.ParseInt(t.MeetingStartTime, 10, 64)
	if err != nil || ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// Transcript fetches a meeting's transcript. The fileId the SPA also sends is
// optional, so a meeting id alone is enough.
func (c *Client) Transcript(ctx context.Context, meetingID string) (*Transcript, error) {
	if strings.TrimSpace(meetingID) == "" {
		return nil, errors.New("a meeting id is required")
	}
	path := "/api/bridge/meeting/transcripts/v2?meetingId=" + url.QueryEscape(meetingID)
	var out Transcript
	if err := c.do(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
