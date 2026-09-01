package sequence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var workflowNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// WorkflowDefinition is an immutable named workflow definition.
type WorkflowDefinition struct {
	Name                   string        `json:"name"`
	Target                 string        `json:"target,omitempty"`
	TargetIndependent      bool          `json:"target_independent"`
	Steps                  []Action      `json:"steps"`
	MaxDuration            time.Duration `json:"-"`
	UnexpectedScreenPolicy string        `json:"unexpected_screen_policy,omitempty"`
	Revision               string        `json:"revision"`
}

type workflowInput struct {
	Name                   string   `json:"name"`
	Target                 *string  `json:"target"`
	TargetIndependent      bool     `json:"target_independent"`
	Steps                  []Action `json:"steps"`
	MaxDurationMS          int      `json:"max_duration_ms"`
	UnexpectedScreenPolicy string   `json:"unexpected_screen_policy"`
	Revision               *string  `json:"revision"`
}

// Repository stores validated workflows in deterministic name order.
type Repository struct {
	definitions []WorkflowDefinition
	byName      map[string]WorkflowDefinition
}

// LoadWorkflowRepository loads a JSON workflow repository from path.
func LoadWorkflowRepository(path string) (Repository, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Repository{}, fmt.Errorf("unable to load workflow definitions: %w", err)
	}
	return LoadWorkflowRepositoryBytes(data)
}

// LoadWorkflowRepositoryBytes loads a JSON array or an object containing workflows.
func LoadWorkflowRepositoryBytes(data []byte) (Repository, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Repository{}, errors.New("workflow file must contain a list or workflows list")
	}
	var entries []json.RawMessage
	if data[0] == '[' {
		if err := json.Unmarshal(data, &entries); err != nil {
			return Repository{}, fmt.Errorf("invalid workflow definitions: %w", err)
		}
	} else {
		var envelope struct {
			Workflows json.RawMessage `json:"workflows"`
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&envelope); err != nil || len(envelope.Workflows) == 0 || bytes.Equal(envelope.Workflows, []byte("null")) {
			if err != nil {
				return Repository{}, fmt.Errorf("invalid workflow definitions: %w", err)
			}
			return Repository{}, errors.New("workflow file must contain a list or workflows list")
		}
		if err := json.Unmarshal(envelope.Workflows, &entries); err != nil {
			return Repository{}, fmt.Errorf("invalid workflow definitions: %w", err)
		}
	}
	definitions := make([]WorkflowDefinition, 0, len(entries))
	for _, entry := range entries {
		var input workflowInput
		dec := json.NewDecoder(bytes.NewReader(entry))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&input); err != nil {
			return Repository{}, fmt.Errorf("invalid workflow definition: %w", err)
		}
		definition, err := newWorkflowDefinition(input)
		if err != nil {
			return Repository{}, err
		}
		definitions = append(definitions, definition)
	}
	return newWorkflowRepository(definitions)
}

func newWorkflowDefinition(input workflowInput) (WorkflowDefinition, error) {
	if input.Name == "" || !workflowNamePattern.MatchString(input.Name) {
		return WorkflowDefinition{}, errors.New("invalid workflow name")
	}
	if len(input.Steps) == 0 {
		return WorkflowDefinition{}, errors.New("steps must be non-empty")
	}
	if input.TargetIndependent {
		if input.Target != nil {
			return WorkflowDefinition{}, errors.New("target-independent workflow cannot declare target")
		}
	} else if input.Target == nil || strings.TrimSpace(*input.Target) == "" {
		return WorkflowDefinition{}, errors.New("target must be a non-empty string")
	}
	maxMS := input.MaxDurationMS
	if maxMS == 0 {
		maxMS = 30000
	}
	policy := input.UnexpectedScreenPolicy
	if policy == "" {
		policy = "abort"
	}
	target := ""
	if input.Target != nil {
		target = *input.Target
	}
	planTarget := target
	if input.TargetIndependent {
		planTarget = "__target_independent__"
	}
	plan := Plan{Target: planTarget, Actions: cloneActions(input.Steps), MaxDuration: time.Duration(maxMS) * time.Millisecond, UnexpectedScreenPolicy: policy}
	if err := Validate(plan); err != nil {
		return WorkflowDefinition{}, fmt.Errorf("invalid workflow plan: %w", err)
	}
	revision := workflowRevision(input.Name, input.TargetIndependent, plan)
	if input.Revision != nil && *input.Revision != revision {
		return WorkflowDefinition{}, errors.New("workflow revision mismatch")
	}
	return WorkflowDefinition{Name: input.Name, Target: target, TargetIndependent: input.TargetIndependent, Steps: cloneActions(input.Steps), MaxDuration: plan.MaxDuration, UnexpectedScreenPolicy: policy, Revision: revision}, nil
}

func workflowRevision(name string, independent bool, plan Plan) string {
	target := any(plan.Target)
	if independent {
		target = nil
	}
	payload := struct {
		Name              string `json:"name"`
		Target            any    `json:"target"`
		TargetIndependent bool   `json:"target_independent"`
		Plan              struct {
			Target                 any      `json:"target"`
			Actions                []Action `json:"actions"`
			MaxDurationMS          int64    `json:"max_duration_ms"`
			UnexpectedScreenPolicy string   `json:"unexpected_screen_policy"`
		} `json:"plan"`
	}{Name: name, Target: target, TargetIndependent: independent}
	payload.Plan.Target = target
	payload.Plan.Actions = plan.Actions
	payload.Plan.MaxDurationMS = plan.MaxDuration.Milliseconds()
	payload.Plan.UnexpectedScreenPolicy = plan.UnexpectedScreenPolicy
	encoded, _ := json.Marshal(payload)
	hash := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func newWorkflowRepository(definitions []WorkflowDefinition) (Repository, error) {
	byName := make(map[string]WorkflowDefinition, len(definitions))
	for _, definition := range definitions {
		if _, exists := byName[definition.Name]; exists {
			return Repository{}, errors.New("duplicate workflow name")
		}
		byName[definition.Name] = definition
	}
	ordered := append([]WorkflowDefinition(nil), definitions...)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j].Name < ordered[j-1].Name; j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	return Repository{definitions: ordered, byName: byName}, nil
}

func cloneActions(actions []Action) []Action {
	return append([]Action(nil), actions...)
}

// List returns validated workflow definitions sorted by name.
func (r Repository) List() []WorkflowDefinition {
	out := make([]WorkflowDefinition, len(r.definitions))
	for i, definition := range r.definitions {
		out[i] = definition
		out[i].Steps = cloneActions(definition.Steps)
	}
	return out
}

// Resolve validates the immutable revision and binds a target for execution.
func (r Repository) Resolve(name, revision, target string) (Plan, error) {
	definition, ok := r.byName[name]
	if !ok {
		return Plan{}, errors.New("unknown workflow")
	}
	if revision != definition.Revision {
		return Plan{}, errors.New("workflow revision mismatch")
	}
	if definition.TargetIndependent {
		if strings.TrimSpace(target) == "" {
			return Plan{}, errors.New("target is required")
		}
	} else if target != definition.Target {
		return Plan{}, errors.New("workflow target mismatch")
	}
	return Plan{Target: target, Actions: cloneActions(definition.Steps), MaxDuration: definition.MaxDuration, UnexpectedScreenPolicy: definition.UnexpectedScreenPolicy}, nil
}

// Inspect returns canonical JSON with action values and screen assertions redacted.
func (r Repository) Inspect(name, revision, target string) ([]byte, error) {
	definition, ok := r.byName[name]
	if !ok {
		return nil, errors.New("unknown workflow")
	}
	if revision != "" && revision != definition.Revision {
		return nil, errors.New("workflow revision mismatch")
	}
	if target != "" {
		if _, err := r.Resolve(name, definition.Revision, target); err != nil {
			return nil, err
		}
	}
	actions := cloneActions(definition.Steps)
	for i := range actions {
		if actions[i].Value != "" {
			actions[i].Value = "[REDACTED]"
		}
		if actions[i].Contains != "" {
			actions[i].Contains = "[REDACTED]"
		}
	}
	var targetValue any = definition.Target
	if definition.TargetIndependent {
		targetValue = nil
	}
	result := struct {
		Name                   string   `json:"name"`
		Revision               string   `json:"revision"`
		Target                 any      `json:"target"`
		TargetIndependent      bool     `json:"target_independent"`
		MaxDurationMS          int64    `json:"max_duration_ms"`
		UnexpectedScreenPolicy string   `json:"unexpected_screen_policy"`
		Actions                []Action `json:"actions"`
		Steps                  []Action `json:"steps"`
	}{definition.Name, definition.Revision, targetValue, definition.TargetIndependent, definition.MaxDuration.Milliseconds(), definition.UnexpectedScreenPolicy, actions, actions}
	return json.Marshal(result)
}
