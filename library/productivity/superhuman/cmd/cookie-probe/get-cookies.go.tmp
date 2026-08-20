package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

func main() {
	tmp, _ := os.MkdirTemp("", "sh-c-*")
	defer os.RemoveAll(tmp)
	cookiesPath := os.ExpandEnv("$HOME/Library/Application Support/Google/Chrome/Default/Cookies")
	snap := filepath.Join(tmp, "Cookies")
	copyFile(cookiesPath, snap)
	pwBytes, _ := exec.Command("security", "find-generic-password", "-s", "Chrome Safe Storage", "-w").Output()
	pw := strings.TrimSpace(string(pwBytes))
	key := pbkdf2.Key([]byte(pw), []byte("saltysalt"), 1003, 16, sha1.New)
	iv := bytes.Repeat([]byte{' '}, 16)
	db, _ := sql.Open("sqlite", snap)
	defer db.Close()
	host := os.Args[1]
	rows, _ := db.Query("SELECT name, encrypted_value FROM cookies WHERE host_key = ?", host)
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var name string
		var enc []byte
		rows.Scan(&name, &enc)
		if !bytes.HasPrefix(enc, []byte("v10")) {
			parts = append(parts, name+"="+string(enc))
			continue
		}
		block, _ := aes.NewCipher(key)
		ct := enc[3:]
		if len(ct)%aes.BlockSize != 0 {
			continue
		}
		pt := make([]byte, len(ct))
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(pt, ct)
		pad := int(pt[len(pt)-1])
		if pad > aes.BlockSize {
			continue
		}
		pt = pt[:len(pt)-pad]
		if len(pt) > 32 {
			pt = pt[32:]
		}
		parts = append(parts, name+"="+string(pt))
	}
	fmt.Println(strings.Join(parts, "; "))
}

func copyFile(src, dst string) error {
	s, _ := os.Open(src); defer s.Close()
	d, _ := os.Create(dst); defer d.Close()
	io.Copy(d, s)
	return nil
}
