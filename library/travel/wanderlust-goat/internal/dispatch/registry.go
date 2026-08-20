// Package dispatch implements the v2 two-stage funnel orchestrator.
//
// v1 had a parallel fanout that planned 12-source dispatches but only
// invoked 4 (overpass, wikipedia, reddit, atlasobscura). v2 inverts that:
// Stage 1 seeds candidates from one real geo-search source (Google Places),
// then Stage 2 deep-researches each candidate against locale-aware
// sources from the regions table.
//
// The Registry below names every source the dispatcher CAN route to,
// keyed by slug. The wiring test asserts this set matches the union of
// every internal/<source>/ package and every regions.AllSourceSlugs()
// entry — so "scaffolded but never invoked" regressions are impossible.
package dispatch

import (
	"sync"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/derfeinschmecker"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dianping"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dissapore"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/eltenedor"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/falstaff"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/gamberorosso"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/hatena"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/hotdinners"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/hotpepper"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/kakaomap"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/lafourchette"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/lefooding"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/mafengwo"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/mangoplate"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/naverblog"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/navermap"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/notecom"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/observerfood"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/pudlo"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/retty"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/slowfood"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/squaremeal"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/tabelog"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/verema"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/xiaohongshu"
)

// Registry holds one Client per slug. The default registry is built
// lazily on first call and contains every source the regions table can
// name. Tests can construct a custom Registry with NewRegistry.
type Registry struct {
	clients map[string]sourcetypes.Client
}

// NewRegistry returns an empty Registry. Use Add to populate.
func NewRegistry() *Registry {
	return &Registry{clients: map[string]sourcetypes.Client{}}
}

// Add registers a client by its Slug. Overwrites any existing entry.
func (r *Registry) Add(c sourcetypes.Client) {
	if r.clients == nil {
		r.clients = map[string]sourcetypes.Client{}
	}
	r.clients[c.Slug()] = c
}

// Get returns the client for slug, or nil if not registered.
func (r *Registry) Get(slug string) sourcetypes.Client {
	if r == nil || r.clients == nil {
		return nil
	}
	return r.clients[slug]
}

// Slugs returns every registered slug. Order is not stable; callers that
// care should sort.
func (r *Registry) Slugs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.clients))
	for s := range r.clients {
		out = append(out, s)
	}
	return out
}

// All returns every registered Client, including stubs.
func (r *Registry) All() []sourcetypes.Client {
	if r == nil {
		return nil
	}
	out := make([]sourcetypes.Client, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, c)
	}
	return out
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

// DefaultRegistry returns the shared registry holding every source slug
// named by internal/regions/regions.go. Built once.
func DefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() {
		r := NewRegistry()
		// Real-impl Stage-2 sources (JP, KR, FR).
		r.Add(tabelog.NewClient())
		r.Add(retty.NewClient())
		r.Add(hotpepper.NewClient())
		r.Add(navermap.NewClient())
		r.Add(naverblog.NewClient())
		r.Add(lefooding.NewClient())
		// Stub Stage-2 sources — every slug named by regions.AllSourceSlugs()
		// must resolve to a Client (real or stub) for the wiring test to pass.
		r.Add(notecom.NewClient())
		r.Add(hatena.NewClient())
		r.Add(kakaomap.NewClient())
		r.Add(mangoplate.NewClient())
		r.Add(pudlo.NewClient())
		r.Add(lafourchette.NewClient())
		r.Add(gamberorosso.NewClient())
		r.Add(slowfood.NewClient())
		r.Add(dissapore.NewClient())
		r.Add(falstaff.NewClient())
		r.Add(derfeinschmecker.NewClient())
		r.Add(verema.NewClient())
		r.Add(eltenedor.NewClient())
		r.Add(squaremeal.NewClient())
		r.Add(hotdinners.NewClient())
		r.Add(observerfood.NewClient())
		r.Add(dianping.NewClient())
		r.Add(mafengwo.NewClient())
		r.Add(xiaohongshu.NewClient())
		defaultRegistry = r
	})
	return defaultRegistry
}
