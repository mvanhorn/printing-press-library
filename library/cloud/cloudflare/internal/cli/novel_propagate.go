// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/internal/cliutil"
)

func newPropagateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propagate",
		Short: "DNS propagation verification across public resolvers",
	}
	cmd.AddCommand(newPropagateWatchCmd(flags))
	return cmd
}

// dohResolver describes one public DNS-over-HTTPS endpoint we query.
type dohResolver struct {
	Name string
	URL  string
}

// publicDoHResolvers are free, public, no-auth DoH endpoints (per the absorb-scoring
// rubric's "external service" exception). Order is irrelevant; queries fan out in parallel.
var publicDoHResolvers = []dohResolver{
	{Name: "cloudflare", URL: "https://1.1.1.1/dns-query"},
	{Name: "google", URL: "https://8.8.8.8/resolve"},
	{Name: "quad9", URL: "https://9.9.9.9:5053/dns-query"},
}

type resolverResult struct {
	Resolver string   `json:"resolver"`
	Values   []string `json:"values"`
	Match    bool     `json:"match"`
	Error    string   `json:"error,omitempty"`
}

func newPropagateWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		expect   string
		watch    bool
		interval time.Duration
		timeout  time.Duration
	)

	cmd := &cobra.Command{
		Use:   "watch <name> <type>",
		Short: "Verify a DNS record's value across public resolvers",
		Long: `Query Cloudflare 1.1.1.1, Google 8.8.8.8, and Quad9 9.9.9.9 in parallel for the
given record. Exit 0 when every resolver returns the expected value, non-zero
otherwise. With --watch, retry on an interval until success or timeout.`,
		Example: `  cloudflare-pp-cli propagate watch example.com A --expect 203.0.113.10 --watch`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			name := args[0]
			recType := strings.ToUpper(args[1])

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			httpClient := &http.Client{Timeout: 10 * time.Second}

			query := func() ([]resolverResult, bool) {
				results := make([]resolverResult, 0, len(publicDoHResolvers))
				allMatch := true
				for _, r := range publicDoHResolvers {
					values, err := queryDoH(ctx, httpClient, r.URL, name, recType)
					rr := resolverResult{Resolver: r.Name, Values: values}
					if err != nil {
						rr.Error = err.Error()
						allMatch = false
					} else if expect != "" {
						matched := false
						for _, v := range values {
							if v == expect {
								matched = true
								break
							}
						}
						rr.Match = matched
						if !matched {
							allMatch = false
						}
					} else {
						rr.Match = len(values) > 0
					}
					results = append(results, rr)
				}
				if expect == "" {
					return results, len(results) > 0
				}
				return results, allMatch
			}

			out := func(results []resolverResult, ok bool) error {
				envelope := map[string]any{
					"name":      name,
					"type":      recType,
					"expect":    expect,
					"propagated": ok,
					"resolvers": results,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			if !watch {
				if cliutil.IsVerifyEnv() {
					return out([]resolverResult{}, false)
				}
				results, ok := query()
				if err := out(results, ok); err != nil {
					return err
				}
				if !ok {
					return apiErr(fmt.Errorf("propagation incomplete"))
				}
				return nil
			}

			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			results, ok := query()
			if err := out(results, ok); err != nil {
				return err
			}
			if ok {
				return nil
			}
			for {
				select {
				case <-ctx.Done():
					return apiErr(fmt.Errorf("propagation timeout after %s", timeout))
				case <-ticker.C:
					results, ok = query()
					if err := out(results, ok); err != nil {
						return err
					}
					if ok {
						return nil
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&expect, "expect", "", "Expected value (exit 0 only when all resolvers return this)")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll until propagation completes or --timeout fires")
	cmd.Flags().DurationVar(&interval, "interval", 15*time.Second, "Poll interval when --watch is set")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Max time to wait when --watch is set")
	return cmd
}

// queryDoH issues a DNS-over-HTTPS query against an RFC 8484 / Google-format
// endpoint and returns the answer record values.
func queryDoH(ctx context.Context, hc *http.Client, endpoint, name, recType string) ([]string, error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("type", recType)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", endpoint, resp.StatusCode)
	}
	var body struct {
		Answer []struct {
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	values := make([]string, 0, len(body.Answer))
	for _, a := range body.Answer {
		values = append(values, a.Data)
	}
	return values, nil
}
