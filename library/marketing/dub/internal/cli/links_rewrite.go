// Hand-written novel feature: links rewrite.
// Show every link that would change and the exact patch BEFORE sending.
// Mass UTM or domain migrations with dry-run safety.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type rewritePlan struct {
	ID      string `json:"id"`
	Domain  string `json:"domain"`
	Key     string `json:"key"`
	OldURL  string `json:"oldUrl"`
	NewURL  string `json:"newUrl"`
	Applied bool   `json:"applied"`
	Error   string `json:"error,omitempty"`
}

func newLinksRewriteCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var match string
	var replace string
	var apply bool
	var maxLinks int

	cmd := &cobra.Command{
		Use:   "rewrite",
		Short: "Show every link that would change and the exact patch BEFORE sending",
		Long: `Bulk URL/UTM rewrite with diff preview. Match a substring in each link's
destination URL and replace it. Defaults to dry-run; pass --apply to actually
PATCH /links/{id} for each affected link.

Always run with --dry-run first to confirm the plan.`,
		Example: strings.Trim(`
  # Preview a UTM swap (dry-run by default)
  dub-pp-cli links rewrite --match 'utm_source=launch' --replace 'utm_source=summer'

  # Apply after reviewing the diff
  dub-pp-cli links rewrite --match 'utm_source=launch' --replace 'utm_source=summer' --apply --yes

  # Cap blast radius to 25 links per run
  dub-pp-cli links rewrite --match 'old.example.com' --replace 'new.example.com' --max-links 25
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if match == "" && replace == "" {
				return cmd.Help()
			}
			if match == "" {
				return usageErr(fmt.Errorf("--match is required"))
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(domain,''), COALESCE("key",''), COALESCE(data,'{}')
				FROM links
			`)
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()

			var plans []rewritePlan
			seen := 0
			for rows.Next() {
				var id, domain, key string
				var blob sql.NullString
				if err := rows.Scan(&id, &domain, &key, &blob); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				oldURL := extractFromData(blobBytes, "url")
				if oldURL == "" || !strings.Contains(oldURL, match) {
					continue
				}
				newURL := strings.ReplaceAll(oldURL, match, replace)
				if newURL == oldURL {
					continue
				}
				plans = append(plans, rewritePlan{
					ID: id, Domain: domain, Key: key,
					OldURL: oldURL, NewURL: newURL,
				})
				if maxLinks > 0 && len(plans) >= maxLinks {
					break
				}
			}
			if seen == 0 {
				return hintEmptyStore("links")
			}

			// Default behavior is dry-run; --apply opts in to mutation.
			if !apply || dryRunOK(flags) {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), plans, flags)
				}
				if len(plans) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No links matched.")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would change %d link(s):\n\n", len(plans))
				for _, p := range plans {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s/%s\n", p.Domain, p.Key)
					fmt.Fprintf(cmd.OutOrStdout(), "    -%s\n", truncate(p.OldURL, 110))
					fmt.Fprintf(cmd.OutOrStdout(), "    +%s\n\n", truncate(p.NewURL, 110))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --apply --yes to send the patches.")
				return nil
			}

			if !flags.yes && len(plans) > 5 {
				return usageErr(fmt.Errorf("rewriting %d links requires --yes", len(plans)))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			ok := 0
			for i := range plans {
				if err := applyRewrite(ctx, c, &plans[i]); err != nil {
					plans[i].Error = err.Error()
					continue
				}
				plans[i].Applied = true
				ok++
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), plans, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Applied %d / %d rewrite(s).\n", ok, len(plans))
			for _, p := range plans {
				if p.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  FAIL %s/%s — %s\n", p.Domain, p.Key, p.Error)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&match, "match", "", "Substring to match within each link's destination URL")
	cmd.Flags().StringVar(&replace, "replace", "", "Replacement substring")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the patches (default is dry-run preview)")
	cmd.Flags().IntVar(&maxLinks, "max-links", 0, "Cap the number of links rewritten in one run (0 = no cap)")
	return cmd
}

// applyRewrite issues a PATCH /links/{linkId} with {"url": newURL}.
func applyRewrite(ctx context.Context, c apiClient, plan *rewritePlan) error {
	body := map[string]any{"url": plan.NewURL}
	bodyBytes, _ := json.Marshal(body)
	_ = ctx
	_, _, err := c.Patch("/links/"+plan.ID, json.RawMessage(bodyBytes))
	return err
}

// apiClient is a tiny shim to type-check against the generated *client.Client.
// The generated client exposes Patch(path, body) (raw, status, err).
type apiClient interface {
	Patch(path string, body any) (json.RawMessage, int, error)
}
