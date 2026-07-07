// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PostSummary is the normalized list-item shape every source emits for feed,
// search, and author-archive results. It is intentionally a small, source-
// agnostic projection: RSS, the article page, and GraphQL each populate the
// subset of fields they can, and the CLI/store layers consume the union.
type PostSummary struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	AuthorID    string    `json:"author_id"`
	Username    string    `json:"username"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Subtitle    string    `json:"subtitle"`
	Tags        []string  `json:"tags"`
}

// Article is the normalized full-read shape produced by the read path. Markdown
// mirrors the v1 oracle's markdown style (# H1, ### H3, ![](img), > quote,
// blank-line-separated paragraphs) so the differential test can compare it
// against the saved oracle output.
//
// IsLocked vs IsPreviewOnly: IsLocked reflects Medium's member-paywall flag on
// the post; IsPreviewOnly reflects whether THIS fetch returned only the
// truncated preview (true for a locked post fetched anonymously, false once a
// valid member cookie unlocks the full body).
type Article struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Subtitle      string    `json:"subtitle"`
	Author        string    `json:"author"`
	AuthorID      string    `json:"author_id"`
	URL           string    `json:"url"`
	Markdown      string    `json:"markdown"`
	IsLocked      bool      `json:"is_locked"`
	IsPreviewOnly bool      `json:"is_preview_only"`
	PublishedAt   time.Time `json:"published_at"`
	WordCount     int       `json:"word_count"`
}

// Caps declares which Source methods a given source can serve. The Resolver
// uses it to skip sources that structurally cannot answer a command (e.g. an
// RSS source has no Search), so a missing capability is a clean skip rather
// than a wasted call that errors.
type Caps struct {
	Feed          bool
	ReadArticle   bool
	Search        bool
	AuthorArchive bool
}

// Source is one fetch surface (RSS, page-parse, GraphQL). The Resolver holds
// several of these in legitimacy order and dispatches each command to the
// most-preferred capable source, falling back to the next on failure.
//
// A source that does not support a method should declare it false in
// Capabilities() AND return ErrUnsupported from that method, so both the
// fast-path skip and a direct mis-dispatch are handled safely (never a panic).
type Source interface {
	Name() string
	Feed(ctx context.Context, ref string) ([]PostSummary, error)
	ReadArticle(ctx context.Context, idOrURL string) (*Article, error)
	Search(ctx context.Context, query string, limit int) ([]PostSummary, error)
	AuthorArchive(ctx context.Context, userIDOrHandle string, max int) ([]PostSummary, error)
	Capabilities() Caps
}

// ErrUnsupported is returned by a Source method the source does not implement.
// It signals the Resolver to fall through to the next source rather than treat
// the call as a hard failure.
var ErrUnsupported = errors.New("source: capability not supported by this source")

// ErrSurfaceUnavailable is the typed graceful-degradation error. The Resolver
// returns it (wrapped with context) when no capable source could satisfy a
// command — most importantly when Medium's internal GraphQL surface is down,
// so search/author-archive degrade to a clear, actionable message instead of
// a panic or an opaque transport error. Feed/read, which do not depend on
// GraphQL, are unaffected.
//
// Callers test for it with errors.Is(err, ErrSurfaceUnavailable).
var ErrSurfaceUnavailable = errors.New("source: surface unavailable; Medium may have changed its internal API")

// surfaceUnavailable wraps the last underlying error behind
// ErrSurfaceUnavailable so callers can errors.Is the sentinel while operators
// still see the root cause. When no source was even capable, cause is nil and
// the message stands alone.
func surfaceUnavailable(command string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%s: %w", command, ErrSurfaceUnavailable)
	}
	return fmt.Errorf("%s: %w (last error: %v)", command, ErrSurfaceUnavailable, cause)
}

// Resolver dispatches each command across an ordered list of sources. Order is
// legitimacy/preference: the first capable source is tried first, the next is
// the fallback, and so on. This is the v2 analogue of v1's single client — it
// is the seam every command calls instead of touching a source directly.
type Resolver struct {
	sources []Source
}

// NewResolver returns a Resolver over the given sources, in preference order
// (most-legitimate / most-preferred first).
func NewResolver(sources ...Source) *Resolver {
	return &Resolver{sources: sources}
}

// Sources exposes the configured sources in order (read-only view for callers
// that need to introspect, e.g. doctor/diagnostics).
func (r *Resolver) Sources() []Source {
	out := make([]Source, len(r.sources))
	copy(out, r.sources)
	return out
}

// Feed dispatches to the first capable source, falling back on error.
func (r *Resolver) Feed(ctx context.Context, ref string) ([]PostSummary, error) {
	var lastErr error
	tried := false
	for _, s := range r.sources {
		if !s.Capabilities().Feed {
			continue
		}
		tried = true
		out, err := s.Feed(ctx, ref)
		if err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return nil, dispatchErr("feed", tried, lastErr)
}

// ReadArticle dispatches to the first capable source, falling back on error.
func (r *Resolver) ReadArticle(ctx context.Context, idOrURL string) (*Article, error) {
	var lastErr error
	tried := false
	for _, s := range r.sources {
		if !s.Capabilities().ReadArticle {
			continue
		}
		tried = true
		out, err := s.ReadArticle(ctx, idOrURL)
		if err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return nil, dispatchErr("read", tried, lastErr)
}

// Search dispatches to the first capable source, falling back on error. When
// every search-capable source fails (GraphQL down), it degrades to
// ErrSurfaceUnavailable rather than surfacing a raw transport error.
func (r *Resolver) Search(ctx context.Context, query string, limit int) ([]PostSummary, error) {
	var lastErr error
	tried := false
	for _, s := range r.sources {
		if !s.Capabilities().Search {
			continue
		}
		tried = true
		out, err := s.Search(ctx, query, limit)
		if err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return nil, dispatchErr("search", tried, lastErr)
}

// AuthorArchive dispatches to the first capable source, falling back on error.
// Degrades to ErrSurfaceUnavailable when every capable source fails.
func (r *Resolver) AuthorArchive(ctx context.Context, userIDOrHandle string, max int) ([]PostSummary, error) {
	var lastErr error
	tried := false
	for _, s := range r.sources {
		if !s.Capabilities().AuthorArchive {
			continue
		}
		tried = true
		out, err := s.AuthorArchive(ctx, userIDOrHandle, max)
		if err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return nil, dispatchErr("author-archive", tried, lastErr)
}

// dispatchErr builds the failure result for a command after all capable
// sources have been exhausted. Whether any source was even capable (tried) or
// they were all capable-but-failing collapses to the same caller-facing
// outcome: the surface is unavailable. Folding both into ErrSurfaceUnavailable
// is deliberate — it is exactly the graceful-degradation contract the spec
// requires (typed, never a panic), and it keeps the "GraphQL down" and "no
// GraphQL source configured" cases indistinguishable to the command layer,
// which only needs to print one clear message either way.
func dispatchErr(command string, tried bool, lastErr error) error {
	return surfaceUnavailable(command, lastErr)
}
