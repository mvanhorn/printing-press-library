// Hand-written: engineering command. GitHub org + repo signal.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/github"
	"github.com/spf13/cobra"
)

type engineeringResult struct {
	Domain          string            `json:"domain"`
	GitHubOrg       string            `json:"github_org,omitempty"`
	OrgName         string            `json:"org_name,omitempty"`
	Description     string            `json:"description,omitempty"`
	PublicRepos     int               `json:"public_repos"`
	Followers       int               `json:"followers"`
	CreatedAt       string            `json:"created_at,omitempty"`
	TopRepos        []engineeringRepo `json:"top_repos,omitempty"`
	LanguagesByRepo map[string]int    `json:"languages_by_repo,omitempty"`
	RecentlyActive  int               `json:"recently_active_repos"`
	Note            string            `json:"note,omitempty"`
}

type engineeringRepo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
	Stars       int    `json:"stars"`
	PushedAt    string `json:"pushed_at,omitempty"`
	URL         string `json:"url,omitempty"`
}

func newEngineeringCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags

	cmd := &cobra.Command{
		Use:   "engineering [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "GitHub org metadata: repo count, contributor count, commit cadence, top languages.",
		Long: `engineering surfaces a GitHub org's public footprint: number of public repos, top repos by recent activity, language mix, and how many repos have seen commits in the last 90 days.

Useful as an "is this team actually building?" signal. A company with 200 public repos but zero pushed in 18 months is a different signal than one with 12 repos all updated this month.

Without GITHUB_TOKEN, rate-limited to 60 req/hr. Set GITHUB_TOKEN or run 'gh auth login' to raise to 5000 req/hr.`,
		Example: strings.Trim(`
  company-goat-pp-cli engineering anthropic
  company-goat-pp-cli engineering vercel --json
  company-goat-pp-cli engineering --domain stripe.com
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			gh := github.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			// Pass the resolved name (from args) to help match alt org names.
			name := strings.Join(args, " ")
			org, err := gh.FindOrgFromDomain(ctx, domain, name)
			if err != nil {
				return fmt.Errorf("github: %w", err)
			}

			result := engineeringResult{Domain: domain}
			if org == nil {
				result.Note = fmt.Sprintf("no GitHub org found for %s (tried domain stem and name variants)", domain)
				renderEngineering(cmd, flags, result)
				return nil
			}
			result.GitHubOrg = org.Login
			result.OrgName = org.Name
			result.Description = org.Description
			result.PublicRepos = org.PublicRepos
			result.Followers = org.Followers
			result.CreatedAt = org.CreatedAt

			// Repos.
			rctx, rcancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer rcancel()
			repos, err := gh.ListRepos(rctx, org.Login, 50)
			if err != nil {
				result.Note = "couldn't fetch repo list (rate limit?): " + err.Error()
				renderEngineering(cmd, flags, result)
				return nil
			}
			now := time.Now()
			langs := map[string]int{}
			recent := 0
			topRepos := make([]engineeringRepo, 0, len(repos))
			for _, r := range repos {
				if r.Fork || r.Archived || r.Private {
					continue
				}
				if r.Language != "" {
					langs[r.Language]++
				}
				if r.PushedAt != "" {
					if pushed, err := time.Parse(time.RFC3339, r.PushedAt); err == nil {
						if now.Sub(pushed) < 90*24*time.Hour {
							recent++
						}
					}
				}
				topRepos = append(topRepos, engineeringRepo{
					Name:        r.Name,
					Description: r.Description,
					Language:    r.Language,
					Stars:       r.StargazersCount,
					PushedAt:    r.PushedAt,
					URL:         r.HTMLURL,
				})
			}
			sort.SliceStable(topRepos, func(i, j int) bool { return topRepos[i].Stars > topRepos[j].Stars })
			if len(topRepos) > 8 {
				topRepos = topRepos[:8]
			}
			result.TopRepos = topRepos
			result.LanguagesByRepo = langs
			result.RecentlyActive = recent
			renderEngineering(cmd, flags, result)
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	return cmd
}

func renderEngineering(cmd *cobra.Command, flags *rootFlags, r engineeringResult) {
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		_ = flags.printJSON(cmd, r)
		return
	}
	fmt.Fprintf(w, "Domain: %s\n", r.Domain)
	if r.GitHubOrg == "" {
		fmt.Fprintf(w, "GitHub: %s\n", r.Note)
		return
	}
	fmt.Fprintf(w, "GitHub: github.com/%s  (%s)\n", r.GitHubOrg, r.OrgName)
	if r.Description != "" {
		fmt.Fprintf(w, "  %s\n", r.Description)
	}
	fmt.Fprintf(w, "Public repos: %d  Followers: %d  Recently active (90d): %d\n", r.PublicRepos, r.Followers, r.RecentlyActive)
	if len(r.LanguagesByRepo) > 0 {
		// Render top 5 languages.
		type lang struct {
			Name  string
			Count int
		}
		var ls []lang
		for n, c := range r.LanguagesByRepo {
			ls = append(ls, lang{n, c})
		}
		sort.SliceStable(ls, func(i, j int) bool { return ls[i].Count > ls[j].Count })
		max := 5
		if len(ls) < max {
			max = len(ls)
		}
		fmt.Fprintf(w, "Top languages: ")
		for i := 0; i < max; i++ {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "%s (%d)", ls[i].Name, ls[i].Count)
		}
		fmt.Fprintln(w)
	}
	if len(r.TopRepos) > 0 {
		fmt.Fprintf(w, "\nTop repos by stars:\n")
		for _, repo := range r.TopRepos {
			fmt.Fprintf(w, "  %4d★  %-30s  %s  %s\n", repo.Stars, repo.Name, repo.Language, fundingTruncate(repo.Description, 50))
		}
	}
	if r.Note != "" {
		fmt.Fprintf(w, "\nNote: %s\n", r.Note)
	}
}
