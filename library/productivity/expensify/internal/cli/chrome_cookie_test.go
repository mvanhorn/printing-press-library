// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestDecryptChromeCookie_V20IsAppBound(t *testing.T) {
	enc := append([]byte("v20"), bytes.Repeat([]byte{0x01}, 16)...)
	_, err := decryptChromeCookie(enc, make([]byte, 16))
	if err == nil {
		t.Fatal("expected App-Bound Encryption error for v20")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("App-Bound")) {
		t.Errorf("error should mention App-Bound Encryption, got %v", err)
	}
}

func TestDecryptChromeCookie_V10RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 16)
	plaintext := []byte("auth-token-value-1234567890")
	// Encrypt the same way Chrome v10 does: AES-128-CBC, IV = 16 spaces, PKCS7.
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	iv := bytes.Repeat([]byte{' '}, aes.BlockSize)
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	enc := append([]byte("v10"), ct...)

	got, err := decryptChromeCookie(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != string(plaintext) {
		t.Errorf("round-trip mismatch: want %q got %q", plaintext, got)
	}
}

func TestPkcs7Unpad(t *testing.T) {
	in := append([]byte("abc"), 0x03, 0x03, 0x03)
	if got := string(pkcs7Unpad(in)); got != "abc" {
		t.Errorf("want abc, got %q", got)
	}
	// Invalid padding is returned unchanged (defensive).
	bad := []byte{0x01, 0xff}
	if got := pkcs7Unpad(bad); len(got) != len(bad) {
		t.Errorf("invalid padding should be left intact")
	}
}

func TestAllPrintableASCII(t *testing.T) {
	if !allPrintableASCII([]byte("hello-123")) {
		t.Error("expected printable")
	}
	if allPrintableASCII([]byte{0x00, 0x41}) {
		t.Error("expected non-printable for NUL byte")
	}
}

// pkcs7Pad is a test helper mirroring the padding decryptChromeCookie expects.
func pkcs7Pad(b []byte, blockSize int) []byte {
	pad := blockSize - len(b)%blockSize
	return append(b, bytes.Repeat([]byte{byte(pad)}, pad)...)
}
