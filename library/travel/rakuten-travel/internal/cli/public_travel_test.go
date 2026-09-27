package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
)

func runPublicTravelCLI(args ...string) (string, string, error) {
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, diagnostics bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostics)
	err := cmd.Execute()
	return out.String(), diagnostics.String(), err
}

func forbidPublicTravelIO(t *testing.T) {
	t.Helper()
	original := publicTravelClientFactory
	publicTravelClientFactory = func(travel.Config) (travel.API, error) {
		t.Error("validation/help/dry-run unexpectedly constructed the source client")
		return nil, fmt.Errorf("unexpected I/O")
	}
	t.Cleanup(func() { publicTravelClientFactory = original })
}

func futureTravelDates() (string, string) {
	zone := time.FixedZone("JST", 9*60*60)
	in := time.Now().In(zone).AddDate(0, 0, 14)
	return in.Format("2006-01-02"), in.AddDate(0, 0, 2).Format("2006-01-02")
}

func offerCLIArgs(command string) []string {
	in, out := futureTravelDates()
	return []string{"offers", command, "--hotel", "51870", "--checkin", in, "--checkout", out, "--rooms", "1", "--adults-per-room", "2"}
}

func replaceTravelFlag(args []string, name, value string) []string {
	copied := append([]string{}, args...)
	for i := range copied {
		if copied[i] == name && i+1 < len(copied) {
			copied[i+1] = value
			return copied
		}
	}
	return append(copied, name, value)
}

func TestPublicTravelDryRunShortCircuitsBeforeValidationAndIO(t *testing.T) {
	testenv.Isolate(t)
	forbidPublicTravelIO(t)
	for _, path := range [][]string{{"areas", "list"}, {"hotels", "search"}, {"hotels", "show"}, {"offers", "search"}, {"offers", "show"}, {"compare"}} {
		args := append(append([]string{}, path...), "--dry-run", "--agent", "--data-source", "local")
		out, diagnostics, err := runPublicTravelCLI(args...)
		if err != nil {
			t.Fatalf("%v: %v; stderr=%s", path, err, diagnostics)
		}
		var result struct {
			DryRun bool   `json:"dry_run"`
			Action string `json:"action"`
		}
		if json.Unmarshal([]byte(out), &result) != nil || !result.DryRun || result.Action != strings.Join(path, " ") {
			t.Fatalf("%v dry-run=%s", path, out)
		}
	}
}

func TestPublicTravelGroupsHelpAndUnknownLeaf(t *testing.T) {
	testenv.Isolate(t)
	forbidPublicTravelIO(t)
	for _, group := range []string{"areas", "hotels", "offers"} {
		out, _, err := runPublicTravelCLI(group, "--agent")
		if err != nil || !strings.Contains(out, "Available Commands:") {
			t.Fatalf("%s group did not render help: %v %s", group, err, out)
		}
		_, diagnostics, err := runPublicTravelCLI(group, "bogus")
		if ExitCode(err) != 2 || !strings.Contains(diagnostics, "unknown subcommand") {
			t.Fatalf("%s bogus: err=%v stderr=%s", group, err, diagnostics)
		}
	}
}

func TestPublicTravelCommandTreeKeepsExactLeafFlagsAndIdentity(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	cases := []struct {
		path, source string
		flags        []string
	}{
		{"areas list", "live", []string{"parent", "limit"}},
		{"hotels search", "live", []string{"query", "area", "page"}},
		{"hotels show", "live", []string{"hotel"}},
		{"offers search", "live", []string{"hotel", "checkin", "checkout", "rooms", "adults-per-room"}},
		{"offers show", "live", []string{"hotel", "plan", "room", "max-scan-pages"}},
		{"compare", "computed", []string{"hotels", "checkins", "nights"}},
	}
	for _, tc := range cases {
		parts := strings.Fields(tc.path)
		leaf, remaining, err := root.Find(parts)
		if err != nil || len(remaining) != 0 || leaf.CommandPath() != root.Name()+" "+tc.path || leaf.Use != parts[len(parts)-1] {
			t.Fatalf("%s resolved wrong command: cmd=%v remaining=%v err=%v", tc.path, leaf, remaining, err)
		}
		for _, name := range tc.flags {
			if leaf.Flags().Lookup(name) == nil {
				t.Fatalf("%s is missing its own --%s flag", tc.path, name)
			}
		}
		if leaf.Annotations["mcp:read-only"] != "true" || leaf.Annotations["pp:data-source"] != tc.source {
			t.Fatalf("%s annotations=%v", tc.path, leaf.Annotations)
		}
		for _, token := range strings.Split(leaf.Annotations["pp:happy-args"], ";") {
			if !strings.HasPrefix(token, "--") || !strings.Contains(token, "=") || strings.ContainsAny(token, " \t\n") {
				t.Fatalf("%s verifier token=%q", tc.path, token)
			}
		}
	}
}

func TestPublicTravelRejectsInvalidQueriesBeforeIO(t *testing.T) {
	testenv.Isolate(t)
	forbidPublicTravelIO(t)
	base := offerCLIArgs("search")
	in, _ := futureTravelDates()
	cases := []struct {
		name string
		args []string
	}{
		{"hotel missing selector", []string{"hotels", "search"}},
		{"hotel selectors conflict", []string{"hotels", "search", "--query", "Tokyo", "--area", "tokyo/E"}},
		{"invalid area", []string{"hotels", "search", "--area", "../tokyo/E"}},
		{"zero page", []string{"hotels", "search", "--query", "Tokyo", "--page", "0"}},
		{"zero limit", []string{"hotels", "search", "--query", "Tokyo", "--limit", "0"}},
		{"negative offset", []string{"hotels", "search", "--query", "Tokyo", "--offset", "-1"}},
		{"source page cap", []string{"hotels", "search", "--query", "Tokyo", "--page", "101"}},
		{"output cap", []string{"hotels", "search", "--query", "Tokyo", "--limit", "101"}},
		{"parent path rejected", []string{"areas", "list", "--parent", "tokyo/E"}},
		{"property missing", []string{"hotels", "show"}},
		{"property invalid", []string{"hotels", "show", "--hotel", "ABC"}},
		{"offer missing date party", []string{"offers", "search", "--hotel", "51870"}},
		{"malformed date", replaceTravelFlag(base, "--checkin", "2026-02-30")},
		{"equal dates", replaceTravelFlag(base, "--checkout", in)},
		{"zero adults", replaceTravelFlag(base, "--adults-per-room", "0")},
		{"too many adults", replaceTravelFlag(base, "--adults-per-room", "11")},
		{"zero rooms", replaceTravelFlag(base, "--rooms", "0")},
		{"too many rooms", replaceTravelFlag(base, "--rooms", "11")},
		{"negative children", append(append([]string{}, base...), "--infant-none", "-1")},
		{"child cap", append(append([]string{}, base...), "--child-upper", "11")},
		{"zero offer page", replaceTravelFlag(base, "--page", "0")},
		{"offer output cap", replaceTravelFlag(base, "--limit", "101")},
		{"show missing exact tuple", offerCLIArgs("show")},
		{"show plan malformed", append(offerCLIArgs("show"), "--plan", "123x", "--room", "s-double-")},
		{"show room whitespace", append(offerCLIArgs("show"), "--plan", "123", "--room", "s double")},
		{"show scan cap", append(offerCLIArgs("show"), "--plan", "123", "--room", "x-", "--max-scan-pages", "4")},
		{"show request cap", append(offerCLIArgs("show"), "--plan", "123", "--room", "x-", "--max-requests", "13")},
		{"offline unsupported", append(append([]string{}, base...), "--data-source", "local")},
		{"inventory TTL cap", append(append([]string{}, base...), "--inventory-cache-seconds", "61")},
		{"budget cap", append(append([]string{}, base...), "--max-requests", "61")},
		{"negative timeout", append(append([]string{}, base...), "--timeout", "0s")},
		{"unsupported raw csv", append(append([]string{}, base...), "--csv")},
		{"unsupported rate override", append(append([]string{}, base...), "--rate-limit", "0")},
		{"unexpected positional", append(append([]string{}, base...), "surprise")},
		{"compare missing selectors", []string{"compare"}},
		{"compare matrix cap", []string{"compare", "--hotels", "1,2,3,4", "--checkins", in + "," + time.Now().AddDate(0, 0, 15).Format("2006-01-02") + "," + time.Now().AddDate(0, 0, 16).Format("2006-01-02"), "--nights", "2", "--rooms", "1", "--adults-per-room", "2"}},
		{"compare repeated hotel", []string{"compare", "--hotels", "51870,51870", "--checkins", in, "--nights", "2", "--rooms", "1", "--adults-per-room", "2"}},
		{"compare invalid hotel", []string{"compare", "--hotels", "x", "--checkins", in, "--nights", "2", "--rooms", "1", "--adults-per-room", "2"}},
		{"compare invalid nights", []string{"compare", "--hotels", "51870", "--checkins", in, "--nights", "29", "--rooms", "1", "--adults-per-room", "2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, diagnostics, err := runPublicTravelCLI(tc.args...)
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("usage error expected: err=%v stdout=%s stderr=%s", err, out, diagnostics)
			}
			if diagnostics == "" {
				t.Fatal("usage diagnostic missing from stderr")
			}
			if out != "" {
				t.Fatalf("unexpected stdout on invalid query: %s", out)
			}
		})
	}
}
