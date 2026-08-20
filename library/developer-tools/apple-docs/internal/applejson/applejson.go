// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// Package applejson decodes the DocC JSON shapes Apple serves at
// developer.apple.com/tutorials/data/. These shapes are documented in
// the open-source apple/swift-docc-render Vue.js code (RenderNode /
// RenderReference Swift types), which is the canonical schema source.
package applejson

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/client"
)

// PlatformAvailability is one row from metadata.platforms[].
type PlatformAvailability struct {
	Name         string `json:"name"`
	IntroducedAt string `json:"introducedAt,omitempty"`
	DeprecatedAt string `json:"deprecatedAt,omitempty"`
	Beta         bool   `json:"beta,omitempty"`
	Unavailable  bool   `json:"unavailable,omitempty"`
	Deprecated   bool   `json:"deprecated,omitempty"`
}

// Reference is one entry in the references{} map of a doc page.
type Reference struct {
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	Kind       string   `json:"kind"`
	Role       string   `json:"role,omitempty"`
	Type       string   `json:"type,omitempty"`
	URL        string   `json:"url,omitempty"`
	Abstract   []Inline `json:"abstract,omitempty"`
}

// Inline is one fragment of inline rendered content (used in abstracts).
type Inline struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	Code       string `json:"code,omitempty"`
	Identifier string `json:"identifier,omitempty"`
}

// DocPage is the parsed shape of one doc page.
//
// Identifier and URL are kept as synonyms for historical reasons (both
// populated from metadata.identifier.url); callers may use whichever
// reads more naturally at the call site.
type DocPage struct {
	Identifier            string                 `json:"identifier"`
	Kind                  string                 `json:"kind"`
	Role                  string                 `json:"role"`
	Title                 string                 `json:"title"`
	Modules               []string               `json:"modules,omitempty"`
	SymbolKind            string                 `json:"symbol_kind,omitempty"`
	Abstract              string                 `json:"abstract,omitempty"`
	Platforms             []PlatformAvailability `json:"platforms,omitempty"`
	Declaration           string                 `json:"declaration,omitempty"`
	URL                   string                 `json:"url"`
	References            map[string]Reference   `json:"references,omitempty"`
	RelationshipsSections []json.RawMessage      `json:"-"`
	RawJSON               json.RawMessage        `json:"-"`
}

// FetchDoc retrieves /tutorials/data/documentation/<path>.json.
// path may be a bare framework slug ("swiftui") or a nested path
// ("swiftui/view/onappear(perform:)"). Leading/trailing slashes are
// trimmed and lowercase normalization is applied.
func FetchDoc(ctx context.Context, c *client.Client, path string) (*DocPage, error) {
	docPath := DocPath(path)
	raw, err := c.Get(ctx, docPath, nil)
	if err != nil {
		return nil, err
	}
	return ParseDoc(raw)
}

// FetchIndex retrieves /tutorials/data/index/<framework>.json.
func FetchIndex(ctx context.Context, c *client.Client, framework string) (*FrameworkIndex, error) {
	framework = strings.ToLower(strings.Trim(framework, "/ "))
	raw, err := c.Get(ctx, "/tutorials/data/index/"+framework+".json", nil)
	if err != nil {
		return nil, err
	}
	return ParseIndex(raw)
}

// DocPath normalizes a user-supplied doc path into the URL the API expects.
func DocPath(p string) string {
	p = strings.ToLower(strings.Trim(p, "/ "))
	// If user pasted a full URL prefix, strip it.
	p = strings.TrimPrefix(p, "documentation/")
	p = strings.TrimPrefix(p, "tutorials/data/documentation/")
	return "/tutorials/data/documentation/" + p + ".json"
}

// ParseDoc decodes a raw doc-page JSON into a DocPage.
func ParseDoc(raw json.RawMessage) (*DocPage, error) {
	var envelope struct {
		Identifier struct {
			URL               string `json:"url"`
			InterfaceLanguage string `json:"interfaceLanguage,omitempty"`
		} `json:"identifier"`
		Kind     string `json:"kind"`
		Metadata struct {
			Title      string `json:"title"`
			Role       string `json:"role,omitempty"`
			SymbolKind string `json:"symbolKind,omitempty"`
			Modules    []struct {
				Name string `json:"name"`
			} `json:"modules,omitempty"`
			Platforms []PlatformAvailability `json:"platforms,omitempty"`
		} `json:"metadata"`
		Abstract               []Inline             `json:"abstract,omitempty"`
		References             map[string]Reference `json:"references,omitempty"`
		PrimaryContentSections []json.RawMessage    `json:"primaryContentSections,omitempty"`
		RelationshipsSections  []json.RawMessage    `json:"relationshipsSections,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parsing doc envelope: %w", err)
	}
	page := &DocPage{
		Identifier:            envelope.Identifier.URL,
		Kind:                  envelope.Kind,
		Role:                  envelope.Metadata.Role,
		Title:                 envelope.Metadata.Title,
		SymbolKind:            envelope.Metadata.SymbolKind,
		Platforms:             envelope.Metadata.Platforms,
		Abstract:              InlineText(envelope.Abstract),
		URL:                   envelope.Identifier.URL,
		References:            envelope.References,
		Declaration:           ExtractDeclaration(envelope.PrimaryContentSections),
		RelationshipsSections: envelope.RelationshipsSections,
		RawJSON:               raw,
	}
	for _, m := range envelope.Metadata.Modules {
		page.Modules = append(page.Modules, m.Name)
	}
	return page, nil
}

// InlineText flattens an []Inline into plain text.
func InlineText(in []Inline) string {
	var sb strings.Builder
	for _, frag := range in {
		switch frag.Type {
		case "text", "":
			sb.WriteString(frag.Text)
		case "codeVoice":
			sb.WriteString("`")
			sb.WriteString(frag.Code)
			sb.WriteString("`")
		case "reference":
			sb.WriteString(frag.Identifier)
		default:
			sb.WriteString(frag.Text)
		}
	}
	return strings.TrimSpace(sb.String())
}

// ExtractDeclaration walks primaryContentSections looking for the first
// section of kind "declarations" and returns the joined tokens as a
// single signature string.
func ExtractDeclaration(sections []json.RawMessage) string {
	for _, raw := range sections {
		var meta struct {
			Kind         string `json:"kind"`
			Declarations []struct {
				Tokens []struct {
					Kind string `json:"kind,omitempty"`
					Text string `json:"text,omitempty"`
				} `json:"tokens"`
			} `json:"declarations,omitempty"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		if meta.Kind != "declarations" {
			continue
		}
		for _, decl := range meta.Declarations {
			var sb strings.Builder
			for _, tok := range decl.Tokens {
				sb.WriteString(tok.Text)
			}
			out := strings.TrimSpace(sb.String())
			if out != "" {
				return out
			}
		}
	}
	return ""
}

// IsAvailableOn reports whether the page is available on the named
// platform (case-insensitive) according to its metadata.platforms[]
// entries. A symbol is available on a platform when an entry with that
// name is present, not unavailable, and not deprecated by either the
// boolean `deprecated` flag or a non-empty `deprecatedAt` version. The
// DeprecatedAt check mirrors IsDeprecatedOn — Apple emits "deprecatedAt"
// independently of the boolean, so checking only `Deprecated` would
// surface scheduled-for-removal symbols as valid replacements.
func (p *DocPage) IsAvailableOn(platform string) bool {
	platform = strings.ToLower(platform)
	for _, plat := range p.Platforms {
		if strings.ToLower(plat.Name) != platform {
			continue
		}
		if plat.Unavailable {
			return false
		}
		if plat.Deprecated || plat.DeprecatedAt != "" {
			return false
		}
		return true
	}
	return false
}

// IsDeprecatedOn reports whether the page is marked deprecated on a
// platform.
func (p *DocPage) IsDeprecatedOn(platform string) bool {
	platform = strings.ToLower(platform)
	for _, plat := range p.Platforms {
		if strings.ToLower(plat.Name) != platform {
			continue
		}
		return plat.Deprecated || plat.DeprecatedAt != ""
	}
	return false
}
