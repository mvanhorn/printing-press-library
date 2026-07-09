package june

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/secretbox"
)

type srpVectors struct {
	Damm map[string]string `json:"damm"`
	SRP  struct {
		Password    string `json:"password"`
		SaltHex     string `json:"salt_hex"`
		BHex        string `json:"b_hex"`
		AHex        string `json:"A_hex"`
		XHex        string `json:"x_hex"`
		VerifierHex string `json:"verifier_hex"`
		BPubHex     string `json:"B_hex"`
		UHex        string `json:"u_hex"`
		SHex        string `json:"S_hex"`
		KHex        string `json:"K_hex"`
	} `json:"srp"`
	Secretbox struct {
		NonceHex         string `json:"nonce_hex"`
		Plaintext        string `json:"plaintext"`
		CompanionInfoB64 string `json:"companion_info_b64"`
	} `json:"secretbox"`
}

func loadSRPVectors(t *testing.T) srpVectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "vectors.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var v srpVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	return v
}

func mustHexInt(t *testing.T, s string) *big.Int {
	t.Helper()
	s = trim0x(s)
	n, ok := new(big.Int).SetString(s, 16)
	if !ok {
		t.Fatalf("bad hex int: %s", s)
	}
	return n
}

func trim0x(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}

func TestDammVector(t *testing.T) {
	v := loadSRPVectors(t)
	// The plugin/Python vector: buildShownCode('46605',3) == '46605037'.
	if want := v.Damm["buildShownCode('46605',3)"]; want != "" {
		if damm(want) != 0 {
			t.Errorf("damm(%q) should validate to 0", want)
		}
		// The last digit is the check digit over the first 7.
		base := want[:7]
		if got := damm(base); got != int(want[7]-'0') {
			t.Errorf("damm check digit mismatch: base=%s got=%d want=%c", base, got, want[7])
		}
	}
}

func TestSRPServerVector(t *testing.T) {
	v := loadSRPVectors(t)
	salt, _ := hex.DecodeString(trim0x(v.SRP.SaltHex))
	bBytes := mustHexInt(t, v.SRP.BHex).Bytes()
	srv := newSRPServer(v.SRP.Password, salt, bBytes)

	if got := srv.v; got.Cmp(mustHexInt(t, v.SRP.VerifierHex)) != 0 {
		t.Errorf("verifier mismatch")
	}
	if got := srv.B; got.Cmp(mustHexInt(t, v.SRP.BPubHex)) != 0 {
		t.Errorf("B mismatch")
	}
	A := mustHexInt(t, v.SRP.AHex)
	S := srv.secret(A)
	if got := hex.EncodeToString(S); got != trim0x(v.SRP.SHex) {
		t.Errorf("S mismatch:\n got %s\nwant %s", got, trim0x(v.SRP.SHex))
	}
	K := blake2b.Sum256(S)
	if got := hex.EncodeToString(K[:]); got != trim0x(v.SRP.KHex) {
		t.Errorf("K mismatch: got %s want %s", got, trim0x(v.SRP.KHex))
	}
}

func TestSecretboxVector(t *testing.T) {
	v := loadSRPVectors(t)
	kBytes, _ := hex.DecodeString(trim0x(loadSRPVectors(t).SRP.KHex))
	var K [32]byte
	copy(K[:], kBytes)
	nonceBytes, _ := hex.DecodeString(trim0x(v.Secretbox.NonceHex))
	var nonce [24]byte
	copy(nonce[:], nonceBytes)
	sealed := secretbox.Seal(nil, []byte(v.Secretbox.Plaintext), &nonce, &K)
	got := base64.StdEncoding.EncodeToString(append(nonce[:], sealed...))
	if got != v.Secretbox.CompanionInfoB64 {
		t.Errorf("secretbox mismatch:\n got %s\nwant %s", got, v.Secretbox.CompanionInfoB64)
	}
}
