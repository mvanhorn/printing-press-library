// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored SNAP-vs-legacy routing policy. Company stance: route to SNAP
// wherever the payment method supports it; legacy is used only where no SNAP
// surface exists (cards, BNPL, online banking, retail stores, batch payouts).
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/config"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/snap"
)

// Surface identifies which API generation serves a request.
type Surface string

const (
	SurfaceSNAP   Surface = "snap"
	SurfaceLegacy Surface = "legacy"
)

// methodRoute describes how one payment method is served.
type methodRoute struct {
	Method     string  // normalized method name
	Surface    Surface // preferred surface per company policy
	SNAPPath   string  // SNAP endpoint path when Surface == snap
	LegacyType string  // legacy /payments/charge "type" value when applicable
	Reason     string  // one-line policy rationale
}

// paymentRoutes is the method->surface policy table for accepting payments.
var paymentRoutes = map[string]methodRoute{
	"qris":           {Method: "qris", Surface: SurfaceSNAP, SNAPPath: "/v1.0/qr/qr-mpm-generate", LegacyType: "QRIS", Reason: "QRIS is BI-regulated; SNAP is mandatory-preferred"},
	"ewallet":        {Method: "ewallet", Surface: SurfaceSNAP, SNAPPath: "/v1.0/debit/payment-host-to-host", LegacyType: "EWALLET", Reason: "e-wallet payments are SNAP-covered; SNAP preferred"},
	"va":             {Method: "va", Surface: SurfaceSNAP, SNAPPath: "/v1.0/transfer-va/create-va", LegacyType: "VA", Reason: "virtual accounts are SNAP-covered; SNAP preferred"},
	"card":           {Method: "card", Surface: SurfaceLegacy, LegacyType: "CARD", Reason: "cards are not SNAP-regulated; legacy is the only surface"},
	"bnpl":           {Method: "bnpl", Surface: SurfaceLegacy, LegacyType: "BNPL", Reason: "BNPL is not SNAP-regulated; legacy is the only surface"},
	"online-banking": {Method: "online-banking", Surface: SurfaceLegacy, LegacyType: "ONLINE_BANKING", Reason: "online banking is not SNAP-regulated; legacy is the only surface"},
	"retail-store":   {Method: "retail-store", Surface: SurfaceLegacy, LegacyType: "RETAIL_STORE", Reason: "retail store payments are not SNAP-regulated; legacy is the only surface"},
}

// payoutRoutes is the destination->surface policy table for sending money.
var payoutRoutes = map[string]methodRoute{
	"bank":    {Method: "bank", Surface: SurfaceSNAP, SNAPPath: "/v1.0/transfer-interbank", Reason: "bank pay-outs are SNAP-covered; SNAP preferred (legacy batch via 'disbursements submit')"},
	"ewallet": {Method: "ewallet", Surface: SurfaceSNAP, SNAPPath: "/v1.0/emoney/topup", Reason: "e-wallet pay-outs are SNAP-covered; SNAP preferred"},
}

// normalizeMethod maps user spellings to policy-table keys.
func normalizeMethod(m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	switch m {
	case "virtual-account", "virtual_account", "va":
		return "va"
	case "e-wallet", "e_wallet", "wallet", "ewallet":
		return "ewallet"
	case "online_banking", "onlinebanking", "internet-banking", "online-banking":
		return "online-banking"
	case "retail", "retail_store", "convenience-store", "retail-store":
		return "retail-store"
	case "credit-card", "debit-card", "cards", "card":
		return "card"
	}
	return m
}

// resolveRoute picks the surface for a method, honoring a --surface override.
// Overriding to a surface the method does not support is an error, not a fallback.
func resolveRoute(table map[string]methodRoute, method, surfaceOverride string) (methodRoute, error) {
	key := normalizeMethod(method)
	route, ok := table[key]
	if !ok {
		known := make([]string, 0, len(table))
		for k := range table {
			known = append(known, k)
		}
		return methodRoute{}, fmt.Errorf("unknown method %q (known: %s)", method, strings.Join(known, ", "))
	}
	switch strings.ToLower(surfaceOverride) {
	case "", "auto":
		return route, nil
	case "snap":
		if route.SNAPPath == "" {
			return methodRoute{}, fmt.Errorf("method %q has no SNAP surface (%s)", key, route.Reason)
		}
		route.Surface = SurfaceSNAP
		return route, nil
	case "legacy":
		if route.LegacyType == "" {
			return methodRoute{}, fmt.Errorf("method %q has no legacy surface", key)
		}
		route.Surface = SurfaceLegacy
		return route, nil
	default:
		return methodRoute{}, fmt.Errorf("invalid --surface %q: must be auto, snap, or legacy", surfaceOverride)
	}
}

// snapClientFromFlags builds the SNAP signing client from the same config the
// legacy client uses, so --env/profile base-URL overrides carry over. This is
// the single SNAP client constructor used across the CLI.
func snapClientFromFlags(flags *rootFlags) (*snap.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	sc := snap.LoadConfig(cfg.BaseURL)
	c := snap.NewClient(sc)
	c.DryRun = dryRunOK(flags)
	return c, nil
}

// classifySNAPCallError maps an error from a SNAP client call to the right
// typed CLI error: rate-limit (429) and missing-credentials get dedicated
// exit codes; everything else falls through to the shared HTTP classifier.
func classifySNAPCallError(err error, flags *rootFlags) error {
	var rle *cliutil.RateLimitError
	if errors.As(err, &rle) {
		return rateLimitErr(err)
	}
	if errors.Is(err, snap.ErrMissingCredentials) {
		return authErr(err)
	}
	return classifyAPIError(err, flags)
}
