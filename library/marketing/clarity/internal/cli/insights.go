package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/clarity/internal/cliutil"
	"github.com/spf13/cobra"
)

const clarityExportEndpoint = "https://www.clarity.ms/export-data/api/v1/project-live-insights"

var clarityDimensionNames = map[string]string{
	"browser": "Browser",
	"device":  "Device",
	// PATCH(printing-press-library#308): Clarity documents Country/Region, but the live API only returns the requested geography for Country.
	"country/region": "Country",
	"country":        "Country",
	"region":         "Country",
	"os":             "OS",
	"source":         "Source",
	"medium":         "Medium",
	"campaign":       "Campaign",
	"channel":        "Channel",
	"url":            "URL",
}

func newInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Read Microsoft Clarity Data Export API insights",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newInsightsLiveCmd(flags))
	return cmd
}

func newInsightsLiveCmd(flags *rootFlags) *cobra.Command {
	days := 1
	var dimensions []string
	cmd := &cobra.Command{
		Use:   "live",
		Short: "Fetch project live insights from the Microsoft Clarity Data Export API",
		Long: `Fetch project live insights from the Microsoft Clarity Data Export API.

Set PP_CLARITY_API_TOKEN, MICROSOFT_CLARITY_API_TOKEN, or CLARITY_API_TOKEN in
the environment, or place the token in ~/.config/clarity-pp-cli/api-token.
The API token is generated in Microsoft Clarity under Settings -> Data Export
-> Generate new API token.`,
		Example: `  clarity-pp-cli insights live --days 1 --dimension OS --json
  clarity-pp-cli insights live --days 3 --dimension Source --dimension Campaign --agent --select metricName,information`,
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := buildInsightsQuery(days, dimensions)
			if err != nil {
				return usageErr(err)
			}
			target := clarityExportEndpoint + "?" + query.Encode()

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s\nAuthorization: Bearer <redacted>\n", target)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{
					{
						"metricName": "Traffic",
						"information": []map[string]any{
							{
								"totalSessionCount":    "0",
								"totalBotSessionCount": "0",
								"distantUserCount":     "0",
							},
						},
					},
				}, flags)
			}

			token, tokenSource := clarityAPITokenFromEnv()
			if token == "" {
				return authErr(fmt.Errorf("missing Microsoft Clarity Data Export API token; set PP_CLARITY_API_TOKEN, MICROSOFT_CLARITY_API_TOKEN, CLARITY_API_TOKEN, or write ~/.config/clarity-pp-cli/api-token"))
			}

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, target, nil)
			if err != nil {
				return apiErr(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "clarity-pp-cli/insights")

			client := &http.Client{Timeout: flags.timeout}
			resp, err := client.Do(req)
			if err != nil {
				return apiErr(err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return apiErr(err)
			}
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return authErr(fmt.Errorf("Microsoft Clarity Data Export API rejected token from %s with HTTP %d", tokenSource, resp.StatusCode))
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return apiErr(fmt.Errorf("Microsoft Clarity Data Export API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
			}

			if !json.Valid(body) {
				return apiErr(fmt.Errorf("Microsoft Clarity Data Export API returned non-JSON response"))
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(body), flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 1, "Number of recent days to export: 1, 2, or 3")
	cmd.Flags().StringArrayVar(&dimensions, "dimension", nil, "Dimension to break down insights by; repeat up to three times; country aliases send Country because Clarity drops Country/Region")
	return cmd
}

func buildInsightsQuery(days int, dimensions []string) (url.Values, error) {
	if days < 1 || days > 3 {
		return nil, fmt.Errorf("--days must be 1, 2, or 3")
	}
	if len(dimensions) > 3 {
		return nil, fmt.Errorf("at most three --dimension values are allowed")
	}
	values := url.Values{}
	values.Set("numOfDays", fmt.Sprintf("%d", days))
	for i, dimension := range dimensions {
		normalized, err := normalizeClarityDimension(dimension)
		if err != nil {
			return nil, err
		}
		values.Set(fmt.Sprintf("dimension%d", i+1), normalized)
	}
	return values, nil
}

func normalizeClarityDimension(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--dimension cannot be empty")
	}
	if canonical, ok := clarityDimensionNames[strings.ToLower(value)]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("invalid --dimension %q: use Browser, Device, Country, OS, Source, Medium, Campaign, Channel, or URL", value)
}

func clarityAPITokenFromEnv() (string, string) {
	if path := strings.TrimSpace(os.Getenv("PP_CLARITY_API_TOKEN_FILE")); path != "" {
		if token := readTokenFile(path); token != "" {
			return token, "PP_CLARITY_API_TOKEN_FILE"
		}
	}
	for _, name := range []string{"PP_CLARITY_API_TOKEN", "MICROSOFT_CLARITY_API_TOKEN", "CLARITY_API_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value, name
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if token := readTokenFile(home + "/.config/clarity-pp-cli/api-token"); token != "" {
			return token, "~/.config/clarity-pp-cli/api-token"
		}
	}
	return "", ""
}

func readTokenFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
