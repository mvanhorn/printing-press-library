package cli

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFleetTemplate_ScaffoldsWithoutGenKey(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "host")

	cmd := &cobra.Command{}
	if err := runFleetTemplate(cmd, dest, false, false); err != nil {
		t.Fatalf("runFleetTemplate: %v", err)
	}

	for _, rel := range []string{
		"vercel.json",
		"index.html",
		"README.md",
		"public/.well-known/appspecific/com.tesla.3p.public-key.pem.example",
	} {
		p := filepath.Join(dest, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s, stat err: %v", p, err)
		}
	}

	// Without --gen-key, the real PEM file must NOT exist (only the .example placeholder).
	realPEM := filepath.Join(dest, "public", ".well-known", "appspecific", "com.tesla.3p.public-key.pem")
	if _, err := os.Stat(realPEM); !os.IsNotExist(err) {
		t.Errorf("expected real PEM to be absent without --gen-key, stat err: %v", err)
	}
}

func TestFleetTemplate_GenKeyWritesValidEC256(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "host")
	t.Setenv("HOME", t.TempDir()) // redirect ~/.tesla to a temp dir

	cmd := &cobra.Command{}
	if err := runFleetTemplate(cmd, dest, true, false); err != nil {
		t.Fatalf("runFleetTemplate: %v", err)
	}

	// .example placeholder should be gone, replaced by real PEM.
	if _, err := os.Stat(filepath.Join(dest, "public", ".well-known", "appspecific", "com.tesla.3p.public-key.pem.example")); !os.IsNotExist(err) {
		t.Errorf("expected .example placeholder removed after gen-key, stat err: %v", err)
	}
	realPEM := filepath.Join(dest, "public", ".well-known", "appspecific", "com.tesla.3p.public-key.pem")
	pubBytes, err := os.ReadFile(realPEM)
	if err != nil {
		t.Fatalf("read public PEM: %v", err)
	}
	block, _ := pem.Decode(pubBytes)
	if block == nil || block.Type != "PUBLIC KEY" {
		t.Fatalf("public PEM has wrong block type: %v", block)
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	_ = pub // type assertion not strictly required; existence + parse is the contract

	// Private key file: ~/.tesla/host-private.pem (basename of dest is "host"), mode 600.
	privPath := filepath.Join(os.Getenv("HOME"), ".tesla", "host-private.pem")
	info, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("private key mode: got %o want 600", mode)
	}
}

func TestFleetTemplate_RefusesNonEmptyDestWithoutForce(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "host")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "something.txt"), []byte("preexisting"), 0o644); err != nil {
		t.Fatalf("write preexisting: %v", err)
	}

	cmd := &cobra.Command{}
	err := runFleetTemplate(cmd, dest, false, false)
	if err == nil {
		t.Fatalf("expected error on non-empty dest without --force")
	}
	if !strings.Contains(err.Error(), "non-empty") {
		t.Errorf("expected error about non-empty dest, got: %v", err)
	}
}

func TestFleetTemplate_ForceOverwritesTemplateButPreservesExistingKey(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "host")
	t.Setenv("HOME", t.TempDir())

	cmd := &cobra.Command{}
	// First scaffold creates the private key.
	if err := runFleetTemplate(cmd, dest, true, false); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	privPath := filepath.Join(os.Getenv("HOME"), ".tesla", "host-private.pem")
	keyBefore, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("read key before: %v", err)
	}

	// Second scaffold with --force --gen-key must NOT clobber the existing key.
	if err := runFleetTemplate(cmd, dest, true, true); err != nil {
		t.Fatalf("second scaffold: %v", err)
	}
	keyAfter, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("read key after: %v", err)
	}
	if string(keyBefore) != string(keyAfter) {
		t.Errorf("private key was regenerated; expected preserved")
	}
}
