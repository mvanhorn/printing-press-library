// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

//go:build windows

package airbnb

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	_ "modernc.org/sqlite"
	"golang.org/x/sys/windows"
)

// importChromeCookies extracts and decrypts Airbnb cookies from the local
// Chrome profile on Windows. It reads the AES key from "Local State"
// (DPAPI-encrypted), then decrypts each cookie's AES-256-GCM (v10/v11) value.
func importChromeCookies(profile string) (map[string]string, error) {
	if profile == "" {
		profile = "Default"
	}
	userData, err := chromeUserDataDir()
	if err != nil {
		return nil, err
	}

	aesKey, err := chromeAESKey(userData)
	if err != nil {
		return nil, err
	}

	cookiesDB := filepath.Join(userData, profile, "Network", "Cookies")
	if _, err := os.Stat(cookiesDB); err != nil {
		// Older Chrome kept Cookies at the profile root.
		alt := filepath.Join(userData, profile, "Cookies")
		if _, e2 := os.Stat(alt); e2 == nil {
			cookiesDB = alt
		} else {
			return nil, fmt.Errorf("Chrome cookies DB not found for profile %q under %s", profile, userData)
		}
	}

	// Chrome holds the DB open; copy to a temp file so we can read it without
	// contending for the lock. Chrome opens with FILE_SHARE_READ, so the copy
	// succeeds even while the browser is running.
	tmp, err := copyToTemp(cookiesDB)
	if err != nil {
		return nil, fmt.Errorf("copying Chrome cookies DB (try closing Chrome, or use `auth login --cookies`): %w", err)
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(tmp)+"?mode=ro&immutable=1")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name, encrypted_value FROM cookies WHERE host_key LIKE '%airbnb%'`)
	if err != nil {
		return nil, fmt.Errorf("querying cookies: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	var appBound int
	for rows.Next() {
		var name string
		var enc []byte
		if err := rows.Scan(&name, &enc); err != nil {
			continue
		}
		val, err := decryptChromeValue(enc, aesKey)
		if err != nil {
			appBound++
			continue
		}
		if val != "" {
			out[name] = val
		}
	}
	if len(out) == 0 && appBound > 0 {
		return nil, fmt.Errorf("Chrome cookies are app-bound-encrypted (Chrome 127+); this profile can't be read headlessly. Use `auth login --cookies \"<paste>\"` instead")
	}
	return out, nil
}

func chromeUserDataDir() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", fmt.Errorf("LOCALAPPDATA not set")
	}
	dir := filepath.Join(local, "Google", "Chrome", "User Data")
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("Chrome user data dir not found at %s", dir)
	}
	return dir, nil
}

// chromeAESKey reads Local State, base64-decodes os_crypt.encrypted_key, strips
// the 5-byte "DPAPI" prefix and DPAPI-decrypts the remainder into the AES key.
func chromeAESKey(userData string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(userData, "Local State"))
	if err != nil {
		return nil, fmt.Errorf("reading Local State: %w", err)
	}
	var ls struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("parsing Local State: %w", err)
	}
	if ls.OSCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("no encrypted_key in Local State")
	}
	blob, err := base64.StdEncoding.DecodeString(ls.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, err
	}
	if len(blob) < 5 || string(blob[:5]) != "DPAPI" {
		return nil, fmt.Errorf("unexpected encrypted_key format")
	}
	return dpapiDecrypt(blob[5:])
}

// decryptChromeValue decrypts a Chrome cookie value. v10/v11 values are
// AES-256-GCM: "v10"|"v11" prefix + 12-byte nonce + ciphertext + 16-byte tag.
// Older values without the prefix are DPAPI-encrypted.
func decryptChromeValue(enc, aesKey []byte) (string, error) {
	if len(enc) > 3 && (string(enc[:3]) == "v10" || string(enc[:3]) == "v11") {
		if len(enc) < 3+12+16 {
			return "", fmt.Errorf("short gcm value")
		}
		nonce := enc[3:15]
		ct := enc[15:]
		block, err := aes.NewCipher(aesKey)
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		pt, err := gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			return "", err
		}
		return string(pt), nil
	}
	// Legacy DPAPI-encrypted value.
	pt, err := dpapiDecrypt(enc)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// --- DPAPI (CryptUnprotectData) ---

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32                = windows.NewLazySystemDLL("crypt32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

func dpapiDecrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty DPAPI blob")
	}
	in := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %v", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	result := make([]byte, out.cbData)
	copy(result, unsafe.Slice(out.pbData, out.cbData))
	return result, nil
}

func copyToTemp(src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "airbnb-cookies-*.db")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
