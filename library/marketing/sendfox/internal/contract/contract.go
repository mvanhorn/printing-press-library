// Package contract carries the documented SendFox v1.4.0 request schemas.
package contract

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"net/url"
	"strconv"
	"strings"
)

//go:embed operations.json
var files embed.FS

type Parameter struct {
	Name        string         `json:"name"`
	WireName    string         `json:"wire_name"`
	In          string         `json:"in"`
	Required    bool           `json:"required"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}
type Operation struct {
	ID          string         `json:"id"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	Parameters  []Parameter    `json:"parameters"`
	BodySchema  map[string]any `json:"body_schema"`
	Responses   map[string]any `json:"responses"`
}

var Operations = load()

func load() []Operation {
	b, err := files.ReadFile("operations.json")
	if err != nil {
		panic(err)
	}
	var v []Operation
	if err = json.Unmarshal(b, &v); err != nil {
		panic(err)
	}
	return v
}
func Find(id string) *Operation {
	for i := range Operations {
		if Operations[i].ID == id {
			return &Operations[i]
		}
	}
	return nil
}
func Match(method, path string) (*Operation, map[string]string) {
	path = strings.SplitN(path, "?", 2)[0]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Exact routes win over /{id} routes (e.g. contacts/batch).
	for i := range Operations {
		o := &Operations[i]
		if o.Method == method && o.Path == path {
			return o, map[string]string{}
		}
	}
	for i := range Operations {
		o := &Operations[i]
		if o.Method != method {
			continue
		}
		pattern := strings.Split(strings.Trim(o.Path, "/"), "/")
		if len(parts) != len(pattern) {
			continue
		}
		p := map[string]string{}
		ok := true
		for n, s := range pattern {
			if strings.HasPrefix(s, "{") {
				v, err := url.PathUnescape(parts[n])
				if err != nil {
					ok = false
					break
				}
				p[strings.Trim(s, "{}")] = v
			} else if s != parts[n] {
				ok = false
				break
			}
		}
		if ok {
			return o, p
		}
	}
	return nil, nil
}
func Validate(schema map[string]any, value any) error {
	if len(schema) == 0 {
		return nil
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("urn:sendfox:schema", schema); err != nil {
		return err
	}
	s, err := c.Compile("urn:sendfox:schema")
	if err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var v any
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	return s.Validate(v)
}
func Scalar(s string, schema map[string]any) any {
	switch schema["type"] {
	case "integer", "number":
		v, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return v
		}
	case "boolean":
		v, err := strconv.ParseBool(s)
		if err == nil {
			return v
		}
	case "array", "object":
		var v any
		if json.Unmarshal([]byte(s), &v) == nil {
			return v
		}
	}
	return s
}
func ValidateRequest(o *Operation, pathParams, query map[string]string, body any) error {
	for _, p := range o.Parameters {
		var value string
		var found bool
		switch p.In {
		case "path":
			value, found = pathParams[p.WireName]
		case "query":
			value, found = query[p.WireName]
		default:
			continue
		}
		if !found {
			if p.Required {
				return fmt.Errorf("%s requires %s parameter %s", o.ID, p.In, p.Name)
			}
			continue
		}
		if err := Validate(p.Schema, Scalar(value, p.Schema)); err != nil {
			return fmt.Errorf("%s parameter %s: %w", o.ID, p.Name, err)
		}
	}
	if len(o.BodySchema) > 0 {
		if body == nil {
			body = map[string]any{}
		}
		if err := Validate(o.BodySchema, body); err != nil {
			return fmt.Errorf("%s body: %w", o.ID, err)
		}
	}
	return nil
}

// Sensitive means external delivery, activation, destructive or public configuration effects.
func Sensitive(method, path string, body any) bool {
	if method == "GET" || method == "HEAD" {
		return false
	}
	if method == "DELETE" || strings.HasPrefix(path, "/forms") || strings.HasPrefix(path, "/domains") || strings.HasSuffix(path, "/send") {
		return true
	}
	m, _ := body.(map[string]any)
	if v, ok := m["scheduled_at"]; ok && v != nil && v != "" {
		return true
	}
	return m["active"] == true
}
