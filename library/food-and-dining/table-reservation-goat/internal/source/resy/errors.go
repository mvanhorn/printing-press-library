// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import "errors"

// Sentinel errors mirror the typed-error pattern used by internal/source/opentable
// and internal/source/tock so the top-level book/cancel commands can map Resy
// failures into the same JSON error categories agents already key on.
var (
	// ErrAuthMissing is returned when an authenticated endpoint is called
	// without a stored Resy token. The CLI surfaces this as `auth_required`
	// with a hint to run `auth login --resy`.
	ErrAuthMissing = errors.New("resy: no auth token configured")

	// ErrAuthExpired is returned on 401/419 from any authenticated endpoint.
	// Resy tokens are long-lived JWTs but they do rotate; the surface
	// mirrors opentable.ErrAuthExpired.
	ErrAuthExpired = errors.New("resy: auth token expired or rejected")

	// ErrSlotTaken is returned when /3/details or /3/book reports the slot
	// is no longer available. Empirically Resy returns a 410 or a 200 with
	// an error envelope; the client folds both into this sentinel.
	ErrSlotTaken = errors.New("resy: slot is no longer available")

	// ErrNoPaymentMethod is returned when /3/details returns a user with no
	// payment_methods. Resy requires a card on file even for "free"
	// reservations.
	ErrNoPaymentMethod = errors.New("resy: no payment method on file")

	// ErrCanaryUnrecognizedBody is the catch-all canary for API drift.
	// Returned when the parser can't make sense of a 200 response — useful
	// for distinguishing genuine API shape changes from upstream outages.
	ErrCanaryUnrecognizedBody = errors.New("resy: response shape unrecognized")
)
