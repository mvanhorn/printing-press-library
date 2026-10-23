package mcp

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveMCPMultipartFile_Base64Default(t *testing.T) {
	t.Setenv(mcpUploadRootEnvVar, "")
	path, cleanup, err := resolveMCPMultipartFile("file", "", base64.StdEncoding.EncodeToString([]byte("HELLO")))
	if err != nil {
		t.Fatalf("base64 content rejected: %v", err)
	}
	defer cleanup()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read staged: %v", err)
	}
	if string(b) != "HELLO" {
		t.Fatalf("staged = %q, want HELLO", b)
	}
	if filepath.Base(path) != "file" {
		t.Fatalf("default filename = %q, want the parameter name \"file\"", filepath.Base(path))
	}
}

func TestResolveMCPMultipartFile_FilenameHint(t *testing.T) {
	t.Setenv(mcpUploadRootEnvVar, "")
	path, cleanup, err := resolveMCPMultipartFile("file", "photo.png", base64.StdEncoding.EncodeToString([]byte("IMG")))
	if err != nil {
		t.Fatalf("hint rejected: %v", err)
	}
	cleanup()
	if filepath.Base(path) != "photo.png" {
		t.Fatalf("filename = %q, want photo.png", filepath.Base(path))
	}

	for _, bad := range []string{"", ".", "..", "a/b.png", "a\\b.png", "x\x00y", "a:b.png", "\u202eevil.png", strings.Repeat("x", 300) + ".png"} {
		p, c, err := resolveMCPMultipartFile("file", bad, base64.StdEncoding.EncodeToString([]byte("Z")))
		if err != nil {
			t.Fatalf("content with bad hint %q rejected: %v", bad, err)
		}
		if filepath.Base(p) != "file" {
			t.Fatalf("bad hint %q produced filename %q, want fallback \"file\"", bad, filepath.Base(p))
		}
		c()
	}
}

func TestResolveMCPMultipartFile_RejectsRawPathByDefault(t *testing.T) {
	t.Setenv(mcpUploadRootEnvVar, "")
	_, _, err := resolveMCPMultipartFile("file", "", "/etc/passwd")
	if err == nil {
		t.Fatal("default mode accepted a raw local path instead of base64 content")
	}
	if !strings.Contains(err.Error(), mcpUploadRootEnvVar) {
		t.Fatalf("error %q does not name %s", err, mcpUploadRootEnvVar)
	}
	if !strings.Contains(err.Error(), "base64") {
		t.Fatalf("error %q does not mention base64", err)
	}
}

func TestResolveMCPMultipartFile_RequiresString(t *testing.T) {
	t.Setenv(mcpUploadRootEnvVar, "")
	if _, _, err := resolveMCPMultipartFile("file", "", 123); err == nil {
		t.Fatal("non-string multipart value was accepted")
	}
}

func TestResolveMCPMultipartFile_RootModeConfines(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.bin"), []byte("INSIDE"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(mcpUploadRootEnvVar, root)

	path, cleanup, err := resolveMCPMultipartFile("file", "ignored.txt", "ok.bin")
	if err != nil {
		t.Fatalf("in-root relative path rejected: %v", err)
	}
	b, _ := os.ReadFile(path)
	if filepath.Base(path) != "ok.bin" {
		t.Fatalf("path-mode filename = %q, want ok.bin", filepath.Base(path))
	}
	cleanup()
	if string(b) != "INSIDE" {
		t.Fatalf("staged = %q, want INSIDE", b)
	}

	if _, _, err := resolveMCPMultipartFile("file", "", "../escape"); err == nil {
		t.Fatal("root mode accepted a traversal path")
	}
	if _, _, err := resolveMCPMultipartFile("file", "", "/etc/passwd"); err == nil {
		t.Fatal("root mode accepted an absolute path outside the root")
	}

	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, _, err := resolveMCPMultipartFile("file", "", "link"); err == nil {
		t.Fatal("root mode followed a symlink out of the root")
	}

	if err := os.Mkdir(filepath.Join(root, "adir"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveMCPMultipartFile("file", "", "adir"); err == nil {
		t.Fatal("root mode accepted a non-regular file")
	}
}
