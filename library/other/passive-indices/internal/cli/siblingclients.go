// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/indiapassivefunds"
	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/niftyindices"
)

// newNiftyIndicesClient builds the hand-written niftyindices.com sibling
// client, honoring the root --timeout and --rate-limit flags the same way
// flags.newClient() does for the generated client.
func newNiftyIndicesClient(flags *rootFlags) *niftyindices.Client {
	return niftyindices.New(flags.timeout, flags.rateLimit)
}

// newIndiaPassiveFundsClient builds the hand-written indiapassivefunds.com
// sibling client, which mints and caches its own Bearer token.
func newIndiaPassiveFundsClient(flags *rootFlags) *indiapassivefunds.Client {
	return indiapassivefunds.New(flags.timeout, flags.rateLimit)
}
