// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Helpers for building the JSON body Polymarket's POST /order expects.
//
// The on-chain Order struct (signed via EIP-712) and the wire-format Order
// body (sent as JSON) carry the SAME fields but differ in three ways:
//
//  1. `side` is uint8 on-chain (0=BUY, 1=SELL), STRING in JSON ("BUY"/"SELL").
//  2. `signatureType` is uint8 on-chain, INTEGER in JSON (not string).
//  3. All uint256 numeric fields ship as DECIMAL STRINGS in JSON (because
//     JavaScript JSON parsers can't represent uint256 in a number type).
//
// SaltRandom256 returns a cryptographically random uint256 suitable for the
// salt field. py-clob-client uses `ts_ms + rand` which is fine but smaller
// entropy; a full 256-bit random is safer and what the contract expects.

package clob

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// SaltRandom256 returns a cryptographically random salt as *big.Int. We
// cap the value at 2^63 (≈9.2e18) to stay compatible with py-clob-client's
// `int(random() * 1e18)` convention — Polymarket's CLOB validator accepts
// full uint256 in principle but some downstream relayers truncate to int64.
// Using a smaller range avoids that hazard with negligible loss of entropy
// (still 63 bits, plenty for replay protection).
func SaltRandom256() (*big.Int, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("read random salt bytes: %w", err)
	}
	// Mask high bit so the value is positive and fits in int64.
	bytes[0] &= 0x7F
	return new(big.Int).SetBytes(bytes), nil
}

// SideString converts the on-chain uint8 side (0/1) to the wire-format
// string Polymarket expects in the JSON body. Returns "" on invalid input
// so the caller can fail fast.
func SideString(side uint8) string {
	switch side {
	case 0:
		return "BUY"
	case 1:
		return "SELL"
	default:
		return ""
	}
}

// SideFromString accepts case-insensitive "buy"/"sell" and returns the
// on-chain uint8 encoding. Returns 255 + non-nil error on unrecognized input.
func SideFromString(s string) (uint8, error) {
	switch s {
	case "BUY", "buy", "Buy":
		return 0, nil
	case "SELL", "sell", "Sell":
		return 1, nil
	default:
		return 255, fmt.Errorf("invalid side %q: expected BUY or SELL", s)
	}
}

// AmountsForOrder computes makerAmount + takerAmount in 6-decimal fixed
// point given a human price (0–1 probability), a size in tokens, and a
// side. Polymarket uses 6-decimal scaling for both USDC and CTF tokens.
//
//	BUY:  makerAmount = USDC (price × size × 1e6), takerAmount = tokens (size × 1e6)
//	SELL: makerAmount = tokens (size × 1e6),       takerAmount = USDC (price × size × 1e6)
//
// Float input is rounded to nearest integer in 6-decimal space. Polymarket
// CLOB validates that price * size results in an integer USDC amount; if
// price * size has fractional cents the CLOB returns INVALID_AMOUNT.
// Callers should pre-snap price to the market's minimum_tick_size and
// size to a multiple of 100 (or 1) to avoid this.
func AmountsForOrder(side uint8, price, size float64) (maker, taker *big.Int) {
	usdc := big.NewInt(int64(price*size*1_000_000 + 0.5))
	tokens := big.NewInt(int64(size*1_000_000 + 0.5))
	if side == 0 { // BUY
		return usdc, tokens
	}
	return tokens, usdc
}
