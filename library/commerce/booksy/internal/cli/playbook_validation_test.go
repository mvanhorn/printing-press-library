// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/store"
)

func TestPlaybookRejectsUnapprovedCommandsAndArguments(t *testing.T) {
	for _, command := range []string{
		"businesses get 1; touch marker", "businesses get $(id)", "businesses get `id`",
		"businesses get 1 | cat", "businesses get 1 > marker", "businesses get 1\nwhoami",
		"businesses get 1 && whoami", "sh -c whoami", "auth set-token", "businesses get",
		"businesses get 1 extra", "businesses get 1 --token secret", "businesses get 1 --base-url https://example.com",
		"businesses get 1 --deliver file:marker", "businesses get 1 --unknown value", "businesses search --page nope",
	} {
		t.Run(command, func(t *testing.T) {
			wire, _ := json.Marshal(learn.Playbook{Steps: []learn.PlaybookStep{{Cmd: command}}})
			if _, err := resolveInlinePlaybook(string(wire)); err == nil {
				t.Fatal("unsafe command accepted")
			}
			filename := writePlaybookFile(t, t.TempDir(), "playbook.json", string(wire))
			if _, _, err := resolvePlaybookInputs(filename, "", ""); err == nil {
				t.Fatal("unsafe file accepted")
			}
		})
	}
}

func TestPlaybookPreservesSeparateArguments(t *testing.T) {
	for _, body := range []string{
		`{"steps":[{"cmd":"booksy-pp-cli businesses get {business.id} --json"}]}`,
		`{"steps":[{"argv":["businesses","search","--query","hair salon","--page","2"]}]}`,
		`{"steps":[{"argv":["businesses","search","--query","R&B"]}]}`,
		`{"steps":[{"argv":["businesses","get","$(id)"]}]}`,
		`{"steps":[{"cmd":"businesses search --query <str>"}]}`,
		`{"steps":[{"cmd":"businesses search --page <int>"}]}`,
	} {
		stored, err := resolveInlinePlaybook(body)
		if err != nil {
			t.Fatal(err)
		}
		var pb learn.Playbook
		if err := json.Unmarshal([]byte(stored), &pb); err != nil {
			t.Fatal(err)
		}
		if pb.Steps[0].Cmd != "" || len(pb.Steps[0].Argv) == 0 {
			t.Fatalf("expected argv only, got %+v", pb)
		}
	}
	for _, body := range []string{
		`{"steps":[{"argv":["businesses","get",""]}]}`,
		`{"steps":[{"cmd":"businesses search --query <redacted>"}]}`,
		`{"steps":[{"cmd":"businesses get 1","argv":["businesses","get","2"]}]}`,
		`{"steps":[{"client_side":"eval","args":{"script":"anything"}}]}`,
	} {
		if _, err := resolveInlinePlaybook(body); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestPlaybookRequiresValidServiceVariant(t *testing.T) {
	for _, command := range []string{
		"availability 297360", "availability 297360 --service-variant 0",
		"availability 297360 --service-variant nope", "earliest 297360",
	} {
		if _, err := resolveInlinePlaybook(`{"steps":[{"cmd":"` + command + `"}]}`); err == nil {
			t.Errorf("accepted %q without a valid service variant", command)
		}
	}
	for _, command := range []string{
		"availability 297360 --service-variant 20193554",
		"earliest 297360 --service-variant <int>",
	} {
		if _, err := resolveInlinePlaybook(`{"steps":[{"cmd":"` + command + `"}]}`); err != nil {
			t.Errorf("rejected valid command %q: %v", command, err)
		}
	}
}

func TestPlaybookValidationDoesNotChangeColor(t *testing.T) {
	previous := noColor
	t.Cleanup(func() { noColor = previous })
	for _, initial := range []bool{false, true} {
		noColor = initial
		if _, err := resolveInlinePlaybook(`{"steps":[{"cmd":"businesses get 1 --no-color"}]}`); err != nil {
			t.Fatal(err)
		}
		if noColor != initial {
			t.Fatalf("validation changed noColor from %v to %v", initial, noColor)
		}
	}
}

func TestRecallOmitsUnsafeLegacyPlaybook(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	query := "find nearby businesses"
	_, stderr, err := runRootArgs(t, "teach-playbook", "--query", query, "--playbook-json", `{"steps":[{"cmd":"businesses search --query barber"}]}`, "--db", dbPath, "--agent")
	if err != nil {
		t.Fatalf("teach: %v: %s", err, stderr)
	}
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB().Exec(`UPDATE learning_playbooks SET playbook_json = ?`, `{"steps":[{"cmd":"businesses search; touch marker"}]}`)
	s.Close()
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runRootArgs(t, "recall", query, "--db", dbPath, "--agent")
	if err != nil {
		t.Fatalf("recall: %v: %s", err, stderr)
	}
	var response map[string]any
	unmarshalAgentResults(t, stdout, &response)
	if response["playbook"] != nil || !strings.Contains(stdout, "unsafe_playbook_omitted") || strings.Contains(stdout, "touch marker") {
		t.Fatalf("unsafe legacy record surfaced: %s", stdout)
	}
}
