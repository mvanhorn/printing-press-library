package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const confirmTokenTTL = 15 * time.Minute

type confirmTokenFile struct {
	Cancel map[string]confirmTokenEntry `json:"cancel"`
}

type confirmTokenEntry struct {
	ReservationID  string `json:"reservation_id"`
	Expires        string `json:"expires"`
	Reason         string `json:"reason,omitempty"`
	RefundAsCredit bool   `json:"refund_as_credit,omitempty"`
}

// CancelBinding is what the owner saw at preview time. The token carries it so a
// later cancel can only do what was previewed: same reservation, same reason, same
// refund destination.
type CancelBinding struct {
	Reason         string
	RefundAsCredit bool
}

var tokenFileMu sync.Mutex

// lockTokens serializes token use across processes. The mutex alone only covers one
// process, which would let two concurrent cancels spend a single-use token.
func lockTokens(home string) (func(), error) {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(home, "confirm-tokens.lock")
	for i := 0; i < 100; i++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- fixed lock filename under the CLI's own config dir
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		// A crash can strand the lock; it guards a millisecond of work, so old means dead.
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > 30*time.Second {
			_ = os.Remove(path)
			continue
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, fmt.Errorf("confirm token file is locked; try again")
}

func confirmTokensPath(home string) string {
	return filepath.Join(home, "confirm-tokens.json")
}

func loadConfirmTokens(home string) (confirmTokenFile, error) {
	path := confirmTokensPath(home)
	b, err := os.ReadFile(path) // #nosec G304 -- fixed filename under the CLI's own config dir
	if err != nil {
		if os.IsNotExist(err) {
			return confirmTokenFile{Cancel: map[string]confirmTokenEntry{}}, nil
		}
		return confirmTokenFile{}, err
	}
	var f confirmTokenFile
	if err := json.Unmarshal(b, &f); err != nil {
		return confirmTokenFile{}, fmt.Errorf("parse confirm tokens: %w", err)
	}
	if f.Cancel == nil {
		f.Cancel = map[string]confirmTokenEntry{}
	}
	return f, nil
}

func saveConfirmTokens(home string, f confirmTokenFile) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	if f.Cancel == nil {
		f.Cancel = map[string]confirmTokenEntry{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	path := confirmTokensPath(home)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func pruneExpiredCancel(f *confirmTokenFile) {
	now := time.Now()
	for token, entry := range f.Cancel {
		exp, err := time.Parse(time.RFC3339, entry.Expires)
		if err != nil || now.After(exp) {
			delete(f.Cancel, token)
		}
	}
}

// IssueCancelToken stores a single-use cancel confirmation token on disk.
func IssueCancelToken(home, reservationID string) (string, error) {
	return IssueBoundCancelToken(home, reservationID, CancelBinding{})
}

// IssueBoundCancelToken stores a single-use token bound to the previewed refund.
func IssueBoundCancelToken(home, reservationID string, binding CancelBinding) (string, error) {
	if home == "" {
		return "", fmt.Errorf("config home required for cancel tokens")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	tokenFileMu.Lock()
	defer tokenFileMu.Unlock()
	unlock, err := lockTokens(home)
	if err != nil {
		return "", err
	}
	defer unlock()
	f, err := loadConfirmTokens(home)
	if err != nil {
		return "", err
	}
	pruneExpiredCancel(&f)
	f.Cancel[token] = confirmTokenEntry{
		ReservationID:  reservationID,
		Expires:        time.Now().Add(confirmTokenTTL).UTC().Format(time.RFC3339),
		Reason:         binding.Reason,
		RefundAsCredit: binding.RefundAsCredit,
	}
	if err := saveConfirmTokens(home, f); err != nil {
		return "", err
	}
	return token, nil
}

// ValidateCancelToken checks a cancel confirmation token without consuming it.
func ValidateCancelToken(home, token, reservationID string) error {
	_, err := checkCancelToken(home, token, reservationID, false)
	return err
}

// PeekCancelBinding validates a token and returns its binding without consuming it.
func PeekCancelBinding(home, token, reservationID string) (CancelBinding, error) {
	return checkCancelToken(home, token, reservationID, false)
}

// ConsumeCancelToken validates and deletes a cancel confirmation token.
func ConsumeCancelToken(home, token, reservationID string) error {
	_, err := checkCancelToken(home, token, reservationID, true)
	return err
}

func checkCancelToken(home, token, reservationID string, consume bool) (CancelBinding, error) {
	if home == "" {
		return CancelBinding{}, fmt.Errorf("config home required for cancel tokens")
	}
	tokenFileMu.Lock()
	defer tokenFileMu.Unlock()
	unlock, err := lockTokens(home)
	if err != nil {
		return CancelBinding{}, err
	}
	defer unlock()
	f, err := loadConfirmTokens(home)
	if err != nil {
		return CancelBinding{}, err
	}
	pruneExpiredCancel(&f)
	entry, ok := f.Cancel[token]
	if !ok {
		return CancelBinding{}, fmt.Errorf("invalid or expired confirm token; run cancel preview again")
	}
	exp, err := time.Parse(time.RFC3339, entry.Expires)
	if err != nil || time.Now().After(exp) {
		delete(f.Cancel, token)
		_ = saveConfirmTokens(home, f)
		return CancelBinding{}, fmt.Errorf("confirm token expired; run cancel preview again")
	}
	if entry.ReservationID != reservationID {
		return CancelBinding{}, fmt.Errorf("confirm token does not match reservation %s", reservationID)
	}
	if consume {
		delete(f.Cancel, token)
		if err := saveConfirmTokens(home, f); err != nil {
			return CancelBinding{}, err
		}
	}
	return CancelBinding{Reason: entry.Reason, RefundAsCredit: entry.RefundAsCredit}, nil
}
