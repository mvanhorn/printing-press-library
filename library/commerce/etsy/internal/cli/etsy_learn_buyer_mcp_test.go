package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsyLearnListFixture seeds buyer-bearing and clean learn rows, candidates, playbooks and
// teach-log warnings, all carrying synthetic sentinels.
func etsyLearnListFixture(t *testing.T) (string, *store.Store) {
	t.Helper()
	RootCmd()
	etsyDB := filepath.Join(withTempLearnHome(t), "data.db")
	etsyStore, etsyErr := store.OpenWithContext(context.Background(), etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(func() { etsyStore.Close() })
	for _, etsyIn := range []store.UpsertLearningInput{
		{Query: "orders for QZXBUYERNAME", ResourceID: "910002", ResourceType: "receipts", Source: store.LearningSourceTaught, Notes: "QZXMSG"},
		{Query: "open user QZXBUYERNAME", ResourceID: "920002", ResourceType: "users", Source: store.LearningSourceTaught},
		{Query: "find linen apron", ResourceID: "1000000001", ResourceType: "listings", Source: store.LearningSourceTaught},
	} {
		if _, _, etsyErr = etsyStore.UpsertLearning(context.Background(), etsyIn); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	for _, etsyCand := range []struct{ sig, family string }{{"sig-buyer", "qzxbuyername"}, {"sig-clean", "find items category"}} {
		if _, _, etsyErr = etsyStore.DeriveCandidate(store.CandidateClassPlaybookCandidate,
			`{"playbook_json":"{}","notes_text":"note for `+etsyCand.family+`"}`, etsyCand.sig, etsyCand.family, ""); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	for _, etsyPB := range []store.UpsertPlaybookInput{
		{QueryFamily: "qzxbuyername", NotesText: "QZXMSG buyer note"},
		{QueryFamily: "find items category", NotesText: "clean note"},
	} {
		if _, _, etsyErr = etsyStore.UpsertPlaybook(etsyPB); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	for _, etsyW := range []learn.Warning{
		{Code: "no_entity_overlap", Resource: "910002", Detail: "resource entities [QZXBUYERNAME]"},
		{Code: "no_entity_overlap", Resource: "1000000001", Detail: "resource entities [linen apron]"},
		{Code: "no_entity_overlap", Resource: "424242", Detail: "resource entities [QZXPERSONAL] (no learning row)"},
	} {
		if etsyErr = learn.AppendTeachLogWarning("teach", "synthetic query", etsyW); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	return etsyDB, etsyStore
}

func etsyCandidateID(t *testing.T, s *store.Store, family string) int64 {
	t.Helper()
	etsyRows, etsyErr := s.ListCandidates(store.ListCandidatesFilter{Status: store.CandidateStatusOpen, QueryFamily: family})
	if etsyErr != nil || len(etsyRows) != 1 {
		t.Fatalf("candidate %q: rows=%d err=%v", family, len(etsyRows), etsyErr)
	}
	return etsyRows[0].ID
}

func TestEtsyLearningsFamilyHidesBuyerRowsUnderMCP(t *testing.T) {
	etsyDB, _ := etsyLearnListFixture(t)
	for _, etsyCase := range []struct {
		name, mcp, optIn string
		restricted       bool
	}{
		{"mcp restricted", "1", "", true},
		{"mcp opted in", "1", "1", false},
		{"plain cli", "", "", false},
	} {
		for _, etsyCmd := range []struct {
			name       string
			args       []string
			wantClean  string
			wantBuyer  []string // sentinels that must appear only when not restricted
			noun       string
			hiddenWant int
		}{
			{"list", []string{"learnings", "list"}, "find linen apron", []string{"qzxbuyername", "910002", "920002", "QZXMSG", "receipts"}, "learnings", 2},
			{"list warnings", []string{"learnings", "list", "--warnings"}, "linen apron", []string{"QZXBUYERNAME", "910002", "QZXPERSONAL", "424242"}, "warnings", 2},
			{"candidates", []string{"learnings", "candidates"}, "find items category", []string{"qzxbuyername", "note for qzxbuyername"}, "candidates", 1},
			{"playbook list", []string{"playbook", "list"}, "find items category", []string{"qzxbuyername", "QZXMSG"}, "playbooks", 1},
		} {
			t.Run(etsyCase.name+"/"+etsyCmd.name, func(t *testing.T) {
				t.Setenv(cliutil.MCPChildEnvVar, etsyCase.mcp)
				t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", etsyCase.optIn)
				etsyOut, _, etsyErr := runRootArgs(t, append(append([]string{}, etsyCmd.args...), "--json", "--db", etsyDB)...)
				if etsyErr != nil {
					t.Fatal(etsyErr)
				}
				if !strings.Contains(etsyOut, etsyCmd.wantClean) {
					t.Fatalf("clean row %q missing: %s", etsyCmd.wantClean, etsyOut)
				}
				etsyLower := strings.ToLower(etsyOut)
				for _, etsyNeedle := range etsyCmd.wantBuyer {
					if strings.Contains(etsyLower, strings.ToLower(etsyNeedle)) == etsyCase.restricted {
						t.Fatalf("restricted=%v but needle %q visibility is %v: %s", etsyCase.restricted, etsyNeedle, !etsyCase.restricted, etsyOut)
					}
				}
				etsyWantNote := etsyBuyerHiddenNote(etsyCmd.hiddenWant, etsyCmd.noun)
				if strings.Contains(etsyOut, etsyWantNote) != etsyCase.restricted {
					t.Fatalf("restricted=%v: want note %q: %s", etsyCase.restricted, etsyWantNote, etsyOut)
				}
				if etsyCase.restricted && !strings.Contains(etsyOut, "ETSY_MCP_ALLOW_BUYER_PII=1") {
					t.Fatalf("note lacks the opt-in hint: %s", etsyOut)
				}
			})
		}
	}
}

// In MCP restricted mode the output shape must not depend on the data: even with nothing hidden
// the rows come wrapped in the object form (buyer_hidden 0, no note). Outside restricted mode the
// bare array stays.
func TestEtsyLearningsFamilyRestrictedModeAlwaysObjectEvenWhenNothingHidden(t *testing.T) {
	RootCmd()
	etsyDB := filepath.Join(withTempLearnHome(t), "data.db")
	etsyStore, etsyErr := store.OpenWithContext(context.Background(), etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyStore.Close()
	if _, _, etsyErr = etsyStore.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "find linen apron", ResourceID: "1000000001", ResourceType: "listings", Source: store.LearningSourceTaught}); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	for _, etsyCase := range []struct {
		name, mcp, optIn string
		wantObject       bool
	}{{"restricted", "1", "", true}, {"opted in", "1", "1", false}, {"plain cli", "", "", false}} {
		t.Run(etsyCase.name, func(t *testing.T) {
			t.Setenv(cliutil.MCPChildEnvVar, etsyCase.mcp)
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", etsyCase.optIn)
			etsyOut, _, etsyErr := runRootArgs(t, "learnings", "list", "--json", "--db", etsyDB)
			if etsyErr != nil {
				t.Fatal(etsyErr)
			}
			if !strings.Contains(etsyOut, "1000000001") {
				t.Fatalf("clean row missing: %s", etsyOut)
			}
			if etsyCase.wantObject {
				var etsyObj map[string]any
				if etsyErr = json.Unmarshal([]byte(etsyOut), &etsyObj); etsyErr != nil {
					t.Fatalf("restricted mode must emit an object even with zero hidden rows: %v: %s", etsyErr, etsyOut)
				}
				if hidden, ok := etsyObj["buyer_hidden"].(float64); !ok || hidden != 0 {
					t.Fatalf("want buyer_hidden=0: %s", etsyOut)
				}
				if _, ok := etsyObj["note"]; ok {
					t.Fatalf("no note expected when nothing is hidden: %s", etsyOut)
				}
				if _, ok := etsyObj["learnings"].([]any); !ok {
					t.Fatalf("learnings array missing: %s", etsyOut)
				}
				return
			}
			if !strings.HasPrefix(strings.TrimSpace(etsyOut), "[") || strings.Contains(etsyOut, "buyer_hidden") {
				t.Fatalf("expected the bare array: %s", etsyOut)
			}
		})
	}
}

// Filtering must happen before --limit truncation: a buyer row at the head of the window must
// not shrink the visible page below the limit.
func TestEtsyLearningsListLimitAppliesAfterBuyerFilter(t *testing.T) {
	RootCmd()
	etsyDB := filepath.Join(withTempLearnHome(t), "data.db")
	etsyStore, etsyErr := store.OpenWithContext(context.Background(), etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyStore.Close()
	for _, etsyIn := range []store.UpsertLearningInput{
		{Query: "find linen apron", ResourceID: "1000000001", ResourceType: "listings", Source: store.LearningSourceTaught},
		{Query: "find juniper mug", ResourceID: "1000000002", ResourceType: "listings", Source: store.LearningSourceTaught},
		{Query: "orders for QZXBUYERNAME", ResourceID: "910002", ResourceType: "receipts", Source: store.LearningSourceTaught},
	} {
		if _, _, etsyErr = etsyStore.UpsertLearning(context.Background(), etsyIn); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	etsyOut, _, etsyErr := runRootArgs(t, "learnings", "list", "--limit", "2", "--json", "--db", etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if !strings.Contains(etsyOut, "1000000001") || !strings.Contains(etsyOut, "1000000002") || strings.Contains(etsyOut, "910002") || !strings.Contains(etsyOut, etsyBuyerHiddenNote(1, "learnings")) {
		t.Fatalf("limit applied before the buyer filter: %s", etsyOut)
	}
}

func TestEtsyLearningsConfirmRejectRefuseBuyerCandidateUnderMCP(t *testing.T) {
	for _, etsyVerb := range []string{"confirm", "reject"} {
		t.Run(etsyVerb, func(t *testing.T) {
			etsyDB, etsyStore := etsyLearnListFixture(t)
			etsyBuyerID := etsyCandidateID(t, etsyStore, "qzxbuyername")
			etsyCleanID := etsyCandidateID(t, etsyStore, "find items category")
			etsyArg := func(id int64) string { return strconv.FormatInt(id, 10) }

			t.Setenv(cliutil.MCPChildEnvVar, "1")
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
			etsyOut, _, etsyErr := runRootArgs(t, "learnings", etsyVerb, etsyArg(etsyBuyerID), "--json", "--db", etsyDB)
			if etsyErr == nil || ExitCode(etsyErr) != 2 || !strings.Contains(etsyErr.Error(), "ETSY_MCP_ALLOW_BUYER_PII") {
				t.Fatalf("expected buyer refusal, got err=%v out=%s", etsyErr, etsyOut)
			}
			if strings.Contains(strings.ToLower(etsyOut+etsyErr.Error()), "qzxbuyername") {
				t.Fatalf("refusal leaked the buyer family: %s %v", etsyOut, etsyErr)
			}
			etsyRow, etsyOK, _ := etsyStore.GetCandidate(etsyBuyerID)
			if !etsyOK || etsyRow.Status != store.CandidateStatusOpen {
				t.Fatalf("refused %s mutated the buyer candidate: %+v", etsyVerb, etsyRow)
			}
			if _, _, etsyErr = runRootArgs(t, "learnings", etsyVerb, etsyArg(etsyCleanID), "--json", "--db", etsyDB); etsyErr != nil {
				t.Fatalf("clean candidate %s refused: %v", etsyVerb, etsyErr)
			}

			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "1")
			if _, _, etsyErr = runRootArgs(t, "learnings", etsyVerb, etsyArg(etsyBuyerID), "--json", "--db", etsyDB); etsyErr != nil {
				t.Fatalf("opted-in %s refused: %v", etsyVerb, etsyErr)
			}
		})
	}
}

func TestEtsyDropBuyerWarningsFailsClosedOnUnclassifiable(t *testing.T) {
	etsyIdx := &etsyBuyerIndex{tokens: map[string]struct{}{}, buyerIDs: map[string]struct{}{"1": {}}, otherIDs: map[string]struct{}{"1": {}, "2": {}}}
	etsyKept, etsyHidden := etsyDropBuyerWarnings(etsyIdx, []learn.TeachLogEntry{{Resource: "1"}, {Resource: "2"}, {Resource: "3"}, {Resource: ""}})
	if len(etsyKept) != 1 || etsyKept[0].Resource != "2" || etsyHidden != 3 {
		t.Fatalf("kept=%+v hidden=%d", etsyKept, etsyHidden)
	}
}

func TestEtsyLearningsStatsAndForgetEmitNoStoredRows(t *testing.T) {
	etsyDB, _ := etsyLearnListFixture(t)
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	etsyOut, _, etsyErr := runRootArgs(t, "learnings", "stats", "--json", "--db", etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if strings.Contains(strings.ToLower(etsyOut), "qzx") || strings.Contains(etsyOut, "910002") {
		t.Fatalf("stats emitted a stored row: %s", etsyOut)
	}
	// forget echoes only the caller's own query and a count, never a stored query_pattern.
	etsyOut, _, etsyErr = runRootArgs(t, "learnings", "forget", "absent synthetic learning", "--all", "--json", "--db", etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if strings.Contains(strings.ToLower(etsyOut), "qzx") || strings.Contains(etsyOut, "910002") || !strings.Contains(etsyOut, `"deleted"`) {
		t.Fatalf("forget output: %s", etsyOut)
	}
}

// recall surfaces open candidates for any query; over MCP the buyer-bearing ones must not ride the
// envelope, and the count of hidden ones must.
func TestEtsyRecallCandidatesHideBuyerFamiliesUnderMCP(t *testing.T) {
	etsyDB, _ := etsyLearnListFixture(t)
	for _, etsyCase := range []struct {
		name, mcp, optIn string
		restricted       bool
	}{
		{"mcp restricted", "1", "", true},
		{"mcp opted in", "1", "1", false},
		{"plain cli", "", "", false},
	} {
		t.Run(etsyCase.name, func(t *testing.T) {
			t.Setenv(cliutil.MCPChildEnvVar, etsyCase.mcp)
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", etsyCase.optIn)
			etsyOut, _, etsyErr := runRootArgs(t, "recall", "open order zz", "--json", "--db", etsyDB)
			if etsyErr != nil {
				t.Fatal(etsyErr)
			}
			if !strings.Contains(etsyOut, "find items category") {
				t.Fatalf("clean candidate missing: %s", etsyOut)
			}
			if strings.Contains(strings.ToLower(etsyOut), "qzxbuyername") == etsyCase.restricted {
				t.Fatalf("restricted=%v buyer family visibility wrong: %s", etsyCase.restricted, etsyOut)
			}
			if strings.Contains(etsyOut, `"buyer_candidates_hidden": 1`) != etsyCase.restricted || strings.Contains(etsyOut, learn.EtsyWarningBuyerCandidatesHidden) != etsyCase.restricted {
				t.Fatalf("restricted=%v hidden count/warning wrong: %s", etsyCase.restricted, etsyOut)
			}
		})
	}
}
