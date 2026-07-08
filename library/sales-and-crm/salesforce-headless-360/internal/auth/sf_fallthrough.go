package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var minSFVersion = version{major: 2, minor: 60, patch: 0}

type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

type SFOptions struct {
	Runner      CommandRunner
	LookPath    func(string) (string, error)
	VersionText string
}

func LoginSF(ctx context.Context, alias string, opts SFOptions) (*Result, error) {
	if strings.TrimSpace(alias) == "" {
		return nil, &Error{Kind: "sf_alias", Message: "sf alias is required"}
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath("sf"); err != nil {
		return nil, &Error{Kind: "sf_not_installed", Message: "sf not installed", Hint: "install Salesforce CLI and ensure sf is on PATH", Err: err}
	}
	runner := opts.Runner
	if runner == nil {
		runner = runCommand
	}
	versionText := opts.VersionText
	if versionText == "" {
		out, err := runner(ctx, "sf", "--version")
		if err != nil {
			return nil, &Error{Kind: "sf_version", Message: "checking sf version", Err: err}
		}
		versionText = string(out)
	}
	got, err := parseSFVersion(versionText)
	if err != nil {
		return nil, &Error{Kind: "sf_version", Message: "parsing sf version", Err: err}
	}
	if got.less(minSFVersion) {
		return nil, &Error{Kind: "sf_version", Message: fmt.Sprintf("sf version %s is too old", got), Hint: "upgrade Salesforce CLI to >= 2.60.0"}
	}
	out, err := runner(ctx, "sf", "org", "display", "--target-org", alias, "--verbose", "--json")
	if err != nil {
		return nil, &Error{Kind: "sf_org_display", Message: "running sf org display", Err: err}
	}
	var payload struct {
		Result struct {
			AccessToken string `json:"accessToken"`
			InstanceURL string `json:"instanceUrl"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, &Error{Kind: "sf_json", Message: "parsing sf org display JSON", Err: err}
	}
	if payload.Result.AccessToken == "" || payload.Result.InstanceURL == "" {
		return nil, &Error{Kind: "sf_json", Message: "sf org display missing accessToken or instanceUrl"}
	}
	return &Result{
		AccessToken: payload.Result.AccessToken,
		InstanceURL: payload.Result.InstanceURL,
		AuthMethod:  MethodSFFallthrough,
		TokenType:   "Bearer",
	}, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type version struct {
	major int
	minor int
	patch int
}

func parseSFVersion(text string) (version, error) {
	re := regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)\b`)
	m := re.FindStringSubmatch(text)
	if len(m) != 4 {
		return version{}, fmt.Errorf("no semantic version in %q", strings.TrimSpace(text))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return version{major: major, minor: minor, patch: patch}, nil
}

func (v version) less(other version) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

func (v version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}
