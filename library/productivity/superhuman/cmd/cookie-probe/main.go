package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

func main() {
	cookies, err := readDecryptCookies("accounts.superhuman.com")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Decrypted %d cookies for accounts.superhuman.com\n", len(cookies))

	// Build cookie jar
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://accounts.superhuman.com/")
	for name, val := range cookies {
		jar.SetCookies(u, []*http.Cookie{{Name: name, Value: val, Path: "/", Secure: true, HttpOnly: true}})
		fmt.Fprintf(os.Stderr, "  cookie %s: %d bytes\n", name, len(val))
	}

	client := &http.Client{Jar: jar}

	// Step 1: getCsrfToken
	fmt.Fprintln(os.Stderr, "\n=== POST /~backend/v3/sessions.getCsrfToken ===")
	csrfReq, _ := http.NewRequest("POST", "https://accounts.superhuman.com/~backend/v3/sessions.getCsrfToken", strings.NewReader("{}"))
	csrfReq.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	csrfReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	csrfReq.Header.Set("Origin", "https://mail.superhuman.com")
	csrfReq.Header.Set("Referer", "https://mail.superhuman.com/")
	resp, err := client.Do(csrfReq)
	if err != nil {
		fmt.Fprintln(os.Stderr, "csrf request:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d\n", resp.StatusCode)
	bodyPreview := string(body)
	if len(bodyPreview) > 300 {
		bodyPreview = bodyPreview[:300] + "..."
	}
	fmt.Fprintf(os.Stderr, "Body: %s\n", bodyPreview)
	if resp.StatusCode != 200 {
		os.Exit(1)
	}

	// Parse CSRF token from response
	var csrfResp struct {
		CSRFToken string `json:"csrfToken"`
		ExpiresIn int    `json:"expiresIn"`
	}
	if err := json.Unmarshal(body, &csrfResp); err != nil {
		fmt.Fprintln(os.Stderr, "parse csrf:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Got CSRF: len=%d, expires_in=%d\n", len(csrfResp.CSRFToken), csrfResp.ExpiresIn)

	// Step 2: getTokens with CSRF
	fmt.Fprintln(os.Stderr, "\n=== POST /~backend/v3/sessions.getTokens ===")
	tokReq, _ := http.NewRequest("POST", "https://accounts.superhuman.com/~backend/v3/sessions.getTokens", strings.NewReader("{}"))
	tokReq.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	tokReq.Header.Set("User-Agent", csrfReq.Header.Get("User-Agent"))
	tokReq.Header.Set("Origin", "https://mail.superhuman.com")
	tokReq.Header.Set("Referer", "https://mail.superhuman.com/")
	tokReq.Header.Set("x-superhuman-csrf-token", csrfResp.CSRFToken)
	resp2, err := client.Do(tokReq)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tokens request:", err)
		os.Exit(1)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d\n", resp2.StatusCode)
	bodyPreview2 := string(body2)
	// Redact long token-like values for display
	display := bodyPreview2
	if len(display) > 500 {
		display = display[:500] + "..."
	}
	// Detect JWT
	if strings.Contains(string(body2), "eyJ") {
		fmt.Fprintln(os.Stderr, "JWT FOUND IN RESPONSE")
	}
	fmt.Fprintf(os.Stderr, "Body (truncated): %s\n", display)
	fmt.Fprintf(os.Stderr, "Body total bytes: %d\n", len(body2))
}

func readDecryptCookies(host string) (map[string]string, error) {
	cookiesPath := os.ExpandEnv("$HOME/Library/Application Support/Google/Chrome/Default/Cookies")
	tmp, err := os.MkdirTemp("", "sh-cookies-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	snap := filepath.Join(tmp, "Cookies")
	if err := copyFile(cookiesPath, snap); err != nil {
		return nil, err
	}

	pwBytes, err := exec.Command("security", "find-generic-password", "-s", "Chrome Safe Storage", "-w").Output()
	if err != nil {
		return nil, err
	}
	pw := strings.TrimSpace(string(pwBytes))
	key := pbkdf2.Key([]byte(pw), []byte("saltysalt"), 1003, 16, sha1.New)
	iv := bytes.Repeat([]byte{' '}, 16)

	db, err := sql.Open("sqlite", snap)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name, encrypted_value FROM cookies WHERE host_key = ?", host)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name string
		var enc []byte
		if err := rows.Scan(&name, &enc); err != nil {
			continue
		}
		if !bytes.HasPrefix(enc, []byte("v10")) {
			out[name] = string(enc)
			continue
		}
		pt, err := decryptV10(enc[3:], key, iv)
		if err != nil {
			continue
		}
		if len(pt) > 32 {
			pt = pt[32:]
		}
		out[name] = string(pt)
	}
	return out, nil
}

func decryptV10(ct, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d not aligned", len(ct))
	}
	pt := make([]byte, len(ct))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(pt, ct)
	pad := int(pt[len(pt)-1])
	if pad > aes.BlockSize {
		return pt, nil
	}
	return pt[:len(pt)-pad], nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}
