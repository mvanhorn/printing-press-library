// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"

// agodaTestProperty is a compact way to express the priced properties a watch
// run would have just fetched.
type agodaTestProperty struct {
	id    int
	price float64
}

type agodaTestProperties []agodaTestProperty

func (in agodaTestProperties) toProperties() []agoda.Property {
	out := make([]agoda.Property, 0, len(in))
	for _, p := range in {
		out = append(out, agoda.Property{PropertyID: p.id, Name: "Test Property", PriceAllIn: p.price})
	}
	return out
}
