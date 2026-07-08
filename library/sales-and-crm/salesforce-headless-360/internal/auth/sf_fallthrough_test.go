package auth

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestLoginSFParsesOrgDisplay(t *testing.T) {
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch joined {
		case "--version":
			return []byte("sf @salesforce/cli/2.60.0 darwin-arm64"), nil
		case "org display --target-org prod --verbose --json":
			return []byte(`{"result":{"accessToken":"sf-token","instanceUrl":"https://example.my.salesforce.com"}}`), nil
		default:
			t.Fatalf("unexpected command: %s %s", name, joined)
			return nil, nil
		}
	}
	result, err := LoginSF(context.Background(), "prod", SFOptions{
		Runner:   runner,
		LookPath: func(string) (string, error) { return "/usr/bin/sf", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "sf-token" || result.InstanceURL != "https://example.my.salesforce.com" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.AuthMethod != MethodSFFallthrough {
		t.Fatalf("auth method = %q", result.AuthMethod)
	}
}

func TestLoginSFRequiresInstalledCLI(t *testing.T) {
	_, err := LoginSF(context.Background(), "prod", SFOptions{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	})
	var authErr *Error
	if !errors.As(err, &authErr) || authErr.Kind != "sf_not_installed" {
		t.Fatalf("expected sf_not_installed, got %v", err)
	}
}

func TestLoginSFVersionHint(t *testing.T) {
	_, err := LoginSF(context.Background(), "prod", SFOptions{
		LookPath:    func(string) (string, error) { return "/usr/bin/sf", nil },
		VersionText: "sf @salesforce/cli/2.59.0 darwin-arm64",
		Runner:      func(context.Context, string, ...string) ([]byte, error) { return nil, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "upgrade Salesforce CLI to >= 2.60.0") {
		t.Fatalf("expected upgrade hint, got %v", err)
	}
}
