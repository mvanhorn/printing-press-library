package advisor

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	codeFenceRe   = regexp.MustCompile("(?m)^```")
	reasoningRe   = regexp.MustCompile(`(?i)\b(step[ -]by[ -]step|reason\s+through|prove|derive|first principles|chain[ -]of[ -]thought|let'?s think)\b`)
	toolUseRe     = regexp.MustCompile(`(?i)\b(use the (\w+) tool|call (\w+)\(|function call|tool[_ ]choice|tools?:\s*\[)\b`)
	imageAttachRe = regexp.MustCompile(`(?i)\b(image|screenshot|attached file|attachment|data:image/)`)
)

func ExtractFeatures(prompt string, sess *Session) Features {
	combined := prompt
	if sess != nil {
		for _, m := range sess.Messages {
			combined += "\n" + m.Content
		}
	}
	f := Features{
		InputTokens:          approxTokens(combined),
		InputTokensMethod:    "approximation:char-bpe-cl100k",
		InputTokensMarginPct: 10,
		CodeFenceDensity:     codeFenceDensity(combined),
		Languages:            detectLanguages(combined),
		ReasoningDepthHints:  len(reasoningRe.FindAllStringIndex(combined, -1)),
		ToolUseMentions:      len(toolUseRe.FindAllStringIndex(combined, -1)),
		AttachmentCount:      len(imageAttachRe.FindAllStringIndex(combined, -1)),
	}
	f.HasVisionInput = f.AttachmentCount > 0
	if sess != nil {
		f.SessionTurnCount = len(sess.Messages)
	}
	return f
}

func approxTokens(s string) int {
	if s == "" {
		return 0
	}
	chars := utf8.RuneCountInString(s)
	return int(float64(chars)/3.6 + 0.5)
}

func codeFenceDensity(s string) float64 {
	if s == "" {
		return 0
	}
	fences := codeFenceRe.FindAllStringIndex(s, -1)
	if len(fences) < 2 {
		return 0
	}
	var inside int
	for i := 0; i+1 < len(fences); i += 2 {
		inside += fences[i+1][0] - fences[i][1]
	}
	return float64(inside) / float64(len(s))
}

func detectLanguages(s string) []string {
	out := []string{}
	lower := strings.ToLower(s)
	// Fixed-order slice (not a map) so the Languages list in --explain output is
	// deterministic — map iteration order is randomised per runtime.
	patterns := []struct{ lang, sig string }{
		{"go", "package main\nimport "},
		{"python", "def "},
		{"typescript", "const "},
		{"rust", "fn main"},
		{"sql", "select "},
		{"shell", "#!/bin/"},
	}
	for _, p := range patterns {
		if strings.Contains(lower, p.sig) {
			out = append(out, p.lang)
		}
	}
	return out
}
