// internal/provider/provider.go
package provider

import (
	"context"
	"errors"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
)

// ErrBibNotFound means the event resolved but no runner has that bib.
var ErrBibNotFound = errors.New("bib not found")

// Provider looks up a single runner by bib within a resolved event.
type Provider interface {
	Name() string
	Lookup(ctx context.Context, event domain.Event, bib string) (domain.Result, error)
}

// Registry maps provider names to implementations.
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// NameSearcher is an optional capability: search a runner by name within a
// resolved event. Providers implement it when their search accepts a name.
type NameSearcher interface {
	SearchByName(ctx context.Context, event domain.Event, name string) ([]domain.Result, error)
}

// AthleteSearcher is an optional capability: a cross-event athlete index.
type AthleteSearcher interface {
	FindAthletes(ctx context.Context, name string) ([]domain.Athlete, error)
	AthleteHistory(ctx context.Context, racerID string) ([]domain.Result, error)
}
