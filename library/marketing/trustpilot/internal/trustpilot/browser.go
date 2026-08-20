package trustpilot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SeedURL is the page we hit during cookie harvest. Picking a popular review
// page (trustpilot.com/review/trustpilot.com) keeps the first-load behavior
// well-trodden and avoids edge cases that empty / niche pages can have.
const SeedURL = "https://www.trustpilot.com/review/trustpilot.com"

// SearchSeedURL backfills the search-pages build id after the primary harvest.
const SearchSeedURL = "https://www.trustpilot.com/search?query=trustpilot"

// HarvestOptions controls the Chrome harvest behavior.
type HarvestOptions struct {
	ChromeExecutable string // path to a Chrome/Chromium binary; auto-detected if empty
	Headed           bool   // open a visible window (default: headless)
	Timeout          time.Duration
}

// HarvestSession opens a one-shot Chrome session via agent-browser, navigates
// to the seed URLs, and extracts the aws-waf-token cookie plus both Next.js
// build ids needed by the JSON-API replay path. Returns a populated Session
// ready to feed into NewClient.
//
// Failure modes (each returns a typed error chain that contains the original
// agent-browser stderr where helpful):
//   - agent-browser binary missing
//   - Chrome cannot launch
//   - WAF challenge fails to clear within Timeout
//   - __NEXT_DATA__ extraction returns empty buildId
//
// This function is intentionally a thin shell-out: chromedp is heavier and
// brings a half-dozen transitive deps, and we only need a one-shot harvest.
func HarvestSession(ctx context.Context, opts HarvestOptions) (Session, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 90 * time.Second
	}
	if _, err := exec.LookPath("agent-browser"); err != nil {
		return Session{}, fmt.Errorf("agent-browser not on PATH; install with `brew install agent-browser` or `npm install -g agent-browser`")
	}
	if opts.ChromeExecutable == "" {
		opts.ChromeExecutable = detectChrome()
		if opts.ChromeExecutable == "" {
			return Session{}, fmt.Errorf("Chrome not found; install Chrome or set AGENT_BROWSER_EXECUTABLE_PATH")
		}
	}

	hctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	env := []string{
		"AGENT_BROWSER_EXECUTABLE_PATH=" + opts.ChromeExecutable,
	}
	if !opts.Headed {
		env = append(env, "AGENT_BROWSER_HEADED=0")
	}

	// Step 1: open review seed, wait for network idle, dump __NEXT_DATA__ and cookies.
	if _, err := runAgentBrowser(hctx, env, "open", SeedURL); err != nil {
		return Session{}, fmt.Errorf("open review seed: %w", err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = runAgentBrowser(closeCtx, env, "close", "--all")
		closeCancel()
	}()
	if _, err := runAgentBrowser(hctx, env, "wait", "--load", "networkidle"); err != nil {
		return Session{}, fmt.Errorf("wait for review seed networkidle: %w", err)
	}
	reviewsBlob, err := runAgentBrowser(hctx, env, "eval", `document.getElementById("__NEXT_DATA__")?.textContent || ""`, "--json")
	if err != nil {
		return Session{}, fmt.Errorf("extract review __NEXT_DATA__: %w", err)
	}
	reviewsBuildID, err := buildIDFromBlob(reviewsBlob)
	if err != nil {
		return Session{}, fmt.Errorf("parse review buildId: %w", err)
	}
	cookieBlob, err := runAgentBrowser(hctx, env, "eval", `document.cookie`, "--json")
	if err != nil {
		return Session{}, fmt.Errorf("extract document.cookie: %w", err)
	}
	cookieJar, err := stringFromEvalEnvelope(cookieBlob)
	if err != nil {
		return Session{}, fmt.Errorf("decode cookie jar: %w", err)
	}
	wafToken := extractCookie(cookieJar, "aws-waf-token")
	if wafToken == "" {
		return Session{}, fmt.Errorf("no aws-waf-token cookie issued; the WAF challenge did not complete")
	}

	// Step 2: hit the search seed in the same session to harvest the search build id.
	if _, err := runAgentBrowser(hctx, env, "open", SearchSeedURL); err != nil {
		return Session{}, fmt.Errorf("open search seed: %w", err)
	}
	if _, err := runAgentBrowser(hctx, env, "wait", "--load", "networkidle"); err != nil {
		return Session{}, fmt.Errorf("wait for search seed networkidle: %w", err)
	}
	searchBlob, err := runAgentBrowser(hctx, env, "eval", `document.getElementById("__NEXT_DATA__")?.textContent || ""`, "--json")
	if err != nil {
		return Session{}, fmt.Errorf("extract search __NEXT_DATA__: %w", err)
	}
	searchBuildID, err := buildIDFromBlob(searchBlob)
	if err != nil {
		return Session{}, fmt.Errorf("parse search buildId: %w", err)
	}

	return Session{
		AWSWAFToken:    wafToken,
		CookieJar:      EncodeCookieJar(cookieJar),
		ReviewsBuildID: reviewsBuildID,
		SearchBuildID:  searchBuildID,
		HarvestedAt:    time.Now(),
		UserAgent:      DefaultUserAgent,
	}, nil
}

func detectChrome() string {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}
	if env := os.Getenv("AGENT_BROWSER_EXECUTABLE_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func runAgentBrowser(ctx context.Context, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "agent-browser", args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		originalErr := fmt.Errorf("agent-browser %s: %w (output: %s)", strings.Join(args, " "), err, truncate(string(out), 200))
		// PATCH: Warm a cold agent-browser daemon once when eval/open races startup.
		if strings.Contains(strings.ToLower(string(out)+" "+err.Error()), "connection refused") {
			fmt.Fprintln(os.Stderr, "agent-browser daemon appears cold; warming up...")
			warmup := exec.CommandContext(ctx, "agent-browser", "open", "https://www.trustpilot.com")
			warmup.Env = append(os.Environ(), env...)
			_, _ = warmup.CombinedOutput()
			time.Sleep(500 * time.Millisecond)

			retry := exec.CommandContext(ctx, "agent-browser", args...)
			retry.Env = append(os.Environ(), env...)
			retryOut, retryErr := retry.CombinedOutput()
			if retryErr == nil {
				return string(retryOut), nil
			}
			return "", fmt.Errorf("agent-browser daemon is not responding after warmup; try running `agent-browser open https://www.trustpilot.com` manually: %w", originalErr)
		}
		return "", originalErr
	}
	return string(out), nil
}

// agent-browser eval --json wraps results in {"success":bool,"data":{"origin":string,"result":any}}.
// stringFromEvalEnvelope returns the string at .data.result.
func stringFromEvalEnvelope(raw string) (string, error) {
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Result any `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return "", fmt.Errorf("decoding eval envelope: %w (raw: %s)", err, truncate(raw, 200))
	}
	if !env.Success {
		return "", fmt.Errorf("agent-browser eval reported failure: %s", truncate(raw, 200))
	}
	switch v := env.Data.Result.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	default:
		// Some agent-browser builds return a structured object; coerce to JSON.
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

// buildIDFromBlob parses a __NEXT_DATA__ JSON string (the eval result) and
// returns its top-level buildId.
func buildIDFromBlob(envelope string) (string, error) {
	body, err := stringFromEvalEnvelope(envelope)
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", fmt.Errorf("__NEXT_DATA__ was empty (likely still loading or behind challenge)")
	}
	var nd struct {
		BuildID string `json:"buildId"`
	}
	if err := json.Unmarshal([]byte(body), &nd); err != nil {
		return "", fmt.Errorf("decoding __NEXT_DATA__: %w", err)
	}
	if nd.BuildID == "" {
		return "", fmt.Errorf("__NEXT_DATA__ lacked buildId")
	}
	return nd.BuildID, nil
}

func extractCookie(jar, name string) string {
	prefix := name + "="
	for _, p := range strings.Split(jar, ";") {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, prefix) {
			return strings.TrimPrefix(p, prefix)
		}
	}
	return ""
}
