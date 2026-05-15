// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

package odoo

import (
	"regexp"
	"strings"
)

var (
	reTag       = regexp.MustCompile(`<[^>]+>`)
	reMultiWS   = regexp.MustCompile(`\s+`)
	reHTMLEnt   = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
		"&apos;", "'",
	)
)

// StripHTML removes HTML tags and decodes common entities, returning plain text.
func StripHTML(html string) string {
	text := reTag.ReplaceAllString(html, " ")
	text = reHTMLEnt.Replace(text)
	text = reMultiWS.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
