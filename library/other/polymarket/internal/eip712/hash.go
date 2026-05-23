// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// EIP-712 v4 digest computation. We use go-ethereum's apitypes.TypedData
// which does the heavy lifting (struct hashing, encoded-data layout, type
// dependency resolution) — we only need to assemble the final
// `0x19 0x01 || domainSep || structHash` digest.

package eip712

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Digest returns the 32-byte EIP-712 hash that an EOA must sign to produce
// a Polymarket-compatible signature. The same function works for any
// TypedData built by this package (Order, ClobAuth).
func Digest(td apitypes.TypedData) ([]byte, error) {
	domainSep, err := td.HashStruct("EIP712Domain", td.Domain.Map())
	if err != nil {
		return nil, fmt.Errorf("hash domain: %w", err)
	}
	structHash, err := td.HashStruct(td.PrimaryType, td.Message)
	if err != nil {
		return nil, fmt.Errorf("hash %s struct: %w", td.PrimaryType, err)
	}
	raw := append([]byte{0x19, 0x01}, domainSep...)
	raw = append(raw, structHash...)
	digest := ethcrypto.Keccak256(raw)
	if len(digest) != 32 {
		return nil, errors.New("keccak256 returned non-32-byte digest")
	}
	return digest, nil
}

// OrderHashHex returns the EIP-712 order hash as a 0x-prefixed hex string.
// Polymarket's CLOB requires this as the `hash` field in the POST /order body.
func OrderHashHex(o *Order, negRisk bool) (string, error) {
	td := OrderTypedData(o, negRisk)
	d, err := Digest(td)
	if err != nil {
		return "", err
	}
	return "0x" + common.Bytes2Hex(d), nil
}
