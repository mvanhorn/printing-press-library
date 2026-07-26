// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package subjects maps photographic vocabulary onto OpenStreetMap tags.
//
// Overpass takes a query language, not a subject. Someone looking for water
// towers has to know that they are man_made=water_tower, that a few are tagged
// building=water_tower instead, and that they can be nodes, ways, or
// relations. That mapping is the actual product here, which is why it lives in
// one inspectable table that the `types` command prints verbatim.
package subjects

import (
	"fmt"
	"sort"
	"strings"
)

// Tag is one OpenStreetMap key/value selector.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// Regex marks a value that should match as a regular expression, for
	// keys like building where several values mean the same subject.
	Regex bool `json:"regex,omitempty"`
}

// Selector renders the tag as an Overpass filter fragment.
func (t Tag) Selector() string {
	if t.Value == "" {
		return fmt.Sprintf("[%q]", t.Key)
	}
	if t.Regex {
		return fmt.Sprintf("[%q~%q]", t.Key, t.Value)
	}
	return fmt.Sprintf("[%q=%q]", t.Key, t.Value)
}

// Type is one searchable photographic subject.
type Type struct {
	Name string `json:"name"`
	// Group buckets types for browsing.
	Group string `json:"group"`
	// Description says what the subject is, in a photographer's terms.
	Description string `json:"description"`
	// Tags are alternatives: an element matching ANY of them is a hit.
	Tags []Tag `json:"tags"`
	// Aliases are other names a user might type.
	Aliases []string `json:"aliases,omitempty"`
	// Note records a coverage caveat worth saying out loud.
	Note string `json:"note,omitempty"`
}

// catalogue is the curated taxonomy.
//
// Every entry was chosen because it is a thing people photograph, is tagged
// consistently enough in OpenStreetMap to be findable, and is not trivially
// expressible as a single obvious tag the user would have guessed anyway.
var catalogue = []Type{
	{
		Name: "water_tower", Group: "industrial",
		Description: "Water towers — isolated vertical structures, strong against open sky",
		Tags:        []Tag{{Key: "man_made", Value: "water_tower"}, {Key: "building", Value: "water_tower"}},
		Aliases:     []string{"watertower", "water-tower"},
	},
	{
		Name: "lighthouse", Group: "coastal",
		Description: "Lighthouses and beacons",
		Tags:        []Tag{{Key: "man_made", Value: "lighthouse"}},
	},
	{
		Name: "pier", Group: "coastal",
		Description: "Piers and jetties reaching into water",
		Tags:        []Tag{{Key: "man_made", Value: "pier"}},
	},
	{
		Name: "brutalist", Group: "architecture",
		Description: "Buildings tagged as brutalist architecture",
		Tags:        []Tag{{Key: "building:architecture", Value: "brutalist"}},
		Note:        "Architectural style tagging is sparse and volunteer-contributed; expect far fewer results than actually exist.",
	},
	{
		Name: "art_deco", Group: "architecture",
		Description: "Buildings tagged as art deco",
		Tags:        []Tag{{Key: "building:architecture", Value: "art_deco"}},
		Aliases:     []string{"deco", "artdeco"},
		Note:        "Architectural style tagging is sparse; expect far fewer results than actually exist.",
	},
	{
		Name: "parking_structure", Group: "architecture",
		Description: "Multi-storey car parks — repeating concrete geometry",
		// amenity=parking used to be an OR-branch here, but it is
		// overwhelmingly flat surface lots, which have none of the geometry
		// this type exists to find and contradict its own description.
		// building=parking and parking=multi-storey both mean a structure.
		Tags: []Tag{
			{Key: "building", Value: "parking"},
			{Key: "parking", Value: "multi-storey"},
		},
		Aliases: []string{"car_park", "parking_garage", "multistorey"},
	},
	{
		Name: "viewpoint", Group: "vantage",
		Description: "Marked viewpoints and overlooks",
		Tags:        []Tag{{Key: "tourism", Value: "viewpoint"}},
		Aliases:     []string{"overlook", "vista"},
	},
	{
		Name: "observatory", Group: "vantage",
		Description: "Astronomical observatories",
		Tags:        []Tag{{Key: "man_made", Value: "observatory"}},
	},
	{
		Name: "tower", Group: "industrial",
		Description: "Communication, observation, and cooling towers",
		Tags:        []Tag{{Key: "man_made", Value: "tower"}},
	},
	{
		Name: "chimney", Group: "industrial",
		Description: "Industrial chimneys and smokestacks",
		Tags:        []Tag{{Key: "man_made", Value: "chimney"}},
		Aliases:     []string{"smokestack"},
	},
	{
		Name: "silo", Group: "industrial",
		Description: "Grain silos and storage tanks",
		Tags:        []Tag{{Key: "man_made", Value: "silo"}, {Key: "man_made", Value: "storage_tank"}},
	},
	{
		Name: "windmill", Group: "industrial",
		Description: "Windmills and wind turbines",
		Tags:        []Tag{{Key: "man_made", Value: "windmill"}, {Key: "generator:source", Value: "wind"}},
		Aliases:     []string{"wind_turbine", "turbine"},
	},
	{
		Name: "bridge", Group: "infrastructure",
		Description: "Named bridges and viaducts",
		Tags:        []Tag{{Key: "man_made", Value: "bridge"}},
	},
	{
		Name: "ruins", Group: "decay",
		Description: "Ruined and abandoned structures",
		Tags:        []Tag{{Key: "historic", Value: "ruins"}, {Key: "building", Value: "ruins"}},
		Aliases:     []string{"ruin", "abandoned"},
	},
	{
		Name: "mural", Group: "street",
		Description: "Murals and large-scale painted street art",
		// tourism=artwork used to be an OR-branch here and swamped the
		// result: it covers every public artwork, so a downtown Los Angeles
		// search returned statues and memorials — George Washington,
		// Christopher Columbus, the Vietnam Veterans Memorial — and no
		// murals. A search that confidently returns the wrong subject is
		// worse than one that returns fewer of the right one, so the branch
		// is gone and artwork_type carries the type on its own.
		Tags:    []Tag{{Key: "artwork_type", Value: "mural"}},
		Aliases: []string{"street_art", "graffiti"},
		Note:    "Only artwork explicitly tagged artwork_type=mural matches. Plenty of real murals are mapped as generic public art and will not appear.",
	},
	{
		Name: "gallery", Group: "art",
		Description: "Art galleries and museums",
		Tags:        []Tag{{Key: "tourism", Value: "gallery"}, {Key: "tourism", Value: "museum"}},
		Aliases:     []string{"museum"},
	},
	{
		Name: "water", Group: "landscape",
		Description: "Lakes, reservoirs, and large water bodies",
		Tags:        []Tag{{Key: "natural", Value: "water"}},
		Aliases:     []string{"lake", "reservoir"},
	},
	{
		Name: "peak", Group: "landscape",
		Description: "Named summits",
		Tags:        []Tag{{Key: "natural", Value: "peak"}},
		Aliases:     []string{"summit", "mountain"},
	},
}

// Groups returns the distinct groups, sorted.
func Groups() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range catalogue {
		if !seen[t.Group] {
			seen[t.Group] = true
			out = append(out, t.Group)
		}
	}
	sort.Strings(out)
	return out
}

// All returns every type, sorted by group then name.
func All() []Type {
	out := make([]Type, len(catalogue))
	copy(out, catalogue)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// InGroup returns the types in one group.
func InGroup(group string) []Type {
	var out []Type
	for _, t := range All() {
		if strings.EqualFold(t.Group, group) {
			out = append(out, t)
		}
	}
	return out
}

// Names lists every canonical type name, sorted.
func Names() []string {
	out := make([]string, 0, len(catalogue))
	for _, t := range catalogue {
		out = append(out, t.Name)
	}
	sort.Strings(out)
	return out
}

// Lookup resolves a name or alias to a type.
//
// Unknown names fail loudly with the full list rather than falling back to a
// guess: an empty result from the wrong tag is indistinguishable from an empty
// result from a real one, and the user would believe the wrong answer.
func Lookup(name string) (Type, error) {
	q := strings.ToLower(strings.TrimSpace(name))
	q = strings.ReplaceAll(q, "-", "_")
	q = strings.ReplaceAll(q, " ", "_")
	if q == "" {
		return Type{}, fmt.Errorf("name a subject type; try one of: %s", strings.Join(Names(), ", "))
	}
	for _, t := range catalogue {
		if strings.EqualFold(t.Name, q) {
			return t, nil
		}
		for _, a := range t.Aliases {
			if strings.EqualFold(a, q) {
				return t, nil
			}
		}
	}
	return Type{}, fmt.Errorf("unknown subject type %q; known types: %s", name, strings.Join(Names(), ", "))
}
