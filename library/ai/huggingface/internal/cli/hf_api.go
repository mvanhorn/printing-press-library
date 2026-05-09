package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

// hfAPIBase is the canonical Hugging Face Hub API base. Hard-coded because
// the seed locks the data source; commands do not multi-host.
const hfAPIBase = "https://huggingface.co"

// hfDefaultUA is sent on every outbound request so HF can attribute traffic
// (and so we can find the requests in their logs if rate-limit semantics
// surprise us).
const hfDefaultUA = "huggingface-pp-cli/0.1.0"

// hfModel is the parsed shape of /api/models/{id} (and list-element shape from
// /api/models). We deliberately decode only the fields the seed commands use;
// the rest of the response is preserved verbatim in Raw for --explain and
// future use.
type hfModel struct {
	ID            string         `json:"id"`
	Author        string         `json:"author"`
	LibraryName   string         `json:"library_name"`
	PipelineTag   string         `json:"pipeline_tag"`
	Tags          []string       `json:"tags"`
	Downloads     int            `json:"downloads"`
	Likes         int            `json:"likes"`
	LastModified  string         `json:"lastModified"`
	Gated         json.RawMessage `json:"gated"`
	Private       bool           `json:"private"`
	Disabled      bool           `json:"disabled"`
	Siblings      []hfSibling    `json:"siblings"`
	CardData      hfCardData     `json:"cardData"`
	Config        map[string]any `json:"config"`
	GGUF          map[string]any `json:"gguf"`
	ModelIndex    json.RawMessage `json:"model-index"`
	Raw           json.RawMessage `json:"-"`
}

type hfSibling struct {
	Path     string `json:"rfilename"`
	Size     int64  `json:"size"`
	BlobID   string `json:"blobId"`
	LFS      *hfLFS `json:"lfs"`
}

type hfLFS struct {
	Size       int64  `json:"size"`
	Sha256     string `json:"sha256"`
	PointerSize int   `json:"pointerSize"`
}

type hfCardData struct {
	License       string   `json:"license"`
	BaseModel     any      `json:"base_model"` // string or []string
	Language      []string `json:"language"`
	Tags          []string `json:"tags"`
	Datasets      []string `json:"datasets"`
	PipelineTag   string   `json:"pipeline_tag"`
	LibraryName   string   `json:"library_name"`
	ContextLength int      `json:"context_length"`
}

// hfFetchModel GETs /api/models/{id}?blobs=true. Always passes blobs=true so
// sibling sizes are populated (per API mechanics: siblings without blobs query
// have 0 sizes).
func hfFetchModel(ctx context.Context, id string, token string) (*hfModel, int, error) {
	// HF rejects %2F-encoded slashes in repo paths; preserve the literal slash
	// while still escaping any other unsafe chars (rare but safer than raw concat).
	u := fmt.Sprintf("%s/api/models/%s?blobs=true", hfAPIBase, hfPathPreserveSlash(id))
	body, status, err := hfDoGET(ctx, u, token)
	if err != nil {
		return nil, status, err
	}
	if status == http.StatusNotFound {
		return nil, status, fmt.Errorf("model %q not found", id)
	}
	if status == http.StatusTooManyRequests {
		return nil, status, fmt.Errorf("rate limited (HTTP 429)")
	}
	if status >= 400 {
		return nil, status, fmt.Errorf("HTTP %d fetching model %s: %s", status, id, snippet(body))
	}
	var m hfModel
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, status, fmt.Errorf("parsing model card: %w", err)
	}
	m.Raw = body
	return &m, status, nil
}

// hfListModels GETs /api/models with the given query params. Caller assembles
// the search/filter/sort/limit/author args.
func hfListModels(ctx context.Context, q url.Values, token string) ([]hfModel, int, error) {
	u := hfAPIBase + "/api/models?" + q.Encode()
	body, status, err := hfDoGET(ctx, u, token)
	if err != nil {
		return nil, status, err
	}
	if status == http.StatusTooManyRequests {
		return nil, status, fmt.Errorf("rate limited (HTTP 429)")
	}
	if status >= 400 {
		return nil, status, fmt.Errorf("HTTP %d listing models: %s", status, snippet(body))
	}
	var ms []hfModel
	if err := json.Unmarshal(body, &ms); err != nil {
		return nil, status, fmt.Errorf("parsing model list: %w", err)
	}
	return ms, status, nil
}

// hfFetchRaw GETs /api/models/{id}/raw/{rev}/{path}. Used to fetch
// config.json and README.md.
func hfFetchRaw(ctx context.Context, id, revision, path, token string) ([]byte, int, error) {
	if revision == "" {
		revision = "main"
	}
	u := fmt.Sprintf("%s/%s/raw/%s/%s", hfAPIBase, hfPathPreserveSlash(id), revision, path)
	return hfDoGET(ctx, u, token)
}

// hfRequestState carries per-process rate-limit + state-dir context into
// hfDoGET. Set once at command boot via hfSetRequestState; nil-safe (helpers
// fall through to plain HTTP without persistence when unset, e.g. tests).
type hfRequestState struct {
	stateDir string
	noWrite  bool
}

var globalRequestState hfRequestState

func hfSetRequestState(stateDir string, noWrite bool) {
	globalRequestState = hfRequestState{stateDir: stateDir, noWrite: noWrite}
}

func hfDoGET(ctx context.Context, u, token string) ([]byte, int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
	}

	// Pre-flight: refuse outbound when the persisted bucket says remaining=0
	// and the reset window has not elapsed. CC + JARVIS + cron see the same
	// state via flock.
	if globalRequestState.stateDir != "" {
		s := hfx.LoadRateLimit(globalRequestState.stateDir)
		if s.Remaining == 0 && s.ResetSeconds > 0 && time.Since(s.UpdatedAt) < time.Duration(s.ResetSeconds)*time.Second {
			waitSec := s.ResetSeconds - int(time.Since(s.UpdatedAt).Seconds())
			return nil, http.StatusTooManyRequests, fmt.Errorf("rate limit exhausted; %ds until reset (shared bucket)", waitSec)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", hfDefaultUA)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)

	// Snapshot rate-limit header on every response (success OR failure).
	// Persist iff we have a state dir AND --no-write isn't set. A persist
	// failure is non-fatal — pre-flight reads the on-disk view and the next
	// process starts cold.
	if globalRequestState.stateDir != "" {
		if rem, reset, raw := hfx.ParseRateLimitHeader(resp.Header); rem >= 0 || reset >= 0 {
			_ = hfx.SaveRateLimit(globalRequestState.stateDir, hfx.RateLimitState{
				Remaining:    rem,
				ResetSeconds: reset,
				WindowSeen:   raw,
			}, globalRequestState.noWrite)
		}
	}

	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	return body, resp.StatusCode, nil
}

func snippet(body []byte) string {
	s := string(body)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return strings.TrimSpace(s)
}

// hfBaseModelStrings extracts base-model ids from cardData.base_model, which
// can be a string OR a []string. Returns nil for the unset case.
func hfBaseModelStrings(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// hfHumanGB formats a byte count as a 1-decimal GB string (e.g. "18.4 GB").
func hfHumanGB(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1f GB", gb)
}

// hfMaxStrings de-dupes and caps a string slice at n.
func hfMaxStrings(s []string, n int) []string {
	seen := map[string]bool{}
	out := make([]string, 0, n)
	for _, v := range s {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) >= n {
			break
		}
	}
	return out
}

// hfPathPreserveSlash escapes path-segment-unsafe characters while leaving
// literal slashes alone. HuggingFace rejects %2F-encoded slashes in
// /api/models/{owner}/{name} paths.
func hfPathPreserveSlash(id string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// hfTokenForRequests resolves the auth token using the doctor's resolver.
// Centralized here so every command picks up the same precedence.
func hfTokenForRequests() string {
	t, _ := resolveHFToken()
	return t
}

// hfOpenclawRoot returns the root directory the CLI's stack-aware commands
// auto-discover for `data/openclaw.json` and `workspace/scripts/model-eval-harness/`.
//
// Resolution: HF_OPENCLAW_ROOT env > "" (CWD only). Stack-aware commands fall
// back to the relative paths `data/openclaw.json` and
// `workspace/scripts/model-eval-harness` against the returned root, so
// running the CLI from inside the OpenClaw-style repo works out of the box;
// running anywhere else is a clean exit-6 unless --config-path / --harness
// are passed.
//
// No hardcoded user-specific defaults — the seed plan's "live-config gracefully
// optional" rule means missing context exits 6 with a clear message rather
// than guessing at unowned filesystem locations.
func hfOpenclawRoot() string {
	if v := os.Getenv("HF_OPENCLAW_ROOT"); v != "" {
		return v
	}
	return ""
}

// hfMatrixContextHash returns a short identifier for the active matrix's
// freshness, used in --explain blocks. Cheap helper.
func hfMatrixContextHash(m hfx.BackendMatrix) string {
	if len(m.Entries) == 0 {
		return "empty"
	}
	return fmt.Sprintf("%d-entries", len(m.Entries))
}
