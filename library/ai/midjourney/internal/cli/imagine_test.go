// Copyright 2026 Dave Fano and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestSubmitJobBuildPromptReferences(t *testing.T) {
	t.Parallel()
	flags := submitJobFlags{
		userID:       "user-123",
		version:      "7",
		profile:      "auto",
		imagePrompts: []string{"https://cdn.midjourney.com/example.png"},
		styleRefs:    []string{"https://s.mj.run/style"},
		omniRefs:     []string{"https://s.mj.run/omni"},
		imageWeight:  "1.5",
		raw:          true,
	}
	got, err := flags.buildPrompt("green glass sphere")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://cdn.midjourney.com/example.png green glass sphere --sref https://s.mj.run/style --oref https://s.mj.run/omni --profile user-123 --v 7 --iw 1.5 --style raw"
	if got != want {
		t.Fatalf("buildPrompt() = %q, want %q", got, want)
	}
}

func TestSubmitJobBuildPromptMergesImageAlias(t *testing.T) {
	t.Parallel()
	flags := submitJobFlags{
		version:      "",
		imagePrompts: []string{"https://cdn.midjourney.com/first.png"},
		imageAliases: []string{"https://cdn.midjourney.com/alias.png"},
	}
	got, err := flags.buildPrompt("green glass sphere")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://cdn.midjourney.com/first.png https://cdn.midjourney.com/alias.png green glass sphere"
	if got != want {
		t.Fatalf("buildPrompt() = %q, want %q", got, want)
	}
}

func TestSubmitJobMetadataCountsImageAlias(t *testing.T) {
	t.Parallel()
	flags := submitJobFlags{
		imagePrompts: []string{"image-a"},
		imageAliases: []string{"image-b"},
	}
	got := flags.metadata()
	if got.ImagePrompts != 2 {
		t.Fatalf("ImagePrompts = %v, want 2", got.ImagePrompts)
	}
}

func TestSubmitJobBuildPromptStyleAndNiji(t *testing.T) {
	t.Parallel()
	flags := submitJobFlags{
		version:     "7",
		style:       "cute",
		niji:        "6",
		aspectRatio: "4:5",
	}
	got, err := flags.buildPrompt("tiny friendly robot")
	if err != nil {
		t.Fatal(err)
	}
	want := "tiny friendly robot --ar 4:5 --style cute --niji 6"
	if got != want {
		t.Fatalf("buildPrompt() = %q, want %q", got, want)
	}
}

func TestSubmitJobMetadataCountsReferences(t *testing.T) {
	t.Parallel()
	flags := submitJobFlags{
		imagePrompts: []string{"image-a", " "},
		styleRefs:    []string{"style-a", "style-b"},
		omniRefs:     []string{"omni-a"},
	}
	got := flags.metadata()
	if got.ImagePrompts != 1 {
		t.Fatalf("ImagePrompts = %v, want 1", got.ImagePrompts)
	}
	if got.ImageReferences != 2 {
		t.Fatalf("ImageReferences = %v, want 2", got.ImageReferences)
	}
	if got.CharacterReferences != 1 {
		t.Fatalf("CharacterReferences = %v, want 1", got.CharacterReferences)
	}
	if got.DepthReferences != 0 {
		t.Fatalf("DepthReferences = %v, want 0", got.DepthReferences)
	}
}

func TestSubmitJobResolveChannelID(t *testing.T) {
	t.Setenv("MIDJOURNEY_USER_ID", "")
	flags := submitJobFlags{userID: "abc"}
	got, err := flags.resolveChannelID()
	if err != nil {
		t.Fatal(err)
	}
	if got != "singleplayer_abc" {
		t.Fatalf("resolveChannelID() = %q, want singleplayer_abc", got)
	}

	flags = submitJobFlags{channelID: "custom-channel", userID: "abc"}
	got, err = flags.resolveChannelID()
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom-channel" {
		t.Fatalf("resolveChannelID() = %q, want custom-channel", got)
	}
}

func TestSubmitJobVerifyShortCircuitEnvelope(t *testing.T) {
	t.Parallel()
	got, err := submitJobVerifyShortCircuitEnvelope("imagine")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\"__pp_verify_synthetic__\":true",
		"\"reason\":\"verify_short_circuit\"",
		"\"method\":\"POST\"",
		"\"path\":\"/api/submit-jobs\"",
		"\"job_type\":\"imagine\"",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("verify envelope %s missing %s", got, want)
		}
	}
}

func TestJSStringLiteralEscapesLineSeparators(t *testing.T) {
	t.Parallel()
	got := jsStringLiteral("{\"prompt\":\"line\u2028para\u2029end\"}")
	if strings.Contains(got, "\u2028") || strings.Contains(got, "\u2029") {
		t.Fatalf("jsStringLiteral() left raw JavaScript line separator in %q", got)
	}
	for _, want := range []string{"\\u2028", "\\u2029"} {
		if !strings.Contains(got, want) {
			t.Fatalf("jsStringLiteral() = %q, want escaped %s", got, want)
		}
	}
}

func TestReadWebSocketFrameRejectsOversizedLength(t *testing.T) {
	t.Parallel()
	frame := []byte{0x82, 0x7f, 0x80, 0, 0, 0, 0, 0, 0, 0}
	_, _, err := readWebSocketFrame(bufio.NewReader(bytes.NewReader(frame)))
	if err == nil {
		t.Fatal("expected oversized websocket frame to return an error")
	}
	if !strings.Contains(err.Error(), "websocket frame too large") {
		t.Fatalf("error = %v, want websocket frame too large", err)
	}
}
