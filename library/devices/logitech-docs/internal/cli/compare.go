// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command — implemented from the absorb manifest transcendence row
// "Spec comparison".
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/internal/cliutil"
	"github.com/spf13/cobra"
)

var (
	trRe   = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	cellRe = regexp.MustCompile(`(?s)<(t[dh])[^>]*>(.*?)</t[dh]>`)
	tagRe  = regexp.MustCompile(`<[^>]+>`)
)

// stripHTMLCell removes tags and decodes HTML entities, then trims whitespace.
func stripHTMLCell(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = cliutil.CleanText(s)
	return strings.TrimSpace(s)
}

// extractSpecPairs pulls key/value pairs out of an HTML spec-sheet body.
// Each table row's first cell is the key and second cell the value; repeated
// keys are appended so multi-column tables keep every measurement.
// Header rows (every cell is a <th>) carry column labels rather than specs, so
// they are skipped — otherwise "Component"/"Model Number (M/N)" would compare
// as if it were a real measurement.
func extractSpecPairs(body string) map[string][]string {
	out := map[string][]string{}
	for _, row := range trRe.FindAllStringSubmatch(body, -1) {
		cells := cellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 2 {
			continue
		}
		headerRow := true
		for _, cell := range cells {
			if cell[1] != "th" {
				headerRow = false
				break
			}
		}
		if headerRow {
			continue
		}
		key := stripHTMLCell(cells[0][2])
		val := stripHTMLCell(cells[1][2])
		if key == "" || val == "" {
			continue
		}
		out[key] = append(out[key], val)
	}
	return out
}

type compareRow struct {
	Spec string `json:"spec"`
	A    string `json:"a"`
	B    string `json:"b"`
}

type compareResult struct {
	ProductA string       `json:"product_a"`
	ProductB string       `json:"product_b"`
	Rows     []compareRow `json:"rows"`
	Note     string       `json:"note,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "compare <product-a> <product-b>",
		Short: "Side-by-side spec comparison between two products from synced spec sheets.",
		Long: "Searches each product's spec sheet (webcontent=productspecs), extracts the spec tables, and " +
			"shows side-by-side values for the specs both products document.",
		Example: "  logitech-docs-pp-cli compare \"MeetUp\" \"Rally Bar\"\n" +
			"  logitech-docs-pp-cli compare \"MeetUp\" \"Brio\" --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "a=MeetUp;b=Rally Bar"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare specs")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare requires two product names, e.g. %s compare \"MeetUp\" \"Rally Bar\"", cmd.CommandPath()))
			}
			productA := args[0]
			productB := strings.Join(args[1:], " ")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			res := compareResult{
				ProductA: productA,
				ProductB: productB,
				Rows:     make([]compareRow, 0),
			}

			specA, errA := fetchTopSpec(c, ctx, productA)
			specB, errB := fetchTopSpec(c, ctx, productB)
			if errA != nil {
				res.Note = errA.Error()
			} else if errB != nil {
				res.Note = errB.Error()
			} else {
				pairsA := extractSpecPairs(specA)
				pairsB := extractSpecPairs(specB)
				for key, valsA := range pairsA {
					valsB, ok := pairsB[key]
					if !ok {
						continue
					}
					res.Rows = append(res.Rows, compareRow{Spec: key, A: strings.Join(valsA, "; "), B: strings.Join(valsB, "; ")})
				}
				if len(res.Rows) == 0 {
					res.Note = "no common spec fields found between the two products"
				}
			}
			if limit > 0 && len(res.Rows) > limit {
				res.Rows = res.Rows[:limit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), res, flags, "live")
			}
			if res.Note != "" && len(res.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			table := make([]map[string]any, 0, len(res.Rows))
			for _, r := range res.Rows {
				table = append(table, map[string]any{"spec": r.Spec, "a": r.A, "b": r.B})
			}
			if err := printAutoTable(cmd.OutOrStdout(), table); err != nil {
				return err
			}
			if res.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum spec rows to show (0 = all)")
	return cmd
}

// fetchTopSpec searches a product's spec sheets and returns the body of the
// first result that actually contains spec tables.
func fetchTopSpec(c *client.Client, ctx context.Context, product string) (string, error) {
	data, err := c.Get(ctx, "/api/v2/help_center/articles/search.json", map[string]string{
		"query":       product,
		"label_names": "webcontent=productspecs",
		"per_page":    "5",
	})
	if err != nil {
		return "", err
	}
	var env struct {
		Results []struct {
			ID int64 `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("parsing search: %w", err)
	}
	if len(env.Results) == 0 {
		return "", fmt.Errorf("no spec sheet found for %q", product)
	}
	for _, r := range env.Results {
		body, err := c.Get(ctx, fmt.Sprintf("/api/v2/help_center/en-us/articles/%d.json", r.ID), nil)
		if err != nil {
			continue
		}
		var article struct {
			Article struct {
				Body string `json:"body"`
			} `json:"article"`
		}
		if err := json.Unmarshal(body, &article); err != nil {
			continue
		}
		if len(extractSpecPairs(article.Article.Body)) > 0 {
			return article.Article.Body, nil
		}
	}
	return "", fmt.Errorf("no technical specifications available for %q", product)
}
