// Copyright 2026 Hiten Shah and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-intel/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-intel/internal/cliutil"
	"github.com/spf13/cobra"
)

type repoSummary struct {
	FullName        string `json:"full_name"`
	Description     string `json:"description,omitempty"`
	Language        string `json:"language,omitempty"`
	Stars           int    `json:"stars"`
	Forks           int    `json:"forks"`
	OpenIssues      int    `json:"open_issues"`
	Archived        bool   `json:"archived"`
	License         string `json:"license,omitempty"`
	PushedAt        string `json:"pushed_at,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	DefaultBranch   string `json:"default_branch,omitempty"`
	HTMLURL         string `json:"html_url,omitempty"`
	LastRelease     string `json:"last_release,omitempty"`
	LastReleaseDate string `json:"last_release_date,omitempty"`
}

type releaseSummary struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
	HTMLURL     string `json:"html_url,omitempty"`
}

type advisorySummary struct {
	GHSAID      string `json:"ghsa_id"`
	CVEID       string `json:"cve_id,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Severity    string `json:"severity,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	HTMLURL     string `json:"html_url,omitempty"`
}

func newTrendingCmd(flags *rootFlags) *cobra.Command {
	var language, topic, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Find newly active repositories by language or topic",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, err := fetchTrending(cmd, c, language, topic, since, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&language, "language", "", "Language qualifier, e.g. go or typescript")
	cmd.Flags().StringVar(&topic, "topic", "", "Topic qualifier, e.g. ai or agents")
	cmd.Flags().StringVar(&since, "since", "weekly", "Freshness window: daily, weekly, monthly, or a duration such as 30d")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum repositories to return")
	return cmd
}

func newReleasesIntelCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "releases <owner/repo>",
		Short: "List recent releases for a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			owner, repo, err := splitRepo(args[0])
			if err != nil {
				return err
			}
			items, err := fetchReleases(cmd.Context(), c, owner, repo, since, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Only include releases after this window: 7d, 30d, monthly, or an RFC3339 date")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum releases to return")
	return cmd
}

func newAdvisoriesIntelCmd(flags *rootFlags) *cobra.Command {
	var ecosystem, pkg, severity, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "advisories",
		Short: "Brief GitHub security advisories for a package or ecosystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, err := fetchAdvisories(cmd, c, ecosystem, pkg, severity, since, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&ecosystem, "ecosystem", "", "Package ecosystem, e.g. npm, pip, go, rubygems")
	cmd.Flags().StringVar(&pkg, "package", "", "Affected package name")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity filter: low, medium, high, critical")
	cmd.Flags().StringVar(&since, "since", "", "Only include advisories published after this window or date")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum advisories to return")
	return cmd
}

func newRepoHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo-health <owner/repo>",
		Short: "Summarize repository adoption and maintenance signals",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			owner, repo, err := splitRepo(args[0])
			if err != nil {
				return err
			}
			summary, err := fetchRepoSummary(cmd.Context(), c, owner, repo, true)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	return cmd
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <owner/repo> [owner/repo...]",
		Short: "Compare repository health and momentum signals",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, errs := cliutil.FanoutRun(cmd.Context(), args, func(arg string) string {
				return arg
			}, func(ctx context.Context, arg string) (repoSummary, error) {
				owner, repo, err := splitRepo(arg)
				if err != nil {
					return repoSummary{}, err
				}
				return fetchRepoSummary(ctx, c, owner, repo, true)
			}, cliutil.WithConcurrency(min(len(args), 4)))
			if len(errs) > 0 {
				return fmt.Errorf("%s: %w", errs[0].Source, errs[0].Err)
			}
			out := make([]repoSummary, 0, len(results))
			for _, result := range results {
				out = append(out, result.Value)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Stars == out[j].Stars {
					return out[i].PushedAt > out[j].PushedAt
				}
				return out[i].Stars > out[j].Stars
			})
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func fetchTrending(cmd *cobra.Command, c *client.Client, language, topic, since string, limit int) ([]repoSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	after, err := sinceTime(since, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	parts := []string{fmt.Sprintf("pushed:>%s", after.Format("2006-01-02"))}
	if language != "" {
		parts = append(parts, "language:"+language)
	}
	if topic != "" {
		parts = append(parts, "topic:"+topic)
	}
	params := map[string]string{
		"q":        strings.Join(parts, " "),
		"sort":     "stars",
		"order":    "desc",
		"per_page": strconv.Itoa(limit),
	}
	raw, err := c.Get(cmd.Context(), "/search/repositories", params)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]repoSummary, 0, len(payload.Items))
	for _, item := range payload.Items {
		summary, err := parseRepoSummary(item)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func fetchRepoSummary(ctx context.Context, c *client.Client, owner, repo string, includeRelease bool) (repoSummary, error) {
	raw, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s", owner, repo), nil)
	if err != nil {
		return repoSummary{}, err
	}
	summary, err := parseRepoSummary(raw)
	if err != nil {
		return repoSummary{}, err
	}
	if includeRelease {
		releases, err := fetchReleases(ctx, c, owner, repo, "", 1)
		if err == nil && len(releases) > 0 {
			summary.LastRelease = releases[0].TagName
			summary.LastReleaseDate = releases[0].PublishedAt
		}
	}
	return summary, nil
}

func fetchReleases(ctx context.Context, c *client.Client, owner, repo, since string, limit int) ([]releaseSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if since == "" {
		params := map[string]string{"per_page": strconv.Itoa(limit)}
		raw, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/releases", owner, repo), params)
		if err != nil {
			return nil, err
		}
		var releases []releaseSummary
		if err := json.Unmarshal(raw, &releases); err != nil {
			return nil, err
		}
		return releases, nil
	}
	after, err := sinceTime(since, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	const perPage = 100
	out := make([]releaseSummary, 0, limit)
	for page := 1; ; page++ {
		params := map[string]string{
			"per_page": strconv.Itoa(perPage),
			"page":     strconv.Itoa(page),
		}
		raw, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/releases", owner, repo), params)
		if err != nil {
			return nil, err
		}
		var releases []releaseSummary
		if err := json.Unmarshal(raw, &releases); err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			break
		}
		pagePastWindow := false
		for _, release := range releases {
			if release.PublishedAt == "" {
				continue
			}
			published, err := time.Parse(time.RFC3339, release.PublishedAt)
			if err != nil {
				continue
			}
			if published.Before(after) {
				pagePastWindow = true
				continue
			}
			out = append(out, release)
			if len(out) >= limit {
				return out, nil
			}
		}
		if len(releases) < perPage || pagePastWindow {
			break
		}
	}
	return out, nil
}

func fetchAdvisories(cmd *cobra.Command, c *client.Client, ecosystem, pkg, severity, since string, limit int) ([]advisorySummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	params := map[string]string{"per_page": strconv.Itoa(limit)}
	if ecosystem != "" {
		params["ecosystem"] = ecosystem
	}
	if pkg != "" {
		params["affects"] = pkg
	}
	if severity != "" {
		params["severity"] = severity
	}
	if since != "" {
		after, err := sinceTime(since, 30*24*time.Hour)
		if err != nil {
			return nil, err
		}
		params["published"] = ">=" + after.Format("2006-01-02")
	}
	raw, err := c.Get(cmd.Context(), "/advisories", params)
	if err != nil {
		return nil, err
	}
	var out []advisorySummary
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseRepoSummary(raw json.RawMessage) (repoSummary, error) {
	var payload struct {
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Language      string `json:"language"`
		Stars         int    `json:"stargazers_count"`
		Forks         int    `json:"forks_count"`
		OpenIssues    int    `json:"open_issues_count"`
		Archived      bool   `json:"archived"`
		PushedAt      string `json:"pushed_at"`
		CreatedAt     string `json:"created_at"`
		DefaultBranch string `json:"default_branch"`
		HTMLURL       string `json:"html_url"`
		License       *struct {
			SPDXID string `json:"spdx_id"`
		} `json:"license"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return repoSummary{}, err
	}
	license := ""
	if payload.License != nil {
		license = payload.License.SPDXID
	}
	return repoSummary{
		FullName:      payload.FullName,
		Description:   payload.Description,
		Language:      payload.Language,
		Stars:         payload.Stars,
		Forks:         payload.Forks,
		OpenIssues:    payload.OpenIssues,
		Archived:      payload.Archived,
		License:       license,
		PushedAt:      payload.PushedAt,
		CreatedAt:     payload.CreatedAt,
		DefaultBranch: payload.DefaultBranch,
		HTMLURL:       payload.HTMLURL,
	}, nil
}

func splitRepo(value string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repository must be owner/repo")
	}
	return parts[0], parts[1], nil
}

func sinceTime(value string, fallback time.Duration) (time.Time, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return time.Now().Add(-fallback), nil
	}
	switch value {
	case "daily":
		return time.Now().Add(-24 * time.Hour), nil
	case "weekly":
		return time.Now().Add(-7 * 24 * time.Hour), nil
	case "monthly":
		return time.Now().Add(-30 * 24 * time.Hour), nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days < 0 {
			return time.Time{}, fmt.Errorf("invalid --since value %q", value)
		}
		return time.Now().Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q", value)
}
