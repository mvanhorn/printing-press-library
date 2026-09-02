// tesla_key_helpers.go — shared utilities for validating and matching Fleet/BLE
// signing keys.
package cli

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePrivateKeyPEM checks that path is a regular file containing a
// PEM-encoded ECDSA private key (PKCS#8 or EC PRIVATE KEY). Tesla Fleet
// and vehicle-command signing require ECDSA; RSA and other key types fail.
func validatePrivateKeyPEM(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file (mode %s)", path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("%q contains no PEM block", path)
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return fmt.Errorf("%q is an RSA private key; Tesla signing requires an ECDSA private key", path)
	case "EC PRIVATE KEY", "PRIVATE KEY":
		// Parse below and require *ecdsa.PrivateKey.
	default:
		return fmt.Errorf("%q PEM block type %q is not a private key", path, block.Type)
	}
	var parsed any
	var parseErr error
	if block.Type == "EC PRIVATE KEY" {
		parsed, parseErr = x509.ParseECPrivateKey(block.Bytes)
	} else {
		parsed, parseErr = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if parseErr != nil {
		return fmt.Errorf("%q is not a valid private key: %w", path, parseErr)
	}
	if _, ok := parsed.(*ecdsa.PrivateKey); !ok {
		return fmt.Errorf("%q is not an ECDSA private key; Tesla signing requires an ECDSA private key", path)
	}
	return nil
}

// derivePublicKeyBytes reads a PEM private key and returns the encoded public
// key bytes for comparison. Returns nil if the key cannot be parsed.
func derivePublicKeyBytes(privPath string) []byte {
	data, err := os.ReadFile(privPath)
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil
	}
	var pub any
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch k := key.(type) {
		case *ecdsa.PrivateKey:
			pub = &k.PublicKey
		default:
			return nil
		}
	} else if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		pub = &key.PublicKey
	} else {
		return nil
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil
	}
	return pubBytes
}

// readPublicKeyBytes reads a PEM public key file and returns the encoded
// public key bytes for comparison. Returns nil if the file cannot be parsed.
func readPublicKeyBytes(pubPath string) []byte {
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil
	}
	return pubBytes
}

// scanValidPrivateKeys scans a directory for *-private.pem files that are
// valid private keys (regular file, parseable PEM). Returns the list of
// absolute paths.
func scanValidPrivateKeys(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var valid []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "-private.pem") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if validatePrivateKeyPEM(p) == nil {
			valid = append(valid, p)
		}
	}
	return valid
}

// selectKeyByPublicMatch returns the unique candidate whose derived public
// key matches the PEM at targetPubPath. Sibling *-public.pem self-consistency
// is not used: a local pair is not proof the key is Fleet-registered.
// Returns "" when targetPubPath is empty, unreadable, or the match is not unique.
func selectKeyByPublicMatch(candidates []string, targetPubPath string) string {
	if targetPubPath == "" {
		return ""
	}
	targetPubBytes := readPublicKeyBytes(targetPubPath)
	if targetPubBytes == nil {
		return ""
	}
	var matched []string
	for _, priv := range candidates {
		if privPub := derivePublicKeyBytes(priv); privPub != nil && bytes.Equal(privPub, targetPubBytes) {
			matched = append(matched, priv)
		}
	}
	if len(matched) == 1 {
		return matched[0]
	}
	return ""
}

// errMultipleCandidates returns a formatted error listing the candidate keys.
func errMultipleCandidates(dir string, candidates []string, hint string) error {
	var buf strings.Builder
	fmt.Fprintf(&buf, "multiple signing keys in %s:\n", dir)
	for _, c := range candidates {
		fmt.Fprintf(&buf, "  %s\n", c)
	}
	fmt.Fprintf(&buf, "%s", hint)
	return errors.New(buf.String())
}
