// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `surprise` — one high-interest unvisited wonder, deterministically seeded by
// date so it is stable within a day (hand-authored).
package cli

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/client"
)

// seedCities are wonder-dense fallbacks used when no --near is given and the
// local cache is empty. The chosen city is itself date-seeded.
var seedCities = []struct {
	name     string
	lat, lng float64
}{
	{"Paris", 48.8566, 2.3522}, {"London", 51.5072, -0.1276}, {"Rome", 41.9028, 12.4964},
	{"Tokyo", 35.6762, 139.6503}, {"New York", 40.7128, -74.0060}, {"Edinburgh", 55.9533, -3.1883},
	{"Berlin", 52.5200, 13.4050}, {"Istanbul", 41.0082, 28.9784}, {"Mexico City", 19.4326, -99.1332},
	{"Lisbon", 38.7223, -9.1393}, {"Prague", 50.0755, 14.4378}, {"Kyoto", 35.0116, 135.7681},
}

func newNovelSurpriseCmd(flags *rootFlags) *cobra.Command {
	var near string
	var category string
	var excludeVisited bool
	var seed string

	cmd := &cobra.Command{
		Use:   "surprise",
		Short: "Pick one high-interest wonder you haven't visited, seeded by date so it's stable per day.",
		Long: "Surface a single high-interest wonder. The pick is seeded by date, so it stays the\n" +
			"same all day and changes tomorrow — handy for a daily agent heartbeat. With --near it\n" +
			"draws from that area; otherwise from your local cache (or a seeded fallback city).\n" +
			"Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli surprise --json\n" +
			"  atlas-obscura-pp-cli surprise --near \"Tokyo\" --exclude-visited",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would pick a surprise wonder")
				return nil
			}
			if seed == "" {
				seed = time.Now().UTC().Format("2006-01-02")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			pool, origin, err := surprisePool(cmd, c, near, seed)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Exclude visited if requested.
			if excludeVisited {
				if s, derr := aoDB(cmd.Context()); derr == nil {
					if ensureAOTables(s) == nil {
						if visited, verr := visitedIDs(s); verr == nil {
							pool = filterVisited(pool, visited)
						}
					}
					_ = s.Close()
				}
			}
			if category != "" {
				pool = filterByCategoryHint(pool, strings.ToLower(category))
			}
			if len(pool) == 0 {
				return notFoundErr(fmt.Errorf("no candidate wonders found; try --near <place> or run a search first"))
			}

			pick := pickSeeded(pool, seed)
			if s, derr := aoDB(cmd.Context()); derr == nil {
				cachePlace(s, pick)
				_ = s.Close()
			}
			return aoEmit(cmd, flags, map[string]any{
				"source": aoSourceNote,
				"seed":   seed,
				"origin": origin,
				"place":  pick,
			})
		},
	}
	cmd.Flags().StringVar(&near, "near", "", "Draw the surprise from near this place or \"lat,lng\"")
	cmd.Flags().StringVar(&category, "category", "", "Prefer places hinting at this category in title/subtitle")
	cmd.Flags().BoolVar(&excludeVisited, "exclude-visited", false, "Exclude places already marked visited")
	cmd.Flags().StringVar(&seed, "seed", "", "Override the date seed (any string) for deterministic picks")
	return cmd
}

// surprisePool builds the candidate pool: from --near, else the local cache,
// else a date-seeded fallback city.
func surprisePool(cmd *cobra.Command, c *client.Client, near, seed string) ([]AOPlace, string, error) {
	if near != "" {
		lat, lng, label, err := resolvePoint(cmd.Context(), near)
		if err != nil {
			return nil, "", err
		}
		pool, _, _, err := collectNear(cmd.Context(), c, lat, lng, nearFilter{limit: 60, maxScanPages: 4})
		return pool, label, err
	}

	// Try the local cache first (offline, no network).
	if s, err := aoDB(cmd.Context()); err == nil {
		raw, _ := s.List("places", 500)
		_ = s.Close()
		pool := decodePlaces(raw)
		if len(pool) >= 5 {
			return pool, "your local cache", nil
		}
	}

	// Fallback: a date-seeded wonder-dense city.
	city := seedCities[seededIndex(seed, len(seedCities))]
	pool, _, _, err := collectNear(cmd.Context(), c, city.lat, city.lng, nearFilter{limit: 60, maxScanPages: 4})
	return pool, city.name, err
}

func decodePlaces(raw []json.RawMessage) []AOPlace {
	out := make([]AOPlace, 0, len(raw))
	for _, r := range raw {
		var p AOPlace
		if json.Unmarshal(r, &p) == nil && p.ID != 0 {
			out = append(out, p)
		}
	}
	return out
}

func filterVisited(pool []AOPlace, visited map[int]bool) []AOPlace {
	out := pool[:0:0]
	for _, p := range pool {
		if !visited[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

func filterByCategoryHint(pool []AOPlace, cat string) []AOPlace {
	out := pool[:0:0]
	for _, p := range pool {
		hay := strings.ToLower(p.Title + " " + p.Subtitle + " " + strings.Join(p.Categories, " "))
		if strings.Contains(hay, cat) {
			out = append(out, p)
		}
	}
	return out
}

// pickSeeded deterministically selects a high-interest place: sort by score,
// then choose within the top tier using the date seed.
func pickSeeded(pool []AOPlace, seed string) AOPlace {
	cp := make([]AOPlace, len(pool))
	copy(cp, pool)
	for i := range cp {
		if cp[i].Score == 0 {
			cp[i].Score = aoScore(cp[i])
		}
	}
	sort.SliceStable(cp, func(i, j int) bool {
		if cp[i].Score != cp[j].Score {
			return cp[i].Score > cp[j].Score
		}
		return cp[i].ID < cp[j].ID
	})
	topK := len(cp)
	if topK > 25 {
		topK = 25
	}
	return cp[seededIndex(seed, topK)]
}

func seededIndex(seed string, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	// h.Sum32() is non-negative and int is 64-bit on supported platforms, so
	// the conversion never overflows; reduce in int space to keep gosec happy.
	return int(h.Sum32()) % n
}
