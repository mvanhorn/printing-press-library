// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// api_hpn_test.go: tests for the `api hpn` CLI command tree.
//
// Two layers of coverage live here:
//
//  1. Help-flag and dry-run smoke tests against the cobra command tree.
//     These confirm the parent and every subcommand registers, that
//     --help renders without panicking, and that --dry-run on a
//     credit-spending command does NOT actually require a network round
//     trip.
//
//  2. Renderer + classifier unit tests against helper functions
//     (runHpnSearch, emitHpnSearchEnvelope, classifyHpnError,
//     checkSearchBudget). These exercise the JSON envelope shape and
//     exit-code mapping without driving cobra. Full integration through
//     the bearer client lives in source_selection_test.go which already
//     uses the httptest fixture pattern.
//
// We intentionally do NOT exercise the cookie-quota code path here —
// source_selection_test.go owns that. This file owns the bearer-only
// surface.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

// newAPIHpnRootCmd returns the same command tree the binary wires up,
// minus everything that is not under `api hpn`. Used by help-flag and
// dry-run tests so we don't drag in unrelated init.
func newAPIHpnRootCmd(t *testing.T, flags *rootFlags) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "contact-goat-pp-cli", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	apiCmd := &cobra.Command{Use: "api"}
	apiCmd.AddCommand(newAPIHpnCmd(flags))
	root.AddCommand(apiCmd)
	return root
}

// runCmd executes the root command with the given argv and returns
// (stdout, stderr, err). Buffers are wired before execution so output
// goes to the buffers rather than os.Stdout / os.Stderr (which would
// pollute test output).
func runCmd(t *testing.T, root *cobra.Command, argv []string) (string, string, error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(argv)
	err := root.ExecuteContext(context.Background())
	return out.String(), errBuf.String(), err
}

// --- 1. Help-flag and registration smoke tests ---

// TestAPIHpn_HelpRegistration locks down that every subcommand in the
// plan's tree exists and renders --help without panicking. Each row of
// the table is one (path, expected substring) pair.
func TestAPIHpn_HelpRegistration(t *testing.T) {
	cases := []struct {
		name   string
		argv   []string
		wantIn string
	}{
		{"api hpn", []string{"api", "hpn", "--help"}, "Happenstance public REST API"},
		{"api hpn search", []string{"api", "hpn", "search", "--help"}, "Costs 2 credits"},
		{"api hpn search find-more", []string{"api", "hpn", "search", "find-more", "--help"}, "new page id"},
		{"api hpn search get", []string{"api", "hpn", "search", "get", "--help"}, "Free probe"},
		{"api hpn research", []string{"api", "hpn", "research", "--help"}, "Costs 1 credit per call"},
		{"api hpn research get", []string{"api", "hpn", "research", "get", "--help"}, "Free probe"},
		{"api hpn groups", []string{"api", "hpn", "groups", "--help"}, "Happenstance groups"},
		{"api hpn groups list", []string{"api", "hpn", "groups", "list", "--help"}, "List all Happenstance groups"},
		{"api hpn groups get", []string{"api", "hpn", "groups", "get", "--help"}, "single Happenstance group"},
		{"api hpn usage", []string{"api", "hpn", "usage", "--help"}, "credit balance"},
		{"api hpn user", []string{"api", "hpn", "user", "--help"}, "/v1/users/me"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{}
			root := newAPIHpnRootCmd(t, flags)
			out, _, err := runCmd(t, root, tc.argv)
			if err != nil {
				t.Fatalf("--help returned error: %v", err)
			}
			if !strings.Contains(out, tc.wantIn) {
				t.Errorf("--help output missing %q.\ngot: %s", tc.wantIn, out)
			}
		})
	}
}

// TestAPIHpn_FlagsRegistered confirms every documented flag is wired
// through cobra (so the verify-skill script will pass when SKILL.md is
// authored in unit 9). One row per (subcommand, flag).
func TestAPIHpn_FlagsRegistered(t *testing.T) {
	cases := []struct {
		path []string
		flag string
	}{
		{[]string{"api", "hpn", "search"}, "include-friends-connections"},
		{[]string{"api", "hpn", "search"}, "include-my-connections"},
		{[]string{"api", "hpn", "search"}, "group-id"},
		{[]string{"api", "hpn", "search"}, "budget"},
		{[]string{"api", "hpn", "search"}, "poll-timeout"},
		{[]string{"api", "hpn", "search"}, "poll-interval"},
		{[]string{"api", "hpn", "search", "find-more"}, "budget"},
		{[]string{"api", "hpn", "search", "get"}, "page-id"},
		{[]string{"api", "hpn", "research"}, "no-wait"},
		{[]string{"api", "hpn", "research"}, "budget"},
	}
	flags := &rootFlags{}
	root := newAPIHpnRootCmd(t, flags)
	for _, tc := range cases {
		t.Run(strings.Join(tc.path, " ")+"--"+tc.flag, func(t *testing.T) {
			cmd, _, err := root.Find(tc.path)
			if err != nil {
				t.Fatalf("Find(%v): %v", tc.path, err)
			}
			if cmd.Flags().Lookup(tc.flag) == nil {
				t.Errorf("flag --%s not registered on %s", tc.flag, strings.Join(tc.path, " "))
			}
		})
	}
}

// --- 2. Edge cases on the cobra layer (no network) ---

// TestAPIHpnSearch_EmptyText asserts the empty-text edge case from the
// plan: usage exit 2 with a clear message, no API call attempted.
func TestAPIHpnSearch_EmptyText(t *testing.T) {
	withEnv(t, api.KeyEnvVar, "hpn_live_personal_test")
	flags := &rootFlags{yes: true, noInput: true}
	root := newAPIHpnRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"api", "hpn", "search", "  "})
	if err == nil {
		t.Fatal("want usage error on empty text")
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("want *cliError, got %T (%v)", err, err)
	}
	if ce.code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", ce.code)
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty text, got: %v", err)
	}
}

// TestAPIHpnSearch_BudgetExceeded asserts the --budget refusal path
// from the plan: a 2-credit search call against --budget 1 exits 0
// with a "would exceed budget" notice, never hits the API.
func TestAPIHpnSearch_BudgetExceeded(t *testing.T) {
	withEnv(t, api.KeyEnvVar, "hpn_live_personal_test")
	flags := &rootFlags{yes: true, noInput: true, asJSON: true}
	root := newAPIHpnRootCmd(t, flags)
	out, _, err := runCmd(t, root, []string{"api", "hpn", "search", "VPs at NBA", "--budget", "1"})
	if err != nil {
		t.Fatalf("budget refusal should exit 0, got: %v", err)
	}
	if !strings.Contains(out, "would exceed budget") {
		t.Errorf("stdout should contain 'would exceed budget', got: %s", out)
	}
	var decoded map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &decoded); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", jsonErr, out)
	}
	if decoded["status"] != "skipped" {
		t.Errorf("status = %v, want \"skipped\"", decoded["status"])
	}
	if decoded["would_spend"].(float64) != 2 {
		t.Errorf("would_spend = %v, want 2", decoded["would_spend"])
	}
}

// TestAPIHpnSearch_NoAPIKey asserts the auth edge case: with no
// HAPPENSTANCE_API_KEY set, exit 4 with the canonical hint.
func TestAPIHpnSearch_NoAPIKey(t *testing.T) {
	withEnv(t, api.KeyEnvVar, "")
	flags := &rootFlags{yes: true, noInput: true}
	root := newAPIHpnRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"api", "hpn", "search", "VPs at NBA"})
	if err == nil {
		t.Fatal("want auth error when API key is missing")
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("want *cliError, got %T (%v)", err, err)
	}
	if ce.code != 4 {
		t.Errorf("exit code = %d, want 4 (auth required)", ce.code)
	}
	if !strings.Contains(err.Error(), api.KeyEnvVar) {
		t.Errorf("error should mention %s, got: %v", api.KeyEnvVar, err)
	}
}

// TestAPIHpnSearch_DryRunRedacts confirms that --dry-run does not leak
// the bearer key and surfaces the canonical RedactedBearerLine. Stderr
// is checked because the api client's printDryRun writes there.
func TestAPIHpnSearch_DryRunRedacts(t *testing.T) {
	const secret = "hpn_live_personal_should_not_appear_in_output"
	withEnv(t, api.KeyEnvVar, secret)
	flags := &rootFlags{dryRun: true, asJSON: true}
	root := newAPIHpnRootCmd(t, flags)

	// Capture os.Stderr while the command runs because the bearer
	// client's dry-run preview writes to os.Stderr directly (not
	// cmd.ErrOrStderr).
	out, errBuf, err := runCmdWithStderrCapture(t, root, []string{"api", "hpn", "search", "VPs at NBA"})
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	combined := out + errBuf
	if strings.Contains(combined, secret) {
		t.Errorf("bearer key leaked into output:\n%s", combined)
	}
	if !strings.Contains(errBuf, api.RedactedBearerLine) {
		t.Errorf("dry-run preview should contain %q, got stderr: %s", api.RedactedBearerLine, errBuf)
	}
}

// --- 3. runHpnSearch / runHpnResearch against httptest fixtures ---

// newFakeUserServer is a minimal httptest fixture that returns a canned
// {email, name, friends:[]} on /users/me. Used by TestAPIHpnUser_HappyPath.
func newFakeUserServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/me":
			_, _ = w.Write([]byte(`{"email":"matt@example.com","name":"Matt","friends":[{"name":"Alice","email":"alice@example.com"}]}`))
		case "/usage":
			_, _ = w.Write([]byte(`{"balance_credits":0,"has_credits":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

// TestAPIHpnUser_HappyPath_Helper exercises the user code path against
// an httptest fixture. We do not drive cobra here because cobra would
// build its own client via flags.newHappenstanceAPIClient(); instead we
// drive the api.Client directly to confirm the {email, name, friends}
// envelope decodes the way the cobra render path expects.
func TestAPIHpnUser_HappyPath_Helper(t *testing.T) {
	srv := newFakeUserServer(t)
	defer srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(srv.URL))
	u, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if u.Email != "matt@example.com" {
		t.Errorf("Email = %q, want matt@example.com", u.Email)
	}
	if u.Name != "Matt" {
		t.Errorf("Name = %q, want Matt", u.Name)
	}
	if len(u.Friends) != 1 || u.Friends[0].Name != "Alice" {
		t.Errorf("Friends = %v, want [Alice]", u.Friends)
	}
}

// TestAPIHpnUsage_HasCreditsFalse confirms /usage decoding surfaces
// has_credits:false correctly when the upstream returns balance 0.
func TestAPIHpnUsage_HasCreditsFalse(t *testing.T) {
	srv := newFakeUserServer(t)
	defer srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(srv.URL))
	u, err := c.Usage(context.Background())
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if u.BalanceCredits != 0 {
		t.Errorf("BalanceCredits = %d, want 0", u.BalanceCredits)
	}
	if u.HasCredits {
		t.Error("HasCredits = true, want false")
	}
}

// TestAPIHpnSearch_HappyPath exercises the POST + poll + render flow
// against the same httptest fixture used in source_selection_test.go.
// Confirms (a) the run helper drives the bearer client correctly and
// (b) the JSON envelope shape is jq-friendly with .results[].name.
func TestAPIHpnSearch_HappyPath(t *testing.T) {
	srv := newFakeBearerServer(t)
	defer srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(srv.URL))

	env, err := runHpnSearch(context.Background(), c, "VPs at NBA", &api.SearchOptions{}, &api.PollSearchOptions{})
	if err != nil {
		t.Fatalf("runHpnSearch: %v", err)
	}
	if env.Status != api.StatusCompleted {
		t.Errorf("Status = %q, want COMPLETED", env.Status)
	}
	if len(env.Results) != 2 {
		t.Fatalf("Results count = %d, want 2", len(env.Results))
	}

	// Build the JSON envelope the cobra render path emits, then walk it
	// like jq -r '.results[].name' would.
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "VPs at NBA"); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	var decoded struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
		Source string `json:"source"`
	}
	if jerr := json.Unmarshal(out.Bytes(), &decoded); jerr != nil {
		t.Fatalf("decode JSON envelope: %v\nout: %s", jerr, out.String())
	}
	if decoded.Source != "api" {
		t.Errorf("source = %q, want api", decoded.Source)
	}
	if len(decoded.Results) != 2 {
		t.Fatalf("decoded results count = %d, want 2", len(decoded.Results))
	}
	wantNames := []string{"Alice Example", "Bob Example"}
	for i, want := range wantNames {
		if decoded.Results[i].Name != want {
			t.Errorf("results[%d].name = %q, want %q", i, decoded.Results[i].Name, want)
		}
	}
}

// TestAPIHpnResearch_FailedAmbiguousExits5 mirrors the plan's
// FAILED_AMBIGUOUS surfacing case: the helper drives the bearer client
// against a fixture that returns FAILED_AMBIGUOUS, and the renderer
// returns an apiErr (exit 5).
func TestAPIHpnResearch_FailedAmbiguousExits5(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/research" {
			_, _ = w.Write([]byte(`{"id":"rsh_amb1","url":"https://happenstance.ai/r/rsh_amb1"}`))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/research/") {
			_, _ = w.Write([]byte(`{"id":"rsh_amb1","status":"FAILED_AMBIGUOUS"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(srv.URL))
	env, err := runHpnResearch(context.Background(), c, "Some ambiguous person", false)
	if err != nil {
		t.Fatalf("runHpnResearch: %v", err)
	}
	if env.Status != api.StatusFailedAmbiguous {
		t.Fatalf("Status = %q, want FAILED_AMBIGUOUS", env.Status)
	}
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	emitErr := emitHpnResearchEnvelope(cmd, flags, env, "Some ambiguous person")
	if emitErr == nil {
		t.Fatal("want emit error for FAILED_AMBIGUOUS status")
	}
	var ce *cliError
	if !errors.As(emitErr, &ce) {
		t.Fatalf("want *cliError, got %T", emitErr)
	}
	if ce.code != 5 {
		t.Errorf("exit code = %d, want 5", ce.code)
	}
	if !strings.Contains(emitErr.Error(), "FAILED_AMBIGUOUS") {
		t.Errorf("error should mention FAILED_AMBIGUOUS verbatim, got: %v", emitErr)
	}
}

// --- 4. Pure unit tests on classifier and budget gate ---

func TestCheckSearchBudget(t *testing.T) {
	cases := []struct {
		name        string
		budget      int
		cost        int
		wantBlocked bool
	}{
		{"unlimited budget allows any cost", 0, 100, false},
		{"negative budget treated as unlimited", -1, 5, false},
		{"cost equals budget allowed", 2, 2, false},
		{"cost under budget allowed", 5, 2, false},
		{"cost over budget blocked", 1, 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocked, _ := checkSearchBudget(tc.budget, tc.cost)
			if blocked != tc.wantBlocked {
				t.Errorf("checkSearchBudget(%d, %d) blocked = %v, want %v", tc.budget, tc.cost, blocked, tc.wantBlocked)
			}
		})
	}
}

func TestClassifyHpnError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"401 unauthorized", errors.New("happenstance api: 401 unauthorized — HAPPENSTANCE_API_KEY missing"), 4},
		{"404 not found", errors.New("happenstance api: 404 not found — GET /search/x"), 3},
		{"402 payment required surfaces as api err", errors.New("happenstance api: 402 payment required — out of credits"), 5},
		{"rate limit error", &api.RateLimitError{RetryAfterSeconds: 30}, 7},
		{"generic", errors.New("network blew up"), 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyHpnError(tc.err)
			if tc.want == 0 {
				if got != nil {
					t.Errorf("classifyHpnError(nil) = %v, want nil", got)
				}
				return
			}
			var ce *cliError
			if !errors.As(got, &ce) {
				t.Fatalf("want *cliError, got %T (%v)", got, got)
			}
			if ce.code != tc.want {
				t.Errorf("exit code = %d, want %d", ce.code, tc.want)
			}
		})
	}
}

// --- 5. Stderr capture helper ---

// runCmdWithStderrCapture executes the cobra command but ALSO redirects
// os.Stderr to a buffer (in addition to cmd.ErrOrStderr). The bearer
// client's printDryRun writes directly to os.Stderr, so the standard
// cobra-buffer capture misses it. We restore os.Stderr on cleanup.
func runCmdWithStderrCapture(t *testing.T, root *cobra.Command, argv []string) (string, string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	prev := os.Stderr
	os.Stderr = w
	defer func() {
		_ = w.Close()
		os.Stderr = prev
	}()

	out, _, runErr := runCmd(t, root, argv)
	_ = w.Close()
	os.Stderr = prev
	captured, _ := io.ReadAll(r)
	return out, string(captured), runErr
}
