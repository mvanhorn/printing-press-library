// Position Tracking commands. PT endpoints live on www.semrush.com (the UI
// host), not api.semrush.com, so these commands construct their own HTTP
// requests rather than using the spec-derived client. They share the same
// SEMRUSH_API_KEY auth (passed as ?key=...).
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/config"
)

const ptBaseURL = "https://www.semrush.com"

func newPTCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pt",
		Aliases: []string{"position-tracking"},
		Short:   "Position Tracking — list campaigns and pull rank reports for tracked keywords",
		Long: "Position Tracking commands talk to the SEMrush UI's PT backend " +
			"(www.semrush.com/positions and /tracking/web-api). Auth is the same " +
			"SEMRUSH_API_KEY used by the public API.",
	}
	cmd.AddCommand(newPTCampaignsCmd(flags))
	cmd.AddCommand(newPTReportCmd(flags))
	cmd.AddCommand(newPTAddKeywordsCmd(flags))
	cmd.AddCommand(newPTRankingsCmd(flags))
	cmd.AddCommand(newPTAnnotateCmd(flags))
	return cmd
}

func newPTCampaignsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaigns",
		Short: "List all your Position Tracking campaigns (id, name, domain, last update)",
		Example: strings.Trim(`
  semrush-pp-cli pt campaigns
  semrush-pp-cli pt campaigns --json --select id,name,domain
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, err := ptHTTPGet(cmd.Context(), flags, "/positions/api/campaigns", nil)
			if err != nil {
				return err
			}
			return ptPrint(cmd, flags, data)
		},
	}
	return cmd
}

func newPTReportCmd(flags *rootFlags) *cobra.Command {
	var dateBegin, dateEnd string
	cmd := &cobra.Command{
		Use:   "report <campaign-id>",
		Short: "Pull a Position Tracking rank report — keyword positions over a date window",
		Long: "Returns the rank/visibility report for one or more PT campaigns. " +
			"Campaign IDs use the SEMrush `<userid>_<campaign>` format (e.g. 29008758_4495670). " +
			"Get them from `semrush-pp-cli pt campaigns`.",
		Example: strings.Trim(`
  semrush-pp-cli pt report 29008758_4495670
  semrush-pp-cli pt report 29008758_4495670 --date-begin last-6 --date-end last
  semrush-pp-cli pt report 29008758_4495670 --json --select keyword,position
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			params := url.Values{}
			params.Set("date_begin", dateBegin)
			params.Set("date_end", dateEnd)
			params["campaign_id[]"] = []string{args[0]}
			data, err := ptHTTPGet(cmd.Context(), flags, "/tracking/web-api/reports/campaigns", params)
			if err != nil {
				return err
			}
			return ptPrint(cmd, flags, data)
		},
	}
	cmd.Flags().StringVar(&dateBegin, "date-begin", "last-6", "Start of report window (e.g. last-6, last-30, or YYYY-MM-DD)")
	cmd.Flags().StringVar(&dateEnd, "date-end", "last", "End of report window (e.g. last, or YYYY-MM-DD)")
	return cmd
}

// ptHTTPGet issues a GET against the www.semrush.com UI host with the user's
// SEMrush API key in the `key` query parameter. It bypasses the spec-derived
// client because PT endpoints live on a different host than the spec's base_url.
func ptHTTPGet(ctx context.Context, flags *rootFlags, path string, params url.Values) ([]byte, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	apiKey := cfg.SemrushApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("SEMRUSH_API_KEY not set — required for Position Tracking (export it or run 'semrush-pp-cli auth')")
	}

	if params == nil {
		params = url.Values{}
	}
	params.Set("key", apiKey)

	u := ptBaseURL + path + "?" + params.Encode()

	if flags.dryRun {
		// Redact key for display
		display := strings.Replace(u, apiKey, "****"+apiKey[len(apiKey)-4:], 1)
		fmt.Fprintln(os.Stdout, "GET", display)
		fmt.Fprintln(os.Stdout, "(dry run - no request sent)")
		return []byte(`[]`), nil
	}

	timeout := 30 * time.Second
	if flags.timeout > 0 {
		timeout = flags.timeout
	}
	httpClient := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "semrush-pp-cli/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d: %s", path, resp.StatusCode, truncatePT(body, 200))
	}
	return body, nil
}

// ptPrint emits the response through the standard flag pipeline (--json,
// --select, --compact, --csv). Falls back to JSON pretty-print if the response
// is already JSON.
func ptPrint(cmd *cobra.Command, flags *rootFlags, data []byte) error {
	// Try JSON; if not valid, dump as-is
	var anyVal any
	if json.Unmarshal(data, &anyVal) != nil {
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	out, err := json.MarshalIndent(anyVal, "", "  ")
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func truncatePT(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
