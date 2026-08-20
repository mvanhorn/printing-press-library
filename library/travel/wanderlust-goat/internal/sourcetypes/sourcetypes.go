// Package sourcetypes is the shared types every Stage-2 source client
// implements. Putting them in their own package avoids an import cycle
// between dispatch/ and per-source clients (each source would otherwise
// need to import dispatch to consume the Hit type).
package sourcetypes

import "context"

// Hit is one Stage-2 lookup result.
type Hit struct {
	// Source is the slug of the source that produced this hit
	// (matches the package directory: "tabelog", "retty", ...).
	Source string `json:"source"`

	// URL is the canonical link to the place's listing on this source.
	URL string `json:"url"`

	// Title is the place's display name as the source spelled it
	// (often local-language; preserved verbatim per brief).
	Title string `json:"title"`

	// Snippet is a short evidence string the dispatcher cites in
	// `near` / `goat` output. <=200 chars.
	Snippet string `json:"snippet,omitempty"`

	// Locale is the language code of the title and snippet (ja, ko,
	// fr, en).
	Locale string `json:"locale,omitempty"`

	// Relevance is the source's reported relevance score (0-1) when
	// available. Used as a tiebreaker by Stage-3 ranking.
	Relevance float64 `json:"relevance,omitempty"`
}

// Client is the contract every Stage-2 source implements. The
// dispatcher's source registry stores []Client for each region.
type Client interface {
	// Slug returns the source's kebab-case identifier (matches the
	// internal/<source>/ package name).
	Slug() string

	// Locale returns the language code the source returns (ja, ko, fr...).
	Locale() string

	// LookupByName searches the source for hits matching `name`,
	// optionally biased to `city`. The returned []Hit is at most
	// `maxResults` long; ordering is source-defined (callers should
	// not assume relevance order).
	LookupByName(ctx context.Context, name, city string, maxResults int) ([]Hit, error)

	// IsStub reports whether this client is a stub (returns
	// ErrNotImplemented for every call). The wiring test allows stub
	// clients to register; the dispatcher skips them with a typed
	// reason in the trace output.
	IsStub() bool
}
