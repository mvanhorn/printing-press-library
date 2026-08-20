// Package normalizecfg defines the declarative, domain-neutral normalization
// config (YAML). It has no dice-fm-specific imports so it can graduate into the
// generator's emitted framework unchanged.
package normalizecfg

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Shape names. Every entity resolves to a canonical entity (the spine); these
// are the optional overlays on top.
const (
	ShapeAlias      = "alias"
	ShapeAttributes = "attributes"
	ShapeVocab      = "vocab"
)

// Rule is an agent-proposed, auto-validated deterministic pattern. match is a
// Go regexp (RE2) over the canonicalized value; set assigns axis values.
type Rule struct {
	Match string            `yaml:"match"`
	Set   map[string]string `yaml:"set"`
}

// Entity declares how one entity type is normalized.
type Entity struct {
	Source     string   `yaml:"source"`               // dotted path into the source resource JSON
	Shape      string   `yaml:"shape"`                // alias | attributes | vocab (core alias-resolution always runs)
	Attributes []string `yaml:"attributes,omitempty"` // axis columns (shape=attributes)
	Vocab      []string `yaml:"vocab,omitempty"`      // controlled set (shape=vocab)
	Rules      []Rule   `yaml:"rules,omitempty"`      // promoted match→set rules (initially empty)
	// StripPattern is an optional Go regexp; matches are removed from each raw
	// value BEFORE canonicalization (e.g. `^[a-z]+:` to drop a namespace
	// prefix). The crosswalk's source value stays the raw value.
	StripPattern string `yaml:"strip_pattern,omitempty"`
}

// Config is the whole normalize config.
type Config struct {
	Version  int               `yaml:"version"`
	Entities map[string]Entity `yaml:"entities"`
}

// validate checks that all entities in c have a known shape and a non-empty
// source, and that every regexp in the entity (strip_pattern and each rule's
// match) compiles. Validating rule.match at load time (alongside strip_pattern)
// turns a typo'd promoted rule into a clear load-time error instead of a rule
// that silently disables itself and matches nothing at classify time.
func validate(c *Config) error {
	for name, e := range c.Entities {
		switch e.Shape {
		case ShapeAlias, ShapeAttributes, ShapeVocab, "":
		default:
			return fmt.Errorf("entity %q: unknown shape %q (want alias|attributes|vocab)", name, e.Shape)
		}
		if e.Source == "" {
			return fmt.Errorf("entity %q: missing source", name)
		}
		if e.StripPattern != "" {
			if _, err := regexp.Compile(e.StripPattern); err != nil {
				return fmt.Errorf("entity %q: invalid strip_pattern %q: %w", name, e.StripPattern, err)
			}
		}
		for i, r := range e.Rules {
			if r.Match == "" {
				return fmt.Errorf("entity %q: rule %d has an empty match pattern", name, i)
			}
			if _, err := regexp.Compile(r.Match); err != nil {
				return fmt.Errorf("entity %q: invalid rule match %q: %w", name, r.Match, err)
			}
		}
	}
	return nil
}

// Parse decodes + validates a YAML config.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing normalize config: %w", err)
	}
	if err := validate(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Merge returns a new Config formed by applying each layer in order; later
// layers override earlier ones per entity name (whole-entity replace). The
// merged result is validated before being returned.
func Merge(layers ...*Config) (*Config, error) {
	out := &Config{
		Version:  1,
		Entities: make(map[string]Entity),
	}
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		if layer.Version != 0 {
			out.Version = layer.Version
		}
		for name, e := range layer.Entities {
			out.Entities[name] = e
		}
	}
	if err := validate(out); err != nil {
		return nil, fmt.Errorf("merged config invalid: %w", err)
	}
	return out, nil
}

// LoadLayered reads the starter file (required) and the operator file
// (optional — absent operator is not an error) and merges them so the operator
// layer wins per entity. A present-but-unreadable or malformed operator file
// is returned as an error.
func LoadLayered(starterPath, operatorPath string) (*Config, error) {
	starterData, err := os.ReadFile(starterPath)
	if err != nil {
		return nil, fmt.Errorf("reading starter config %q: %w", starterPath, err)
	}
	starter, err := Parse(starterData)
	if err != nil {
		return nil, fmt.Errorf("parsing starter config: %w", err)
	}

	opData, err := os.ReadFile(operatorPath)
	if os.IsNotExist(err) {
		// No operator layer — return starter as-is.
		return starter, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading operator config %q: %w", operatorPath, err)
	}
	operator, err := Parse(opData)
	if err != nil {
		return nil, fmt.Errorf("parsing operator config: %w", err)
	}

	return Merge(starter, operator)
}
