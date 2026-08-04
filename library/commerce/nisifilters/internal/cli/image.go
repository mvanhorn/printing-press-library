// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: image. Resolves the featured image of a post,
// page, or portfolio piece to its full-resolution source_url and size variants
// by joining featured_media against the local media mirror (live fallback when
// not synced). For products it reads the inline images array.

// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/internal/store"

	"github.com/spf13/cobra"
)

func newNovelImageCmd(flags *rootFlags) *cobra.Command {
	var typ string

	cmd := &cobra.Command{
		Use:   "image <id>",
		Short: "Resolve the featured image of a post, page, or product",
		Long: "Resolve the full-resolution image behind a content item. For posts, pages,\n" +
			"and pages this joins the item's featured_media id against the\n" +
			"local media mirror to return source_url plus available size variants. For\n" +
			"products it reads the inline images array.\n\n" +
			"Reads the local store first (run `sync`); falls back to live fetches.",
		Example: "  nisifilters-pp-cli image 123\n" +
			"  nisifilters-pp-cli image 123 --type portfolio\n" +
			"  nisifilters-pp-cli image 456 --type products --json",
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
			if typ == "" {
				typ = "posts"
			}

			resourcePath := map[string]string{
				"posts":    "/posts",
				"pages":    "/pages",
				"products": fgWooProductsURL,
			}
			apiPath, ok := resourcePath[typ]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type must be one of posts, pages, products"))
			}

			db, _ := fgOpenStore(cmd.Context())
			if db != nil {
				defer db.Close()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Load the source entity (store first, then live).
			source := fgLoadEntity(cmd, db, c, typ, apiPath, id)
			if source == nil {
				return notFoundErr(fmt.Errorf("no %s with id %s", typ, id))
			}

			view := map[string]any{
				"id":    id,
				"type":  typ,
				"title": fgPlainTitle(source),
				"link":  firstNonEmpty(fgString(source, "link"), fgString(source, "permalink")),
			}

			if typ == "products" {
				// WooCommerce products carry images inline.
				images := fgProductImages(source)
				if len(images) == 0 {
					return notFoundErr(fmt.Errorf("product %s has no images", id))
				}
				view["images"] = images
				view["source_url"] = images[0]["src"]
				return fgEmitImage(cmd, flags, view)
			}

			mediaID, ok := fgInt(source, "featured_media")
			if !ok || mediaID == 0 {
				return notFoundErr(fmt.Errorf("%s %s has no featured image", typ, id))
			}
			view["featured_media"] = mediaID

			media := fgLoadEntity(cmd, db, c, "media", "/media", strconv.Itoa(mediaID))
			if media == nil {
				return notFoundErr(fmt.Errorf("featured media %d not found", mediaID))
			}
			view["source_url"] = fgMediaSourceURL(media)
			view["alt_text"] = fgString(media, "alt_text")
			view["mime_type"] = fgString(media, "mime_type")
			if sizes := fgMediaSizes(media); len(sizes) > 0 {
				view["sizes"] = sizes
			}
			return fgEmitImage(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&typ, "type", "posts", "Source type: posts, pages, products")
	return cmd
}

// fgWooProductsURL is the absolute WooCommerce Store API products base; the
// generated client treats absolute paths without re-concatenating BaseURL.
const fgWooProductsURL = "https://www.nisifilters.it/wp-json/wc/store/v1/products"

// fgLoadEntity loads one entity, store first then live. db may be nil.
func fgLoadEntity(cmd *cobra.Command, db *store.Store, c *client.Client, resource, apiPath, id string) map[string]json.RawMessage {
	if db != nil {
		if raw, err := db.Get(resource, id); err == nil && len(raw) > 0 {
			return fgDecode(raw)
		}
	}
	raw, err := c.Get(cmd.Context(), apiPath+"/"+id, nil)
	if err != nil || len(raw) == 0 {
		return nil
	}
	m := fgDecode(raw)
	if _, ok := m["id"]; !ok {
		return nil
	}
	return m
}

// fgProductImages returns a WooCommerce product's images as [{src,alt}, ...].
func fgProductImages(m map[string]json.RawMessage) []map[string]string {
	raw, ok := m["images"]
	if !ok {
		return nil
	}
	var imgs []struct {
		Src string `json:"src"`
		Alt string `json:"alt"`
	}
	if err := json.Unmarshal(raw, &imgs); err != nil {
		return nil
	}
	out := make([]map[string]string, 0, len(imgs))
	for _, im := range imgs {
		if im.Src == "" {
			continue
		}
		out = append(out, map[string]string{"src": im.Src, "alt": im.Alt})
	}
	return out
}

func fgEmitImage(cmd *cobra.Command, flags *rootFlags, view map[string]any) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	w := cmd.OutOrStdout()
	if t, _ := view["title"].(string); t != "" {
		fmt.Fprintln(w, bold(t))
	}
	if u, _ := view["source_url"].(string); u != "" {
		fmt.Fprintln(w, u)
	}
	if sizes, ok := view["sizes"].(map[string]string); ok {
		for name, u := range sizes {
			fmt.Fprintf(w, "  %s\t%s\n", name, u)
		}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
