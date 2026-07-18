// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// URL resolution: any Substack URL a user pastes -> (canonical host, slug) so
// the reader can fetch <host>/api/v1/posts/<slug>. Four URL shapes exist
// (url-shapes.md): A publication subdomain, B custom domain, C reader-app
// @profile (preview-only — reject), D reader-app numeric post id (resolve via
// substack.com/api/v1/posts/by-id/<id>). The three names — @handle, custom
// domain, and internal .substack.com subdomain — are frequently different
// strings, so the real subdomain MUST come from the API, never be guessed from
// the URL text.
package substack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RefKind classifies a parsed post reference.
type RefKind int

const (
	// RefSlugHost is shapes A/B (or the bare "<pub>/<slug>" form): the host and
	// slug are already known from the URL text.
	RefSlugHost RefKind = iota
	// RefByID is shape D: a reader-app numeric post id needing a by-id lookup.
	RefByID
	// RefProfile is shape C: a reader-app @profile — preview-only, not a post.
	RefProfile
)

// PostRef is the result of parsing a user-supplied reference (no network).
type PostRef struct {
	Kind   RefKind
	Host   string // RefSlugHost: host to fetch (e.g. "uxmentor.substack.com" or "creatoreconomy.so")
	Slug   string // RefSlugHost
	ID     string // RefByID
	Handle string // RefProfile
}

// ParsePostRef parses a user-supplied post reference into a PostRef WITHOUT any
// network call. Accepted forms:
//
//	astralcodexten/open-thread-441              (bare handle/slug -> subdomain)
//	uxmentor.substack.com/p/<slug>              (shape A)
//	creatoreconomy.so/p/<slug>                  (shape B, custom domain)
//	https://substack.com/home/post/p-<id>       (shape D, numeric)
//	https://substack.com/inbox/post/<id>        (shape D, numeric)
//	https://substack.com/@handle                (shape C -> rejected with guidance)
func ParsePostRef(input string) (PostRef, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return PostRef{}, fmt.Errorf("empty post reference")
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "/")

	host := s
	path := ""
	if i := strings.IndexByte(s, '/'); i >= 0 {
		host = s[:i]
		path = s[i+1:]
	}
	hl := strings.ToLower(host)

	// Reader-app host (shapes C and D).
	if hl == "substack.com" || hl == "www.substack.com" {
		if strings.HasPrefix(path, "@") {
			handle := strings.TrimPrefix(path, "@")
			if i := strings.IndexByte(handle, '/'); i >= 0 {
				handle = handle[:i]
			}
			return PostRef{Kind: RefProfile, Handle: handle}, nil
		}
		if idx := strings.Index(path, "post/"); idx >= 0 {
			id := path[idx+len("post/"):]
			if j := strings.IndexByte(id, '/'); j >= 0 {
				id = id[:j]
			}
			id = strings.TrimPrefix(id, "p-")
			if id == "" {
				return PostRef{}, fmt.Errorf("could not parse a post id from %q", input)
			}
			return PostRef{Kind: RefByID, ID: id}, nil
		}
		return PostRef{}, fmt.Errorf("unrecognized substack.com URL %q; point to a specific post (…/home/post/p-<id>) or a publication post (<pub>.substack.com/p/<slug>)", input)
	}

	// Publication host or bare handle (shapes A/B).
	if path == "" {
		return PostRef{}, fmt.Errorf("no post slug in %q; use <pub>/<slug> or <pub>.substack.com/p/<slug>", input)
	}
	slug := path
	if strings.HasPrefix(path, "p/") {
		slug = strings.TrimPrefix(path, "p/")
	} else if i := strings.Index(path, "/p/"); i >= 0 {
		slug = path[i+len("/p/"):]
	}
	if j := strings.IndexByte(slug, '/'); j >= 0 {
		slug = slug[:j]
	}
	if slug == "" {
		return PostRef{}, fmt.Errorf("could not parse a slug from %q", input)
	}
	if !strings.Contains(host, ".") {
		host += ".substack.com"
	}
	return PostRef{Kind: RefSlugHost, Host: host, Slug: slug}, nil
}

// byIDEnvelope is the {post, publication} shape of the by-id endpoint.
type byIDEnvelope struct {
	Post struct {
		Slug         string `json:"slug"`
		Audience     string `json:"audience"`
		CanonicalURL string `json:"canonical_url"`
	} `json:"post"`
	Publication struct {
		Subdomain    string `json:"subdomain"`
		CustomDomain string `json:"custom_domain"`
	} `json:"publication"`
}

// ResolveHostSlug turns a PostRef into the canonical (host, slug) to fetch. For
// RefByID it calls the by-id endpoint (keyless on substack.com) to learn the
// real subdomain/custom_domain — which cannot be guessed from the URL. The
// returned host is the custom domain when the publication has one (the only
// host that serves the full body, and where the cookie is honored when attached
// directly), else <subdomain>.substack.com.
func (r PostRef) ResolveHostSlug(ctx context.Context, c *Client) (host, slug string, err error) {
	switch r.Kind {
	case RefProfile:
		return "", "", fmt.Errorf("@%s is a reader-app profile (preview-only); paste a specific post URL instead (<pub>.substack.com/p/<slug>)", r.Handle)
	case RefByID:
		raw, err := c.FetchPostByID(ctx, r.ID)
		if err != nil {
			return "", "", fmt.Errorf("resolving post id %s: %w", r.ID, err)
		}
		var env byIDEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return "", "", fmt.Errorf("parsing by-id envelope for %s: %w", r.ID, err)
		}
		slug = env.Post.Slug
		if slug == "" {
			return "", "", fmt.Errorf("by-id response for %s carried no slug", r.ID)
		}
		host = CanonicalHost(env.Publication.Subdomain, env.Publication.CustomDomain)
		if host == "" {
			return "", "", fmt.Errorf("by-id response for %s carried no publication host", r.ID)
		}
		return host, slug, nil
	default: // RefSlugHost
		return r.Host, r.Slug, nil
	}
}

// ExtractByIDPost pulls the `post` object out of a by-id envelope
// ({post, publication, ...}). The AUTHED substack.com by-id endpoint returns
// the full body_html + per-user fields (is_viewed) on this post object — the
// universal Tier-1 fetch path, because substack.sid is first-party on
// substack.com for both subdomain and custom-domain publications.
func ExtractByIDPost(raw json.RawMessage) (json.RawMessage, error) {
	var env struct {
		Post json.RawMessage `json:"post"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if len(env.Post) == 0 {
		return nil, fmt.Errorf("by-id response carried no post object")
	}
	return env.Post, nil
}

// CanonicalHost picks the host that serves a publication's full body: the
// custom domain when set (the .substack.com subdomain only 301-redirects to it
// and serves no body), else <subdomain>.substack.com.
func CanonicalHost(subdomain, customDomain string) string {
	if cd := strings.TrimSpace(customDomain); cd != "" {
		return strings.TrimPrefix(strings.TrimPrefix(cd, "https://"), "http://")
	}
	if sub := strings.TrimSpace(subdomain); sub != "" {
		return sub + ".substack.com"
	}
	return ""
}
