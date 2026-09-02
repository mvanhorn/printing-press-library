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
// parseable PEM-encoded private key (ECDSA or RSA). Returns nil on success,
// or an error describing why the path cannot be used as a signing key.
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
	case "EC PRIVATE KEY", "PRIVATE KEY", "RSA PRIVATE KEY":
		// OK — known private key PEM block types.
	default:
		return fmt.Errorf("%q PEM block type %q is not a private key", path, block.Type)
	}
	// Attempt to parse the key to catch corrupt/invalid data.
	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		if _, err2 := x509.ParseECPrivateKey(block.Bytes); err2 != nil {
			if _, err3 := x509.ParsePKCS1PrivateKey(block.Bytes); err3 != nil {
				return fmt.Errorf("%q is not a valid private key (PKCS8: %v; EC: %v; PKCS1: %v)", path, err, err2, err3)
			}
		}
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

// privateKeyMatchesPublic checks if a private key's derived public key matches
// the given public key file. Returns true only if both can be parsed and the
// encoded public key bytes are identical.
func privateKeyMatchesPublic(privPath, pubPath string) bool {
	privPub := derivePublicKeyBytes(privPath)
	if privPub == nil {
		return false
	}
	pubBytes := readPublicKeyBytes(pubPath)
	if pubBytes == nil {
		return false
	}
	return bytes.Equal(privPub, pubBytes)
}

// findSiblingPublicKey looks for a *-public.pem sibling to a *-private.pem
// file. Returns the path if found, or "" if not.
func findSiblingPublicKey(privPath string) string {
	if !strings.HasSuffix(privPath, "-private.pem") {
		return ""
	}
	pubPath := strings.TrimSuffix(privPath, "-private.pem") + "-public.pem"
	if info, err := os.Stat(pubPath); err == nil && info.Mode().IsRegular() {
		return pubPath
	}
	return ""
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

// selectKeyByPublicMatch tries to select a unique private key from candidates
// by matching against sibling public keys. Returns the matched key path, or ""
// if no unique match exists. If targetPubPath is non-empty, it's used as the
// reference public key; otherwise each candidate's sibling is checked.
func selectKeyByPublicMatch(candidates []string, targetPubPath string) string {
	var matched []string
	var targetPubBytes []byte
	if targetPubPath != "" {
		targetPubBytes = readPublicKeyBytes(targetPubPath)
	}
	for _, priv := range candidates {
		if targetPubBytes != nil {
			// Match against explicit target public key.
			if privPub := derivePublicKeyBytes(priv); privPub != nil && bytes.Equal(privPub, targetPubBytes) {
				matched = append(matched, priv)
			}
		} else {
			// Match against sibling public key.
			if pubPath := findSiblingPublicKey(priv); pubPath != "" {
				if privateKeyMatchesPublic(priv, pubPath) {
					matched = append(matched, priv)
				}
			}
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
