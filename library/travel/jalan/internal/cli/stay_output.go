package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
)

// Evidence URLs often repeat a long exact dated source URL. Share them once
// while leaving canonical property/room URLs and short evidence text intact.
func stayCompactEvidence(response jalan.Response) (jalan.Response, error) {
	sources := map[string]string{}
	references := map[string]string{}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if entries, ok := node["evidence"].([]any); ok {
				for _, entry := range entries {
					object, ok := entry.(map[string]any)
					if !ok {
						continue
					}
					url, ok := object["url"].(string)
					if !ok || url == "" {
						continue
					}
					ref := references[url]
					if ref == "" {
						ref = fmt.Sprintf("s%d", len(sources)+1)
						references[url] = ref
						sources[ref] = url
					}
					delete(object, "url")
					object["source_ref"] = ref
				}
			}
			// Visit in stable key order: refs stay deterministic across runs.
			keys := make([]string, 0, len(node))
			for key := range node {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				visit(node[key])
			}
		case []any:
			for _, item := range node {
				visit(item)
			}
		}
	}
	results := make([]any, 0, len(response.Results))
	for _, item := range response.Results {
		raw, err := json.Marshal(item)
		if err != nil {
			return response, err
		}
		var decoded any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err != nil {
			return response, err
		}
		visit(decoded)
		results = append(results, decoded)
	}
	response.Results = results
	if len(sources) != 0 {
		meta := make(map[string]any, len(response.Meta)+1)
		for key, value := range response.Meta {
			meta[key] = value
		}
		meta["sources"] = sources
		response.Meta = meta
	}
	return response, nil
}
