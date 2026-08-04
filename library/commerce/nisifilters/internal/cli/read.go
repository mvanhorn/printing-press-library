// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: read. Prints one post, page, or product from
// the local mirror as title + plain-text content + key meta, stripping the
// WordPress response envelope (yoast_head, jetpack_*, class_list, _links). Falls
// back to a live fetch when the item is not in the local store.

// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type fgReadType struct {
	resource string
	apiPath  string
}

// fgReadTypes maps the --type / store resource_type to its live API path
// prefix. products resolves through the absolute WooCommerce Store API base
// (the client passes absolute URLs through untouched).
var fgReadTypes = []fgReadType{
	{"posts", "/posts"},
	{"pages", "/pages"},
	{"products", fgWooProductsURL},
}

func newNovelReadCmd(flags *rootFlags) *cobra.Command {
	var typ string
	var asHTML bool

	cmd := &cobra.Command{
		Use:   "read <id>",
		Short: "Print one post, page, or product as title plus plain-text content and key meta",
		Long: "Read one post, page, or shop product from the local mirror as a clean,\n" +
			"bounded view — title, plain-text content, and key meta — dropping the\n" +
			"WordPress envelope (yoast_head, jetpack_*, class_list, _links). Products\n" +
			"resolve via the WooCommerce Store API and include price and stock.\n\n" +
			"Reads the local store first (run `sync`); falls back to a live fetch when\n" +
			"the item is not mirrored. Add --html to keep the rendered markup.",
		Example: "  nisifilters-pp-cli read 123\n" +
			"  nisifilters-pp-cli read 123 --type pages\n" +
			"  nisifilters-pp-cli read 278495 --type products\n" +
			"  nisifilters-pp-cli read 123 --json --html",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an item id is required"))
			}
			id := args[0]
			if _, err := strconv.Atoi(id); err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id must be numeric, got %q", id))
			}

			candidates := fgReadTypes
			if typ != "" {
				match := fgReadType{}
				for _, t := range fgReadTypes {
					if t.resource == typ {
						match = t
						break
					}
				}
				if match.resource == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--type must be one of posts, pages, products"))
				}
				candidates = []fgReadType{match}
			}

			// 1) Local store lookup.
			var obj map[string]json.RawMessage
			var matchedType string
			if db, err := fgOpenStore(cmd.Context()); err == nil {
				defer db.Close()
				for _, c := range candidates {
					if raw, gerr := db.Get(c.resource, id); gerr == nil && len(raw) > 0 {
						obj = fgDecode(raw)
						matchedType = c.resource
						break
					}
				}
			}

			// 2) Live fallback.
			if obj == nil {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				for _, cand := range candidates {
					raw, gerr := c.Get(cmd.Context(), cand.apiPath+"/"+id, nil)
					if gerr != nil || len(raw) == 0 {
						continue
					}
					m := fgDecode(raw)
					if _, ok := m["id"]; ok {
						obj = m
						matchedType = cand.resource
						break
					}
				}
			}

			if obj == nil {
				return notFoundErr(fmt.Errorf("no post, page, or product with id %s", id))
			}

			// WooCommerce Store API products use flat fields (description,
			// short_description, permalink) instead of the WP post envelope.
			contentKey, excerptKey, linkKey := "content", "excerpt", "link"
			if matchedType == "products" {
				contentKey, excerptKey, linkKey = "description", "short_description", "permalink"
			}
			rawContent := fgRendered(obj, contentKey)
			content := rawContent
			if !asHTML {
				content = fgStripHTML(rawContent)
			}
			idVal := any(id)
			if n, ok := fgInt(obj, "id"); ok {
				idVal = n
			}
			view := map[string]any{
				"id":       idVal,
				"type":     matchedType,
				"title":    fgPlainTitle(obj),
				"slug":     fgString(obj, "slug"),
				"link":     fgString(obj, linkKey),
				"date":     fgString(obj, "date"),
				"modified": fgString(obj, "modified"),
				"excerpt":  fgStripHTML(fgRendered(obj, excerptKey)),
				"content":  content,
			}
			if matchedType == "products" {
				view["stock"] = fgStockStatus(obj)
				if disp, _, ok := fgWooPrice(obj["prices"]); ok {
					view["price"] = disp
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, bold(fgPlainTitle(obj)))
			if matchedType == "products" {
				price, _, _ := fgWooPrice(obj["prices"])
				fmt.Fprintf(w, "%s · %s · %s · %s\n\n", matchedType, price, fgStockStatus(obj), fgString(obj, linkKey))
			} else {
				fmt.Fprintf(w, "%s · %s · %s\n\n", matchedType, fgString(obj, "date"), fgString(obj, linkKey))
			}
			fmt.Fprintln(w, content)
			return nil
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "Restrict lookup to one type: posts, pages, products")
	cmd.Flags().BoolVar(&asHTML, "html", false, "Keep rendered HTML instead of stripping to plain text")
	return cmd
}
