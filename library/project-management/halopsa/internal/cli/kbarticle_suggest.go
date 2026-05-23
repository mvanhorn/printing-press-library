// Hand-written novel feature. Not generated.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newKBArticleSuggestCmd(flags *rootFlags) *cobra.Command {
	var (
		ticketID string
		query    string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "FTS-rank KB articles against a ticket's summary + details + last action text",
		Long: `Builds a search query from the ticket's summary, details, and most recent action
body, then queries the live /KBArticle endpoint and returns the top N matches.`,
		Example: strings.Trim(`
  # Suggest KB articles for a ticket id
  halopsa-pp-cli kbarticle suggest --ticket 12345 --limit 5

  # Override with a raw query (e.g., from a stand-alone search)
  halopsa-pp-cli kbarticle suggest --query "VPN reconnect after lock screen"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if ticketID == "" && query == "" {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			searchText := query
			if searchText == "" {
				raw, err := c.Get("/Tickets/"+ticketID, map[string]string{"includedetails": "true"})
				if err != nil {
					return fmt.Errorf("fetching ticket %s: %w", ticketID, err)
				}
				var t map[string]any
				if err := json.Unmarshal(raw, &t); err != nil {
					return fmt.Errorf("decoding ticket %s: %w", ticketID, err)
				}
				parts := []string{}
				for _, k := range []string{"summary", "details", "details_plaintext", "last_action_text", "actions_text"} {
					if v, ok := t[k]; ok && v != nil {
						s := strings.TrimSpace(fmt.Sprintf("%v", v))
						if s != "" {
							parts = append(parts, s)
						}
					}
				}
				searchText = strings.Join(parts, " ")
				// Keep only the first ~500 chars to avoid pathological query strings
				if len(searchText) > 500 {
					searchText = searchText[:500]
				}
			}
			if strings.TrimSpace(searchText) == "" {
				return fmt.Errorf("no searchable text from ticket %s; pass --query explicitly", ticketID)
			}
			kbRaw, err := c.Get("/KBArticle", map[string]string{
				"search":    searchText,
				"page_size": fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return fmt.Errorf("kb search: %w", err)
			}
			articles := unwrapList(kbRaw, "articles")
			type suggestion struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			}
			out := []suggestion{}
			for _, a := range articles {
				s := suggestion{
					ID:    fmt.Sprintf("%v", a["id"]),
					Title: fmt.Sprintf("%v", a["name"]),
				}
				if v, ok := a["resolution"]; ok && v != nil {
					s.Snippet = trim(fmt.Sprintf("%v", v), 200)
				} else if v, ok := a["description"]; ok && v != nil {
					s.Snippet = trim(fmt.Sprintf("%v", v), 200)
				}
				out = append(out, s)
				if len(out) >= limit {
					break
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"ticket_id":   ticketID,
					"query":       searchText,
					"suggestions": out,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Suggestions (q=%q):\n\n", trim(searchText, 80))
			for i, s := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. [#%s] %s\n   %s\n\n", i+1, s.ID, s.Title, s.Snippet)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No KB articles matched.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ticketID, "ticket", "", "Ticket id to extract search text from")
	cmd.Flags().StringVar(&query, "query", "", "Override: raw search query (skip ticket fetch)")
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of suggestions to return")
	return cmd
}
