package june

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/devices/juneoven/internal/cliutil"
)

// June app constants (hard-coded in the Android app, identical for all users).
const (
	APIBase       = "https://api.junelife.com"
	MessagingBase = "https://messaging.junelife.com"
	WSURL         = "wss://messaging.junelife.com/1/messaging/websocket/companion"
	UserAgent     = "okhttp/4.8.1"
	clientID      = "dcxqbcv2dY-G12elqDoAhCP8E12V0zC8XWThT-4U"                // #nosec G101 -- public OAuth client id baked into June's Android app, identical for all users; not a user secret
	clientSecret  = "tmoSUwt3OOZCcfMaIadAGD7-x-qPht85HkCgdvuhTKk1yFtfMcfJEyd" // #nosec G101 -- public OAuth client secret shipped in June's Android app, identical for all users; not a user secret
	appVersion    = "1.24.1.11"
)

// Identity is the per-oven secret bundle produced at pairing. Anyone holding it
// can control the oven; it is stored at 0600 and never printed.
type Identity struct {
	DeviceID     string `json:"device_id"`
	DeviceName   string `json:"device_name"`
	Password     string `json:"password"`
	Ed25519Seed  string `json:"ed25519_seed_hex"`
	OvenID       string `json:"oven_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// IdentityPath is the on-disk location of the identity file.
func IdentityPath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "identity.json"), nil
}

// LoadIdentity reads the stored identity, or returns a not-paired error.
func LoadIdentity() (*Identity, error) {
	path, err := IdentityPath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is the fixed per-user identity file under the config dir, not user-controlled input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no paired oven found — run 'juneoven-pp-cli pair' first")
		}
		return nil, err
	}
	var id Identity
	if err := json.Unmarshal(raw, &id); err != nil {
		return nil, fmt.Errorf("parsing identity: %w", err)
	}
	return &id, nil
}

// Save writes the identity atomically at 0600.
func (id *Identity) Save() error {
	path, err := IdentityPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(id, "", "  ") // #nosec G117 -- identity is deliberately serialized to the 0600 credential file on disk, never transmitted
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Signer builds the Ed25519 signer for this identity.
func (id *Identity) Signer() (*Signer, error) {
	seed, err := hex.DecodeString(id.Ed25519Seed)
	if err != nil {
		return nil, fmt.Errorf("decoding seed: %w", err)
	}
	return NewSigner(seed)
}
