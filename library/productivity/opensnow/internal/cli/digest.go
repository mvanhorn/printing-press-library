package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/opensnow/internal/cliutil"

	"github.com/spf13/cobra"
)

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagFull bool

	cmd := &cobra.Command{
		Use:     "digest",
		Short:   "Show a digest of Daily Snow posts for favorite regions",
		Long:    "Fetches Daily Snow posts for each favorite region, strips HTML from content, and shows a summary digest.",
		Example: "  opensnow-pp-cli digest\n  opensnow-pp-cli digest --region colorado\n  opensnow-pp-cli digest --full",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			var slugs []string
			if flagRegion != "" {
				slugs = []string{flagRegion}
			} else {
				db, favs, err := loadFavorites(cmd.Context())
				if err != nil {
					return err
				}
				db.Close()
				slugs = favs
			}

			if len(slugs) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No favorites configured. Add some with: opensnow-pp-cli favorites add <slug>")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type digestData struct {
				Slug string
				Data map[string]any
			}

			results, errs := cliutil.FanoutRun(
				cmd.Context(),
				slugs,
				func(s string) string { return s },
				func(ctx context.Context, slug string) (digestData, error) {
					path := "/daily-reads/" + slug + "/content"
					data, err := c.Get(path, map[string]string{})
					if err != nil {
						return digestData{}, err
					}
					data = extractResponseData(data)
					var obj map[string]any
					if err := json.Unmarshal(data, &obj); err != nil {
						return digestData{}, err
					}
					return digestData{Slug: slug, Data: obj}, nil
				},
			)
			cliutil.FanoutReportErrors(os.Stderr, errs)

			if len(results) == 0 {
				return fmt.Errorf("no Daily Snow data available")
			}

			type digestRow struct {
				Region  string `json:"region"`
				Author  string `json:"author"`
				Posted  string `json:"posted"`
				Summary string `json:"summary"`
				Content string `json:"content"`
			}

			rows := make([]digestRow, 0, len(results))
			for _, r := range results {
				row := digestRow{Region: r.Value.Slug}

				if v, ok := r.Value.Data["name"].(string); ok {
					row.Region = v
				} else if v, ok := r.Value.Data["title"].(string); ok {
					row.Region = v
				}
				if v, ok := r.Value.Data["author"].(string); ok {
					row.Author = v
				} else if author, ok := r.Value.Data["author"].(map[string]any); ok {
					if name, ok := author["name"].(string); ok {
						row.Author = name
					}
				}
				if v, ok := r.Value.Data["published_at"].(string); ok {
					row.Posted = v
				} else if v, ok := r.Value.Data["date"].(string); ok {
					row.Posted = v
				} else if v, ok := r.Value.Data["created_at"].(string); ok {
					row.Posted = v
				}
				if v, ok := r.Value.Data["summary"].(string); ok {
					row.Summary = cliutil.StripHTML(v)
				}
				if v, ok := r.Value.Data["content"].(string); ok {
					cleaned := cliutil.StripHTML(v)
					if !flagFull && len(cleaned) > 500 {
						cleaned = cleaned[:500] + "..."
					}
					row.Content = cleaned
				} else if v, ok := r.Value.Data["body"].(string); ok {
					cleaned := cliutil.StripHTML(v)
					if !flagFull && len(cleaned) > 500 {
						cleaned = cleaned[:500] + "..."
					}
					row.Content = cleaned
				}

				rows = append(rows, row)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			// Card-style output for readability
			for i, r := range rows {
				if i > 0 {
					fmt.Fprintln(cmd.OutOrStdout())
					fmt.Fprintln(cmd.OutOrStdout(), "---")
					fmt.Fprintln(cmd.OutOrStdout())
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", bold(r.Region))
				if r.Author != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Author: %s\n", r.Author)
				}
				if r.Posted != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Posted: %s\n", r.Posted)
				}
				if r.Summary != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Summary: %s\n", r.Summary)
				}
				if r.Content != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", r.Content)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagRegion, "region", "", "Specific region slug (defaults to all favorites)")
	cmd.Flags().BoolVar(&flagFull, "full", false, "Show full content without truncation")
	return cmd
}
