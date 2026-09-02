// tesla_key_helpers.go — shared utilities for validating and matching Fleet/BLE
// signing keys.
package cli

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// validatePrivateKeyPEM checks that path is a regular file containing a
// PEM-encoded ECDSA P-256 private key. Tesla Fleet and vehicle-command
// signing require ECDSA P-256 (secp256r1); RSA, P-384, P-521, and other
// key types are rejected with a clear error.
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
		return fmt.Errorf("%q is an RSA private key; Tesla signing requires an ECDSA P-256 private key", path)
	case "EC PRIVATE KEY", "PRIVATE KEY":
		// Parse below and require *ecdsa.PrivateKey with P-256 curve.
	default:
		return fmt.Errorf("%q PEM block type %q is not a private key", path, block.Type)
	}
	var ecKey *ecdsa.PrivateKey
	var parseErr error
	if block.Type == "EC PRIVATE KEY" {
		ecKey, parseErr = x509.ParseECPrivateKey(block.Bytes)
	} else {
		var parsed any
		parsed, parseErr = x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr == nil {
			var ok bool
			ecKey, ok = parsed.(*ecdsa.PrivateKey)
			if !ok {
				return fmt.Errorf("%q is not an ECDSA private key; Tesla signing requires an ECDSA P-256 private key", path)
			}
		}
	}
	if parseErr != nil {
		return fmt.Errorf("%q is not a valid private key: %w", path, parseErr)
	}
	// Tesla's vehicle-command requires NIST P-256 (secp256r1).
	if ecKey.Curve != elliptic.P256() {
		curveName := "unknown"
		if ecKey.Curve != nil && ecKey.Curve.Params() != nil {
			curveName = ecKey.Curve.Params().Name
		}
		return fmt.Errorf("%q uses curve %s; Tesla signing requires an ECDSA P-256 private key", path, curveName)
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

// parsePublicKeyPEM parses a PEM-encoded PKIX public key and returns the
// marshaled public key bytes used for equality checks. Returns nil on failure.
func parsePublicKeyPEM(data []byte) []byte {
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

// readPublicKeyBytes reads a PEM public key file and returns the encoded
// public key bytes for comparison. Returns nil if the file cannot be parsed.
func readPublicKeyBytes(pubPath string) []byte {
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return nil
	}
	return parsePublicKeyPEM(data)
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

// selectKeyByPublicBytes returns a candidate whose derived public key matches
// targetPubBytes. Duplicate files of the same key material count as one
// identity; the lexicographically first path is returned. Returns "" when
// target is empty, nothing matches, or matched paths derive different keys.
func selectKeyByPublicBytes(candidates []string, targetPubBytes []byte) string {
	if len(targetPubBytes) == 0 {
		return ""
	}
	var matched []string
	for _, priv := range candidates {
		if privPub := derivePublicKeyBytes(priv); privPub != nil && bytes.Equal(privPub, targetPubBytes) {
			matched = append(matched, priv)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	firstPub := derivePublicKeyBytes(matched[0])
	for _, p := range matched[1:] {
		if !bytes.Equal(derivePublicKeyBytes(p), firstPub) {
			return ""
		}
	}
	sort.Strings(matched)
	return matched[0]
}

// matchCandidatesToDomain returns the unique candidate matching the hosted
// well-known public key for domain. An empty domain means no binding
// (("", nil)); the caller may then accept a sole candidate. When domain is
// set, a fetch/parse failure or non-unique/zero match is an error.
func matchCandidatesToDomain(teslaDir string, candidates []string, domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", nil
	}
	targetPub, targetSrc, err := resolveRegisterPublicKeyMaterial(teslaDir, domain)
	if err != nil {
		return "", err
	}
	if matched := selectKeyByPublicBytes(candidates, targetPub); matched != "" {
		return matched, nil
	}
	return "", fmt.Errorf("found key(s) in %s do not match the public key for %s (%s); specify --key-file or set TESLA_FLEET_KEY_FILE", teslaDir, domain, targetSrc)
}

// selectKeyByPublicMatch returns the unique candidate whose derived public
// key matches the PEM at targetPubPath. Sibling *-public.pem self-consistency
// is not used: a local pair is not proof the key is Fleet-registered.
// Returns "" when targetPubPath is empty, unreadable, or the match is not unique.
func selectKeyByPublicMatch(candidates []string, targetPubPath string) string {
	if targetPubPath == "" {
		return ""
	}
	return selectKeyByPublicBytes(candidates, readPublicKeyBytes(targetPubPath))
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
