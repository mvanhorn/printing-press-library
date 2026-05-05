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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
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
	if err := emitHpnSearchEnvelope(cmd, flags, env, "VPs at NBA", "", false, 0); err != nil {
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

// --- 6. U1: currentUUID retag + --first-degree-only + --min-score ---

// envelopeWithBridges builds a synthetic SearchEnvelope mirroring what
// the bearer surface returns when both --include-my-connections and
// --include-friends-connections are set. Three results, three mutuals:
//
//   - Carol (the caller) is index 0 in the envelope mutuals; Result[0]
//     has a self-bridge to Carol (1st-degree, kind self_graph after retag).
//   - Bob (a friend) is index 1; Result[1] has a friend bridge to Bob
//     (2nd-degree, kind friend).
//   - Eve (a public-graph match) is index 2; Result[2] has a weak
//     bridge to Eve.
//
// Caller passes Carol's UUID as currentUUID to enable the self_graph retag.
func envelopeWithBridges(t *testing.T) (api.SearchEnvelope, string) {
	t.Helper()
	carolUUID := "uuid-carol-self"
	env := api.SearchEnvelope{
		Id:     "srch_test",
		Status: api.StatusCompleted,
		Mutuals: []api.SearchMutual{
			{Index: 0, Id: carolUUID, Name: "Carol Self"},
			{Index: 1, Id: "uuid-bob-friend", Name: "Bob Friend"},
			{Index: 2, Id: "uuid-eve-pub", Name: "Eve Public"},
		},
		Results: []api.SearchResult{
			{
				Name:                "Self-Bridged Person",
				CurrentTitle:        "VP",
				CurrentCompany:      "AcmeCo",
				WeightedTraitsScore: 50.0,
				Mutuals:             []api.ResultMutual{{Index: 0, AffinityScore: 50.0}},
			},
			{
				Name:                "Friend-Bridged Person",
				CurrentTitle:        "Director",
				CurrentCompany:      "AcmeCo",
				WeightedTraitsScore: 30.0,
				Mutuals:             []api.ResultMutual{{Index: 1, AffinityScore: 30.0}},
			},
			{
				Name:                "Weak-Signal Person",
				CurrentTitle:        "Engineer",
				CurrentCompany:      "OtherCo",
				WeightedTraitsScore: 2.0,
				Mutuals:             []api.ResultMutual{{Index: 2, AffinityScore: 0.0}},
			},
		},
	}
	return env, carolUUID
}

// decodedHpnBridge / decodedHpnResult / decodedHpnEnvelope are typed
// projections of the JSON envelope emitHpnSearchEnvelope produces, used
// by the U1 tests so assertions can target rows and bridge kinds without
// juggling map[string]any.
type decodedHpnBridge struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type decodedHpnResult struct {
	Name    string             `json:"name"`
	Score   float64            `json:"score"`
	Bridges []decodedHpnBridge `json:"bridges"`
}

type decodedHpnEnvelope struct {
	Count   int                `json:"count"`
	Source  string             `json:"source"`
	Results []decodedHpnResult `json:"results"`
}

// decodeHpnEnvelope walks the JSON output of emitHpnSearchEnvelope into
// a typed struct so test assertions can target individual rows and
// bridge kinds without juggling map[string]any.
func decodeHpnEnvelope(t *testing.T, out *bytes.Buffer) decodedHpnEnvelope {
	t.Helper()
	var decoded decodedHpnEnvelope
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode envelope: %v\nout: %s", err, out.String())
	}
	return decoded
}

// TestEmitHpnSearchEnvelope_RetagsSelfGraphWithUUID is the regression
// test for the U1 root-cause fix. With currentUUID plumbed, the self-bridge
// retags to BridgeKindSelfGraph; without it (old behavior), all bridges
// stay BridgeKindFriend and the agent cannot tell 1st-degree from 2nd.
func TestEmitHpnSearchEnvelope_RetagsSelfGraphWithUUID(t *testing.T) {
	env, currentUUID := envelopeWithBridges(t)
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "test", currentUUID, false, 0); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 3 {
		t.Fatalf("count = %d, want 3", got.Count)
	}
	// Result[0] must have a self_graph bridge after retag.
	if len(got.Results[0].Bridges) != 1 || got.Results[0].Bridges[0].Kind != "self_graph" {
		t.Errorf("results[0].bridges[0].kind = %v, want self_graph (carol's self-entry should retag)", got.Results[0].Bridges)
	}
	// Result[1] must remain a friend bridge (not the caller's self-entry).
	if len(got.Results[1].Bridges) != 1 || got.Results[1].Bridges[0].Kind != "friend" {
		t.Errorf("results[1].bridges[0].kind = %v, want friend", got.Results[1].Bridges)
	}
}

// TestEmitHpnSearchEnvelope_RegressionEmptyUUIDForcesAllFriend documents
// the pre-fix behavior so anyone reverting the U1 fix without realizing
// it has a failing test to catch them. With currentUUID="", normalize.go's
// retag is skipped and every bridge stays BridgeKindFriend — which is
// why bearer-side searches couldn't tell 1st-degree from 2nd-degree.
func TestEmitHpnSearchEnvelope_RegressionEmptyUUIDForcesAllFriend(t *testing.T) {
	env, _ := envelopeWithBridges(t)
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "test", "", false, 0); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	for i, r := range got.Results {
		for j, b := range r.Bridges {
			if b.Kind == "self_graph" {
				t.Errorf("results[%d].bridges[%d].kind = self_graph; with empty currentUUID nothing should retag", i, j)
			}
		}
	}
}

// TestEmitHpnSearchEnvelope_FirstDegreeOnly drops the friend-bridged and
// weak-signal rows, keeping only the self-graph row.
func TestEmitHpnSearchEnvelope_FirstDegreeOnly(t *testing.T) {
	env, currentUUID := envelopeWithBridges(t)
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "test", currentUUID, true, 0); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 1 {
		t.Fatalf("count = %d, want 1 (only self-bridged kept)", got.Count)
	}
	if got.Results[0].Name != "Self-Bridged Person" {
		t.Errorf("results[0].name = %q, want Self-Bridged Person", got.Results[0].Name)
	}
	if !strings.Contains(errBuf.String(), "filters dropped 2 of 3") {
		t.Errorf("stderr should report 2 of 3 dropped, got: %s", errBuf.String())
	}
}

// TestEmitHpnSearchEnvelope_MinScore drops below-threshold rows.
func TestEmitHpnSearchEnvelope_MinScore(t *testing.T) {
	env, currentUUID := envelopeWithBridges(t)
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "test", currentUUID, false, 5); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	// Self-bridged (score 50) and friend-bridged (score 30) survive;
	// weak-signal (score 2) is dropped.
	if got.Count != 2 {
		t.Fatalf("count = %d, want 2 (weak-signal score=2 dropped at min-score=5)", got.Count)
	}
	for _, r := range got.Results {
		if r.Score < 5 {
			t.Errorf("result %q score=%g below min-score=5", r.Name, r.Score)
		}
	}
}

// TestEmitHpnSearchEnvelope_FirstDegreeOnlyAndMinScore is the SF-task
// shape: keep 1st-degree, drop weak-signal noise. Must intersect the
// two filter outputs (only self-bridged AND score >= 5).
func TestEmitHpnSearchEnvelope_FirstDegreeOnlyAndMinScore(t *testing.T) {
	env, currentUUID := envelopeWithBridges(t)
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "test", currentUUID, true, 5); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 1 || got.Results[0].Name != "Self-Bridged Person" {
		t.Fatalf("count = %d (want 1), name = %q (want Self-Bridged Person)", got.Count, got.Results[0].Name)
	}
}

// TestEmitHpnSearchEnvelope_MinScoreZeroIsNoOp confirms the default value
// doesn't accidentally filter anything.
func TestEmitHpnSearchEnvelope_MinScoreZeroIsNoOp(t *testing.T) {
	env, currentUUID := envelopeWithBridges(t)
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := emitHpnSearchEnvelope(cmd, flags, env, "test", currentUUID, false, 0); err != nil {
		t.Fatalf("emitHpnSearchEnvelope: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 3 {
		t.Errorf("count = %d, want 3 (no filter)", got.Count)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr should be empty when no filters set, got: %s", errBuf.String())
	}
}

// TestAPIHpnSearch_NegativeMinScoreUsageErr confirms the flag-validation
// path rejects --min-score values below 0 with a usage error. Sets
// dryRun=true on rootFlags so the budget/credit-prompt paths short-circuit
// before any client construction.
func TestAPIHpnSearch_NegativeMinScoreUsageErr(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newAPIHpnRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"api", "hpn", "search", "--min-score", "-1", "test"})
	if err == nil {
		t.Fatal("want usage error for --min-score=-1")
	}
	if !strings.Contains(err.Error(), "--min-score") {
		t.Errorf("error should mention --min-score, got: %v", err)
	}
}

// --- 7. U2: --all auto-pagination + --max-results + budget gate ---

// TestAPIHpnSearch_AllRequiresCap pins the safety guard: --all alone with
// no --max-results AND no --budget is rejected as a usage error to prevent
// unbounded credit spend.
func TestAPIHpnSearch_AllRequiresCap(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newAPIHpnRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"api", "hpn", "search", "--all", "test"})
	if err == nil {
		t.Fatal("want usage error for --all without --max-results or --budget")
	}
	if !strings.Contains(err.Error(), "--all requires") {
		t.Errorf("error should mention --all requires bound, got: %v", err)
	}
}

// TestAPIHpnSearch_MaxResultsWithoutAllUsageErr: --max-results only makes
// sense in pagination mode.
func TestAPIHpnSearch_MaxResultsWithoutAllUsageErr(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newAPIHpnRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"api", "hpn", "search", "--max-results", "100", "test"})
	if err == nil {
		t.Fatal("want usage error for --max-results without --all")
	}
	if !strings.Contains(err.Error(), "--max-results requires --all") {
		t.Errorf("error should mention --max-results requires --all, got: %v", err)
	}
}

// newFakeBearerPaginatedServer stands up a multi-page fixture: each
// page returns N rows with has_more=true until a configured page count
// is reached. Tracks find-more invocations so tests can assert exact
// pagination depth.
type paginatedServer struct {
	srv          *httptest.Server
	findMoreHits *int32
	pollHits     *int32
}

func newFakeBearerPaginatedServer(t *testing.T, pagesAvailable int, rowsPerPage int) *paginatedServer {
	t.Helper()
	var findMoreHits int32
	var pollHits int32
	mux := http.NewServeMux()

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"id":"srch_paginated","url":"https://h.ai/s/p"}`))
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/find-more"):
			n := atomic.AddInt32(&findMoreHits, 1)
			pageID := fmt.Sprintf("page_%d", n)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"page_id":%q,"parent_search_id":"srch_paginated"}`, pageID)))
		case r.Method == http.MethodGet:
			n := atomic.AddInt32(&pollHits, 1)
			pageID := r.URL.Query().Get("page_id")
			pageNum := 1
			if pageID != "" {
				_, _ = fmt.Sscanf(pageID, "page_%d", &pageNum)
				pageNum++ // page_1 is the second page
			}
			hasMore := pageNum < pagesAvailable
			rows := make([]string, 0, rowsPerPage)
			for i := 0; i < rowsPerPage; i++ {
				rows = append(rows, fmt.Sprintf(`{"name":"P%d-R%d","weighted_traits_score":%d}`, pageNum, i+1, 50-i))
			}
			body := fmt.Sprintf(`{"id":"srch_paginated","status":"COMPLETED","results":[%s],"has_more":%v}`, strings.Join(rows, ","), hasMore)
			_, _ = w.Write([]byte(body))
			_ = n
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	return &paginatedServer{srv: srv, findMoreHits: &findMoreHits, pollHits: &pollHits}
}

// TestRunHpnSearchAll_HappyPath: 3 pages × 5 rows, no cap, accumulates
// all 15 rows.
func TestRunHpnSearchAll_HappyPath(t *testing.T) {
	ps := newFakeBearerPaginatedServer(t, 3, 5)
	defer ps.srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(ps.srv.URL))
	firstEnv, err := runHpnSearch(context.Background(), c, "test", &api.SearchOptions{}, &api.PollSearchOptions{})
	if err != nil {
		t.Fatalf("runHpnSearch: %v", err)
	}
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := runHpnSearchAll(cmd, flags, c, firstEnv, "test", "", false, 0, 0, 100, &api.PollSearchOptions{}); err != nil {
		t.Fatalf("runHpnSearchAll: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 15 {
		t.Errorf("count = %d, want 15 (3 pages * 5 rows)", got.Count)
	}
	if atomic.LoadInt32(ps.findMoreHits) != 2 {
		t.Errorf("find-more hits = %d, want 2 (pages 2 and 3 only; page 1 was the initial POST)", atomic.LoadInt32(ps.findMoreHits))
	}
}

// TestRunHpnSearchAll_MaxResultsCap: hits the cap mid-pagination, emits
// the partial set, prints a "reached --max-results" notice on stderr.
func TestRunHpnSearchAll_MaxResultsCap(t *testing.T) {
	ps := newFakeBearerPaginatedServer(t, 5, 5)
	defer ps.srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(ps.srv.URL))
	firstEnv, _ := runHpnSearch(context.Background(), c, "test", &api.SearchOptions{}, &api.PollSearchOptions{})
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	// Cap at 7: page 1 = 5 rows, page 2 brings total to 10 (>= 7), then
	// loop checks the cap before fetching page 3.
	if err := runHpnSearchAll(cmd, flags, c, firstEnv, "test", "", false, 0, 7, 0, &api.PollSearchOptions{}); err != nil {
		t.Fatalf("runHpnSearchAll: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 10 {
		t.Errorf("count = %d, want 10 (page 1 + page 2; loop stops before page 3 since count >= cap)", got.Count)
	}
	if !strings.Contains(errBuf.String(), "reached --max-results 7") {
		t.Errorf("stderr should report cap reached, got: %s", errBuf.String())
	}
}

// TestRunHpnSearchAll_BudgetExhaustion: budget allows 1 find-more; the
// loop bails before the second find-more.
func TestRunHpnSearchAll_BudgetExhaustion(t *testing.T) {
	ps := newFakeBearerPaginatedServer(t, 5, 5)
	defer ps.srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(ps.srv.URL))
	firstEnv, _ := runHpnSearch(context.Background(), c, "test", &api.SearchOptions{}, &api.PollSearchOptions{})
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	// Budget 4: initial 2 + one find-more (2 more = 4). Next find-more
	// would be 6, exceeding budget; loop bails.
	if err := runHpnSearchAll(cmd, flags, c, firstEnv, "test", "", false, 0, 0, 4, &api.PollSearchOptions{}); err != nil {
		t.Fatalf("runHpnSearchAll: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 10 {
		t.Errorf("count = %d, want 10 (page 1 + page 2 = 10 rows; budget=4 stops before page 3)", got.Count)
	}
	if !strings.Contains(errBuf.String(), "exceed --budget 4") {
		t.Errorf("stderr should report budget exceeded, got: %s", errBuf.String())
	}
}

// TestRunHpnSearchAll_HasMoreFalseStopsImmediately: when the initial
// page already has has_more=false, no find-more calls are made.
func TestRunHpnSearchAll_HasMoreFalseStopsImmediately(t *testing.T) {
	ps := newFakeBearerPaginatedServer(t, 1, 5)
	defer ps.srv.Close()
	c := api.NewClient("hpn_live_personal_test", api.WithBaseURL(ps.srv.URL))
	firstEnv, _ := runHpnSearch(context.Background(), c, "test", &api.SearchOptions{}, &api.PollSearchOptions{})
	if firstEnv.HasMore {
		t.Fatalf("fixture seeded for single page should have HasMore=false, got true")
	}
	flags := &rootFlags{asJSON: true}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runHpnSearchAll(cmd, flags, c, firstEnv, "test", "", false, 0, 100, 100, &api.PollSearchOptions{}); err != nil {
		t.Fatalf("runHpnSearchAll: %v", err)
	}
	got := decodeHpnEnvelope(t, &out)
	if got.Count != 5 {
		t.Errorf("count = %d, want 5 (single page only)", got.Count)
	}
	if atomic.LoadInt32(ps.findMoreHits) != 0 {
		t.Errorf("find-more hits = %d, want 0 (has_more=false stops loop)", atomic.LoadInt32(ps.findMoreHits))
	}
}
