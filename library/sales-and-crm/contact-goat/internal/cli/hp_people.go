// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

// hp people: natural-language people-search against the Happenstance
// graph. Unlike `coverage` (which filters /api/friends/list and only
// sees your 3 top connectors), this command hits the same endpoint the
// web app uses and sees your full synced network across 1st, 2nd, and
// 3rd-degree tiers.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

func newHPCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hp",
		Short: "Happenstance graph commands (1st / 2nd / 3rd degree people-search)",
		Long: `hp groups the Happenstance graph-search commands that wrap the web app's
natural-language search. Unlike the narrow friends/list-backed coverage
command, these hit the full graph (your synced LinkedIn contacts, your
friends' networks, and optionally the public graph).`,
	}
	cmd.AddCommand(newHPPeopleCmd(flags))
	return cmd
}

func newHPPeopleCmd(flags *rootFlags) *cobra.Command {
	var (
		tierConnections bool
		tierFriends     bool
		tierEveryone    bool
		timeoutSec      int
		intervalSec     int
		limit           int
		sourceFlag      string
	)

	cmd := &cobra.Command{
		Use:   "people <query>",
		Short: "Natural-language people-search across your Happenstance graph",
		Long: `Runs a Happenstance people-search with the same endpoint the web app
uses. Supports 1st-degree (your synced connections), 2nd-degree (your
friends' networks), and 3rd-degree (public / search-everyone) tiers.

Each result includes a referrer chain so the CLI can label how you're
connected. The RELATIONSHIP column shows 1st_degree / 2nd_degree /
3rd_degree.`,
		Example: `  # default tiers: 1st + 2nd degree
  contact-goat-pp-cli hp people "people at Weber Inc"

  # 1st-degree only
  contact-goat-pp-cli hp people "engineers at Stripe" --no-friends

  # fan out to the public graph too
  contact-goat-pp-cli hp people "partners at Sequoia" --everyone

  # refine an existing search (see hp people --help for request_id)
  contact-goat-pp-cli hp people "senior" --parent <request_id>

  # JSON for scripting
  contact-goat-pp-cli hp people "people at HBO" --agent`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")

			cfg, cfgErr := config.Load(flags.configPath)
			if cfgErr != nil {
				return configErr(cfgErr)
			}

			// Probe cookie quota cache (no-op when not authenticated).
			cookieClient, _ := flags.newClient()
			cookieAvailable := cookieClient != nil && cookieClient.HasCookieAuth()
			remaining := UnknownSearchesRemaining
			if cookieAvailable {
				remaining = FetchSearchesRemaining(cookieClient, cfg, flags.noCache)
			}

			chosen, deferredErr, hardErr := SelectSource(cmd.Context(), sourceFlag, cfg, cookieAvailable, remaining)
			if hardErr != nil {
				return hardErr
			}
			LogDeferredHint(cmd.ErrOrStderr(), deferredErr)

			cookieRun := CookieRunner(nil)
			if cookieAvailable {
				opts := &client.SearchPeopleOptions{
					IncludeMyConnections: tierConnections,
					IncludeMyFriends:     tierFriends,
					SearchEveryone:       tierEveryone,
					PollTimeout:          time.Duration(timeoutSec) * time.Second,
					PollInterval:         time.Duration(intervalSec) * time.Second,
				}
				cookieRun = func() (*client.PeopleSearchResult, error) {
					return cookieClient.SearchPeopleByQuery(query, opts)
				}
			}
			var bearerRun BearerRunner
			if bc, berr := flags.newHappenstanceAPIClient(); berr == nil {
				bearerRun = func() (*client.PeopleSearchResult, error) {
					return BearerSearchAdapter(cmd.Context(), bc, query, &api.SearchOptions{
						IncludeMyConnections:      tierConnections,
						IncludeFriendsConnections: tierFriends,
					})
				}
			}

			out, err := ExecuteWithSourceFallback(cmd.Context(), chosen, cookieRun, bearerRun, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			res := out.Result

			currentUserUUID := ""
			if cookieAvailable {
				currentUserUUID, _ = fetchCurrentUserUUID(cookieClient)
			}

			if limit > 0 && len(res.People) > limit {
				res.People = res.People[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := buildHPPeopleJSON(res, currentUserUUID)
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return printHPPeopleTable(cmd, res, currentUserUUID)
		},
	}

	cmd.Flags().BoolVar(&tierConnections, "connections", true, "Include your 1st-degree (synced contacts)")
	cmd.Flags().BoolVar(&tierFriends, "friends", true, "Include 2nd-degree via your friends' networks")
	cmd.Flags().BoolVar(&tierEveryone, "everyone", false, "Also include the public / 3rd-degree graph")
	cmd.Flags().IntVar(&timeoutSec, "timeout", int(client.DefaultPollTimeout.Seconds()), "Max seconds to wait for results")
	cmd.Flags().IntVar(&intervalSec, "interval", 1, "Seconds between poll attempts")
	cmd.Flags().IntVar(&limit, "limit", 0, "Client-side cap on results (0 = no cap)")
	cmd.Flags().StringVar(&sourceFlag, "source", SourceFlagAuto, "Auth surface: auto | hp | api. auto routes cookie-first then bearer-fallback")

	// Ergonomic negations: --no-connections / --no-friends land via
	// cobra's `--<name>=false` for bool flags, but also surface a
	// friendlier form below.
	cmd.Flags().Bool("no-connections", false, "Alias for --connections=false (1st-degree off)")
	cmd.Flags().Bool("no-friends", false, "Alias for --friends=false (2nd-degree off)")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if noConns, _ := cmd.Flags().GetBool("no-connections"); noConns {
			tierConnections = false
		}
		if noFriends, _ := cmd.Flags().GetBool("no-friends"); noFriends {
			tierFriends = false
		}
		return nil
	}

	return cmd
}

// hpPeopleJSON is the CLI's normalized output shape. Each row carries
// an explicit `relationship` tier so consumers don't have to rederive
// it from the referrer chain.
type hpPeopleRow struct {
	Name           string                `json:"name"`
	PersonUUID     string                `json:"person_uuid"`
	CurrentTitle   string                `json:"current_title"`
	CurrentCompany string                `json:"current_company"`
	LinkedInURL    string                `json:"linkedin_url"`
	Relationship   client.RelationshipTier `json:"relationship"`
	Referrers      []hpReferrerRow       `json:"referrers,omitempty"`
	Score          float64               `json:"score"`
	Summary        string                `json:"summary,omitempty"`
}

type hpReferrerRow struct {
	Name          string  `json:"name"`
	ID            string  `json:"id"`
	Source        []string `json:"source,omitempty"`
	AffinityLevel string  `json:"affinity_level,omitempty"`
}

type hpPeopleEnvelope struct {
	Query        string          `json:"query"`
	RequestID    string          `json:"request_id"`
	Status       string          `json:"status"`
	Completed    bool            `json:"completed"`
	Count        int             `json:"count"`
	CurrentUser  string          `json:"current_user_uuid,omitempty"`
	Results      []hpPeopleRow   `json:"results"`
}

func buildHPPeopleJSON(res *client.PeopleSearchResult, currentUserUUID string) hpPeopleEnvelope {
	rows := make([]hpPeopleRow, 0, len(res.People))
	for _, p := range res.People {
		refs := make([]hpReferrerRow, 0, len(p.Referrers.Referrers))
		for _, r := range p.Referrers.Referrers {
			refs = append(refs, hpReferrerRow{
				Name: r.Name, ID: r.ID, Source: r.Source, AffinityLevel: r.AffinityLevel,
			})
		}
		rows = append(rows, hpPeopleRow{
			Name:           p.Name,
			PersonUUID:     p.PersonUUID,
			CurrentTitle:   p.CurrentTitle,
			CurrentCompany: p.CurrentCompany,
			LinkedInURL:    p.LinkedInURL,
			Relationship:   p.Tier(currentUserUUID),
			Referrers:      refs,
			Score:          p.Score,
			Summary:        p.Summary,
		})
	}
	return hpPeopleEnvelope{
		Query:       res.Query,
		RequestID:   res.RequestID,
		Status:      res.Status,
		Completed:   res.Completed,
		Count:       len(rows),
		CurrentUser: currentUserUUID,
		Results:     rows,
	}
}

func printHPPeopleTable(cmd *cobra.Command, res *client.PeopleSearchResult, currentUserUUID string) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s - %d results (%s)\n\n", res.Query, len(res.People), res.Status)
	if len(res.People) == 0 {
		fmt.Fprintln(w, "no people found. Try broadening the query or enabling --everyone for public results.")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("RANK")+"\t"+bold("NAME")+"\t"+bold("TITLE")+"\t"+bold("COMPANY")+"\t"+bold("RELATIONSHIP")+"\t"+bold("URL"))
	for i, p := range res.People {
		tier := p.Tier(currentUserUUID)
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			i+1,
			truncate(p.Name, 32),
			truncate(p.CurrentTitle, 32),
			truncate(p.CurrentCompany, 28),
			tier,
			truncate(p.LinkedInURL, 60),
		)
	}
	return tw.Flush()
}

// fetchCurrentUserUUID pulls the current user's Happenstance uuid from
// /api/user so the CLI can label 1st-degree rows correctly. Best-effort:
// on failure we return empty and the CLI falls back to "unknown" tier
// labels rather than erroring out the whole command.
func fetchCurrentUserUUID(c *client.Client) (string, error) {
	raw, err := c.Get("/api/user", nil)
	if err != nil {
		return "", err
	}
	var user struct {
		UUID string `json:"uuid"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return "", err
	}
	if user.UUID != "" {
		return user.UUID, nil
	}
	return user.ID, nil
}
