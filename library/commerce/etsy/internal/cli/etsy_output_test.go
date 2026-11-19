package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEtsyAuditOutputSafety(t *testing.T) {
	for _, kind := range []string{"fresh", "regular", "symlink", "dangling", "directory"} {
		for _, force := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "", true: "-force"}[force], func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "plan.json")
				target := filepath.Join(dir, "target")
				if kind == "regular" {
					if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "directory" {
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "symlink" || kind == "dangling" {
					if kind == "symlink" {
						if err := os.WriteFile(target, []byte("target original"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				_, err := shopxAuditWritePlan(path, nil, force)
				success := kind == "fresh" || kind == "regular" && force
				if success {
					if err != nil {
						t.Fatal(err)
					}
					body, err := os.ReadFile(path)
					if err != nil || !strings.Contains(string(body), `"generated_by":"listings audit"`) {
						t.Fatalf("plan not written: %s %v", body, err)
					}
					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
						t.Fatalf("mode=%o", info.Mode().Perm())
					}
				} else {
					if err == nil {
						t.Fatal("unsafe output accepted")
					}
					if !strings.Contains(err.Error(), path) {
						t.Fatalf("error lacks path: %v", err)
					}
					if kind == "regular" {
						body, _ := os.ReadFile(path)
						if string(body) != "original" || !strings.Contains(err.Error(), "--force") {
							t.Fatalf("existing output changed or missing force hint: %s %v", body, err)
						}
					}
				}
				if kind == "symlink" {
					body, _ := os.ReadFile(target)
					if string(body) != "target original" {
						t.Fatal("symlink target changed")
					}
				}
				if kind == "dangling" {
					if _, err := os.Lstat(target); !os.IsNotExist(err) {
						t.Fatal("dangling target created")
					}
				}
			})
		}
	}
}
