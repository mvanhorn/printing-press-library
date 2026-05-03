package searchbackend

import (
	"context"
	"errors"
	"os"
	"strings"
)

var ErrUnsupported = errors.New("search backend does not support image search")

type SearchOpts struct {
	Limit int
}

type Backend interface {
	Name() string
	Search(ctx context.Context, query string, opts SearchOpts) ([]Result, error)
	ImageSearch(ctx context.Context, photoURL string, opts SearchOpts) ([]Result, error)
}

type Result struct {
	Title   string  `json:"title,omitempty"`
	URL     string  `json:"url,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
	Domain  string  `json:"domain,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

type Factory func() Backend

var registry = map[string]Factory{}

func Register(name string, factory Factory) {
	registry[strings.ToLower(name)] = factory
}

func ByName(name string) Backend {
	if f := registry[strings.ToLower(name)]; f != nil {
		return f()
	}
	return AutoSelect()
}

// AutoSelect returns the highest-priority Backend available given env
// configuration. Use Select for the full fallback chain so a paid backend
// failing or returning zero results still produces an answer via DDG.
func AutoSelect() Backend {
	chain := Select("")
	if len(chain) == 0 {
		return unsupportedBackend{}
	}
	return chain[0]
}

// Select returns an ordered fallback chain of available Backend instances.
// The user's explicit --search-backend choice takes precedence, then any
// env-configured paid backend (parallel, brave, tavily), then ddg. Callers
// should iterate the chain and accept the first non-empty result, surfacing
// fallback events in their response envelopes.
//
// The DDG backend is appended last whenever it is registered, so a paid
// backend with a stale or rate-limited key falls through to a working
// scrape instead of producing an empty candidate set.
func Select(preferred string) []Backend {
	seen := map[string]bool{}
	add := func(out []Backend, name string) []Backend {
		key := strings.ToLower(name)
		if key == "" || seen[key] {
			return out
		}
		f, ok := registry[key]
		if !ok {
			return out
		}
		seen[key] = true
		return append(out, f())
	}
	var chain []Backend
	chain = add(chain, preferred)
	if os.Getenv("PARALLEL_API_KEY") != "" {
		chain = add(chain, "parallel")
	}
	if os.Getenv("BRAVE_SEARCH_API_KEY") != "" {
		chain = add(chain, "brave")
	}
	if os.Getenv("TAVILY_API_KEY") != "" {
		chain = add(chain, "tavily")
	}
	chain = add(chain, "ddg")
	if len(chain) == 0 {
		for _, f := range registry {
			chain = append(chain, f())
		}
	}
	return chain
}

type unsupportedBackend struct{}

func (unsupportedBackend) Name() string { return "unsupported" }
func (unsupportedBackend) Search(context.Context, string, SearchOpts) ([]Result, error) {
	return nil, ErrUnsupported
}
func (unsupportedBackend) ImageSearch(context.Context, string, SearchOpts) ([]Result, error) {
	return nil, ErrUnsupported
}
