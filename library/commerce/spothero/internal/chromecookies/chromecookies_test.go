package chromecookies

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func encrypt(t *testing.T, key []byte, host, value string) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(host))
	plain := append(sum[:], []byte(value)...)
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, []byte(strings.Repeat(string(rune(pad)), pad))...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, []byte(strings.Repeat(" ", aes.BlockSize))).CryptBlocks(out, plain)
	return append([]byte("v10"), out...)
}

func fixtureDB(t *testing.T, key []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Cookies")
	db, err := sql.Open("sqlite", "file:"+p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE meta(key TEXT, value TEXT)`,
		`INSERT INTO meta VALUES('version','24')`,
		`CREATE TABLE cookies(host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ins := func(host, name string, enc []byte) {
		if _, err := db.Exec(`INSERT INTO cookies VALUES(?,?,'',?)`, host, name, enc); err != nil {
			t.Fatal(err)
		}
	}
	ins(".spothero.com", "AUTH_TOKEN", encrypt(t, key, ".spothero.com", "fixture.jwt.value"))
	ins("spothero.com", "csrftoken", encrypt(t, key, "spothero.com", "fixture-csrf"))
	// Host-only on the Auth0 subdomain, never sent to spothero.com.
	ins("accounts.spothero.com", "auth0", encrypt(t, key, "accounts.spothero.com", "not-for-the-api"))
	ins(".example.com", "AUTH_TOKEN", encrypt(t, key, ".example.com", "other-site"))
	return p
}

// The regression this package exists for: value is empty on disk, so reading it
// gives names with no values. Decryption must produce the real value.
func TestReadDecryptsEncryptedValueColumn(t *testing.T) {
	key, _ := Key("test-password", 1003)
	got, err := Read(context.Background(), fixtureDB(t, key), "spothero.com", key)
	if err != nil {
		t.Fatal(err)
	}
	if got["AUTH_TOKEN"] != "fixture.jwt.value" || got["csrftoken"] != "fixture-csrf" {
		t.Fatalf("wrong values for names %d", len(got))
	}
	if _, ok := got["auth0"]; ok {
		t.Fatal("imported a subdomain host-only cookie the API never receives")
	}
}

// With the wrong key nothing decrypts. That must surface as no cookies, never as
// a list of names with empty values.
func TestReadWrongKeyYieldsNoCookies(t *testing.T) {
	key, _ := Key("test-password", 1003)
	wrong, _ := Key("another-password", 1003)
	got, err := Read(context.Background(), fixtureDB(t, key), "spothero.com", wrong)
	if err == nil {
		for name, v := range got {
			if v == "" {
				t.Fatalf("%s imported with an empty value", name)
			}
		}
	}
}

func TestHostMatches(t *testing.T) {
	for host, want := range map[string]bool{".spothero.com": true, "spothero.com": true, "accounts.spothero.com": false, "notspothero.com": false} {
		if HostMatches(host, "spothero.com") != want {
			t.Errorf("%s: want %v", host, want)
		}
	}
}
