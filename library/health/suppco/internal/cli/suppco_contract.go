package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/provider"
)

const printingPressMockAuthorization = "Bearer mock-token-for-testing"

func init() {
	registerClientHook(configureSuppCoClient)
}

func configureSuppCoClient(c *client.Client) error {
	return provider.ConfigureClient(c, allowPrintingPressMockOrigin(c))
}

// PrintingPress's verifier owns a loopback server and injects this fixed,
// non-secret credential. Binding all three facts prevents verify-like ambient
// environment variables from redirecting a real saved bearer token.
func allowPrintingPressMockOrigin(c *client.Client) bool {
	if !cliutil.IsVerifyEnv() || !cliutil.IsVerifyLiveHTTPEnv() || c == nil || c.Config == nil {
		return false
	}
	authorization := strings.TrimSpace(c.Config.AuthHeader())
	return authorization == printingPressMockAuthorization
}
