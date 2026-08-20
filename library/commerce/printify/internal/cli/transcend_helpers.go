package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/printify/internal/store"
)

type ppJSONObj map[string]any

func ppLoadJSONFile(path string) (json.RawMessage, error) {
	if path == "" {
		return nil, fmt.Errorf("missing JSON file path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s is not valid JSON", path)
	}
	return json.RawMessage(bytes.TrimSpace(data)), nil
}

func ppDecodeObject(raw json.RawMessage) (ppJSONObj, error) {
	var obj ppJSONObj
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func ppDecodeObjects(raw json.RawMessage) ([]ppJSONObj, error) {
	var arr []ppJSONObj
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	for _, key := range []string{"data", "items", "results", "variants", "shipping", "profiles", "uploads", "orders", "products"} {
		if nested, ok := wrapped[key]; ok {
			if err := json.Unmarshal(nested, &arr); err == nil {
				return arr, nil
			}
		}
	}
	var obj ppJSONObj
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return []ppJSONObj{obj}, nil
}

func ppOpenReadStore(dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("printify-pp-cli")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("open local store %s: %w", dbPath, err)
	}
	return store.OpenReadOnly(dbPath)
}

func ppLoadStoreObject(dbPath string, resourceTypes []string, id string) (json.RawMessage, string, error) {
	localStore, err := ppOpenReadStore(dbPath)
	if err != nil {
		return nil, "", err
	}
	defer localStore.Close()
	for _, resourceType := range resourceTypes {
		raw, err := localStore.Get(resourceType, id)
		if err == nil {
			return raw, resourceType, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, resourceType, err
		}
	}
	return nil, "", fmt.Errorf("id %q not found in local resources %s; run sync/import first or pass a JSON file", id, strings.Join(resourceTypes, ", "))
}

func ppLoadStoreObjects(dbPath string, resourceTypes []string, limit int) ([]ppJSONObj, error) {
	localStore, err := ppOpenReadStore(dbPath)
	if err != nil {
		return nil, err
	}
	defer localStore.Close()
	var all []ppJSONObj
	var listErrors []error
	for _, resourceType := range resourceTypes {
		rawItems, err := localStore.List(resourceType, limit)
		if err != nil {
			listErrors = append(listErrors, fmt.Errorf("%s: %w", resourceType, err))
			continue
		}
		for _, raw := range rawItems {
			obj, err := ppDecodeObject(raw)
			if err == nil {
				all = append(all, obj)
			}
		}
	}
	if len(listErrors) == len(resourceTypes) {
		return nil, fmt.Errorf("load local resources: %w", errors.Join(listErrors...))
	}
	return all, nil
}

func ppLoadProduct(productFile, dbPath, productID string) (ppJSONObj, error) {
	var raw json.RawMessage
	var err error
	if productFile != "" {
		raw, err = ppLoadJSONFile(productFile)
	} else {
		if productID == "" {
			return nil, fmt.Errorf("set --product-id or --product-file")
		}
		raw, _, err = ppLoadStoreObject(dbPath, []string{"products", "products-json", "products_json", "shops-products-json"}, productID)
	}
	if err != nil {
		return nil, err
	}
	return ppDecodeObject(raw)
}

func ppLoadObjectsFromFileOrStore(filePath, dbPath string, resources []string, limit int) ([]ppJSONObj, error) {
	if filePath != "" {
		raw, err := ppLoadJSONFile(filePath)
		if err != nil {
			return nil, err
		}
		return ppDecodeObjects(raw)
	}
	return ppLoadStoreObjects(dbPath, resources, limit)
}

func ppString(obj ppJSONObj, names ...string) string {
	for _, name := range names {
		if value, ok := ppLookup(obj, name).(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func ppFloat(obj ppJSONObj, names ...string) float64 {
	for _, name := range names {
		switch value := ppLookup(obj, name).(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case json.Number:
			f, _ := value.Float64()
			return f
		case string:
			f, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
			return f
		}
	}
	return 0
}

func ppBool(obj ppJSONObj, names ...string) bool {
	for _, name := range names {
		if value, ok := ppLookup(obj, name).(bool); ok {
			return value
		}
	}
	return false
}

func ppLookup(obj ppJSONObj, name string) any {
	if obj == nil {
		return nil
	}
	if value, ok := obj[name]; ok {
		return value
	}
	squashed := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(name))
	for key, value := range obj {
		if strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key)) == squashed {
			return value
		}
	}
	return nil
}

func ppArray(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items
	default:
		return nil
	}
}

func ppObject(value any) ppJSONObj {
	if obj, ok := value.(map[string]any); ok {
		return obj
	}
	if obj, ok := value.(ppJSONObj); ok {
		return obj
	}
	return nil
}

func ppID(obj ppJSONObj) string {
	return ppString(obj, "id", "product_id", "variant_id", "sku")
}

func ppCentsToDollars(value float64) float64 {
	return value / 100
}

func ppRound2(value float64) float64 {
	return math.Round(value*100) / 100
}

func ppSortedKeys(obj ppJSONObj) []string {
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ppWriteJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func ppSafeOutputName(row map[string]string, index int) string {
	for _, key := range []string{"slug", "handle", "sku", "title", "name", "id"} {
		if value := strings.TrimSpace(row[key]); value != "" {
			if slug := ppSlug(value); slug != "" {
				return slug
			}
		}
	}
	return fmt.Sprintf("manifest-%03d", index+1)
}

func ppSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func ppEnsureOutDir(path string) error {
	if path == "" {
		return fmt.Errorf("set --out")
	}
	return os.MkdirAll(filepath.Clean(path), 0o755)
}
