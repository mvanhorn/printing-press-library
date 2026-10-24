// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// Test-only helpers: production code reads chart categories through the
// built-in genre table and categories --live.

// FlattenCategories lists every category and child depth-first.
func (p ChartsPage) FlattenCategories() []ChartCategory {
	out := make([]ChartCategory, 0)
	var walk func([]ChartCategory)
	walk = func(cs []ChartCategory) {
		for _, c := range cs {
			flat := c
			flat.Children = nil
			out = append(out, flat)
			walk(c.Children)
		}
	}
	walk(p.Categories)
	return out
}
