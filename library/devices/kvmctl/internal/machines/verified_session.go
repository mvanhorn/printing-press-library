// PATCH(library): MAC-authenticated verified-session persistence parity.
package machines

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// VerifiedSessionStore persists only verified selections with HMAC integrity,
// atomic 0600 files, 0700 directories, and cross-process flock.
type VerifiedSessionStore struct {
	Path    string
	KeyPath string
	MaxAge  time.Duration
	Now     func() time.Time // injectable; defaults to time.Now
	Clock   func() time.Time // alias for Now
}

func (s *VerifiedSessionStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func (s *VerifiedSessionStore) keyPath() string {
	if s.KeyPath != "" {
		return s.KeyPath
	}
	return s.Path + ".key"
}

func (s *VerifiedSessionStore) maxAge() time.Duration {
	if s.MaxAge <= 0 {
		return time.Hour
	}
	return s.MaxAge
}

type verifiedPayload struct {
	Endpoint string  `json:"endpoint"`
	Machine  string  `json:"machine"`
	Port     int     `json:"port"`
	State    string  `json:"state"`
	Detail   string  `json:"detail"`
	At       float64 `json:"at"`
}

// envelope wraps payload with HMAC over canonical JSON.
type verifiedEnvelope struct {
	Payload verifiedPayload `json:"payload"`
	Mac     string          `json:"mac"`
}

func ensureDirSecure(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0700)
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("unsafe persistent directory: %s", dir)
	}
	return nil
}

func ensureFilePerm(path string, mode os.FileMode) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != mode.Perm() {
		return fmt.Errorf("unsafe persistent file: %s", path)
	}
	return nil
}

func atomicWriteSecure(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := ensureDirSecure(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	_ = os.Chmod(path, 0600)
	// fsync parent dir
	if f, err := os.Open(dir); err == nil {
		_ = f.Sync()
		f.Close()
	}
	return nil
}

func (s *VerifiedSessionStore) ensureKey() ([]byte, error) {
	kp := s.keyPath()
	dir := filepath.Dir(kp)
	if err := ensureDirSecure(dir); err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(kp); err == nil {
		if err := ensureFilePerm(kp, 0600); err != nil {
			return nil, err
		}
		return b, nil
	}
	// create new 32-byte secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	// O_EXCL create
	f, err := os.OpenFile(kp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			b, err2 := os.ReadFile(kp)
			if err2 != nil {
				return nil, err2
			}
			return b, nil
		}
		return nil, err
	}
	if _, err := f.Write(secret); err != nil {
		f.Close()
		return nil, err
	}
	_ = f.Sync()
	f.Close()
	_ = os.Chmod(kp, 0600)
	return secret, nil
}

func (s *VerifiedSessionStore) readKey() ([]byte, error) {
	kp := s.keyPath()
	b, err := os.ReadFile(kp)
	if err != nil {
		return nil, err
	}
	if err := ensureFilePerm(kp, 0600); err != nil {
		return nil, err
	}
	return b, nil
}

func canonicalJSON(v any) ([]byte, error) {
	// json.Marshal with sort_keys equivalent: Go's map iteration is not sorted,
	// but we use structs with deterministic fields. For payload we manually use struct.
	return json.Marshal(v)
}

// withLock runs fn while holding flock on <Path>.lock
func (s *VerifiedSessionStore) withLock(fn func() error) error {
	dir := filepath.Dir(s.Path)
	if err := ensureDirSecure(dir); err != nil {
		return err
	}
	lockPath := s.Path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = os.Chmod(lockPath, 0600)
	if err := lockExclusive(f); err != nil {
		return err
	}
	defer unlockExclusive(f)
	return fn()
}

// Save persists only Verified records with MAC.
func (s *VerifiedSessionStore) Save(rec SelectionRecord, endpoint string) error {
	if rec.State != Verified {
		return nil
	}
	if rec.Target.Name == "" || endpoint == "" {
		return fmt.Errorf("verified session requires target and endpoint")
	}
	detail := rec.Detail
	if len(detail) > 300 {
		detail = detail[:300]
	}
	payload := verifiedPayload{
		Endpoint: endpoint,
		Machine:  rec.Target.Name,
		Port:     rec.Target.Port,
		State:    string(rec.State),
		Detail:   detail,
		At:       float64(rec.At.UnixNano()) / 1e9,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// canonical: Python uses sort_keys:true separators (',',':') – Go's struct marshal already deterministic.
	var key []byte
	err = s.withLock(func() error {
		k, err := s.ensureKey()
		if err != nil {
			return err
		}
		key = k
		mac := hmac.New(sha256.New, key)
		mac.Write(raw)
		env := verifiedEnvelope{Payload: payload, Mac: hex.EncodeToString(mac.Sum(nil))}
		data, _ := json.Marshal(env)
		return atomicWriteSecure(s.Path, data)
	})
	return err
}

// Load returns a verified record if the persisted file authenticates, matches endpoint, is within MaxAge and port binding.
func (s *VerifiedSessionStore) Load(endpoint string, inv Inventory) (SelectionRecord, bool, error) {
	var rec SelectionRecord
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return rec, false, nil
		}
		return rec, false, err
	}
	if err := ensureFilePerm(s.Path, 0600); err != nil {
		return rec, false, err
	}
	key, err := s.readKey()
	if err != nil {
		return rec, false, nil
	}
	var env verifiedEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return rec, false, fmt.Errorf("authorization store integrity failure: %w", err)
	}
	payloadRaw, _ := json.Marshal(env.Payload)
	mac := hmac.New(sha256.New, key)
	mac.Write(payloadRaw)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(env.Mac)) {
		return rec, false, fmt.Errorf("authorization store integrity failure")
	}
	if env.Payload.Endpoint != endpoint || env.Payload.State != string(Verified) {
		return rec, false, nil
	}
	now := s.now()
	at := time.Unix(0, int64(env.Payload.At*1e9))
	if at.After(now.Add(s.maxAge())) || now.Sub(at) > s.maxAge() {
		return rec, false, nil
	}
	t, err := inv.Resolve(env.Payload.Machine)
	if err != nil {
		return rec, false, nil
	}
	if t.Port != env.Payload.Port {
		return rec, false, nil
	}
	rec = SelectionRecord{Target: t, State: Verified, Detail: env.Payload.Detail, At: at}
	return rec, true, nil
}
