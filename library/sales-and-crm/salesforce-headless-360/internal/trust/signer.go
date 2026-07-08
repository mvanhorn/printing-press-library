// Package trust handles bundle signing, key registration, and offline verification.
// The v1 implementation stores Ed25519 private keys on disk at
// ~/.config/pp/salesforce-headless-360/keys/<org>/<host>_<user>/private.pem.
// OS keychain storage is reserved as a v1.x hardening drop-in; the Signer
// interface here is the seam.
package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

// Signer abstracts bundle signing. v1 is Ed25519-on-disk. TPM / Yubikey / HSM
// are reserved drop-ins that implement the same interface.
type Signer interface {
	Sign(payload []byte) ([]byte, error)
	SignWriteIntent(claims WriteIntentClaims) ([]byte, error)
	VerifyWriteIntent(jws []byte) (WriteIntentClaims, error)
	PublicKeyPEM() string
	KID() string
}

// Verifier verifies a JWS against a public key.
type Verifier interface {
	Verify(jws string) (payload []byte, header map[string]any, err error)
}

// FileSigner is the v1 implementation. Private key on disk in PEM format.
type FileSigner struct {
	priv    ed25519.PrivateKey
	pub     ed25519.PublicKey
	kid     string
	keyPath string
}

// NewFileSigner loads or generates a keypair for the given org at the default
// location. If the private key file does not exist, a new keypair is
// generated and written.
func NewFileSigner(orgAlias string) (*FileSigner, error) {
	identity := LocalIdentity()
	return NewFileSignerWithIdentity(orgAlias, identity.HostFingerprint, identity.UserID)
}

// LocalIdentity identifies this device/user for KID derivation. Environment
// variables let tests and CI keep the value deterministic.
type LocalIdentityInfo struct {
	HostFingerprint string
	UserID          string
}

func LocalIdentity() LocalIdentityInfo {
	hostFingerprint := os.Getenv("SF360_HOST_FINGERPRINT")
	if hostFingerprint == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown-host"
		}
		sum := sha256.Sum256([]byte(hostname))
		hostFingerprint = hex.EncodeToString(sum[:])
	}
	userID := os.Getenv("SF360_USER_ID")
	if userID == "" {
		if current, err := user.Current(); err == nil && current.Username != "" {
			userID = current.Username
		}
	}
	if userID == "" {
		userID = "unknown-user"
	}
	return LocalIdentityInfo{
		HostFingerprint: sanitizeKIDPart(hostFingerprint),
		UserID:          sanitizeKIDPart(userID),
	}
}

// NewFileSignerWithIdentity loads or generates the current keypair for an
// org/device/user tuple.
func NewFileSignerWithIdentity(orgAlias, hostFingerprint, userID string) (*FileSigner, error) {
	return newFileSigner(orgAlias, hostFingerprint, userID, false)
}

// GenerateFileSignerWithIdentity creates a fresh keypair for an
// org/device/user tuple, replacing the current private key while preserving
// any old public KeyRecord used for verification.
func GenerateFileSignerWithIdentity(orgAlias, hostFingerprint, userID string) (*FileSigner, error) {
	return newFileSigner(orgAlias, hostFingerprint, userID, true)
}

func newFileSigner(orgAlias, hostFingerprint, userID string, forceNew bool) (*FileSigner, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	dir := currentKeyDir(home, orgAlias, hostFingerprint, userID)
	privPath := filepath.Join(dir, "private.pem")

	if !forceNew {
		if data, err := os.ReadFile(privPath); err == nil {
			return signerFromPEMWithIdentity(data, privPath, hostFingerprint, userID)
		}
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir key dir: %w", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ed25519 generate: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private: %w", err)
	}
	privBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: privDER}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privBlock), 0o600); err != nil {
		return nil, fmt.Errorf("write private: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("marshal public: %w", err)
	}
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}
	pubPath := filepath.Join(dir, "public.pem")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(pubBlock), 0o644); err != nil {
		return nil, fmt.Errorf("write public: %w", err)
	}

	signer := &FileSigner{
		priv:    priv,
		pub:     pub,
		kid:     computeKIDWithIdentity(pub, hostFingerprint, userID),
		keyPath: privPath,
	}
	if err := PutKeyPair(signer.kid, orgAlias, priv); err != nil {
		return nil, err
	}
	return signer, nil
}

func signerFromPEM(data []byte, path string) (*FileSigner, error) {
	identity := LocalIdentity()
	return signerFromPEMWithIdentity(data, path, identity.HostFingerprint, identity.UserID)
}

func signerFromPEMWithIdentity(data []byte, path, hostFingerprint, userID string) (*FileSigner, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM at %s", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unsupported key type at %s", path)
	}
	pub := priv.Public().(ed25519.PublicKey)
	return &FileSigner{
		priv:    priv,
		pub:     pub,
		kid:     computeKIDWithIdentity(pub, hostFingerprint, userID),
		keyPath: path,
	}, nil
}

// PutKeyPair persists a private key under keys/by-kid/<kid>/private.pem so
// callers can reload a signer by KID even after rotation changes the current
// org key.
func PutKeyPair(kid, orgAlias string, priv ed25519.PrivateKey) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "pp", "salesforce-headless-360", "keys", "by-kid", kid)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir key dir: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal private: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}), 0o600); err != nil {
		return fmt.Errorf("write private: %w", err)
	}
	meta := []byte(fmt.Sprintf("{\n  \"kid\": %q,\n  \"org_alias\": %q\n}\n", kid, orgAlias))
	return os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o600)
}

// GetSignerByKid loads a private key previously persisted by PutKeyPair.
func GetSignerByKid(kid string) (*FileSigner, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	path := filepath.Join(home, ".config", "pp", "salesforce-headless-360", "keys", "by-kid", kid, "private.pem")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := signerFromPEM(data, path)
	if err != nil {
		return nil, err
	}
	signer.kid = kid
	return signer, nil
}

// Sign returns the Ed25519 signature bytes over payload.
func (s *FileSigner) Sign(payload []byte) ([]byte, error) {
	return ed25519.Sign(s.priv, payload), nil
}

// PublicKeyPEM returns the signer's public key in PEM format.
func (s *FileSigner) PublicKeyPEM() string {
	pubDER, err := x509.MarshalPKIXPublicKey(s.pub)
	if err != nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
}

// KID returns the stable key identifier: base64url of the first 16 bytes of
// sha256(public_key). Used in JWS headers so verifiers can look up the right
// key.
func (s *FileSigner) KID() string {
	return s.kid
}

// KeyPath returns the on-disk location of the private key for diagnostic
// purposes (doctor, trust list-keys).
func (s *FileSigner) KeyPath() string {
	return s.keyPath
}

// PublicKeyBytes returns raw Ed25519 public key bytes (32 bytes).
func (s *FileSigner) PublicKeyBytes() []byte {
	return s.pub
}

func computeKID(pub ed25519.PublicKey) string {
	return computeKIDWithIdentity(pub, "", "")
}

func computeKIDWithIdentity(pub ed25519.PublicKey, hostFingerprint, userID string) string {
	h := sha256.Sum256(pub)
	base := b64url(h[:16])
	hostFingerprint = sanitizeKIDPart(hostFingerprint)
	userID = sanitizeKIDPart(userID)
	if hostFingerprint == "" && userID == "" {
		return base
	}
	if len(hostFingerprint) > 8 {
		hostFingerprint = hostFingerprint[:8]
	}
	if hostFingerprint == "" {
		hostFingerprint = "unknown"
	}
	if userID == "" {
		userID = "unknown-user"
	}
	return base + "_" + hostFingerprint + "_" + userID
}

var kidPartRE = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func currentKeyDir(home, orgAlias, hostFingerprint, userID string) string {
	base := filepath.Join(home, ".config", "pp", "salesforce-headless-360", "keys", orgAlias)
	hostFingerprint = sanitizeKIDPart(hostFingerprint)
	userID = sanitizeKIDPart(userID)
	if len(hostFingerprint) > 8 {
		hostFingerprint = hostFingerprint[:8]
	}
	if hostFingerprint == "" || userID == "" {
		return base
	}
	return filepath.Join(base, hostFingerprint+"_"+userID)
}

func sanitizeKIDPart(s string) string {
	s = strings.TrimSpace(s)
	s = kidPartRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	return s
}
