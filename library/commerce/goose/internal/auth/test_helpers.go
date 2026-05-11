package auth

import "encoding/base64"

// base64URLEncode is a small wrapper used only by tests so the test file
// stays readable. Lives in non-_test.go because go vet warns about test files
// exporting helpers; tests in the same package use this directly.
func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
