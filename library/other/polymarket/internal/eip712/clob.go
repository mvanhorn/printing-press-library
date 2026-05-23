// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// EIP-712 typed-data definitions for Polymarket CLOB.
//
// Two distinct typed structures are signed in different flows:
//
//  1. ClobAuth — used by `auth derive` to prove EOA control before the CLOB
//     issues L2 HMAC credentials. Domain has no verifyingContract.
//
//  2. Order — used by `orders create --broadcast` for every CLOB order.
//     Domain pins to the CTF Exchange (or NegRisk CTF Exchange) contract.
//
// Contract addresses are PUBLIC and verifiable on Polygonscan; pinning them
// here matches the official py-clob-client / java-clob-client / Rust CLI
// implementations.

package eip712

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Polygon chain ID. Polymarket runs only on Polygon mainnet.
const ChainID = 137

// CTF Exchange contracts. NegRisk variant routes orders for negative-risk
// (mutually exclusive multi-outcome) markets; the default Exchange handles
// binary yes/no markets.
const (
	CTFExchangeAddress        = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	NegRiskCTFExchangeAddress = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
)

// Side encoding inside the on-chain Order struct (uint8 — NOT a string).
const (
	SideBuy  uint8 = 0
	SideSell uint8 = 1
)

// SignatureType encoding inside the Order struct.
const (
	SigTypeEOA           uint8 = 0
	SigTypePolyProxy     uint8 = 1
	SigTypePolyGnosisSafe uint8 = 2
)

// Order is the canonical Polymarket CLOB limit order struct. Field order
// and types MUST match the on-chain Solidity struct exactly; the CLOB
// rejects signatures over any other layout with "INVALID_SIGNATURE".
type Order struct {
	Salt          *big.Int       // random uint256, replay protection
	Maker         common.Address // funder address (proxy if SigType > 0)
	Signer        common.Address // EOA that produces the signature
	Taker         common.Address // 0x0 for open orders (book maker)
	TokenID       *big.Int       // CLOB outcome token id (uint256)
	MakerAmount   *big.Int       // USDC (6-decimal) on BUY, tokens (6-decimal) on SELL
	TakerAmount   *big.Int       // inverse of MakerAmount
	Expiration    *big.Int       // unix seconds; 0 for non-GTD
	Nonce         *big.Int       // per-maker nonce, 0 is fine for first order
	FeeRateBps    *big.Int       // protocol fee (Polymarket sets 0 today)
	Side          uint8          // 0=BUY, 1=SELL
	SignatureType uint8          // 0=EOA, 1=PROXY, 2=GNOSIS
}

// OrderTypedData returns the apitypes.TypedData ready for hashing. Pass
// negRisk=true if the market metadata declares neg_risk = true; that
// switches the domain's verifyingContract to NegRiskCTFExchange.
func OrderTypedData(o *Order, negRisk bool) apitypes.TypedData {
	verifying := CTFExchangeAddress
	if negRisk {
		verifying = NegRiskCTFExchangeAddress
	}
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": []apitypes.Type{
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "taker", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "expiration", Type: "uint256"},
				{Name: "nonce", Type: "uint256"},
				{Name: "feeRateBps", Type: "uint256"},
				{Name: "side", Type: "uint256"},
				{Name: "signatureType", Type: "uint256"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "Polymarket CTF Exchange",
			Version:           "1",
			ChainId:           math.NewHexOrDecimal256(ChainID),
			VerifyingContract: verifying,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          o.Salt.String(),
			"maker":         o.Maker.Hex(),
			"signer":        o.Signer.Hex(),
			"taker":         o.Taker.Hex(),
			"tokenId":       o.TokenID.String(),
			"makerAmount":   o.MakerAmount.String(),
			"takerAmount":   o.TakerAmount.String(),
			"expiration":    o.Expiration.String(),
			"nonce":         o.Nonce.String(),
			"feeRateBps":    o.FeeRateBps.String(),
			"side":          fmt.Sprintf("%d", o.Side),
			"signatureType": fmt.Sprintf("%d", o.SignatureType),
		},
	}
}

// ClobAuth is the typed data Polymarket signs to mint L2 HMAC creds. All
// four message fields are encoded as the types declared below; in
// particular `timestamp` is a STRING (not uint256) and `nonce` is
// uint256. Mismatch on either renders the digest invalid.
type ClobAuth struct {
	Address   string
	Timestamp string
	Nonce     *big.Int
	Message   string
}

// DefaultClobAuthMessage is the exact ASCII Polymarket's clients embed in
// the ClobAuth typed payload. The CLOB compares this string verbatim when
// validating the signature; do not paraphrase.
const DefaultClobAuthMessage = "This message attests that I control the given wallet"

// ClobAuthTypedData builds the apitypes.TypedData for the auth challenge.
func ClobAuthTypedData(a *ClobAuth) apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
			"ClobAuth": []apitypes.Type{
				{Name: "address", Type: "address"},
				{Name: "timestamp", Type: "string"},
				{Name: "nonce", Type: "uint256"},
				{Name: "message", Type: "string"},
			},
		},
		PrimaryType: "ClobAuth",
		Domain: apitypes.TypedDataDomain{
			Name:    "ClobAuthDomain",
			Version: "1",
			ChainId: math.NewHexOrDecimal256(ChainID),
		},
		Message: apitypes.TypedDataMessage{
			"address":   a.Address,
			"timestamp": a.Timestamp,
			"nonce":     a.Nonce.String(),
			"message":   a.Message,
		},
	}
}
