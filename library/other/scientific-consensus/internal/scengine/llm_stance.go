package scengine

// Optional LLM-assisted stance classification.
//
// This is an OPT-IN upgrade path layered on top of the keyless heuristic
// ClassifyStance in stance.go. When one of the supported provider API keys is
// present in the environment, ClassifyStanceWithLLM asks a small, cheap model
// to classify a study's stance toward a claim and returns strict JSON. If no
// key is set, or the call/parse fails for any reason, the caller falls back to
// the heuristic — the LLM path never blocks, never retries indefinitely, and
// never crashes the command.
//
// All six integrations use net/http directly: no external SDKs, no Python, no
// subprocess calls. Each provider's request/response handling lives in its own
// small private function (callAnthropic, callOpenAI, callDeepSeek, callGemini,
// callGroq, callMistral) so they can be reviewed independently.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// llmTimeout bounds a single classification attempt. One attempt only — no
// retries — so a slow or unreachable provider degrades to the heuristic fast.
const llmTimeout = 15 * time.Second

// provider identifies which API key was found and which integration to call.
// They are checked in this fixed priority order.
type provider struct {
	name   string // short label surfaced to CLI output, e.g. "anthropic"
	envVar string
	call   func(ctx context.Context, apiKey, prompt string) (string, error)
}

// providers is the priority-ordered list. The FIRST one whose env var is set
// (non-empty) wins.
//
// deepseek sits after the two incumbents and before the rest: it is the
// cheapest per-call option here, which matters because stance classification
// issues ONE call PER WORK, so a 25-work run is 25 calls. Placing it above
// gemini/groq/mistral means a user who has configured several keys gets the
// low-cost path by default, while anyone who deliberately set an Anthropic or
// OpenAI key still gets what they asked for.
var providers = []provider{
	{name: "anthropic", envVar: "ANTHROPIC_API_KEY", call: callAnthropic},
	{name: "openai", envVar: "OPENAI_API_KEY", call: callOpenAI},
	{name: "deepseek", envVar: "DEEPSEEK_API_KEY", call: callDeepSeek},
	{name: "gemini", envVar: "GEMINI_API_KEY", call: callGemini},
	{name: "groq", envVar: "GROQ_API_KEY", call: callGroq},
	{name: "mistral", envVar: "MISTRAL_API_KEY", call: callMistral},
}

// activeProvider returns the first configured provider and its key, or nil if
// none of the supported keys are set.
func activeProvider() (*provider, string) {
	for i := range providers {
		if key := strings.TrimSpace(os.Getenv(providers[i].envVar)); key != "" {
			return &providers[i], key
		}
	}
	return nil, ""
}

// LLMConfigured reports whether any supported provider key is set, without
// making a network call. Used by the dispatcher and CLI to decide whether the
// LLM path should be attempted.
func LLMConfigured() bool {
	p, _ := activeProvider()
	return p != nil
}

// LLMProviderName returns the name of the active provider ("anthropic",
// "openai", …) or "" if none is configured.
func LLMProviderName() string {
	if p, _ := activeProvider(); p != nil {
		return p.name
	}
	return ""
}

// ClassifyStanceWithLLM classifies a single work's stance toward claim using the
// first configured provider. ctx carries the caller's deadline/cancellation; the
// per-call llmTimeout still applies on top as an upper bound. It returns an
// error (so the caller falls back to the heuristic) if no key is set, the HTTP
// call fails, or the response can't be parsed into a valid {stance, confidence}
// pair. confidence is clamped to 0..1.
func ClassifyStanceWithLLM(ctx context.Context, title, abstract, claim string) (Stance, float64, error) {
	p, key := activeProvider()
	if p == nil {
		return "", 0, errors.New("no LLM provider configured (set one of ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, GEMINI_API_KEY, GROQ_API_KEY, MISTRAL_API_KEY)")
	}

	ctx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	raw, err := p.call(ctx, key, stancePrompt(title, abstract, claim))
	if err != nil {
		return "", 0, fmt.Errorf("%s stance classification failed: %w", p.name, err)
	}
	return parseStanceJSON(raw)
}

// stancePrompt builds the shared instruction sent to every provider. It pins the
// model to STRICT JSON with exactly the two fields we parse.
func stancePrompt(title, abstract, claim string) string {
	var b strings.Builder
	b.WriteString("You classify whether a single scientific study supports, refutes, ")
	b.WriteString("or is mixed/inconclusive about a specific claim.\n\n")
	b.WriteString("Claim: ")
	b.WriteString(claim)
	b.WriteString("\n\nStudy title: ")
	b.WriteString(title)
	b.WriteString("\n\nStudy abstract: ")
	if strings.TrimSpace(abstract) == "" {
		b.WriteString("(none provided)")
	} else {
		b.WriteString(abstract)
	}
	b.WriteString("\n\nRespond with STRICT JSON ONLY, no prose, no markdown fences, ")
	b.WriteString(`exactly this shape: {"stance": "supporting|refuting|mixed|inconclusive", "confidence": 0.0-1.0}. `)
	b.WriteString("\"stance\" must be one of those four literal values. ")
	b.WriteString("\"confidence\" is your certainty as a number from 0.0 to 1.0.")
	return b.String()
}

// jsonObjectRe extracts the first {...} object from a model reply, tolerating
// stray prose or markdown fences a model might add despite instructions.
var jsonObjectRe = regexp.MustCompile(`(?s)\{.*\}`)

type stanceReply struct {
	Stance     string  `json:"stance"`
	Confidence float64 `json:"confidence"`
}

// parseStanceJSON turns a model reply into a Stance + confidence. It fails
// (returning an error so the caller can fall back) on any malformed or
// out-of-range output.
func parseStanceJSON(raw string) (Stance, float64, error) {
	match := jsonObjectRe.FindString(raw)
	if match == "" {
		return "", 0, fmt.Errorf("no JSON object in model reply: %q", truncateForErr(raw))
	}

	var reply stanceReply
	if err := json.Unmarshal([]byte(match), &reply); err != nil {
		return "", 0, fmt.Errorf("invalid stance JSON: %w", err)
	}

	stance := Stance(strings.ToLower(strings.TrimSpace(reply.Stance)))
	switch stance {
	case StanceSupporting, StanceRefuting, StanceMixed, StanceInconclusive:
		// valid
	default:
		return "", 0, fmt.Errorf("unrecognized stance value: %q", reply.Stance)
	}

	conf := reply.Confidence
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}
	return stance, conf, nil
}

func truncateForErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

// credentialParamRe matches credential query-param values so an API key carried
// in a URL (e.g. Gemini's ?key=…) never leaks into error text sent to stderr.
var credentialParamRe = regexp.MustCompile(`(?i)([?&](?:api_key|apikey|key|token|access_token)=)[^&\s"]*`)

// redactCredentials replaces credential query-param values in s with REDACTED,
// leaving s unchanged when no credential param is present. Used to scrub the
// *url.Error transport-failure messages returned by doJSON.
func redactCredentials(s string) string {
	return credentialParamRe.ReplaceAllString(s, "${1}REDACTED")
}

// httpClient is the single-attempt client shared by all providers.
var httpClient = &http.Client{Timeout: llmTimeout}

// doJSON posts body to url with the given headers and returns the response body,
// erroring on any non-2xx status. Shared by every provider.
func doJSON(ctx context.Context, url string, body any, headers map[string]string) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		// Transport failures (DNS/timeout/TLS) yield a *url.Error whose Error()
		// embeds the full request URL. Gemini carries its API key in the query
		// string (?key=…), so return a redacted message to keep the key out of
		// stderr. See redactCredentials.
		return nil, errors.New(redactCredentials(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncateForErr(string(data)))
	}
	return data, nil
}

// ---- Anthropic Messages API -------------------------------------------------

func callAnthropic(ctx context.Context, apiKey, prompt string) (string, error) {
	body := map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 256,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	data, err := doJSON(ctx, "https://api.anthropic.com/v1/messages", body, map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Content) == 0 {
		return "", errors.New("empty anthropic response")
	}
	return out.Content[0].Text, nil
}

// ---- OpenAI Chat Completions API -------------------------------------------

func callOpenAI(ctx context.Context, apiKey, prompt string) (string, error) {
	return openAICompatible(ctx,
		"https://api.openai.com/v1/chat/completions",
		"gpt-4o-mini", apiKey, prompt)
}

// ---- DeepSeek (OpenAI-compatible) ------------------------------------------

// callDeepSeek uses DeepSeek's OpenAI-compatible endpoint. The model name is a
// deliberate constant rather than an env var: a stance corpus measured under
// one model is not comparable to one measured under another, and letting the
// model drift silently between runs would make every future regression
// unattributable.
func callDeepSeek(ctx context.Context, apiKey, prompt string) (string, error) {
	return openAICompatible(ctx,
		"https://api.deepseek.com/v1/chat/completions",
		"deepseek-chat", apiKey, prompt)
}

// ---- Groq (OpenAI-compatible) ----------------------------------------------

func callGroq(ctx context.Context, apiKey, prompt string) (string, error) {
	return openAICompatible(ctx,
		"https://api.groq.com/openai/v1/chat/completions",
		"llama-3.3-70b-versatile", apiKey, prompt)
}

// ---- Mistral (OpenAI-compatible) -------------------------------------------

func callMistral(ctx context.Context, apiKey, prompt string) (string, error) {
	return openAICompatible(ctx,
		"https://api.mistral.ai/v1/chat/completions",
		"mistral-small-latest", apiKey, prompt)
}

// openAICompatible handles the four providers that share the OpenAI Chat
// Completions request/response shape (OpenAI, DeepSeek, Groq, Mistral).
func openAICompatible(ctx context.Context, url, model, apiKey, prompt string) (string, error) {
	body := map[string]any{
		"model":       model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	data, err := doJSON(ctx, url, body, map[string]string{
		"Authorization": "Bearer " + apiKey,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("empty chat-completions response")
	}
	return out.Choices[0].Message.Content, nil
}

// ---- Google Gemini API ------------------------------------------------------

func callGemini(ctx context.Context, apiKey, prompt string) (string, error) {
	// The API key goes on the query string for the generateContent endpoint.
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + apiKey
	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
	}
	data, err := doJSON(ctx, url, body, nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty gemini response")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}
