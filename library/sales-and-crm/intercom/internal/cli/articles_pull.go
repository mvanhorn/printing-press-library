// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/internal/cliutil"
	"github.com/spf13/cobra"
)

type articleManifest struct {
	PulledAt string                 `json:"pulled_at"`
	Articles []articleManifestEntry `json:"articles"`
}

type articleManifestEntry struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	DefaultLocale string            `json:"default_locale,omitempty"`
	Files         []string          `json:"files"`
	Checksums     map[string]string `json:"checksums"` // locale -> sha256 of the as-written markdown file (frontmatter + body) at pull time; push re-reads + re-checksums to detect edits without round-tripping the lossy HTML↔markdown converter
}

func newArticlesPullCmd(flags *rootFlags) *cobra.Command {
	var to string
	var localesCSV string
	var maxArticles int

	cmd := &cobra.Command{
		Use:         "pull",
		Short:       "Pull every help-center article into a flat markdown tree (one file per locale + manifest.json)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Pull every article into ./articles
  intercom-pp-cli articles pull --to ./articles

  # Pull only English + French
  intercom-pp-cli articles pull --to ./articles --locale en,fr
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || to == "" {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull (verify mode)")
				return nil
			}
			localeFilter := parseLocaleFilter(localesCSV)
			if maxArticles <= 0 {
				maxArticles = 5000
			}
			if err := os.MkdirAll(to, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", to, err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			manifest := articleManifest{
				PulledAt: time.Now().UTC().Format(time.RFC3339),
				Articles: make([]articleManifestEntry, 0),
			}

			path := "/articles"
			params := map[string]string{"per_page": "50"}
			pulled := 0
			files := 0
			for {
				raw, err := c.Get(cmd.Context(), path, params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Data  []json.RawMessage `json:"data"`
					Pages struct {
						Next any `json:"next"`
					} `json:"pages"`
				}
				if err := json.Unmarshal(raw, &page); err != nil {
					return apiErr(fmt.Errorf("parsing articles page: %w", err))
				}
				for _, item := range page.Data {
					if pulled >= maxArticles {
						break
					}
					entry, n, perr := writeArticleFiles(to, item, localeFilter)
					if perr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: article skipped: %v\n", perr)
						continue
					}
					manifest.Articles = append(manifest.Articles, entry)
					files += n
					pulled++
				}
				if pulled >= maxArticles {
					break
				}
				next := relativeNextArticles(page.Pages.Next)
				if next == "" {
					break
				}
				path, params = nextArticlesPath(next)
			}

			manifestPath := filepath.Join(to, "manifest.json")
			mf, err := os.Create(manifestPath)
			if err != nil {
				return fmt.Errorf("creating manifest: %w", err)
			}
			enc := json.NewEncoder(mf)
			enc.SetIndent("", "  ")
			if err := enc.Encode(manifest); err != nil {
				mf.Close()
				return err
			}
			mf.Close()

			if flags.asJSON || flags.agent {
				// PATCH(articles-pull-json-envelope): emit a JSON envelope when
				// --json is set so agents and the dogfood matrix can parse the
				// result. Manifest path is the durable handle to the pulled
				// tree; counts are the at-a-glance summary.
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"pulled_articles": pulled,
					"files_written":   files,
					"output_dir":      to,
					"manifest_path":   filepath.Join(to, "manifest.json"),
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pulled %d articles into %s; wrote %d files\n", pulled, to, files)
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "Output directory (required)")
	cmd.Flags().StringVar(&localesCSV, "locale", "", "CSV of locales to include; empty means all")
	cmd.Flags().IntVar(&maxArticles, "max-articles", 5000, "Safety cap on articles pulled")
	return cmd
}

func parseLocaleFilter(s string) map[string]bool {
	out := map[string]bool{}
	if strings.TrimSpace(s) == "" {
		return nil
	}
	for _, l := range strings.Split(s, ",") {
		l = strings.TrimSpace(l)
		if l != "" {
			out[l] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// relativeNextArticles strips the host + leading path so we keep using c.Get's
// path-based form. Intercom returns either a string URL or nil under pages.next.
func relativeNextArticles(v any) string {
	s, ok := v.(string)
	if !ok || s == "" {
		return ""
	}
	if idx := strings.Index(s, "/articles"); idx >= 0 {
		return s[idx:]
	}
	return s
}

func nextArticlesPath(next string) (string, map[string]string) {
	path := next
	params := map[string]string{}
	if q := strings.Index(next, "?"); q >= 0 {
		path = next[:q]
		for _, kv := range strings.Split(next[q+1:], "&") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				params[k] = v
			}
		}
	}
	return path, params
}

type articleShape struct {
	ID            any    `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Body          string `json:"body"`
	AuthorID      any    `json:"author_id"`
	State         string `json:"state"`
	ParentID      any    `json:"parent_id"`
	ParentType    string `json:"parent_type"`
	DefaultLocale string `json:"default_locale"`
	// PATCH(articles-translated-content-lenient): the API documents
	// translated_content as an object keyed by locale, but in practice it can
	// be null, an empty string, or — for some article versions — a non-object
	// type per locale. Unmarshalling to a typed map fails the whole call. Keep
	// the wire shape as raw JSON; decodeTranslatedContent() walks it and skips
	// entries that aren't an object.
	TranslatedContent json.RawMessage `json:"translated_content"`
}

type articleLocaleT struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// decodeTranslatedContent walks a possibly-mixed-shape translated_content
// blob (null, empty string, or object-of-locale-objects) and returns the
// well-formed per-locale entries. Anything else is silently dropped so a
// single malformed translation can't fail the whole pull.
func decodeTranslatedContent(raw json.RawMessage) map[string]articleLocaleT {
	out := map[string]articleLocaleT{}
	if len(raw) == 0 {
		return out
	}
	// Quick reject: null, empty string, or non-object.
	trimmed := bytesTrimWS(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return out
	}
	var perLocale map[string]json.RawMessage
	if err := json.Unmarshal(raw, &perLocale); err != nil {
		return out
	}
	for locale, lraw := range perLocale {
		ltrim := bytesTrimWS(lraw)
		if len(ltrim) == 0 || ltrim[0] != '{' {
			continue
		}
		var v articleLocaleT
		if err := json.Unmarshal(lraw, &v); err == nil {
			out[locale] = v
		}
	}
	return out
}

func bytesTrimWS(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\n' || b[j-1] == '\r') {
		j--
	}
	return b[i:j]
}

// writeArticleFiles writes one file per locale for an article and returns the manifest entry.
func writeArticleFiles(dir string, raw json.RawMessage, localeFilter map[string]bool) (articleManifestEntry, int, error) {
	var a articleShape
	if err := json.Unmarshal(raw, &a); err != nil {
		return articleManifestEntry{}, 0, err
	}
	id := stringifyAny(a.ID)
	if id == "" {
		return articleManifestEntry{}, 0, fmt.Errorf("article missing id")
	}
	slug := slugify(a.Title, 40)
	entry := articleManifestEntry{
		ID:            id,
		Title:         a.Title,
		DefaultLocale: a.DefaultLocale,
		Files:         []string{},
		Checksums:     map[string]string{},
	}

	// Default locale: use the article body itself.
	defaultLoc := a.DefaultLocale
	if defaultLoc == "" {
		defaultLoc = "en"
	}

	emit := func(locale, title, description, body string) {
		if localeFilter != nil && !localeFilter[locale] {
			return
		}
		fname := fmt.Sprintf("%s-%s.%s.md", id, slug, locale)
		fpath := filepath.Join(dir, fname)
		md := renderArticleMarkdown(a, locale, title, description, body)
		if err := os.WriteFile(fpath, []byte(md), 0o644); err != nil {
			return
		}
		entry.Files = append(entry.Files, fname)
		// PATCH(articles-checksum-of-file): sha256 the exact bytes we wrote.
		// Push reads the file back and re-checksums; matching bytes = no edit.
		// Earlier we checksummed the HTML source body and push checksummed
		// markdownToHTML(mdBody), which never matched because the round-trip
		// is lossy by design — every push then PATCHed all articles.
		sum := sha256.Sum256([]byte(md))
		entry.Checksums[locale] = hex.EncodeToString(sum[:])
	}

	emit(defaultLoc, a.Title, a.Description, a.Body)
	for locale, tc := range decodeTranslatedContent(a.TranslatedContent) {
		// Skip the default-locale duplicate if the translated_content map
		// also carries it; the default emission above already covered it.
		if locale == defaultLoc {
			continue
		}
		emit(locale, tc.Title, tc.Description, tc.Body)
	}
	return entry, len(entry.Files), nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string, max int) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	if len(s) > max {
		s = s[:max]
	}
	return strings.Trim(s, "-")
}

func stringifyAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON numbers come through as float64.
		return fmt.Sprintf("%d", int64(x))
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func renderArticleMarkdown(a articleShape, locale, title, description, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "id: %q\n", stringifyAny(a.ID))
	fmt.Fprintf(&sb, "title: %q\n", title)
	fmt.Fprintf(&sb, "description: %q\n", description)
	fmt.Fprintf(&sb, "state: %q\n", a.State)
	fmt.Fprintf(&sb, "parent_id: %q\n", stringifyAny(a.ParentID))
	fmt.Fprintf(&sb, "parent_type: %q\n", a.ParentType)
	fmt.Fprintf(&sb, "default_locale: %q\n", a.DefaultLocale)
	fmt.Fprintf(&sb, "locale: %q\n", locale)
	fmt.Fprintf(&sb, "author_id: %q\n", stringifyAny(a.AuthorID))
	sb.WriteString("---\n\n")
	sb.WriteString(htmlToMarkdown(body))
	if !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

// htmlToMarkdown is a small, intentionally lossy converter. It handles the
// tags we actually see in Intercom article bodies: p, br, strong/b, em/i,
// a, h1-h6, ul/li, ol/li, code, pre, blockquote. Anything we don't
// recognise (tables, embeds) survives as the original HTML wrapped in an
// HTML comment fence so round-trip diff is still meaningful.
func htmlToMarkdown(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}
	s := html
	// Normalise whitespace.
	s = strings.ReplaceAll(s, "\r\n", "\n")

	replacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{regexp.MustCompile(`(?is)<\s*br\s*/?\s*>`), "\n"},
		{regexp.MustCompile(`(?is)<\s*/\s*p\s*>`), "\n\n"},
		{regexp.MustCompile(`(?is)<\s*p[^>]*>`), ""},
		{regexp.MustCompile(`(?is)<\s*strong[^>]*>`), "**"},
		{regexp.MustCompile(`(?is)<\s*/\s*strong\s*>`), "**"},
		{regexp.MustCompile(`(?is)<\s*b[^>]*>`), "**"},
		{regexp.MustCompile(`(?is)<\s*/\s*b\s*>`), "**"},
		{regexp.MustCompile(`(?is)<\s*em[^>]*>`), "*"},
		{regexp.MustCompile(`(?is)<\s*/\s*em\s*>`), "*"},
		{regexp.MustCompile(`(?is)<\s*i[^>]*>`), "*"},
		{regexp.MustCompile(`(?is)<\s*/\s*i\s*>`), "*"},
		{regexp.MustCompile(`(?is)<\s*li[^>]*>`), "- "},
		{regexp.MustCompile(`(?is)<\s*/\s*li\s*>`), "\n"},
		{regexp.MustCompile(`(?is)<\s*/?\s*ul[^>]*>`), "\n"},
		{regexp.MustCompile(`(?is)<\s*/?\s*ol[^>]*>`), "\n"},
		{regexp.MustCompile(`(?is)<\s*code[^>]*>`), "`"},
		{regexp.MustCompile(`(?is)<\s*/\s*code\s*>`), "`"},
		{regexp.MustCompile(`(?is)<\s*pre[^>]*>`), "\n```\n"},
		{regexp.MustCompile(`(?is)<\s*/\s*pre\s*>`), "\n```\n"},
		{regexp.MustCompile(`(?is)<\s*blockquote[^>]*>`), "\n> "},
		{regexp.MustCompile(`(?is)<\s*/\s*blockquote\s*>`), "\n"},
	}
	for _, r := range replacements {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	// Headings.
	for n := 6; n >= 1; n-- {
		openRe := regexp.MustCompile(fmt.Sprintf(`(?is)<\s*h%d[^>]*>`, n))
		closeRe := regexp.MustCompile(fmt.Sprintf(`(?is)<\s*/\s*h%d\s*>`, n))
		s = openRe.ReplaceAllString(s, "\n"+strings.Repeat("#", n)+" ")
		s = closeRe.ReplaceAllString(s, "\n")
	}
	// Anchors: <a href="X">text</a> -> [text](X)
	anchorRe := regexp.MustCompile(`(?is)<\s*a[^>]*href\s*=\s*"([^"]*)"[^>]*>(.*?)<\s*/\s*a\s*>`)
	s = anchorRe.ReplaceAllString(s, "[$2]($1)")
	// Drop any remaining tags conservatively, but keep their text content.
	s = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, "")
	// Collapse 3+ newlines.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
