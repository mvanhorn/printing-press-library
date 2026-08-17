// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source/graphql"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source/page"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source/rss"
)

// newResolver builds the v2 fetch Resolver in legitimacy order. This is the v2
// analogue of rootFlags.newClient: every v2 command (feed, and later read /
// search / author-archive) calls the Resolver instead of touching a source or
// the RapidAPI client directly.
//
// Order is preference order. RSS is the most legitimate feed surface ($0,
// public, stable, no key/cookies) so it leads for feeds; later stages append
// the page and graphql sources, which serve read/search/author-archive. Sources
// that cannot serve a given command are skipped by the Resolver via their
// Capabilities(), so listing a feed-only source first does not block read or
// search once those sources are wired.
//
// The shared HTTP client is the Surf Chrome-impersonation transport that Gate 0
// validated clears Cloudflare with no cookies for Tier 0.
//
// Tier 1 (optional): if the user has supplied a Medium session cookie (via
// MEDIUM_SESSION, --cookie-file, or MEDIUM_COOKIE_FILE), it is attached to the
// page source (so read returns the member full body instead of the truncated
// preview) and to the graphql source (search/author-archive work anonymously,
// but a logged-in session can affect ranking/visibility). RSS never takes a
// cookie. With no cookie configured the resolver runs fully anonymous (Tier 0).
//
// A misconfigured cookie file (named but unreadable/bad JSON) is surfaced as a
// non-fatal stderr warning rather than failing the command: the spec's contract
// is that cookies are never required, so a bad cookie must degrade to anonymous,
// not block feed/read/search.
func (f *rootFlags) newResolver() *source.Resolver {
	client := source.NewHTTPClient(f.timeout)

	cookies, err := auth.Load(auth.Options{CookieFile: f.cookieFile})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\nproceeding anonymously (Tier 0)\n", err)
		cookies = source.Cookies{}
	}

	pageSrc := page.New(client)       // read surface (article-page APOLLO_STATE -> Markdown)
	graphqlSrc := graphql.New(client) // search + author-archive surface (internal GraphQL)
	if !cookies.IsZero() {
		pageSrc = pageSrc.WithCookies(cookies)
		graphqlSrc = graphqlSrc.WithCookies(cookies)
	}

	return source.NewResolver(
		rss.New(client), // feed surface (Tier 0, keyless; never takes a cookie)
		pageSrc,
		graphqlSrc,
	)
}

// newPageSource builds just the page (article/profile) source, wired with any
// optional Tier-1 cookie via the same chain as newResolver. author-archive uses
// it to resolve a @handle/username to a stable user id (ResolveUserID) before
// handing the id to the GraphQL archive surface. A bad cookie file degrades to a
// non-fatal stderr warning + anonymous fetch, matching the resolver's contract.
func (f *rootFlags) newPageSource() *page.Source {
	client := source.NewHTTPClient(f.timeout)
	cookies, err := auth.Load(auth.Options{CookieFile: f.cookieFile})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\nproceeding anonymously (Tier 0)\n", err)
		cookies = source.Cookies{}
	}
	pageSrc := page.New(client)
	if !cookies.IsZero() {
		pageSrc = pageSrc.WithCookies(cookies)
	}
	return pageSrc
}
