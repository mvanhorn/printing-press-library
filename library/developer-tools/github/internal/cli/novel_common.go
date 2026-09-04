// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature support: shared store access, JSON accessors, and
// the pure aggregation logic behind the transcendence commands (dupes, mentions,
// issues context, pulls review-load / stale, repos changelog / who-touched,
// labels coverage). Kept in one file so the logic is unit-testable in isolation
// from Cobra wiring. Not generator-emitted.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/store"

	"github.com/spf13/cobra"
)

// novelDBPath resolves the local SQLite mirror path the novel commands read.
func novelDBPath() string { return defaultDBPath("github-pp-cli") }

// --- JSON accessors (tolerant; missing/null returns zero value) -------------

func nvDecode(data []byte) map[string]any {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return m
}

func nvStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// nvNum returns a JSON number field as int64 (json numbers decode to float64).
func nvNum(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}

// nvNested walks an object path like "author","login".
func nvNested(m map[string]any, keys ...string) any {
	cur := any(m)
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[k]
	}
	return cur
}

func nvNestedStr(m map[string]any, keys ...string) string {
	if s, ok := nvNested(m, keys...).(string); ok {
		return s
	}
	return ""
}

func nvArr(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	if a, ok := m[key].([]any); ok {
		return a
	}
	return nil
}

func nvObj(v any) map[string]any {
	if o, ok := v.(map[string]any); ok {
		return o
	}
	return nil
}

func nvParseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// --- pure aggregation logic (unit-tested in novel_common_test.go) -----------

type issueRow struct {
	Number int64  `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"html_url,omitempty"`
}

func nvIssueRow(data []byte) issueRow {
	m := nvDecode(data)
	return issueRow{
		Number: nvNum(m, "number"),
		Title:  nvStr(m, "title"),
		State:  nvStr(m, "state"),
		URL:    nvStr(m, "html_url"),
	}
}

type mentionRow struct {
	Type   string `json:"type"`
	Number int64  `json:"number,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Title  string `json:"title,omitempty"`
	URL    string `json:"html_url,omitempty"`
}

// nvMentionRow shapes a synced resource (issue/pull/commit) into a tagged hit.
func nvMentionRow(resourceType string, data []byte) mentionRow {
	m := nvDecode(data)
	switch resourceType {
	case "commits":
		return mentionRow{
			Type:  "commit",
			Ref:   nvStr(m, "sha"),
			Title: nvFirstLine(nvNestedStr(m, "commit", "message")),
			URL:   nvStr(m, "html_url"),
		}
	case "pulls":
		return mentionRow{Type: "pull", Number: nvNum(m, "number"), Title: nvStr(m, "title"), URL: nvStr(m, "html_url")}
	default:
		return mentionRow{Type: "issue", Number: nvNum(m, "number"), Title: nvStr(m, "title"), URL: nvStr(m, "html_url")}
	}
}

func nvFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

type reviewerLoad struct {
	Reviewer string `json:"reviewer"`
	OpenPRs  int    `json:"open_prs"`
}

// nvReviewerLoad counts open PRs per requested reviewer login, descending.
func nvReviewerLoad(prs [][]byte, stateFilter string) []reviewerLoad {
	counts := map[string]int{}
	for _, raw := range prs {
		m := nvDecode(raw)
		if stateFilter != "" && stateFilter != "all" && nvStr(m, "state") != stateFilter {
			continue
		}
		for _, rv := range nvArr(m, "requested_reviewers") {
			login := nvStr(nvObj(rv), "login")
			if login != "" {
				counts[login]++
			}
		}
	}
	out := make([]reviewerLoad, 0, len(counts))
	for login, n := range counts {
		out = append(out, reviewerLoad{Reviewer: login, OpenPRs: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OpenPRs != out[j].OpenPRs {
			return out[i].OpenPRs > out[j].OpenPRs
		}
		return out[i].Reviewer < out[j].Reviewer
	})
	return out
}

type stalePR struct {
	Number       int64  `json:"number"`
	Title        string `json:"title"`
	LastActivity string `json:"last_activity"`
	StaleDays    int    `json:"stale_days"`
	URL          string `json:"html_url,omitempty"`
}

// nvStalePRs returns open PRs whose last activity is older than olderThan,
// sorted by staleness (most stale first).
func nvStalePRs(prs [][]byte, olderThan time.Duration, now time.Time) []stalePR {
	var out []stalePR
	for _, raw := range prs {
		m := nvDecode(raw)
		if nvStr(m, "state") != "open" {
			continue
		}
		last := nvStr(m, "updated_at")
		t, ok := nvParseTime(last)
		if !ok {
			continue
		}
		age := now.Sub(t)
		if age < olderThan {
			continue
		}
		out = append(out, stalePR{
			Number:       nvNum(m, "number"),
			Title:        nvStr(m, "title"),
			LastActivity: last,
			StaleDays:    int(age.Hours() / 24),
			URL:          nvStr(m, "html_url"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StaleDays > out[j].StaleDays })
	return out
}

type authorChange struct {
	Author  string   `json:"author"`
	Commits int      `json:"commits"`
	SHAs    []string `json:"shas,omitempty"`
}

// nvGroupCommitsByAuthor groups compare/commits-API commit objects by author
// login (falling back to commit.author.name), descending by count.
func nvGroupCommitsByAuthor(commits []any) []authorChange {
	idx := map[string]*authorChange{}
	var order []string
	for _, c := range commits {
		m := nvObj(c)
		if m == nil {
			continue
		}
		author := nvNestedStr(m, "author", "login")
		if author == "" {
			author = nvNestedStr(m, "commit", "author", "name")
		}
		if author == "" {
			author = "(unknown)"
		}
		sha := nvStr(m, "sha")
		if _, ok := idx[author]; !ok {
			idx[author] = &authorChange{Author: author}
			order = append(order, author)
		}
		idx[author].Commits++
		if sha != "" && len(idx[author].SHAs) < 50 {
			idx[author].SHAs = append(idx[author].SHAs, nvShort(sha))
		}
	}
	out := make([]authorChange, 0, len(order))
	for _, a := range order {
		out = append(out, *idx[a])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Commits != out[j].Commits {
			return out[i].Commits > out[j].Commits
		}
		return out[i].Author < out[j].Author
	})
	return out
}

func nvShort(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

type touchStat struct {
	Author    string `json:"author"`
	Commits   int    `json:"commits"`
	FirstSeen string `json:"first_seen,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
}

// nvWhoTouched ranks commit authors over a path, with first/last commit dates.
func nvWhoTouched(commits []any) []touchStat {
	type acc struct {
		count       int
		first, last time.Time
	}
	idx := map[string]*acc{}
	for _, c := range commits {
		m := nvObj(c)
		if m == nil {
			continue
		}
		author := nvNestedStr(m, "author", "login")
		if author == "" {
			author = nvNestedStr(m, "commit", "author", "name")
		}
		if author == "" {
			author = "(unknown)"
		}
		dateStr := nvNestedStr(m, "commit", "author", "date")
		t, ok := nvParseTime(dateStr)
		a := idx[author]
		if a == nil {
			a = &acc{}
			idx[author] = a
		}
		a.count++
		if ok {
			if a.first.IsZero() || t.Before(a.first) {
				a.first = t
			}
			if t.After(a.last) {
				a.last = t
			}
		}
	}
	out := make([]touchStat, 0, len(idx))
	for author, a := range idx {
		ts := touchStat{Author: author, Commits: a.count}
		if !a.first.IsZero() {
			ts.FirstSeen = a.first.Format(time.RFC3339)
		}
		if !a.last.IsZero() {
			ts.LastSeen = a.last.Format(time.RFC3339)
		}
		out = append(out, ts)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Commits != out[j].Commits {
			return out[i].Commits > out[j].Commits
		}
		return out[i].Author < out[j].Author
	})
	return out
}

type labelStat struct {
	Label  string `json:"label"`
	Open   int    `json:"open"`
	Closed int    `json:"closed"`
	Total  int    `json:"total"`
}

type labelCoverage struct {
	Labels          []labelStat `json:"labels"`
	UnusedLabels    []string    `json:"unused_labels"`
	UnlabeledOpen   int         `json:"unlabeled_open_issues"`
	UnlabeledClosed int         `json:"unlabeled_closed_issues"`
}

// nvLabelCoverage builds the per-label open/closed report plus unused labels
// (defined in repoLabels but on no issue) and unlabeled-issue counts.
func nvLabelCoverage(issues [][]byte, repoLabels [][]byte) labelCoverage {
	open := map[string]int{}
	closed := map[string]int{}
	used := map[string]bool{}
	var unlabeledOpen, unlabeledClosed int
	for _, raw := range issues {
		m := nvDecode(raw)
		state := nvStr(m, "state")
		labels := nvArr(m, "labels")
		if len(labels) == 0 {
			if state == "closed" {
				unlabeledClosed++
			} else {
				unlabeledOpen++
			}
			continue
		}
		for _, l := range labels {
			name := nvStr(nvObj(l), "name")
			if name == "" {
				if s, ok := l.(string); ok {
					name = s
				}
			}
			if name == "" {
				continue
			}
			used[name] = true
			if state == "closed" {
				closed[name]++
			} else {
				open[name]++
			}
		}
	}
	names := map[string]bool{}
	for n := range open {
		names[n] = true
	}
	for n := range closed {
		names[n] = true
	}
	stats := make([]labelStat, 0, len(names))
	for n := range names {
		stats = append(stats, labelStat{Label: n, Open: open[n], Closed: closed[n], Total: open[n] + closed[n]})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Total != stats[j].Total {
			return stats[i].Total > stats[j].Total
		}
		return stats[i].Label < stats[j].Label
	})
	var unused []string
	for _, raw := range repoLabels {
		name := nvStr(nvDecode(raw), "name")
		if name != "" && !used[name] {
			unused = append(unused, name)
		}
	}
	sort.Strings(unused)
	return labelCoverage{Labels: stats, UnusedLabels: unused, UnlabeledOpen: unlabeledOpen, UnlabeledClosed: unlabeledClosed}
}

// nvRaws converts a []json.RawMessage (from the store) to [][]byte for the
// pure aggregation helpers above.
func nvRaws(in []json.RawMessage) [][]byte {
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = []byte(in[i])
	}
	return out
}

// nvDeriveRepo infers owner/repo from the local mirror so live commands
// (changelog, who-touched) work without re-specifying a repo the user already
// synced. It reads one synced issue or pull and parses its repository_url
// (".../repos/{owner}/{repo}") or, for pulls, base.repo.owner.login + name.
func nvDeriveRepo(st *store.Store) (owner, repo string, ok bool) {
	for _, rt := range []string{"issues", "pulls"} {
		rows, err := st.List(rt, 1)
		if err != nil || len(rows) == 0 {
			continue
		}
		m := nvDecode(rows[0])
		if ru := nvStr(m, "repository_url"); ru != "" {
			parts := strings.Split(strings.TrimRight(ru, "/"), "/")
			if len(parts) >= 2 {
				return parts[len(parts)-2], parts[len(parts)-1], true
			}
		}
		if o := nvNestedStr(m, "base", "repo", "owner", "login"); o != "" {
			if r := nvNestedStr(m, "base", "repo", "name"); r != "" {
				return o, r, true
			}
		}
	}
	return "", "", false
}

// openNovelStoreRW opens (creating if needed) the local mirror read-write so a
// command can populate it on demand. The framework `sync` cannot fill GitHub's
// repo-scoped resources (path-templated `/repos/{owner}/{repo}/...`), so the
// novel commands populate their own data from a --repo target.
func openNovelStoreRW(cmd *cobra.Command) (*store.Store, error) {
	dbPath := novelDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store at %s: %w", dbPath, err)
	}
	return st, nil
}

// nvResourceID extracts a stable primary key from a GitHub object: numeric id,
// then commit sha, then node_id.
func nvResourceID(data []byte) string {
	m := nvDecode(data)
	if id := nvNum(m, "id"); id != 0 {
		return strconv.FormatInt(id, 10)
	}
	if sha := nvStr(m, "sha"); sha != "" {
		return sha
	}
	return nvStr(m, "node_id")
}

// nvRepoFromFlag parses an "owner/repo" flag value.
func nvRepoFromFlag(s string) (owner, repo string, ok bool) {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '/'); i > 0 && i < len(s)-1 {
		return s[:i], s[i+1:], true
	}
	return "", "", false
}

// nvCount returns how many rows of a resource_type are in the local store.
func nvCount(st *store.Store, resource string) int {
	var n int
	_ = st.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, resource).Scan(&n)
	return n
}

// nvPopulate fetches a repo-scoped resource live (bounded by maxPages) and
// upserts each item into the store, populating the FTS index. resource is the
// store key: "issues", "pulls", "commits", "labels", or "issues_comments".
func nvPopulate(ctx context.Context, c *client.Client, st *store.Store, owner, repo, resource string, maxPages int) (count int, truncated bool, err error) {
	endpoint := resource
	if resource == "issues_comments" {
		endpoint = "issues/comments"
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/%s", owner, repo, endpoint)
	params := map[string]string{"per_page": "100"}
	if resource == "issues" || resource == "pulls" {
		params["state"] = "all"
	}
	for page := 1; page <= maxPages; page++ {
		params["page"] = strconv.Itoa(page)
		data, err := c.Get(ctx, apiPath, params)
		if err != nil {
			return count, false, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return count, false, fmt.Errorf("parsing %s page %d: %w", resource, page, err)
		}
		pageLen := len(items)
		if pageLen == 0 {
			break
		}
		items = dropGitHubIssuePullRequests(resource, items)
		for _, it := range items {
			id := nvResourceID(it)
			if id == "" {
				continue
			}
			if st.Upsert(resource, id, it) == nil {
				count++
			}
		}
		if pageLen < 100 {
			return count, false, nil
		}
		if page == maxPages {
			return count, true, nil
		}
	}
	return count, false, nil
}

var nvRepoScopedResources = []string{"issues", "pulls", "commits", "labels", "issues_comments"}

func nvClearRepoScopedResources(st *store.Store) error {
	tx, err := st.DB().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, resource := range nvRepoScopedResources {
		if _, err := tx.Exec(`DELETE FROM resources_fts WHERE resource_type = ?`, resource); err != nil {
			return fmt.Errorf("clearing %s search index: %w", resource, err)
		}
		if _, err := tx.Exec(`DELETE FROM resources WHERE resource_type = ?`, resource); err != nil {
			return fmt.Errorf("clearing %s resources: %w", resource, err)
		}
		if _, err := tx.Exec(`DELETE FROM sync_state WHERE resource_type = ?`, resource); err != nil {
			return fmt.Errorf("clearing %s sync state: %w", resource, err)
		}
	}
	return tx.Commit()
}

// nvEnsurePopulated resolves the target repo (from --repo, else inferred from
// already-synced data) and populates each named resource that is empty (or all
// of them when refresh is set). A missing repo is not an error: the command
// proceeds against whatever the store already holds and prints a hint.
func nvEnsurePopulated(ctx context.Context, cmd *cobra.Command, flags *rootFlags, st *store.Store, flagRepo string, refresh bool, maxPages int, resources ...string) error {
	if maxPages <= 0 {
		return usageErr(fmt.Errorf("--max-pages must be greater than zero"))
	}
	explicitOwner, explicitRepo, explicit := nvRepoFromFlag(flagRepo)
	if flagRepo != "" && !explicit {
		return usageErr(fmt.Errorf("invalid --repo %q: expected owner/repo", flagRepo))
	}
	if explicit {
		if cachedOwner, cachedRepo, ok := nvDeriveRepo(st); ok &&
			(!strings.EqualFold(cachedOwner, explicitOwner) || !strings.EqualFold(cachedRepo, explicitRepo)) {
			if err := nvClearRepoScopedResources(st); err != nil {
				return fmt.Errorf("switching local mirror from %s/%s to %s/%s: %w", cachedOwner, cachedRepo, explicitOwner, explicitRepo, err)
			}
			refresh = true
			fmt.Fprintf(cmd.ErrOrStderr(), "switched local mirror from %s/%s to %s/%s\n", cachedOwner, cachedRepo, explicitOwner, explicitRepo)
		}
	}
	needPopulate := refresh
	if !needPopulate {
		for _, r := range resources {
			if nvCount(st, r) == 0 {
				needPopulate = true
				break
			}
		}
	}
	if !needPopulate {
		return nil
	}
	owner, repo, ok := explicitOwner, explicitRepo, explicit
	if !ok {
		owner, repo, ok = nvDeriveRepo(st)
	}
	if !ok {
		fmt.Fprintln(cmd.ErrOrStderr(), "no local data yet and no --repo given; pass --repo owner/repo to populate (e.g. --repo cli/cli)")
		return nil
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	for _, r := range resources {
		if !refresh && nvCount(st, r) > 0 {
			continue
		}
		n, truncated, perr := nvPopulate(ctx, c, st, owner, repo, r, maxPages)
		if perr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: populating %s from %s/%s: %v\n", r, owner, repo, perr)
			continue
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "populated %d %s from %s/%s\n", n, r, owner, repo)
		if truncated {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s population reached the %d-page cap; pass --max-pages N to fetch more\n", r, maxPages)
		}
	}
	return nil
}
