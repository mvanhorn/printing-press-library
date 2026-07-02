// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"regexp"
	"strings"
)

// nonSlugRE matches any run of characters that are not ASCII letters or
// digits. Mercado Livre's search path expects a hyphen-joined slug, so every
// such run collapses to a single hyphen.
var nonSlugRE = regexp.MustCompile(`[^a-z0-9]+`)

// slugifyQuery turns a free-text search term into the hyphenated slug Mercado
// Livre's /lista path expects: lowercased, every run of non-alphanumeric
// characters collapsed to a single hyphen, leading/trailing hyphens trimmed.
//
//	"notebook gamer"        -> "notebook-gamer"
//	"furadeira de impacto"  -> "furadeira-de-impacto"
//	"  Câmera 4K!! "        -> "c-mera-4k"
func slugifyQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	q = nonSlugRE.ReplaceAllString(q, "-")
	return strings.Trim(q, "-")
}
