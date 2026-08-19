// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type providerOption struct {
	EntityID    string  `json:"entityId"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description,omitempty"`
	Score       float64 `json:"score"`
	// Connected reports whether the workspace already holds a credential that
	// looks like it belongs to this provider.
	Connected bool `json:"connected"`
}

type compareView struct {
	Query     string           `json:"query"`
	Options   []providerOption `json:"options"`
	Connected []string         `json:"connectedAccounts"`
}

func newNovelEnrichmentsCompareCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "compare <query>",
		Short: "Rank enrichment providers for a job and flag which ones your workspace can run today.",
		Long: "Use this command when choosing between providers for an enrichment column.\n" +
			"It is advisory and never spends credits. Do NOT use it to run enrichment.",
		Example: "  clay-pp-cli enrichments compare \"find work email\" --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<query>=email;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "enrichments compare")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<query> is required"))
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			query := strings.Join(args, " ")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			searchBody := map[string]any{
				"userQuery": query,
				"types":     []string{"action", "waterfall", "claygent", "function"},
				"limit":     flagLimit,
			}
			raw, status, err := c.Post(ctx, fmt.Sprintf("/workspaces/%s/enrichment-search/query-v2", ws), searchBody)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("searching enrichment catalog: %w", err), flags)
			}
			if status >= 300 {
				return fmt.Errorf("searching enrichment catalog: HTTP %d", status)
			}
			var searchRes struct {
				Results []struct {
					EntityID    string  `json:"entity_id"`
					Name        string  `json:"name"`
					Type        string  `json:"type"`
					Description string  `json:"description"`
					Score       float64 `json:"score"`
				} `json:"results"`
			}
			if err := json.Unmarshal(raw, &searchRes); err != nil {
				return fmt.Errorf("parsing enrichment catalog: %w", err)
			}

			// Connected credentials are advisory context, so a failure here must
			// not fail the command; report the gap instead.
			connected := []string{}
			if accRaw, aErr := c.Get(ctx, fmt.Sprintf("/workspaces/%s/app-accounts", ws), nil); aErr == nil {
				var accounts []struct {
					Name             string `json:"name"`
					AppAccountTypeID string `json:"appAccountTypeId"`
				}
				if json.Unmarshal(accRaw, &accounts) == nil {
					for _, a := range accounts {
						if a.Name != "" {
							connected = append(connected, a.Name)
						}
					}
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not list connected accounts: %v\n", aErr)
			}

			view := compareView{Query: query, Options: make([]providerOption, 0, len(searchRes.Results)), Connected: connected}
			for _, r := range searchRes.Results {
				opt := providerOption{
					EntityID: r.EntityID, Name: r.Name, Type: r.Type,
					Description: truncateText(r.Description, 160), Score: r.Score,
				}
				for _, acct := range connected {
					if acct != "" && strings.Contains(strings.ToLower(r.Name), strings.ToLower(firstWord(acct))) {
						opt.Connected = true
						break
					}
				}
				view.Options = append(view.Options, opt)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Options) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no providers matched %q\n", query)
				return nil
			}
			for _, o := range view.Options {
				mark := " "
				if o.Connected {
					mark = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), " %s %-45s %-10s %s\n", mark, o.Name, o.Type, o.Description)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\n* = a connected credential in this workspace looks like a match")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum providers to compare")
	return cmd
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " -_"); i > 0 {
		return s[:i]
	}
	return s
}
