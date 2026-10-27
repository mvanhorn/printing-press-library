// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Package speccontract checks this CLI's FedEx requests and response decoders
// against the OpenAPI documents vendored in testdata.
package speccontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	shipSchema   = "Full_Schema_Ship"
	cancelSchema = "Full_Schema_Cancel_Shipment"
	// verifyShipmentSchema is the structural shipment-validation schema.
	verifyShipmentSchema = "FullSchema-VerifyShipment"
)

type operation struct {
	Method string
	Path   string
	Schema string
}

var (
	loadOnce sync.Once
	loaded   map[string]*openapi3.T
	loadErr  error
)

func documents() (map[string]*openapi3.T, error) {
	loadOnce.Do(func() {
		loaded = map[string]*openapi3.T{}
		for _, name := range []string{"authorization", "ship", "pickup", "rate", "address"} {
			path := filepath.Join("testdata", name+".openapi.json")
			loader := openapi3.NewLoader()
			doc, err := loader.LoadFromFile(path)
			if err != nil {
				loadErr = fmt.Errorf("load %s: %w", name, err)
				return
			}
			loaded[name] = doc
		}
	})
	return loaded, loadErr
}

func schemaNamed(doc *openapi3.T, name string) (*openapi3.Schema, error) {
	ref := doc.Components.Schemas[name]
	if ref == nil || ref.Value == nil {
		return nil, fmt.Errorf("schema %s is not in the published document", name)
	}
	return ref.Value, nil
}

func validateSchema(doc *openapi3.T, name string, body any) error {
	schema, err := schemaNamed(doc, name)
	if err != nil {
		return err
	}
	value, err := jsonRoundTrip(body)
	if err != nil {
		return err
	}
	if err := schema.VisitJSON(value); err == nil {
		return nil
	} else if name == shipSchema || name == verifyShipmentSchema {
		return ignoreTotalWeightDefect(err)
	} else {
		return err
	}
}

// ignoreTotalWeightDefect drops the one Ship-schema contradiction described in
// testdata/README.md. Every other schema error is returned unchanged.
func ignoreTotalWeightDefect(err error) error {
	if err == nil {
		return nil
	}
	var multi openapi3.MultiError
	if asMulti(err, &multi) {
		var kept openapi3.MultiError
		for _, item := range multi {
			if filtered := ignoreTotalWeightDefect(item); filtered != nil {
				kept = append(kept, filtered)
			}
		}
		if len(kept) == 0 {
			return nil
		}
		if len(kept) == 1 {
			return kept[0]
		}
		return kept
	}
	if isTotalWeightRequired(err) {
		return nil
	}
	return err
}

func asMulti(err error, dest *openapi3.MultiError) bool {
	multi, ok := err.(openapi3.MultiError)
	if !ok {
		return false
	}
	*dest = multi
	return true
}

func isTotalWeightRequired(err error) bool {
	schemaErr, ok := err.(*openapi3.SchemaError)
	if !ok || schemaErr.SchemaField != "required" {
		return false
	}
	pointer := schemaErr.JSONPointer()
	if len(pointer) == 0 || pointer[len(pointer)-1] != "totalWeight" {
		return false
	}
	// FedEx marks totalWeight required, then omits it from the domestic and
	// express examples. Drop only that missing-property error.
	return strings.Contains(schemaErr.Reason, "is missing")
}

func operationSpec(doc *openapi3.T, method, path string) (*openapi3.Operation, error) {
	item := doc.Paths.Find(path)
	if item == nil {
		return nil, fmt.Errorf("published spec has no %s", path)
	}
	var op *openapi3.Operation
	switch method {
	case http.MethodPost:
		op = item.Post
	case http.MethodPut:
		op = item.Put
	default:
		return nil, fmt.Errorf("unsupported method %s", method)
	}
	if op == nil {
		return nil, fmt.Errorf("published spec has no %s %s", method, path)
	}
	return op, nil
}

func requiredHeaders(op *openapi3.Operation) []string {
	var names []string
	for _, param := range op.Parameters {
		if param.Value == nil || param.Value.In != openapi3.ParameterInHeader || !param.Value.Required {
			continue
		}
		names = append(names, param.Value.Name)
	}
	return names
}

func schemaExample(doc *openapi3.T, name string) (any, bool) {
	schema, err := schemaNamed(doc, name)
	if err != nil || schema.Example == nil {
		return nil, false
	}
	return schema.Example, true
}

func responseExample(doc *openapi3.T, method, path, status string) (any, bool) {
	op, err := operationSpec(doc, method, path)
	if err != nil || op.Responses == nil {
		return nil, false
	}
	resp := op.Responses.Value(status)
	if resp == nil || resp.Value == nil || resp.Value.Content == nil {
		return nil, false
	}
	media := resp.Value.Content.Get("application/json")
	if media == nil || media.Example == nil {
		return nil, false
	}
	return media.Example, true
}

func generatedSample(doc *openapi3.T, name string) any {
	ref := doc.Components.Schemas[name]
	return fillExample(ref, 0)
}

func fillExample(ref *openapi3.SchemaRef, depth int) any {
	if ref == nil || ref.Value == nil || depth > 6 {
		return nil
	}
	schema := ref.Value
	if schema.Example != nil {
		return schema.Example
	}
	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}
	if len(schema.AllOf) > 0 {
		merged := map[string]any{}
		for _, part := range schema.AllOf {
			filled := fillExample(part, depth+1)
			object, ok := filled.(map[string]any)
			if !ok {
				continue
			}
			for key, value := range object {
				merged[key] = value
			}
		}
		if len(merged) > 0 {
			return merged
		}
	}
	isArray := schema.Type != nil && schema.Type.Is("array")
	if schema.Items != nil && (isArray || len(schema.Properties) == 0) {
		item := fillExample(schema.Items, depth+1)
		if item == nil {
			return nil
		}
		return []any{item}
	}
	if len(schema.Properties) > 0 {
		object := map[string]any{}
		for name, prop := range schema.Properties {
			filled := fillExample(prop, depth+1)
			if filled == nil {
				continue
			}
			object[name] = filled
		}
		if len(object) == 0 {
			return nil
		}
		return object
	}
	return nil
}

func jsonRoundTrip(body any) (any, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}
