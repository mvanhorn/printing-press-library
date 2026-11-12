package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(args)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), err
}

// Digg removed /api/trending/status, so the commands built on it are gone.
func TestPipelineStatusAndTrendingAreRemoved(t *testing.T) {
	for _, args := range [][]string{{"pipeline", "status"}, {"trending"}} {
		if _, err := runRoot(t, args...); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("%v: err = %v, want unknown command", args, err)
		}
	}
}

// Digg stopped serving what these commands read, so they are gone rather
// than left returning empty rows. `top --view trending` replaces `rising`.
var removedCommands = [][]string{
	{"events"}, {"evidence", "abc12345"}, {"sentiment", "abc12345"}, {"crossref", "abc12345"},
	{"replaced"}, {"rising"}, {"rankings", "emerging"}, {"rankings", "movers"},
}

func TestCommandsDiggNoLongerFeedsAreRemoved(t *testing.T) {
	withTempHome(t)
	// Nothing here may reach digg.com, whichever code runs.
	prevRankings, prevSite, prevFeed, prevMCP := rankingsCompaniesURL, diggSiteURL, feedURLOverride, diggMCPURLOverride
	rankingsCompaniesURL, diggSiteURL, feedURLOverride, diggMCPURLOverride = "http://127.0.0.1:1", "http://127.0.0.1:1", "http://127.0.0.1:1", "http://127.0.0.1:1"
	t.Cleanup(func() {
		rankingsCompaniesURL, diggSiteURL, feedURLOverride, diggMCPURLOverride = prevRankings, prevSite, prevFeed, prevMCP
	})
	for _, args := range removedCommands {
		if _, err := runRoot(t, args...); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%v: err = %v, want unknown command", args, err)
		}
	}
}

// `which` and `agent-context` are how agents find commands; neither may
// point at a removed one.
func TestDiscoveryOmitsRemovedCommands(t *testing.T) {
	removed := map[string]bool{}
	for _, args := range removedCommands {
		if args[0] == "rankings" {
			removed[strings.Join(args, " ")] = true
		} else {
			removed[args[0]] = true
		}
	}
	for _, e := range whichIndex {
		if removed[e.Command] {
			t.Errorf("which index still lists %q", e.Command)
		}
	}
	var flags rootFlags
	for path := range collectAgentCommands(newRootCmd(&flags)) {
		if removed[path] {
			t.Errorf("agent-context still lists %q", path)
		}
	}
}
