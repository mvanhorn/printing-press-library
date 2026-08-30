package scengine

import "context"

// Top-level stance dispatcher: prefer the optional LLM path when a provider key
// is configured, otherwise use the keyless heuristic. ANY LLM error falls back
// to the heuristic so classification always succeeds. The method string it
// returns ("heuristic" or "llm:<provider>") reflects what was actually used, so
// the CLI can surface it.

// ClassifyStanceAuto classifies a work's stance, choosing the method
// automatically. ctx bounds the optional LLM call (a cancelled ctx falls back
// to the heuristic). It returns the stance, confidence (0..1), and the method
// actually used: "heuristic" when no key is set or the LLM call failed, or
// "llm:<provider>" (e.g. "llm:anthropic") on a successful LLM classification.
func ClassifyStanceAuto(ctx context.Context, title, abstract, claim string) (Stance, float64, string) {
	if LLMConfigured() {
		if st, conf, err := ClassifyStanceWithLLM(ctx, title, abstract, claim); err == nil {
			return st, conf, "llm:" + LLMProviderName()
		}
		// Any error (no key race, HTTP failure, bad JSON) → heuristic fallback.
	}
	st, conf := ClassifyStance(title, abstract, claim)
	return st, conf, "heuristic"
}
