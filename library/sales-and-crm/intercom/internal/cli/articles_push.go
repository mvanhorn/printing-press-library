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

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/internal/cliutil"
	"github.com/spf13/cobra"
)

func newArticlesPushCmd(flags *rootFlags) *cobra.Command {
	var from string
	var localesCSV string
	var dryRunOnly bool

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push locally-edited article markdown back to Intercom (diff-driven PATCH)",
		Example: strings.Trim(`
  # Show which articles changed without PATCHing
  intercom-pp-cli articles push --from ./articles --dry-run-only

  # Push every changed article
  intercom-pp-cli articles push --from ./articles
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || from == "" {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would push (verify mode)")
				return nil
			}
			if _, err := os.Stat(from); err != nil {
				return notFoundErr(fmt.Errorf("--from %s does not exist: %w", from, err))
			}
			localeFilter := parseLocaleFilter(localesCSV)

			manifestPath := filepath.Join(from, "manifest.json")
			mfData, err := os.ReadFile(manifestPath)
			if err != nil {
				return notFoundErr(fmt.Errorf("reading manifest: %w", err))
			}
			var mf articleManifest
			if err := json.Unmarshal(mfData, &mf); err != nil {
				return apiErr(fmt.Errorf("parsing manifest: %w", err))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			changed := 0
			dryShown := 0
			for _, entry := range mf.Articles {
				perLocale := map[string]articleLocaleT{}
				topTitle := entry.Title
				topBody := ""
				localesChanged := []string{}
				for _, file := range entry.Files {
					locale := localeFromFilename(file)
					if localeFilter != nil && !localeFilter[locale] {
						continue
					}
					fpath := filepath.Join(from, file)
					raw, err := os.ReadFile(fpath)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: read %s: %v\n", file, err)
						continue
					}
					// PATCH(articles-checksum-of-file): compare raw file bytes
					// against the pull-time checksum. Same byte sequence pull
					// wrote = no edit = skip the PATCH. See articles_pull.go for
					// the writer side of this contract.
					sum := sha256.Sum256(raw)
					newSum := hex.EncodeToString(sum[:])
					if newSum == entry.Checksums[locale] {
						continue
					}
					meta, mdBody := splitFrontmatter(string(raw))
					html := markdownToHTML(mdBody)
					localesChanged = append(localesChanged, locale)
					if locale == entry.DefaultLocale || (entry.DefaultLocale == "" && len(perLocale) == 0) {
						topTitle = meta["title"]
						topBody = html
					}
					perLocale[locale] = articleLocaleT{
						Type:        "article_content",
						Title:       meta["title"],
						Description: meta["description"],
						Body:        html,
					}
				}
				if len(localesChanged) == 0 {
					continue
				}

				if dryRunOnly {
					dryShown++
					// PATCH(articles-push-progress-to-stderr): progress lines go
					// to stderr so the final JSON envelope on stdout stays
					// parseable. Mirrors the pattern used by `patched: ...`
					// below (already on stderr).
					fmt.Fprintf(cmd.ErrOrStderr(), "would patch: %s (locales: %s)\n", entry.ID, strings.Join(localesChanged, ","))
					continue
				}

				body := map[string]any{}
				if topTitle != "" {
					body["title"] = topTitle
				}
				if topBody != "" {
					body["body"] = topBody
				}
				if len(perLocale) > 0 {
					body["translated_content"] = perLocale
				}
				path := "/articles/" + entry.ID
				_, _, err = c.Patch(cmd.Context(), path, body)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: patch %s failed: %v\n", entry.ID, err)
					continue
				}
				changed++
				fmt.Fprintf(cmd.ErrOrStderr(), "patched: %s\n", entry.ID)
			}

			if flags.asJSON || flags.agent {
				// PATCH(articles-push-json-envelope): emit a JSON envelope when
				// --json is set so agents and the dogfood matrix can parse the
				// result. Mirrors articles pull's envelope shape.
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"changed_articles":      changed,
					"dry_run_only_articles": dryShown,
					"source_dir":            from,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pushed %d changed articles (%d dry-run-only)\n", changed, dryShown)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source directory written by 'articles pull' (required)")
	cmd.Flags().StringVar(&localesCSV, "locale", "", "CSV of locales to push; empty means all")
	cmd.Flags().BoolVar(&dryRunOnly, "dry-run-only", false, "Print would-patch list without actually patching")
	return cmd
}

func localeFromFilename(name string) string {
	// "<id>-<slug>.<locale>.md"
	base := strings.TrimSuffix(name, ".md")
	if i := strings.LastIndex(base, "."); i >= 0 {
		return base[i+1:]
	}
	return "en"
}

// splitFrontmatter returns the YAML frontmatter as a map[string]string (best
// effort: quoted-string scalars only) and the markdown body.
func splitFrontmatter(s string) (map[string]string, string) {
	out := map[string]string{}
	if !strings.HasPrefix(s, "---") {
		return out, s
	}
	end := strings.Index(s[3:], "\n---")
	if end < 0 {
		return out, s
	}
	headerEnd := 3 + end
	header := s[3:headerEnd]
	body := s[headerEnd+4:]
	body = strings.TrimLeft(body, "\n")
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// PATCH(articles-frontmatter-unescape): articles_pull.go writes
		// frontmatter values with `%q` formatting, which escapes embedded
		// double-quotes as `\"` and backslashes as `\\`. Just stripping the
		// outer quotes here without un-escaping corrupts any title or
		// description that contains a `"` — the pushed value would land in
		// Intercom with a literal `\"` in the middle.
		if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
			inner := v[1 : len(v)-1]
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			v = inner
		}
		out[k] = v
	}
	return out, body
}

// markdownToHTML is the lossy inverse of htmlToMarkdown.
func markdownToHTML(md string) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	lines := strings.Split(md, "\n")
	var out []string
	inCode := false
	var ulBuf []string
	flushUL := func() {
		if len(ulBuf) == 0 {
			return
		}
		var sb strings.Builder
		sb.WriteString("<ul>")
		for _, item := range ulBuf {
			sb.WriteString("<li>")
			sb.WriteString(item)
			sb.WriteString("</li>")
		}
		sb.WriteString("</ul>")
		out = append(out, sb.String())
		ulBuf = nil
	}
	for _, raw := range lines {
		line := raw
		if strings.HasPrefix(line, "```") {
			flushUL()
			if inCode {
				out = append(out, "</pre>")
				inCode = false
			} else {
				out = append(out, "<pre>")
				inCode = true
			}
			continue
		}
		if inCode {
			out = append(out, escapeHTML(line))
			continue
		}
		trim := strings.TrimSpace(line)
		// Headings.
		if h := headingLevel(trim); h > 0 {
			flushUL()
			text := strings.TrimSpace(strings.TrimLeft(trim, "#"))
			out = append(out, fmt.Sprintf("<h%d>%s</h%d>", h, inlineMD(text), h))
			continue
		}
		// Bullet items.
		if strings.HasPrefix(trim, "- ") {
			ulBuf = append(ulBuf, inlineMD(strings.TrimPrefix(trim, "- ")))
			continue
		}
		if trim == "" {
			flushUL()
			out = append(out, "")
			continue
		}
		flushUL()
		out = append(out, "<p>"+inlineMD(trim)+"</p>")
	}
	flushUL()
	return strings.Join(out, "")
}

func headingLevel(s string) int {
	n := 0
	for _, r := range s {
		if r == '#' {
			n++
			continue
		}
		break
	}
	if n == 0 || n > 6 {
		return 0
	}
	if len(s) <= n || s[n] != ' ' {
		return 0
	}
	return n
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

var mdBoldRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)
var mdItalicRe = regexp.MustCompile(`(^|[^\*])\*([^*]+)\*`)
var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var mdCodeRe = regexp.MustCompile("`([^`]+)`")

func inlineMD(s string) string {
	s = mdLinkRe.ReplaceAllString(s, `<a href="$2">$1</a>`)
	s = mdBoldRe.ReplaceAllString(s, `<strong>$1</strong>`)
	s = mdItalicRe.ReplaceAllString(s, `$1<em>$2</em>`)
	s = mdCodeRe.ReplaceAllString(s, `<code>$1</code>`)
	return s
}
