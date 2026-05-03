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

func AutoSelect() Backend {
	switch {
	case os.Getenv("PARALLEL_API_KEY") != "":
		return ByName("parallel")
	case os.Getenv("BRAVE_SEARCH_API_KEY") != "":
		return ByName("brave")
	case os.Getenv("TAVILY_API_KEY") != "":
		return ByName("tavily")
	default:
		if f := registry["ddg"]; f != nil {
			return f()
		}
		for _, f := range registry {
			return f()
		}
		return unsupportedBackend{}
	}
}

type unsupportedBackend struct{}

func (unsupportedBackend) Name() string { return "unsupported" }
func (unsupportedBackend) Search(context.Context, string, SearchOpts) ([]Result, error) {
	return nil, ErrUnsupported
}
func (unsupportedBackend) ImageSearch(context.Context, string, SearchOpts) ([]Result, error) {
	return nil, ErrUnsupported
}
