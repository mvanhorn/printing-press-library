// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: article, gallery, cook — non-recipe Allrecipes pages. These are
// thin extractors: title + canonical URL + recipe links inside the page.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newArticleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "article <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Extract metadata from an Allrecipes article page",
		Long: "Returns the article title, description, canonical URL, and any recipe\n" +
			"links the article references. Article body text extraction is best-effort\n" +
			"because Allrecipes pages are heavily templated — for full body text,\n" +
			"fetch the page directly and parse it yourself.",
		Example: "  allrecipes-pp-cli article https://www.allrecipes.com/article/some-slug/\n" +
			"  allrecipes-pp-cli article https://www.allrecipes.com/article/some-slug/ --agent",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path, err := pathFromURL(args[0])
			if err != nil {
				return err
			}
			body, err := recipes.FetchHTML(c, path)
			if err != nil {
				return classifyAPIError(err)
			}
			out := map[string]any{
				"url":         args[0],
				"title":       extractPageTitle(body),
				"description": extractMeta(body, "description"),
				"recipeLinks": recipes.ParseSearchResults(body, 50),
			}
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func newGalleryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gallery <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Extract recipe links from an Allrecipes round-up gallery",
		Example: "  allrecipes-pp-cli gallery https://www.allrecipes.com/gallery/best-summer-salads/ --agent",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path, err := pathFromURL(args[0])
			if err != nil {
				return err
			}
			body, err := recipes.FetchHTML(c, path)
			if err != nil {
				return classifyAPIError(err)
			}
			out := map[string]any{
				"url":     args[0],
				"title":   extractPageTitle(body),
				"recipes": recipes.ParseSearchResults(body, 100),
			}
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func newCookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cook <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show a cook profile and the recipes they've published",
		Example: "  allrecipes-pp-cli cook john-mitzewich --agent",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			path := slug
			if !strings.HasPrefix(path, "/") {
				path = "/cook/" + strings.Trim(slug, "/") + "/"
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err := recipes.FetchHTML(c, path)
			if err != nil {
				return classifyAPIError(err)
			}
			out := map[string]any{
				"slug":    slug,
				"name":    extractPageTitle(body),
				"recipes": recipes.ParseSearchResults(body, 50),
			}
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// pathFromURL strips the host from an absolute URL, returning a path the
// generated client can pass to Get. Returns an error if the URL is not on an
// allrecipes.com host.
func pathFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	if host != "" && host != "allrecipes.com" {
		return "", fmt.Errorf("expected allrecipes.com URL, got %s", u.Host)
	}
	path := u.Path
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	if path == "" {
		path = "/"
	}
	return path, nil
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>([^<]+)</title>`)
var metaContentRe = regexp.MustCompile(`(?is)<meta[^>]+name="([^"]+)"[^>]+content="([^"]*)"`)
var metaPropertyRe = regexp.MustCompile(`(?is)<meta[^>]+property="([^"]+)"[^>]+content="([^"]*)"`)

func extractPageTitle(body []byte) string {
	if m := titleRe.FindSubmatch(body); m != nil {
		return strings.TrimSpace(strings.ReplaceAll(string(m[1]), "&amp;", "&"))
	}
	return ""
}

func extractMeta(body []byte, key string) string {
	for _, re := range []*regexp.Regexp{metaContentRe, metaPropertyRe} {
		matches := re.FindAllSubmatch(body, -1)
		for _, m := range matches {
			if strings.EqualFold(string(m[1]), key) || strings.EqualFold(string(m[1]), "og:"+key) {
				return strings.TrimSpace(string(m[2]))
			}
		}
	}
	return ""
}
