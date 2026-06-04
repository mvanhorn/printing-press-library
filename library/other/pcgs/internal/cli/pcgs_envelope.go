// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH envelope-classifier: hand-authored helper that translates PCGS's
// HTTP-200 + envelope error shape into typed cliError values (usageErr,
// notFoundErr) so the CLI's typed exit codes match the help-text contract.
// Wired into resolveRead in data_source.go.
//
// PATCH envelope-docs-expanded: top-of-file docstring documents the three
// classifier paths, the known false-positive on freshly-graded certs (path 3),
// and cross-references retro issue mvanhorn/cli-printing-press#1551.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// classifyPCGSEnvelope inspects a PCGS response envelope and returns a typed
// error when the envelope reports invalid input ("Invalid CertNo" with
// IsValidRequest=false) or no data found ("No data found" with
// IsValidRequest=true). Returns nil otherwise — including when the response
// has no envelope at all (e.g. wrapped-array banknote responses).
//
// PCGS uses HTTP 200 + envelope to communicate both classes of "expected
// failure", so without this classifier the CLI exits 0 on bad input and the
// typed exit codes promised in the help text and SKILL.md are a lie.
//
// # Three classifier paths
//
//  1. IsValidRequest=false (any ServerMessage) → usageErr (exit 2).
//     Example: "Invalid CertNo" on coin facts-cert with non-digit input.
//
//  2. IsValidRequest=true + ServerMessage matches a no-data keyword
//     (No data found, No orders found, not found, not exist, no record)
//     → notFoundErr (exit 3). Example: PCGS responds "No data found,
//     PCGS No Or Grade No might be invalid!" for a non-stocked spec number.
//
//  3. IsValidRequest=true + ServerMessage="Request successful" + the
//     images-endpoint signature (Images=[] + all three Has*Image=false +
//     ImageReady=false) → notFoundErr (exit 3) with a hint pointing the
//     user to coin facts-cert to confirm cert authenticity. This narrow
//     heuristic is path 3 of the classifier; everything else returns nil.
//
// # Known limitation of path 3 (images-endpoint heuristic)
//
// A genuine cert that hasn't had photos uploaded yet (e.g., a freshly-graded
// slab where the TrueView photographer hasn't gotten to it yet) trips
// path 3 as a false positive — the response shape is structurally
// indistinguishable from a bogus cert at PCGS's images endpoint. The error
// message tells the user how to disambiguate ("run coin facts-cert to confirm
// cert authenticity"). This is documented at known-limitation level rather
// than fixed because the PCGS API itself doesn't distinguish "we have this
// cert but no photos yet" from "we don't have this cert" at the images
// endpoint — the same response is returned for both.
//
// Surfaced live during PCGS Phase 5 dogfood (2026-05-16) when the error_path
// dogfood test used __printing_press_invalid__ as a deliberate-bogus cert and
// the images endpoints returned "Request successful" with empty Images. See
// /printing-press-retro issue #1551 for the broader scorer pattern.
func classifyPCGSEnvelope(data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	var env struct {
		IsValidRequest   *bool             `json:"IsValidRequest"`
		ServerMessage    string            `json:"ServerMessage"`
		Images           []json.RawMessage `json:"Images"`
		HasObverseImage  *bool             `json:"HasObverseImage"`
		HasReverseImage  *bool             `json:"HasReverseImage"`
		HasTrueViewImage *bool             `json:"HasTrueViewImage"`
		ImageReady       *bool             `json:"ImageReady"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		// Not a JSON object envelope (could be an array). Treat as success.
		return nil
	}
	if env.IsValidRequest == nil && env.ServerMessage == "" {
		// No envelope fields at all (e.g. wrapped-array responses). Success.
		return nil
	}
	msg := env.ServerMessage
	// Explicit invalid-input signal.
	if env.IsValidRequest != nil && !*env.IsValidRequest {
		return usageErr(fmt.Errorf("PCGS: %s", strings.TrimSpace(msg)))
	}
	// Envelope says request was valid; check for soft "no data" signals.
	if env.IsValidRequest != nil && *env.IsValidRequest {
		for _, kw := range []string{
			"No data found",
			"No orders found",
			"not found",
			"not exist",
			"no record",
		} {
			if strings.Contains(strings.ToLower(msg), strings.ToLower(kw)) {
				return notFoundErr(fmt.Errorf("PCGS: %s", strings.TrimSpace(msg)))
			}
		}
		// Images-endpoint heuristic: the GetImagesByCertNo / GetBanknoteImagesByCertNo
		// endpoints return ServerMessage="Request successful" for ANY well-formed cert
		// number — even bogus ones — with Images=[] and all Has*Image flags false. PCGS
		// does NOT validate cert authenticity at the images endpoint, so without this
		// heuristic the CLI silently exits 0 on a bogus cert and the user only learns
		// the cert is fake when they later call coin facts-cert.
		//
		// The signature is narrow on purpose: only fires when ALL of (CertNo present,
		// Images=[] empty, every Has*Image flag explicitly false, ImageReady false)
		// appear together. A genuine cert that just hasn't had photos uploaded yet may
		// false-positive here — to confirm authenticity, the user can call coin
		// facts-cert which checks IsValidRequest directly. The risk vs reward tradeoff
		// favors surfacing the "bogus cert" case loudly.
		if env.Images != nil && len(env.Images) == 0 &&
			env.HasObverseImage != nil && !*env.HasObverseImage &&
			env.HasReverseImage != nil && !*env.HasReverseImage &&
			env.HasTrueViewImage != nil && !*env.HasTrueViewImage &&
			env.ImageReady != nil && !*env.ImageReady {
			return notFoundErr(fmt.Errorf("PCGS: no images available for this cert (the cert may not exist or photos have not been uploaded yet — run `coin facts-cert` to confirm cert authenticity)"))
		}
	}
	return nil
}
