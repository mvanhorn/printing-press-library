package cli

import (
	"reflect"
	"strings"
)

func applySelect(v any, spec string) any {
	fields := []string{}
	for _, f := range strings.Split(spec, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		return v
	}
	return selectValue(v, fields)
}

func selectValue(v any, fields []string) any {
	switch x := v.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(x))
		for _, m := range x {
			out = append(out, selectMap(m, fields))
		}
		return out
	case map[string]any:
		return selectMap(x, fields)
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		out := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out = append(out, selectStructOrMap(rv.Index(i).Interface(), fields))
		}
		return out
	}
	return selectStructOrMap(v, fields)
}

func selectStructOrMap(v any, fields []string) any {
	b := map[string]any{}
	if m, ok := v.(map[string]any); ok {
		return selectMap(m, fields)
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return v
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		name := rt.Field(i).Tag.Get("json")
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		if name == "" || name == "-" {
			name = strings.ToLower(rt.Field(i).Name)
		}
		for _, f := range fields {
			if f == name {
				b[name] = rv.Field(i).Interface()
			}
		}
	}
	return b
}

func selectMap(m map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if v, ok := m[f]; ok {
			out[f] = v
		}
	}
	return out
}
