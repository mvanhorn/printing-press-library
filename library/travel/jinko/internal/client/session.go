package client

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CLI session TTL — mirrors @gojinko/cli (24h sliding window).
const sessionTTL = 24 * time.Hour

// LoadOrCreateCLISession returns a stable session UUID for ~/.jinko/.session.
// Refreshes the timestamp on every read so a steadily-used CLI keeps the
// same X-Session-ID, but an unused one rotates after the TTL.
//
// Format on disk: "<uuid>\n<unix-seconds>".
// Best-effort — falls back to an ephemeral UUID if the file can't be read/written.
func LoadOrCreateCLISession() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return newUUID(), err
	}
	dir := filepath.Join(home, ".jinko")
	path := filepath.Join(dir, ".session")

	if data, err := os.ReadFile(path); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
		if len(parts) == 2 {
			if ts, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				if time.Since(time.Unix(ts, 0)) < sessionTTL {
					_ = writeSession(path, parts[0])
					return parts[0], nil
				}
			}
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return newUUID(), err
	}
	id := newUUID()
	if err := writeSession(path, id); err != nil {
		return id, err
	}
	return id, nil
}

func writeSession(path, id string) error {
	content := fmt.Sprintf("%s\n%d\n", id, time.Now().Unix())
	return os.WriteFile(path, []byte(content), 0o600)
}

// ClearCLISession removes ~/.jinko/.session, rotating the correlation id
// on the next CLI run. Mirrors @gojinko/cli's auth-logout behaviour.
func ClearCLISession() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".jinko", ".session")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func newUUID() string {
	// RFC 4122 v4 — same generator semantics as crypto.randomUUID() in the TS CLI.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", 32)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// NewTraceID returns a 32-hex-char W3C trace id.
func NewTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewSpanID returns a 16-hex-char W3C span id.
func NewSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
