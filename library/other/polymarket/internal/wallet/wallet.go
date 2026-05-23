// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Wallet primitives for Polymarket v0.2 live broadcast: load EOA private key
// from env or config, derive checksummed address, expose a Signer that signs
// 32-byte hashes (EIP-712 digest) and returns 65-byte (R || S || V) sigs.
//
// SECURITY RULES (do not relax):
//   - Never print the private key hex anywhere (logs, errors, JSON output).
//   - Never write the private key back to any file the CLI creates.
//   - Address (public) is fine to log; that's how the user identifies their
//     wallet on Polygonscan and Polymarket UI.
//   - Sign() is intentionally synchronous and has no retry/auto-fallback so
//     the caller controls exactly which digest is signed, once.

package wallet

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// EnvKeys lists the env vars the wallet loader consults, in priority order.
// PK_FOR_POLYMARKET_LOGIN is the user's existing env var name; the canonical
// POLYMARKET_PRIVATE_KEY remains supported for parity with the official docs.
var EnvKeys = []string{
	"PK_FOR_POLYMARKET_LOGIN",
	"POLYMARKET_PRIVATE_KEY",
}

// Signer wraps a secp256k1 private key and provides EIP-712-compatible
// signing. Callers receive only the address and signing capability — the
// underlying key never leaves this struct's pointer fields.
type Signer struct {
	pk      *ecdsa.PrivateKey
	address common.Address
}

// LoadFromEnv reads the private key from the first present env var in
// EnvKeys. Returns a Signer with address populated. The key string is parsed
// and discarded from local scope immediately; only the parsed ECDSA key
// remains in memory inside the returned Signer.
func LoadFromEnv() (*Signer, error) {
	var raw string
	var src string
	for _, k := range EnvKeys {
		if v := os.Getenv(k); v != "" {
			raw = v
			src = k
			break
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("no private key in env (looked for: %s)", strings.Join(EnvKeys, ", "))
	}
	s, err := fromHex(raw)
	if err != nil {
		return nil, fmt.Errorf("private key from %s: %w", src, err)
	}
	return s, nil
}

// LoadFromString accepts a raw 0x-prefixed (or bare) hex string. Public so
// the auth subsystem can hand-load from a config file when env is empty.
// Callers must scrub the input string from their own scope after calling.
func LoadFromString(raw string) (*Signer, error) {
	return fromHex(raw)
}

func fromHex(raw string) (*Signer, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "0x")
	clean = strings.TrimPrefix(clean, "0X")
	if len(clean) != 64 {
		return nil, fmt.Errorf("invalid private key length: expected 64 hex chars (got %d)", len(clean))
	}
	for _, c := range clean {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return nil, errors.New("invalid private key: non-hex character")
		}
	}
	pk, err := ethcrypto.HexToECDSA(clean)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	pub, ok := pk.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("derive public key: type assertion failed")
	}
	addr := ethcrypto.PubkeyToAddress(*pub)
	return &Signer{pk: pk, address: addr}, nil
}

// Address returns the checksummed Ethereum address derived from the loaded
// private key. Safe to log.
func (s *Signer) Address() common.Address {
	return s.address
}

// AddressHex returns the EIP-55 checksummed hex string of the address.
// Convenience for JSON output and HTTP headers (POLY_ADDRESS).
func (s *Signer) AddressHex() string {
	return s.address.Hex()
}

// PrivateKey returns the underlying *ecdsa.PrivateKey. Exposed because
// go-ethereum's bind.NewKeyedTransactorWithChainID requires it for on-chain
// tx signing. Caller must NOT marshal or log the returned value.
func (s *Signer) PrivateKey() *ecdsa.PrivateKey {
	return s.pk
}

// Sign signs a 32-byte EIP-712 digest with the secp256k1 key and returns a
// 65-byte (R || S || V) signature where V is normalized to 27/28 (Ethereum
// recovery id convention used by Polymarket CLOB).
//
// go-ethereum's Sign returns V as 0/1; CLOB expects 27/28 (the legacy
// Ethereum convention). We bump V accordingly.
func (s *Signer) Sign(digest32 []byte) ([]byte, error) {
	if len(digest32) != 32 {
		return nil, fmt.Errorf("EIP-712 digest must be 32 bytes (got %d)", len(digest32))
	}
	sig, err := ethcrypto.Sign(digest32, s.pk)
	if err != nil {
		return nil, fmt.Errorf("secp256k1 sign: %w", err)
	}
	if len(sig) != 65 {
		return nil, fmt.Errorf("unexpected signature length: %d", len(sig))
	}
	// Normalize V from 0/1 (Ethereum internal) to 27/28 (CLOB convention).
	if sig[64] < 27 {
		sig[64] += 27
	}
	return sig, nil
}

// SignToHex is a convenience that returns the 0x-prefixed hex of Sign().
func (s *Signer) SignToHex(digest32 []byte) (string, error) {
	sig, err := s.Sign(digest32)
	if err != nil {
		return "", err
	}
	return "0x" + common.Bytes2Hex(sig), nil
}
