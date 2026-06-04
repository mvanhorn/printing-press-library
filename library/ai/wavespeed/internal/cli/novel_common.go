// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
)

// novelEnvelopeVersion is the schema version of AgentEnvelope. Bump when the
// envelope shape changes in a way agents must notice.
const novelEnvelopeVersion = "1"

// Shot is the single canonical spec for one generation, shared by every novel
// command. Plan commands emit []Shot; produce/refine commands consume them.
// This one type is what keeps the surface from becoming many ad-hoc input
// builders.
type Shot struct {
	Concept     string         `json:"concept,omitempty"`
	Prompt      string         `json:"prompt"`
	Model       string         `json:"model,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	AspectRatio string         `json:"aspect_ratio,omitempty"`
	Platform    string         `json:"platform,omitempty"`
	Format      string         `json:"format,omitempty"` // feed | story | reel
	Seed        *int64         `json:"seed,omitempty"`
	Brand       string         `json:"brand,omitempty"`
	EstCost     float64        `json:"est_cost,omitempty"`
	// Inputs carries step-to-step wiring for `compose` (e.g. an upstream image
	// URL fed into the next step). Empty for single-step shots.
	Inputs map[string]any `json:"inputs,omitempty"`
}

// toModelInputs renders a Shot into the model input map the client submits.
// Prompt and the explicit params/inputs are merged; params win over the bare
// prompt key only if they set one.
func (s Shot) toModelInputs() map[string]any {
	inputs := map[string]any{}
	for k, v := range s.Params {
		inputs[k] = v
	}
	for k, v := range s.Inputs {
		inputs[k] = v
	}
	if s.Prompt != "" {
		if _, ok := inputs["prompt"]; !ok {
			inputs["prompt"] = s.Prompt
		}
	}
	if s.Seed != nil {
		if _, ok := inputs["seed"]; !ok {
			inputs["seed"] = *s.Seed
		}
	}
	return inputs
}

// AgentEnvelope is the uniform structured output every novel command emits.
// --compact is intentionally suppressed when an envelope is in effect (it
// would mangle the nested JSON), so emitEnvelope always pretty-prints.
type AgentEnvelope struct {
	Command             string   `json:"command"`
	Version             string   `json:"version"`
	Args                any      `json:"args,omitempty"`
	DryRun              bool     `json:"dry_run"`
	PlannerUsed         string   `json:"planner_used,omitempty"`
	Results             []any    `json:"results"`
	SuggestedNext       []string `json:"suggested_next,omitempty"`
	RecommendedAction   string   `json:"recommended_action,omitempty"`
	// Manifests lists per-platform manifest.json paths a producer wrote. Kept
	// distinct from Warnings so agents don't misread successful output as a
	// problem.
	Manifests []string `json:"manifests,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	LibraryRecordErrors []string `json:"library_record_errors"`
	CostSpent           float64  `json:"cost_spent"`
	BalanceAfter        *float64 `json:"balance_after,omitempty"`
	PartialFailure      bool     `json:"partial_failure"`
}

// newEnvelope builds an envelope with non-nil slices so JSON output always has
// stable [] (not null) for results and library_record_errors.
func newEnvelope(command string) *AgentEnvelope {
	return &AgentEnvelope{
		Command:             command,
		Version:             novelEnvelopeVersion,
		Results:             []any{},
		LibraryRecordErrors: []string{},
	}
}

// emitEnvelope writes the envelope as indented JSON to stdout. It ignores
// --compact by design; --json/--agent already select JSON output, and the
// envelope is the contract regardless of the human/JSON toggle.
func emitEnvelope(w io.Writer, env *AgentEnvelope) error {
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}
	_, err = fmt.Fprintln(w, string(raw))
	return err
}

// ---- IDs and hashing ----------------------------------------------------

// newGenerationID returns a sortable-ish unique id: unix-nanos hex + 8 random
// hex chars. crypto/rand is fine in Go (the Math.random ban is a Workflow-JS
// constraint, not a Go one).
func newGenerationID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("gen_%x_%s", time.Now().UTC().UnixNano(), hex.EncodeToString(b[:]))
}

func newBrandID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "brand_" + hex.EncodeToString(b[:])
}

// hashContent returns a short sha256 hex prefix for content-drift detection.
func hashContent(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

// ---- Library + record policy -------------------------------------------

// openLibrary opens the library DB at the resolved path.
func openLibrary() (*store.Store, error) {
	return store.OpenLibrary(libraryDBPath())
}

// openArchiveReadOnly opens the archive DB read-only for cache lookups (e.g.
// cached pricing). Returns an error if the archive file does not exist.
func openArchiveReadOnly() (*store.Store, error) {
	return store.OpenReadOnly(archiveDBPath())
}

// recordPolicyFor normalizes the project's record policy. Default is
// "novel-only": novel commands record, plain `run` does not (unless --record).
func recordPolicyFor(project wavespeedProjectConfig) string {
	switch strings.ToLower(strings.TrimSpace(project.Record)) {
	case "always":
		return "always"
	case "never":
		return "never"
	default:
		return "novel-only"
	}
}

// shouldRecord decides whether a command should write to the library, given
// the project policy, whether the command is novel, and an explicit opt-out.
func shouldRecord(project wavespeedProjectConfig, isNovel, noRecordFlag bool) bool {
	if noRecordFlag {
		return false
	}
	switch recordPolicyFor(project) {
	case "always":
		return true
	case "never":
		return false
	default: // novel-only
		return isNovel
	}
}

// recordGeneration writes one generation to the library. It opens the library
// DB, records, and closes. Any failure is returned (never panics); callers log
// it into library_record_errors and continue — a record failure must never
// fail a successful generation.
func recordGeneration(g store.Generation) error {
	if g.ID == "" {
		g.ID = newGenerationID()
	}
	s, err := openLibrary()
	if err != nil {
		return fmt.Errorf("opening library: %w", err)
	}
	defer s.Close()
	return s.RecordGeneration(g)
}

// recordRunGeneration records a plain `run` invocation when --record is set.
// Used by run.go. Errors are returned for the caller to log.
func recordRunGeneration(modelID string, inputs map[string]any, res submitResult) error {
	params, _ := json.Marshal(inputs)
	prompt, _ := inputs["prompt"].(string)
	g := store.Generation{
		ID:          newGenerationID(),
		Command:     "run",
		ModelID:     modelID,
		Prompt:      prompt,
		Cost:        extractCostFromPricing(res.Pricing),
		ContentHash: hashContent(res.Result),
		Status:      res.Status,
		Params:      json.RawMessage(params),
		Data:        res.Result,
	}
	if seed, ok := seedFromInputs(inputs); ok {
		g.Seed = &seed
	}
	return recordGeneration(g)
}

func seedFromInputs(inputs map[string]any) (int64, bool) {
	switch v := inputs["seed"].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	}
	return 0, false
}

// ---- Cost + balance -----------------------------------------------------

// extractCostFromPricing best-effort reads a numeric cost from a /model/pricing
// response. WaveSpeed pricing shapes vary by model, so we probe a few common
// keys and unwrap a {"data": ...} envelope. Returns 0 when nothing is found.
func extractCostFromPricing(pricing json.RawMessage) float64 {
	if len(pricing) == 0 {
		return 0
	}
	body := unwrapWaveSpeedData(pricing)
	obj := decodeObject(body)
	for _, key := range []string{"price", "cost", "total", "total_price", "amount", "estimated_cost"} {
		if f, ok := toFloat(obj[key]); ok {
			return f
		}
	}
	return 0
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// fetchBalance best-effort reads the account balance for the envelope's
// balance_after field. A failure returns nil (no balance reported), never an
// error — balance is advisory.
func fetchBalance(ctx context.Context, c *client.Client) *float64 {
	data, err := c.GetNoCache(ctx, "/balance", nil)
	if err != nil {
		return nil
	}
	obj := decodeObject(unwrapWaveSpeedData(data))
	for _, key := range []string{"balance", "credits", "available", "amount"} {
		if f, ok := toFloat(obj[key]); ok {
			return &f
		}
	}
	return nil
}

// ---- Brand profiles + precedence ---------------------------------------

// brandProfileBody is the stored brand profile payload (library.db data column).
type brandProfileBody struct {
	StyleAnchors []string       `json:"style_anchors,omitempty"`
	Negative     string         `json:"negative,omitempty"`
	Palette      []string       `json:"palette,omitempty"`
	Voice        string         `json:"voice,omitempty"`
	Models       []string       `json:"models,omitempty"`
	Platforms    []string       `json:"platforms,omitempty"`
	Params       map[string]any `json:"params,omitempty"`
}

// loadBrandProfile fetches a brand profile by name from the library DB.
func loadBrandProfile(name string) (store.BrandProfile, brandProfileBody, error) {
	var body brandProfileBody
	s, err := openLibrary()
	if err != nil {
		return store.BrandProfile{}, body, err
	}
	defer s.Close()
	prof, err := s.GetBrandProfile(name)
	if err != nil {
		return store.BrandProfile{}, body, err
	}
	if len(prof.Data) > 0 {
		_ = json.Unmarshal(prof.Data, &body)
	}
	return prof, body, nil
}

// resolveActiveBrand returns the brand name a command should use, honoring the
// per-command --brand flag over the project's activeBrand pointer.
func resolveActiveBrand(project wavespeedProjectConfig, brandFlag string) string {
	if strings.TrimSpace(brandFlag) != "" {
		return strings.TrimSpace(brandFlag)
	}
	return strings.TrimSpace(project.ActiveBrand)
}

// mergeBrandIntoShot applies a brand profile to a shot following the precedence
// rule: alias defaults < explicit -i/--set < active brand profile < per-command
// --brand flag. The shot passed in already carries alias+explicit values; this
// fills brand-supplied gaps and appends brand prompt anchors. A per-command
// --brand flag resolves to this same body, so flag-vs-active is settled by the
// caller via resolveActiveBrand before this is called.
func mergeBrandIntoShot(shot Shot, brandName string, body brandProfileBody) Shot {
	if brandName == "" {
		return shot
	}
	shot.Brand = brandName
	if shot.Model == "" && len(body.Models) > 0 {
		shot.Model = body.Models[0]
	}
	if shot.Params == nil {
		shot.Params = map[string]any{}
	}
	for k, v := range body.Params {
		if _, ok := shot.Params[k]; !ok { // explicit shot params win
			shot.Params[k] = v
		}
	}
	if body.Negative != "" {
		if _, ok := shot.Params["negative_prompt"]; !ok {
			shot.Params["negative_prompt"] = body.Negative
		}
	}
	anchors := append([]string{}, body.StyleAnchors...)
	if len(body.Palette) > 0 {
		anchors = append(anchors, "palette: "+strings.Join(body.Palette, ", "))
	}
	if len(anchors) > 0 && shot.Prompt != "" {
		shot.Prompt = shot.Prompt + ", " + strings.Join(anchors, ", ")
	} else if len(anchors) > 0 {
		shot.Prompt = strings.Join(anchors, ", ")
	}
	return shot
}

// ---- shotlist IO --------------------------------------------------------

// readShotlist reads a []Shot from a file path or "-" for stdin.
func readShotlist(path string) ([]Shot, error) {
	var raw []byte
	var err error
	if path == "-" || path == "" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading shotlist: %w", err)
	}
	shots, err := decodeShotlist(raw)
	if err != nil {
		return nil, err
	}
	return shots, nil
}

// decodeShotlist accepts either a bare JSON array of shots or an AgentEnvelope
// whose results are shots (so `plan ... | qa ...` piping works directly).
func decodeShotlist(raw []byte) ([]Shot, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty shotlist")
	}
	if trimmed[0] == '[' {
		var shots []Shot
		if err := json.Unmarshal(raw, &shots); err != nil {
			return nil, fmt.Errorf("parsing shotlist array: %w", err)
		}
		return shots, nil
	}
	// Try an envelope whose results are shots.
	var env struct {
		Results []Shot `json:"results"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Results) > 0 {
		return env.Results, nil
	}
	// Single shot object.
	var single Shot
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, fmt.Errorf("parsing shotlist: %w", err)
	}
	return []Shot{single}, nil
}

// ---- suggested_next -----------------------------------------------------

// suggestNext returns a deduped, ordered list of suggested follow-up commands.
func suggestNext(items ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" || seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	return out
}
