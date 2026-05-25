// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

package landbank

import (
	"fmt"
	"strings"
)

// BaseURLForSlug returns the ePropertyPlus public API base URL for a land-bank
// slug. The shape is fixed by the vendor's per-instance hosting convention.
func BaseURLForSlug(slug string) string {
	return fmt.Sprintf("https://public-%s.epropertyplus.com/landmgmtpub/remote/public", slug)
}

// DefaultSlug is the seeded instance used when nothing else is configured.
const DefaultSlug = "kclb"

// ResolveInput carries every base-URL source in precedence order so the
// resolution rule can be unit-tested without touching the filesystem or env.
type ResolveInput struct {
	FlagInstance  string // --instance <slug>   (highest)
	EnvBaseURL    string // EPROPERTYPLUS_BASE_URL (full URL)
	EnvInstance   string // EPROPERTYPLUS_INSTANCE (slug)
	RegistryCurr  string // registry "current" slug
	DefaultSlugIn string // fallback slug (usually DefaultSlug)
}

// Resolution is the outcome of ResolveBaseURL: the chosen base URL plus the
// source label that selected it (for diagnostics/which).
type Resolution struct {
	BaseURL string
	Source  string // "flag" | "env_base_url" | "env_instance" | "registry" | "default"
	Slug    string // slug when the source is slug-based; "" for env_base_url
}

// ResolveBaseURL applies the documented precedence:
//
//	--instance flag > EPROPERTYPLUS_BASE_URL > EPROPERTYPLUS_INSTANCE >
//	registry "current" > default kclb
//
// Whitespace-only values are treated as unset. A non-empty default slug is
// always available, so the result always has a BaseURL.
func ResolveBaseURL(in ResolveInput) Resolution {
	if s := strings.TrimSpace(in.FlagInstance); s != "" {
		return Resolution{BaseURL: BaseURLForSlug(s), Source: "flag", Slug: s}
	}
	if u := strings.TrimSpace(in.EnvBaseURL); u != "" {
		return Resolution{BaseURL: strings.TrimRight(u, "/"), Source: "env_base_url"}
	}
	if s := strings.TrimSpace(in.EnvInstance); s != "" {
		return Resolution{BaseURL: BaseURLForSlug(s), Source: "env_instance", Slug: s}
	}
	if s := strings.TrimSpace(in.RegistryCurr); s != "" {
		return Resolution{BaseURL: BaseURLForSlug(s), Source: "registry", Slug: s}
	}
	slug := strings.TrimSpace(in.DefaultSlugIn)
	if slug == "" {
		slug = DefaultSlug
	}
	return Resolution{BaseURL: BaseURLForSlug(slug), Source: "default", Slug: slug}
}
