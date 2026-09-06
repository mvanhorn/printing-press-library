package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

var localListStreamVisitHook func()

type localListHintWriterKey struct{}

func withLocalListHintWriter(ctx context.Context, writer io.Writer) context.Context {
	if writer == nil {
		writer = os.Stderr
	}
	return context.WithValue(ctx, localListHintWriterKey{}, writer)
}

func emitLegacyLocalListHint(ctx context.Context, resourceType string, typedCount, genericCount int64) {
	writer, _ := ctx.Value(localListHintWriterKey{}).(io.Writer)
	if writer == nil {
		writer = os.Stderr
	}
	if typedCount > 0 {
		fmt.Fprintf(writer, "hint: local %s rows come from the pre-2.0 store layout (typed table holds %d of %d rows); run 'seats-aero-pp-cli sync --resources %s …' to finish populating it\n", resourceType, typedCount, genericCount, resourceType)
		return
	}
	fmt.Fprintf(writer, "hint: local %s rows come from the pre-2.0 store layout; run 'seats-aero-pp-cli sync --resources %s …' once to populate the typed table\n", resourceType, resourceType)
}

func localListRows(ctx context.Context, db *store.Store, resourceType string, params map[string]string) ([]json.RawMessage, []string, error) {
	equality := map[string]string{}
	unsupported := make([]string, 0)
	limit, offset, page := -1, 0, 0
	for key, rawValue := range params {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}
		canon := localParamCanon(key)
		switch {
		case localListCursorParams[canon], localListControlParam(canon):
			unsupported = append(unsupported, key)
		case localListLimitParams[canon]:
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				unsupported = append(unsupported, key)
			} else {
				limit = n
			}
		case localListOffsetParams[canon]:
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				unsupported = append(unsupported, key)
			} else {
				offset = n
			}
		case localListPageParams[canon]:
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				unsupported = append(unsupported, key)
			} else {
				page = n
			}
		default:
			equality[key] = value
		}
	}
	if page > 1 && limit < 0 {
		unsupported = append(unsupported, "page")
		page = 0
	}
	if page > 1 {
		offset += (page - 1) * limit
	}

	columns, err := db.TypedTableColumns(resourceType)
	if err != nil {
		return nil, nil, err
	}
	columnSet := make(map[string]bool, len(columns))
	for _, column := range columns {
		columnSet[column] = true
	}
	typedEquality := make(map[string]string, len(equality))
	typedOK := len(columns) > 0
	for key, value := range equality {
		column := localParamCanon(key)
		if !columnSet[column] {
			column = strings.ReplaceAll(column, "-", "_")
		}
		if !columnSet[column] {
			typedOK = false
			break
		}
		typedEquality[column] = value
	}
	if typedOK {
		genericCount, err := db.Count(resourceType)
		if err != nil {
			return nil, nil, err
		}
		if genericCount == 0 {
			typedOK = false
		} else {
			typedCount, err := db.TypedTableRowCount(ctx, resourceType)
			if err != nil {
				return nil, nil, err
			}
			if typedCount == 0 || typedCount < int64(genericCount) {
				emitLegacyLocalListHint(ctx, resourceType, typedCount, int64(genericCount))
				typedOK = false
			}
		}
	}
	if typedOK {
		items, err := db.ListTypedFiltered(ctx, resourceType, typedEquality, limit, offset)
		sort.Strings(unsupported)
		return items, unsupported, err
	}

	items := make([]json.RawMessage, 0)
	matched := 0
	presentEquality := make(map[string]bool, len(equality))
	err = db.StreamList(ctx, resourceType, func(raw json.RawMessage) bool {
		if localListStreamVisitHook != nil {
			localListStreamVisitHook()
		}
		if !validLocalListRecord(raw) {
			return true
		}
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			return true
		}
		keep := true
		for key, want := range equality {
			if localParamCanon(key) == "cabin" {
				field := map[string]string{"economy": "YAvailable", "premium": "WAvailable", "business": "JAvailable", "first": "FAvailable"}[strings.ToLower(want)]
				got, ok := localObjectField(obj, field)
				if ok {
					presentEquality[key] = true
				}
				if !ok || !localFieldEquals(got, "true") {
					keep = false
				}
				continue
			}
			got, ok := localObjectField(obj, key)
			if !ok {
				keep = false
				continue
			}
			presentEquality[key] = true
			if !localFieldEquals(got, want) {
				keep = false
				break
			}
		}
		if !keep {
			return true
		}
		if matched >= offset {
			items = append(items, raw)
		}
		matched++
		return limit < 0 || len(items) < limit
	})
	if err != nil {
		return nil, nil, err
	}
	for key := range equality {
		if !presentEquality[key] {
			unsupported = append(unsupported, key)
		}
	}
	sort.Strings(unsupported)
	return items, unsupported, nil
}

func validLocalListRecord(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null" && trimmed != "[]" && trimmed != "{}"
}

func localItemMatchesEquality(obj map[string]any, equality map[string]string) bool {
	for key, want := range equality {
		got, ok := localObjectField(obj, key)
		if !ok || !localFieldEquals(got, want) {
			return false
		}
	}
	return true
}
