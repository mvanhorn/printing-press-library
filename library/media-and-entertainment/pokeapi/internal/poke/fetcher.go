package poke

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/internal/cliutil"
)

// LiveFetcher backs the Fetcher interface against the live PokéAPI client.
// Caches in-memory across the lifetime of one CLI command so repeated lookups
// of the same type or pokemon don't re-roundtrip.
//
// PokéAPI is statically hosted with no documented rate limit, but per AGENTS.md
// any sibling internal client doing outbound HTTP must use AdaptiveLimiter and
// surface *cliutil.RateLimitError when 429s exhaust retries — empty-on-throttle
// is indistinguishable from "no data exists" and silently corrupts downstream
// queries. The limiter starts permissive (10 req/sec) since the API's static
// host can sustain it; it self-tunes downward if Cloudflare shields ever start
// returning 429.
type LiveFetcher struct {
	c        *client.Client
	limiter  *cliutil.AdaptiveLimiter
	typeRels map[string]TypeRelations
	pokemon  map[string]json.RawMessage
}

// NewLiveFetcher returns a Fetcher backed by the supplied HTTP client.
func NewLiveFetcher(c *client.Client) *LiveFetcher {
	return &LiveFetcher{
		c:        c,
		limiter:  cliutil.NewAdaptiveLimiter(10),
		typeRels: make(map[string]TypeRelations),
		pokemon:  make(map[string]json.RawMessage),
	}
}

// rateLimitedGet fronts every fetch with the adaptive limiter. We classify
// transport errors that smell like a 429 ourselves and translate them to
// *cliutil.RateLimitError so callers can distinguish throttle from
// "this resource doesn't exist".
func (f *LiveFetcher) rateLimitedGet(path string, params map[string]string) (json.RawMessage, error) {
	if f.limiter != nil {
		f.limiter.Wait()
	}
	raw, err := f.c.Get(path, params)
	if err != nil {
		// The generated client returns plain errors; we sniff "429" / "rate"
		// strings rather than peeking inside the http.Response. Any matching
		// error is rewritten to *cliutil.RateLimitError so callers above
		// can detect it via errors.As.
		if isRateLimitError(err) {
			if f.limiter != nil {
				f.limiter.OnRateLimit()
			}
			return nil, &cliutil.RateLimitError{URL: path, Body: err.Error()}
		}
		return nil, err
	}
	if f.limiter != nil {
		f.limiter.OnSuccess()
	}
	return raw, nil
}

// isRateLimitError checks whether an error from the HTTP client wraps a 429.
// The generated client's error messages embed the status code in text form
// ("HTTP 429" / "rate limit" / "too many requests"), so a substring sniff
// is the most robust path without leaking transport details up here.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests")
}

func normalizeSlug(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// GetTypeRelations fetches /api/v2/type/{name}/ and extracts damage_relations.
// Memoized in-process for the lifetime of one command.
func (f *LiveFetcher) GetTypeRelations(_ context.Context, typeName string) (TypeRelations, error) {
	typeName = normalizeSlug(typeName)
	if r, ok := f.typeRels[typeName]; ok {
		return r, nil
	}
	raw, err := f.rateLimitedGet(fmt.Sprintf("/api/v2/type/%s/", typeName), nil)
	if err != nil {
		return TypeRelations{}, fmt.Errorf("fetching type %q: %w", typeName, err)
	}
	var doc struct {
		DamageRelations struct {
			DoubleDamageFrom []namedRef `json:"double_damage_from"`
			HalfDamageFrom   []namedRef `json:"half_damage_from"`
			NoDamageFrom     []namedRef `json:"no_damage_from"`
			DoubleDamageTo   []namedRef `json:"double_damage_to"`
			HalfDamageTo     []namedRef `json:"half_damage_to"`
			NoDamageTo       []namedRef `json:"no_damage_to"`
		} `json:"damage_relations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return TypeRelations{}, fmt.Errorf("decoding type %q: %w", typeName, err)
	}
	r := TypeRelations{
		DoubleDamageFrom: namesOf(doc.DamageRelations.DoubleDamageFrom),
		HalfDamageFrom:   namesOf(doc.DamageRelations.HalfDamageFrom),
		NoDamageFrom:     namesOf(doc.DamageRelations.NoDamageFrom),
		DoubleDamageTo:   namesOf(doc.DamageRelations.DoubleDamageTo),
		HalfDamageTo:     namesOf(doc.DamageRelations.HalfDamageTo),
		NoDamageTo:       namesOf(doc.DamageRelations.NoDamageTo),
	}
	f.typeRels[typeName] = r
	return r, nil
}

// GetPokemonRaw returns the full /pokemon/{name}/ payload for use by callers
// who need fields outside the narrow `name + types` slice.
func (f *LiveFetcher) GetPokemonRaw(_ context.Context, name string) (json.RawMessage, error) {
	name = normalizeSlug(name)
	if raw, ok := f.pokemon[name]; ok {
		return raw, nil
	}
	raw, err := f.rateLimitedGet(fmt.Sprintf("/api/v2/pokemon/%s/", name), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching pokemon %q: %w", name, err)
	}
	f.pokemon[name] = raw
	return raw, nil
}

// GetPokemonTypes returns the list of type slugs for a Pokémon.
func (f *LiveFetcher) GetPokemonTypes(ctx context.Context, name string) ([]string, error) {
	raw, err := f.GetPokemonRaw(ctx, name)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Types []struct {
			Type namedRef `json:"type"`
		} `json:"types"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decoding pokemon types: %w", err)
	}
	out := make([]string, 0, len(doc.Types))
	for _, t := range doc.Types {
		out = append(out, t.Type.Name)
	}
	return out, nil
}

// CollectTypeRelations is a convenience that returns type relations for every
// type in the supplied slice, using the live fetcher's in-process memo.
func CollectTypeRelations(ctx context.Context, f Fetcher, types []string) ([]TypeRelations, error) {
	out := make([]TypeRelations, 0, len(types))
	for _, t := range types {
		r, err := f.GetTypeRelations(ctx, t)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

type namedRef struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func namesOf(refs []namedRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return out
}
