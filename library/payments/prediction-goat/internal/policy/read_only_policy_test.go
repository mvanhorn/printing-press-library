// Package policy holds structural CI tests that enforce the read-only
// guarantee for prediction-goat-pp-cli. If a new PR introduces a trading
// endpoint path, a wallet/signing import, or an order-placement helper,
// this test fails and the CI workflow `.github/workflows/read-only-lint.yml`
// fails alongside it.
package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNoTradingEndpoints(t *testing.T) {
	banned := []string{
		"/clob/orders",
		"/cancel-orders",
		"/post-orders",
		"/cancel-all",
		"/portfolio/orders",
		"/portfolio/orders/batched",
		"/order_groups",
		"/api_keys/generate",
	}
	scanFiles(t, func(path string, data []byte) {
		s := string(data)
		for _, b := range banned {
			pat := regexp.MustCompile(`["']` + regexp.QuoteMeta(b) + `["']`)
			if pat.MatchString(s) {
				t.Errorf("%s: contains banned trading endpoint reference %q", path, b)
			}
		}
	})
}

func TestNoWalletOrSigningImports(t *testing.T) {
	banned := []string{
		"github.com/ethereum/go-ethereum/crypto",
		"github.com/ethereum/go-ethereum/accounts",
		"github.com/ethereum/go-ethereum/signer",
		"github.com/ethereum/go-ethereum/wallet",
		"\"crypto/ecdsa\"",
		"github.com/polymarket/go-order-utils",
		"github.com/polymarket/go-clob-client",
	}
	scanFiles(t, func(path string, data []byte) {
		s := string(data)
		for _, b := range banned {
			if strings.Contains(s, b) {
				t.Errorf("%s: contains banned import %q", path, b)
			}
		}
	})
}

func TestNoOrderPlacementHelpers(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`func[^{\n]*[Pp]laceOrder`),
		regexp.MustCompile(`func[^{\n]*[Cc]reateOrder`),
		regexp.MustCompile(`func[^{\n]*[Ss]ignOrder`),
		regexp.MustCompile(`func[^{\n]*[Ss]ubmitTrade`),
		regexp.MustCompile(`func[^{\n]*[Ss]ignAndSubmit`),
	}
	scanFiles(t, func(path string, data []byte) {
		s := string(data)
		for _, p := range patterns {
			if p.MatchString(s) {
				t.Errorf("%s: contains banned order-placement helper matching %q", path, p.String())
			}
		}
	})
}

// scanFiles walks the module root and invokes fn for each .go file that
// is not a test file, generated golden fixture, or the policy test itself.
func scanFiles(t *testing.T, fn func(path string, data []byte)) {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "testdata" || base == "golden" || base == ".git" || base == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(path, data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func moduleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd, nil
		}
		dir = parent
	}
}
