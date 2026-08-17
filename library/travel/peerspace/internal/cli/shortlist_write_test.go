// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNovelShortlistCreateBoardHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"shortlist", "create-board", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("shortlist create-board --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "create-board", "listing-id", "name"} {
		if !strings.Contains(help, want) {
			t.Fatalf("shortlist create-board --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelShortlistAddHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"shortlist", "add", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("shortlist add --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "add", "listing-id", "board-id"} {
		if !strings.Contains(help, want) {
			t.Fatalf("shortlist add --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildFavCreateBoardBodyMatchesHAR(t *testing.T) {
	body := buildFavCreateBoardBody(
		"68d468bb44492187e415d4a6",
		"new favorite board",
		"Salon professionnel",
		"Paris, France",
	)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	// Exact shape from www.peerspace.com-favorite.har entry [39]
	want := `{"ns":"FAV_BOARD","value":"68d468bb44492187e415d4a6","project":{"name":"new favorite board","activity":"Salon professionnel","location":"Paris, France"}}`
	if string(raw) != want {
		t.Fatalf("create-board body mismatch\n got: %s\nwant: %s", raw, want)
	}
}

func TestBuildFavAddListingBody(t *testing.T) {
	body := buildFavAddListingBody("68d468bb44492187e415d4a6", "669152994300a86e4a943da5")
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	// Live-validated: project is a bare board-id string, not project_id.
	want := `{"ns":"FAV_BOARD","value":"68d468bb44492187e415d4a6","project":"669152994300a86e4a943da5"}`
	if string(raw) != want {
		t.Fatalf("add body mismatch\n got: %s\nwant: %s", raw, want)
	}
}

func TestPSAccessBearerFromCookieHeader(t *testing.T) {
	got := psAccessBearerFromCookieHeader("PSUser=abc; PSAccess=NzZlYWNm%3D%3D; other=1")
	want := "Bearer NzZlYWNm=="
	if got != want {
		t.Fatalf("bearer = %q, want %q", got, want)
	}
	if psAccessBearerFromCookieHeader("") != "" {
		t.Fatal("empty cookie header should yield empty bearer")
	}
	if psAccessBearerFromCookieHeader("PSUser=only") != "" {
		t.Fatal("missing PSAccess should yield empty bearer")
	}
}

func TestShortlistCreateBoardRequiresFlags(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"shortlist", "create-board", "--name", "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --listing-id missing")
	}
}
