package semantic

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kvmctl-semantic-cache")
	if err != nil {
		os.Exit(1)
	}
	_ = os.Setenv("KVMCTL_CACHE_DIR", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
