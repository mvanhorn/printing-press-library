package log

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

var (
	authHeaderRE   = regexp.MustCompile(`Authorization:\s*Bearer\s+\S+`)
	accessTokenRE  = regexp.MustCompile(`"access_token"\s*:\s*"[^"]+"`)
	refreshTokenRE = regexp.MustCompile(`"refresh_token"\s*:\s*"[^"]+"`)
	assertionRE    = regexp.MustCompile(`"assertion"\s*:\s*"[^"]+"`)

	complianceMu     sync.RWMutex
	complianceFields = map[string]struct{}{}
)

func RegisterComplianceFields(fields []string) {
	complianceMu.Lock()
	defer complianceMu.Unlock()
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			complianceFields[field] = struct{}{}
		}
	}
}

func NewHandler(wrapped slog.Handler) slog.Handler {
	return &handler{wrapped: wrapped}
}

type handler struct {
	wrapped slog.Handler
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.wrapped.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	record.Message = Redact(record.Message)
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.wrapped.Handle(ctx, clean)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, redactAttr(attr))
	}
	return &handler{wrapped: h.wrapped.WithAttrs(clean)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{wrapped: h.wrapped.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if isSensitiveField(attr.Key) || isComplianceField(attr.Key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		clean := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			clean = append(clean, redactAttr(child))
		}
		return slog.Group(attr.Key, attrsToAny(clean)...)
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, Redact(attr.Value.String()))
	}
	return attr
}

func isSensitiveField(field string) bool {
	switch strings.ToLower(field) {
	case "authorization", "access_token", "refresh_token", "assertion":
		return true
	default:
		return false
	}
}

func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, attr)
	}
	return out
}

func Redact(input string) string {
	out := authHeaderRE.ReplaceAllString(input, "Authorization: Bearer [REDACTED]")
	out = accessTokenRE.ReplaceAllString(out, `"access_token":"[REDACTED]"`)
	out = refreshTokenRE.ReplaceAllString(out, `"refresh_token":"[REDACTED]"`)
	out = assertionRE.ReplaceAllString(out, `"assertion":"[REDACTED]"`)
	for _, field := range registeredFields() {
		out = redactJSONField(out, field)
	}
	return out
}

func registeredFields() []string {
	complianceMu.RLock()
	defer complianceMu.RUnlock()
	fields := make([]string, 0, len(complianceFields))
	for field := range complianceFields {
		fields = append(fields, field)
	}
	return fields
}

func isComplianceField(field string) bool {
	complianceMu.RLock()
	defer complianceMu.RUnlock()
	_, ok := complianceFields[field]
	return ok
}

func redactJSONField(input, field string) string {
	pattern := fmt.Sprintf(`"%s"\s*:\s*"[^"]+"`, regexp.QuoteMeta(field))
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(input, fmt.Sprintf(`"%s":"[REDACTED]"`, field))
}
