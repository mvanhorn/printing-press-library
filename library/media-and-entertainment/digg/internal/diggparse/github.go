// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rename-and-github-feeds): library-side new file. Parsers
// for the four /ai/github/* feeds. Reuses DecodeRSC + scanObjectsContaining
// from parser.go.
//
// Parsers for the four GitHub feeds on digg.com:
//
//   /ai/github/stars    — top AI repos ranked by starring activity from
//                         Digg-tracked accounts (rich JSON, two-level
//                         envelope: {repo, starrers, judgment})
//   /ai/github/new      — recently first-seen repos grouped by who first
//                         starred them (event_id + repos[])
//   /ai/github/activity — contributor leaderboard (rank / contributions /
//                         repos count, rendered in <td> cells around a
//                         user JSON object)
//   /ai/github/recent   — live activity stream (github.com URLs in href
//                         attributes, user JSON sidecars per event)
//
// All four pages embed their data as the same self.__next_f.push RSC
// stream the home /ai page uses; we reuse DecodeRSC + scanObjectsContaining
// from parser.go.

package diggparse

import (
	"encoding/json"
	"regexp"
	"strings"
)

// GithubRepoMeta is the upstream repo metadata Digg keeps alongside its
// judgment row. Snake_case mirrors the wire payload. Starrers nests
// inside the repo envelope upstream — keep the same shape.
type GithubRepoMeta struct {
	FullName         string          `json:"full_name,omitempty"`
	StargazersCount  int             `json:"stargazers_count,omitempty"`
	Language         string          `json:"language,omitempty"`
	Description      string          `json:"description,omitempty"`
	TopStarrerLogin  string          `json:"top_starrer_login,omitempty"`
	DistinctStarrers int             `json:"distinct_starrers,omitempty"`
	MostRecentStarAt string          `json:"most_recent_star_at,omitempty"`
	AI1000Stars      int             `json:"ai1000_stars,omitempty"`
	Starrers         []GithubStarrer `json:"starrers,omitempty"`
}

// GithubRepoJudgment is the AI-graded record on each starred repo.
type GithubRepoJudgment struct {
	RepoFullName       string  `json:"repo_full_name,omitempty"`
	BreakoutScore      float64 `json:"breakout_score,omitempty"`
	NovelScore         float64 `json:"novel_score,omitempty"`
	AIRelatedScore     float64 `json:"ai_related_score,omitempty"`
	Description        string  `json:"description,omitempty"`
	OneSentence        string  `json:"one_sentence,omitempty"`
	ClassificationTLDR string  `json:"classification_tldr,omitempty"`
	JudgedAt           string  `json:"judged_at,omitempty"`
	ModelVersion       string  `json:"model_version,omitempty"`
	Topic              string  `json:"topic,omitempty"`
}

// GithubStarrer is one Digg-tracked account that starred a repo.
type GithubStarrer struct {
	XID                string  `json:"x_id,omitempty"`
	Username           string  `json:"username,omitempty"`
	DisplayName        string  `json:"display_name,omitempty"`
	AvatarURL          string  `json:"avatar_url,omitempty"`
	ClassificationTLDR string  `json:"classification_tldr,omitempty"`
	OneSentence        string  `json:"one_sentence,omitempty"`
	Rank               int     `json:"rank,omitempty"`
	Influence          float64 `json:"influence,omitempty"`
}

// GithubRepoEntry is the full row from /ai/github/stars and the inner
// repos[] entries of /ai/github/new.
type GithubRepoEntry struct {
	Repo     GithubRepoMeta     `json:"repo"`
	Judgment GithubRepoJudgment `json:"judgment"`
	RawJSON  json.RawMessage    `json:"-"`
}

// GithubNewCreator describes the Digg-tracked account that first starred
// a freshly-seen repo. github_username is the creator's GitHub login (may
// differ from their X username).
type GithubNewCreator struct {
	XID                string `json:"x_id,omitempty"`
	Username           string `json:"username,omitempty"`
	DisplayName        string `json:"display_name,omitempty"`
	AvatarURL          string `json:"avatar_url,omitempty"`
	Rank               int    `json:"rank,omitempty"`
	OneSentence        string `json:"one_sentence,omitempty"`
	ClassificationTLDR string `json:"classification_tldr,omitempty"`
	GithubUsername     string `json:"github_username,omitempty"`
}

// GithubStarEvent is one entry from /ai/github/new — flat record carrying
// the freshly-seen repo plus the Digg-tracked creator who first starred
// it. event_id is upstream's per-event key (often the repo full name).
type GithubStarEvent struct {
	EventID         string           `json:"event_id,omitempty"`
	EventCreatedAt  string           `json:"event_created_at,omitempty"`
	RepoFullName    string           `json:"repo_full_name,omitempty"`
	Description     string           `json:"description,omitempty"`
	Language        string           `json:"language,omitempty"`
	StargazersCount int              `json:"stargazers_count,omitempty"`
	Creator         GithubNewCreator `json:"creator,omitempty"`
	RawJSON         json.RawMessage  `json:"-"`
}

// GithubContributor is one row on the /ai/github/activity leaderboard.
type GithubContributor struct {
	Username           string `json:"username,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	AvatarURL          string `json:"avatarUrl,omitempty"`
	Rank               int    `json:"rank,omitempty"`
	ClassificationTLDR string `json:"classificationTldr,omitempty"`
	Contributions      int    `json:"contributions,omitempty"`
	ReposCount         int    `json:"repos_count,omitempty"`
}

// GithubActivity is one event from /ai/github/recent.
type GithubActivity struct {
	Username           string   `json:"username,omitempty"`
	DisplayName        string   `json:"displayName,omitempty"`
	AvatarURL          string   `json:"avatarUrl,omitempty"`
	Rank               int      `json:"rank,omitempty"`
	ClassificationTLDR string   `json:"classificationTldr,omitempty"`
	GithubURLs         []string `json:"github_urls,omitempty"`
}

// ParseGithubStars extracts repo entries from /ai/github/stars HTML.
//
// Each row has shape `{"repo":{...},"starrers":[...],"judgment":{...}}`.
// We anchor on `"judgment":{"repo_full_name"` because that string only
// appears in the outer envelope's `judgment` key, and at the position
// of the leading `"` the smallest enclosing object is the outer envelope.
func ParseGithubStars(html []byte) ([]GithubRepoEntry, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, nil
	}
	const needle = `"judgment":{"repo_full_name":`
	objs := scanObjectsContaining(decoded, needle)
	out := make([]GithubRepoEntry, 0, len(objs))
	seen := make(map[string]bool)
	for _, raw := range objs {
		var e GithubRepoEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		key := e.Judgment.RepoFullName
		if key == "" {
			key = e.Repo.FullName
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		e.RawJSON = append(e.RawJSON[:0:0], raw...)
		out = append(out, e)
	}
	return out, nil
}

// ParseGithubNew extracts star-event records from /ai/github/new HTML.
func ParseGithubNew(html []byte) ([]GithubStarEvent, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, nil
	}
	objs := scanObjectsContaining(decoded, `"event_id":"`)
	out := make([]GithubStarEvent, 0, len(objs))
	seen := make(map[string]bool)
	for _, raw := range objs {
		var e GithubStarEvent
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		if e.EventID == "" || seen[e.EventID] {
			continue
		}
		seen[e.EventID] = true
		e.RawJSON = append(e.RawJSON[:0:0], raw...)
		out = append(out, e)
	}
	return out, nil
}

// tdNumberPattern matches a numeric <td> cell: ...{"className":"...","children":"117"}...
var tdNumberPattern = regexp.MustCompile(`"children":"(\d[\d,]*)"`)

// ParseGithubActivity extracts the contributor leaderboard from
// /ai/github/activity. The page renders each row as <td> cells around a
// "user":{...} JSON object; we extract every user record and look for the
// two numeric cells that precede the table-row anchor for contribution
// and repo counts.
//
// A given username can appear many times in the RSC stream (once per
// avatar component instance that references that user), so a naive
// strings.Index lookup of the first occurrence can land on an avatar
// reference whose preceding td numbers belong to a *different* user's
// row. We iterate every occurrence and pick the first one whose lookback
// ends right next to two numeric td cells (the table-row anchor) —
// avatar-reference occurrences typically don't have td-children patterns
// in immediate proximity.
func ParseGithubActivity(html []byte) ([]GithubContributor, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, nil
	}
	users := scanObjectsContaining(decoded, `"username":"`)
	out := make([]GithubContributor, 0, len(users))
	seen := make(map[string]bool)
	for _, raw := range users {
		var u struct {
			Username           string `json:"username"`
			DisplayName        string `json:"displayName"`
			AvatarURL          string `json:"avatarUrl"`
			Rank               int    `json:"rank"`
			ClassificationTLDR string `json:"classificationTldr"`
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			continue
		}
		if u.Username == "" || u.Rank == 0 {
			continue
		}
		if seen[u.Username] {
			continue
		}
		seen[u.Username] = true
		c := GithubContributor{
			Username:           u.Username,
			DisplayName:        u.DisplayName,
			AvatarURL:          u.AvatarURL,
			Rank:               u.Rank,
			ClassificationTLDR: u.ClassificationTLDR,
		}
		needle := `"username":"` + u.Username + `"`
		pos := 0
		for {
			rel := strings.Index(decoded[pos:], needle)
			if rel < 0 {
				break
			}
			abs := pos + rel
			start := abs - 800
			if start < 0 {
				start = 0
			}
			window := decoded[start:abs]
			nums := tdNumberPattern.FindAllStringSubmatch(window, -1)
			// Take the first occurrence whose lookback contains at least
			// two numeric td cells — that's the table-row anchor. Other
			// occurrences (avatar component references later in the
			// stream) typically sit inside non-numeric children. Data
			// sections lack td-children patterns entirely.
			if len(nums) >= 2 {
				c.Contributions = atoiSafe(nums[len(nums)-2][1])
				c.ReposCount = atoiSafe(nums[len(nums)-1][1])
				break
			}
			pos = abs + len(needle)
		}
		out = append(out, c)
	}
	return out, nil
}

var githubURLPattern = regexp.MustCompile(`https://github\.com/[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-/]+)?`)

// ParseGithubRecent extracts the live activity feed from
// /ai/github/recent. Pairs each user record with the nearest github.com
// URLs found in the same activity card (~1500 bytes before the user).
// Deduplicates by username, keeping the entry with the most populated
// fields and the union of its github URLs.
func ParseGithubRecent(html []byte) ([]GithubActivity, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, nil
	}
	users := scanObjectsContaining(decoded, `"username":"`)
	byUser := make(map[string]*GithubActivity)
	order := make([]string, 0, len(users))
	for _, raw := range users {
		var u struct {
			Username           string `json:"username"`
			DisplayName        string `json:"displayName"`
			AvatarURL          string `json:"avatarUrl"`
			Rank               int    `json:"rank"`
			ClassificationTLDR string `json:"classificationTldr"`
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			continue
		}
		if u.Username == "" {
			continue
		}
		a := byUser[u.Username]
		if a == nil {
			a = &GithubActivity{Username: u.Username}
			byUser[u.Username] = a
			order = append(order, u.Username)
		}
		if u.DisplayName != "" && len(u.DisplayName) > len(a.DisplayName) {
			a.DisplayName = u.DisplayName
		}
		if u.AvatarURL != "" && a.AvatarURL == "" {
			a.AvatarURL = u.AvatarURL
		}
		if u.Rank != 0 && a.Rank == 0 {
			a.Rank = u.Rank
		}
		if len(u.ClassificationTLDR) > len(a.ClassificationTLDR) {
			a.ClassificationTLDR = u.ClassificationTLDR
		}
		// Scan every occurrence of this user — recent feed often repeats
		// the same user in multiple events. Window of 2500 chars catches
		// most cards including the repo-description block before the user.
		needle := `"username":"` + u.Username + `"`
		seenURL := make(map[string]bool, len(a.GithubURLs))
		for _, gu := range a.GithubURLs {
			seenURL[gu] = true
		}
		pos := 0
		for {
			idx := strings.Index(decoded[pos:], needle)
			if idx < 0 {
				break
			}
			abs := pos + idx
			start := abs - 2500
			if start < 0 {
				start = 0
			}
			urls := githubURLPattern.FindAllString(decoded[start:abs], -1)
			for _, gu := range urls {
				gu = strings.TrimRight(gu, "/.,)\"")
				if seenURL[gu] {
					continue
				}
				seenURL[gu] = true
				a.GithubURLs = append(a.GithubURLs, gu)
			}
			pos = abs + len(needle)
		}
	}
	out := make([]GithubActivity, 0, len(order))
	for _, name := range order {
		out = append(out, *byUser[name])
	}
	return out, nil
}

func atoiSafe(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
