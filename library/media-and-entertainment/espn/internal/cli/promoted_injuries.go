// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written promoted command. Spec-driven shape declared in spec.yaml.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newInjuriesPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "injuries <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Active injury reports for a league",
		Example: `  espn-pp-cli injuries football nfl
  espn-pp-cli injuries basketball nba --agent
  espn-pp-cli injuries baseball mlb --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: injuries <sport> <league>"))
			}
			sport, league := args[0], args[1]

			// Injuries endpoint lives under site.web.api.espn.com.
			url := fmt.Sprintf("https://site.web.api.espn.com/apis/site/v2/sports/%s/%s/injuries", sport, league)

			body, err := espnHTTPGet(flags.timeout, url)
			if err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				var raw json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					return err
				}
				return enc.Encode(raw)
			}

			return renderInjuries(cmd.OutOrStdout(), body)
		},
	}
	return cmd
}

// espnHTTPGet performs a direct HTTP GET against an absolute ESPN url.
// Used for endpoints that do not live under the default base URL
// (sports.core.api.espn.com, site.web.api.espn.com, etc.).
func espnHTTPGet(timeout time.Duration, url string) ([]byte, error) {
	httpClient := &http.Client{Timeout: timeout}
	if timeout == 0 {
		httpClient.Timeout = 30 * time.Second
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, apiErr(fmt.Errorf("fetching %s: %w", url, err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apiErr(fmt.Errorf("reading response: %w", err))
	}
	if resp.StatusCode == 404 {
		return nil, notFoundErr(fmt.Errorf("%s returned HTTP 404", url))
	}
	if resp.StatusCode == 429 {
		return nil, rateLimitErr(fmt.Errorf("%s rate limited", url))
	}
	if resp.StatusCode >= 400 {
		return nil, apiErr(fmt.Errorf("%s returned HTTP %d: %s", url, resp.StatusCode, truncate(string(body), 200)))
	}
	return body, nil
}

func renderInjuries(w io.Writer, data []byte) error {
	var resp struct {
		Injuries []struct {
			Team struct {
				DisplayName  string `json:"displayName"`
				Abbreviation string `json:"abbreviation"`
			} `json:"team"`
			Injuries []struct {
				Status      string `json:"status"`
				Description string `json:"shortComment"`
				Date        string `json:"date"`
				Athlete     struct {
					DisplayName string `json:"displayName"`
					Position    struct {
						Abbreviation string `json:"abbreviation"`
					} `json:"position"`
				} `json:"athlete"`
				Type struct {
					Description string `json:"description"`
				} `json:"type"`
			} `json:"injuries"`
		} `json:"injuries"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing injuries: %w", err)
	}

	total := 0
	for _, t := range resp.Injuries {
		total += len(t.Injuries)
	}
	if total == 0 {
		fmt.Fprintln(w, "No injuries reported.")
		return nil
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
		bold("TEAM"), bold("PLAYER"), bold("POS"), bold("STATUS"), bold("TYPE"), bold("DATE"))
	for _, team := range resp.Injuries {
		for _, inj := range team.Injuries {
			date := inj.Date
			if len(date) > 10 {
				date = date[:10]
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				team.Team.Abbreviation,
				truncate(inj.Athlete.DisplayName, 25),
				inj.Athlete.Position.Abbreviation,
				inj.Status,
				truncate(inj.Type.Description, 20),
				date)
		}
	}
	return tw.Flush()
}
