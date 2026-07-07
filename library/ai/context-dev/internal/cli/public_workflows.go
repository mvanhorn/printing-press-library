// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type searchResult struct {
	Title   string
	URL     string
	Snippet string
	Rank    int
}

func newScrapeCmd(flags *rootFlags) *cobra.Command {
	params := func(arg string) (map[string]string, error) {
		if _, err := normalizeURLArg(arg); err != nil {
			return nil, err
		}
		return map[string]string{"url": arg}, nil
	}
	return &cobra.Command{
		Use:     "scrape <url>",
		Short:   "Scrape a URL to LLM-ready Markdown (1 credit)",
		Example: "  context-dev-pp-cli scrape https://example.com/page --json",
		Args:    oneArgGetArgs(flags, "scrape", params),
		RunE:    runOneArgGet(flags, "/web/scrape/markdown", params),
	}
}

func newStyleguideCmd(flags *rootFlags) *cobra.Command {
	params := func(arg string) (map[string]string, error) {
		domain, err := normalizeDomainArg(arg)
		if err != nil {
			return nil, err
		}
		return map[string]string{"domain": domain}, nil
	}
	return &cobra.Command{
		Use:     "styleguide <domain|url>",
		Short:   "Extract a website design system (10 credits)",
		Example: "  context-dev-pp-cli styleguide example.com --json",
		Args:    oneArgGetArgs(flags, "styleguide", params),
		RunE:    runOneArgGet(flags, "/web/styleguide", params),
	}
}

func newScreenshotCmd(flags *rootFlags) *cobra.Command {
	params := func(arg string) (map[string]string, error) {
		if strings.Contains(arg, "://") {
			if _, err := normalizeURLArg(arg); err != nil {
				return nil, err
			}
			return map[string]string{"directUrl": arg}, nil
		}
		domain, err := normalizeDomainArg(arg)
		if err != nil {
			return nil, err
		}
		return map[string]string{"domain": domain}, nil
	}
	return &cobra.Command{
		Use:     "screenshot <domain|url>",
		Short:   "Capture a website screenshot (5 credits)",
		Example: "  context-dev-pp-cli screenshot example.com --json",
		Args:    oneArgGetArgs(flags, "screenshot", params),
		RunE:    runOneArgGet(flags, "/web/screenshot", params),
	}
}

func newCrawlCmd(flags *rootFlags) *cobra.Command {
	var maxPages int
	var confirm bool
	var estimate bool

	cmd := &cobra.Command{
		Use:     "crawl <seed>",
		Short:   "Crawl same-domain pages to Markdown (default max 5 pages)",
		Example: "  context-dev-pp-cli crawl https://example.com --max-pages 5 --json\n  context-dev-pp-cli crawl https://example.com --max-pages 30 --estimate",
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("crawl requires a seed URL"))
			}
			_, err := normalizeURLArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if maxPages < 1 {
				return usageErr(fmt.Errorf("--max-pages must be at least 1"))
			}
			if estimate {
				fmt.Fprintf(cmd.OutOrStdout(), "estimated credits: %d (up to %d crawled pages)\n", maxPages, maxPages)
				return nil
			}
			if maxPages > 25 && !confirm && !flags.yes {
				return usageErr(fmt.Errorf("--confirm or --yes is required above 25 pages"))
			}
			u, _ := normalizeURLArg(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"url":      args[0],
				"maxPages": maxPages,
				"urlRegex": exactHostRegex(u.Hostname()),
			}
			data, _, err := c.PostWithParams(cmd.Context(), "/web/crawl", nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return writeRawJSON(cmd, flags, data)
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum same-domain pages to crawl")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm crawls above 25 pages")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Print estimated credit cost without calling the API")
	return cmd
}

func newExtractCmd(flags *rootFlags) *cobra.Command {
	var schemaPath string
	cmd := &cobra.Command{
		Use:     "extract <url> --schema <file.json>",
		Short:   "Extract typed JSON from a URL using a JSON Schema (10 credits)",
		Example: "  context-dev-pp-cli extract https://example.com --schema schema.json --json",
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("extract requires a URL"))
			}
			if _, err := normalizeURLArg(args[0]); err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(schemaPath) == "" {
				return usageErr(fmt.Errorf("--schema is required"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			schema, err := readJSONFile(schemaPath)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostWithParams(cmd.Context(), "/web/extract", nil, map[string]any{"url": args[0], "schema": schema})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return writeRawJSON(cmd, flags, data)
		},
	}
	cmd.Flags().StringVar(&schemaPath, "schema", "", "Path to a JSON Schema file")
	return cmd
}

func oneArgGetArgs(flags *rootFlags, name string, params func(string) (map[string]string, error)) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if flags.dryRun {
			return nil
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("%s requires exactly one argument", name))
		}
		if _, err := params(args[0]); err != nil {
			return usageErr(err)
		}
		return nil
	}
}

func runOneArgGet(flags *rootFlags, path string, params func(string) (map[string]string, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if flags.dryRun {
			return nil
		}
		query, err := params(args[0])
		if err != nil {
			return usageErr(err)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		data, err := c.GetNoCache(cmd.Context(), path, query)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return writeRawJSON(cmd, flags, data)
	}
}

func writeRawJSON(cmd *cobra.Command, flags *rootFlags, data json.RawMessage) error {
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func writeJSONPayload(cmd *cobra.Command, flags *rootFlags, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeRawJSON(cmd, flags, data)
}

func normalizeURLArg(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("value must be an absolute http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL must use http or https")
	}
	return parsed, nil
}

func normalizeDomainArg(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("domain is required")
	}
	if strings.Contains(raw, "://") {
		u, err := normalizeURLArg(raw)
		if err != nil {
			return "", err
		}
		return u.Hostname(), nil
	}
	if strings.ContainsAny(raw, "/?#") || !strings.Contains(raw, ".") {
		return "", fmt.Errorf("value must be a domain or URL")
	}
	return raw, nil
}

func exactHostRegex(host string) string {
	return `^https?://` + regexp.QuoteMeta(host) + `(?::[0-9]+)?(?:/|$)`
}

func readJSONFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema: %w", err)
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", err)
	}
	return parsed, nil
}

func splitCommaValues(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstArray(v any, keys ...string) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range keys {
		if arr, ok := obj[key].([]any); ok {
			return arr
		}
	}
	return nil
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := obj[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstLogo(obj map[string]any) string {
	for _, key := range []string{"logo", "icon", "image"} {
		if v := firstString(obj, key); v != "" {
			return v
		}
	}
	for _, key := range []string{"logos", "images"} {
		if arr, ok := obj[key].([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					return s
				}
				if m, ok := item.(map[string]any); ok {
					if v := firstString(m, "url", "src"); v != "" {
						return v
					}
				}
			}
		}
	}
	return ""
}

func domainFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
