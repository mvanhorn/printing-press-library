// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// Package refdata holds curated static reference tables that the
// CLI ships embedded. Each table carries a Source comment so users
// (and `printing-press absorb-audit`) can trace provenance.
//
// pp:novel-static-reference
// Source: SCA Coffee Tasters' Flavor Wheel (sca.coffee).
// The wheel is a published, copyrighted reference; this package
// encodes the taxonomy only (no descriptive prose) so flavor-wheel
// can map user descriptors to canonical wheel sections without
// redistributing the SCA's full reference card.
package refdata

// FlavorNode is one node in the SCA wheel hierarchy. Level 1 is the
// 9 top-level categories; Level 2 is the second ring; Level 3 is the
// outer-most ring (the most specific descriptors).
type FlavorNode struct {
	Name     string
	Children []FlavorNode
}

// SCAWheel is the full curated taxonomy. Wheel terms reflect the
// published SCA / WCR Coffee Tasters' Flavor Wheel, January 2016.
// Level 3 nodes are a representative subset rather than the
// exhaustive list — flavor-wheel matches descriptors via
// case-insensitive substring, so a tag like "Blackberry" matches
// even though every blackberry subspecies isn't enumerated.
var SCAWheel = []FlavorNode{
	{Name: "Fruity", Children: []FlavorNode{
		{Name: "Berry", Children: []FlavorNode{
			{Name: "Blackberry"}, {Name: "Raspberry"}, {Name: "Blueberry"}, {Name: "Strawberry"},
		}},
		{Name: "Dried Fruit", Children: []FlavorNode{
			{Name: "Raisin"}, {Name: "Prune"}, {Name: "Date"},
		}},
		{Name: "Other Fruit", Children: []FlavorNode{
			{Name: "Coconut"}, {Name: "Cherry"}, {Name: "Pomegranate"},
			{Name: "Pineapple"}, {Name: "Grape"}, {Name: "Apple"}, {Name: "Peach"}, {Name: "Pear"},
		}},
		{Name: "Citrus Fruit", Children: []FlavorNode{
			{Name: "Grapefruit"}, {Name: "Orange"}, {Name: "Lemon"}, {Name: "Lime"},
		}},
	}},
	{Name: "Sour/Fermented", Children: []FlavorNode{
		{Name: "Sour", Children: []FlavorNode{
			{Name: "Sour Aromatics"}, {Name: "Acetic Acid"},
			{Name: "Butyric Acid"}, {Name: "Isovaleric Acid"}, {Name: "Citric Acid"}, {Name: "Malic Acid"},
		}},
		{Name: "Alcohol/Fermented", Children: []FlavorNode{
			{Name: "Winey"}, {Name: "Whiskey"}, {Name: "Fermented"}, {Name: "Overripe"},
		}},
	}},
	{Name: "Green/Vegetative", Children: []FlavorNode{
		{Name: "Olive Oil"}, {Name: "Raw"},
		{Name: "Green/Vegetative", Children: []FlavorNode{
			{Name: "Under-ripe"}, {Name: "Peapod"}, {Name: "Fresh"}, {Name: "Dark Green"},
			{Name: "Vegetative"}, {Name: "Hay-like"}, {Name: "Herb-like"},
		}},
		{Name: "Beany"},
	}},
	{Name: "Other", Children: []FlavorNode{
		{Name: "Papery/Musty", Children: []FlavorNode{
			{Name: "Stale"}, {Name: "Cardboard"}, {Name: "Papery"}, {Name: "Woody"},
			{Name: "Moldy/Damp"}, {Name: "Musty/Dusty"}, {Name: "Musty/Earthy"}, {Name: "Animalic"}, {Name: "Meaty Brothy"}, {Name: "Phenolic"},
		}},
		{Name: "Chemical", Children: []FlavorNode{
			{Name: "Bitter"}, {Name: "Salty"}, {Name: "Medicinal"}, {Name: "Petroleum"},
			{Name: "Skunky"}, {Name: "Rubber"},
		}},
	}},
	{Name: "Roasted", Children: []FlavorNode{
		{Name: "Pipe Tobacco"}, {Name: "Tobacco"},
		{Name: "Burnt", Children: []FlavorNode{
			{Name: "Acrid"}, {Name: "Ashy"}, {Name: "Smoky"}, {Name: "Brown, Roast"},
		}},
		{Name: "Cereal", Children: []FlavorNode{
			{Name: "Grain"}, {Name: "Malt"},
		}},
	}},
	{Name: "Spices", Children: []FlavorNode{
		{Name: "Pungent"}, {Name: "Pepper"},
		{Name: "Brown Spice", Children: []FlavorNode{
			{Name: "Anise"}, {Name: "Nutmeg"}, {Name: "Cinnamon"}, {Name: "Clove"},
		}},
	}},
	{Name: "Nutty/Cocoa", Children: []FlavorNode{
		{Name: "Nutty", Children: []FlavorNode{
			{Name: "Peanuts"}, {Name: "Hazelnut"}, {Name: "Almond"},
		}},
		{Name: "Cocoa", Children: []FlavorNode{
			{Name: "Chocolate"}, {Name: "Dark Chocolate"},
		}},
	}},
	{Name: "Sweet", Children: []FlavorNode{
		{Name: "Brown Sugar", Children: []FlavorNode{
			{Name: "Molasses"}, {Name: "Maple Syrup"}, {Name: "Caramelized"}, {Name: "Honey"},
		}},
		{Name: "Vanilla"}, {Name: "Vanillin"},
		{Name: "Overall Sweet"},
		{Name: "Sweet Aromatics"},
	}},
	{Name: "Floral", Children: []FlavorNode{
		{Name: "Black Tea"},
		{Name: "Floral", Children: []FlavorNode{
			{Name: "Chamomile"}, {Name: "Rose"}, {Name: "Jasmine"},
		}},
	}},
}

// WheelSection represents the path from root to a leaf. flavor-wheel
// uses these paths to surface "which top-level family this descriptor
// landed under".
type WheelSection struct {
	Top    string
	Middle string
	Leaf   string
}

// FlattenSections walks the wheel and returns every (top, middle,
// leaf) path. Order is depth-first, top-to-bottom.
func FlattenSections() []WheelSection {
	var out []WheelSection
	for _, top := range SCAWheel {
		if len(top.Children) == 0 {
			out = append(out, WheelSection{Top: top.Name})
			continue
		}
		for _, mid := range top.Children {
			if len(mid.Children) == 0 {
				out = append(out, WheelSection{Top: top.Name, Middle: mid.Name})
				continue
			}
			for _, leaf := range mid.Children {
				out = append(out, WheelSection{Top: top.Name, Middle: mid.Name, Leaf: leaf.Name})
			}
		}
	}
	return out
}
