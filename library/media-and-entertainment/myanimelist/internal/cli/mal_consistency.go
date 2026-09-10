// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/malhtml"

// malhtmlConsistency keeps the anime_consistency command file focused on Cobra
// wiring while the computation stays in the pure-logic parser package.
func malhtmlConsistency(id int, eps []malhtml.Episode) (malhtml.ReceptionCurve, error) {
	return malhtml.Consistency(id, eps)
}
