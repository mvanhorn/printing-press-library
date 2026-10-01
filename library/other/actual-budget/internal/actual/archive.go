// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrMissingKey means the budget is end-to-end encrypted and no encryption
// password was supplied.
var ErrMissingKey = errors.New("missing-key: budget is end-to-end encrypted; set ACTUAL_ENCRYPTION_PASSWORD")

// ErrDecrypt means the encryption password did not decrypt the budget.
var ErrDecrypt = errors.New("decrypt-failure: the encryption password is wrong")

// DeriveKey reproduces loot-core's createKeyBuffer: PBKDF2-SHA512 over the
// password with the server-provided salt string, 10000 iterations, 32 bytes.
func DeriveKey(password, salt string) ([]byte, error) {
	return pbkdf2.Key(sha512.New, password, []byte(salt), 10000, 32)
}

// Decrypt reverses loot-core's aes-256-gcm encrypt(): the ciphertext and the
// detached auth tag are concatenated for Go's AEAD Open.
func Decrypt(key, ciphertext []byte, meta *EncryptMeta) ([]byte, error) {
	if meta == nil {
		return ciphertext, nil
	}
	if meta.Algorithm != "" && meta.Algorithm != "aes-256-gcm" {
		return nil, fmt.Errorf("unsupported budget encryption algorithm %q", meta.Algorithm)
	}
	iv, err := base64.StdEncoding.DecodeString(meta.IV)
	if err != nil {
		return nil, fmt.Errorf("decoding encryption iv: %w", err)
	}
	tag, err := base64.StdEncoding.DecodeString(meta.AuthTag)
	if err != nil {
		return nil, fmt.Errorf("decoding encryption auth tag: %w", err)
	}
	if len(tag) != 16 {
		return nil, ErrDecrypt
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return nil, err
	}
	sealed := make([]byte, 0, len(ciphertext)+len(tag))
	sealed = append(sealed, ciphertext...)
	sealed = append(sealed, tag...)
	plain, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plain, nil
}

// DBFileName is the SQLite file inside an Actual budget zip and mirror dir.
const DBFileName = "db" + sqliteExt

const sqliteExt = ".sqlite"

// Decompressed size caps (zip-bomb guard). metadata.json is a few hundred
// bytes; the largest real budgets are tens of megabytes.
const (
	maxDBBytes   = 1 << 30
	maxMetaBytes = 1 << 20
)

// Archive is the content of a budget zip.
type Archive struct {
	DB       []byte
	Metadata []byte
}

// ExtractArchive pulls db.sqlite and metadata.json out of a budget zip. Like
// loot-core's importBuffer, both must sit in the archive root or together in
// exactly one directory.
func ExtractArchive(buf []byte) (*Archive, error) {
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("budget download is not a zip file (wrong encryption password or server response?): %w", err)
	}
	dbs := map[string]*zip.File{}
	metas := map[string]*zip.File{}
	for _, f := range zr.File {
		switch {
		case f.Name == DBFileName || strings.HasSuffix(f.Name, "/"+DBFileName):
			dbs[strings.TrimSuffix(f.Name, DBFileName)] = f
		case f.Name == "metadata.json" || strings.HasSuffix(f.Name, "/metadata.json"):
			metas[strings.TrimSuffix(f.Name, "metadata.json")] = f
		}
	}
	var dirs []string
	for d := range dbs {
		if _, ok := metas[d]; ok {
			dirs = append(dirs, d)
		}
	}
	dir := ""
	if _, ok := dbs[""]; !ok || metas[""] == nil {
		if len(dirs) != 1 {
			return nil, errors.New("budget zip does not contain db.sqlite and metadata.json together")
		}
		dir = dirs[0]
	}
	db, err := readEntry(dbs[dir], maxDBBytes)
	if err != nil {
		return nil, err
	}
	meta, err := readEntry(metas[dir], maxMetaBytes)
	if err != nil {
		return nil, err
	}
	return &Archive{DB: db, Metadata: meta}, nil
}

func readEntry(f *zip.File, maxBytes int) ([]byte, error) {
	if maxBytes < 0 || f.UncompressedSize64 > uint64(maxBytes) {
		return nil, fmt.Errorf("%s in budget zip exceeds %d bytes", f.Name, maxBytes)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s in budget zip: %w", f.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, int64(maxBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s in budget zip: %w", f.Name, err)
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("%s in budget zip exceeds %d bytes", f.Name, maxBytes)
	}
	return data, nil
}
