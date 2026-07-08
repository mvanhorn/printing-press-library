package trust

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type SFClient interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

type LocalMirror interface{}

type AuditWriter interface {
	Write(ctx context.Context, row AuditRow) error
}

type AuditRow interface {
	PostPath() string
	PostBody() (any, error)
	Mirror(status, remoteError string) error
	FailureCode() string
	LocalFailureCode() string
}

type AsyncWriter struct {
	Client     SFClient
	LogWarn    func(format string, args ...any)
	WarnFormat string
	wg         sync.WaitGroup
}

// Wait blocks until every goroutine spawned by Write has finished.
// Tests and CLI shutdown paths should call this to avoid leaking goroutines
// or leaving SQLite mirror writes in flight.
func (w *AsyncWriter) Wait() { w.wg.Wait() }

type SyncWriter struct {
	Client SFClient
}

func NewAuditWriter(client SFClient, _ LocalMirror, hipaa bool) AuditWriter {
	if hipaa {
		return &SyncWriter{Client: client}
	}
	return &AsyncWriter{Client: client}
}

func (w *AsyncWriter) Write(_ context.Context, row AuditRow) error {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := postAndMirrorAudit(context.Background(), row, w.Client); err != nil {
			format := w.WarnFormat
			if format == "" {
				format = "audit write failed: %v"
			}
			warnAudit(w.LogWarn, format, err)
		}
	}()
	return nil
}

func (w *SyncWriter) Write(ctx context.Context, row AuditRow) error {
	return postAndMirrorAudit(ctx, row, w.Client)
}

func postAndMirrorAudit(ctx context.Context, row AuditRow, client SFClient) error {
	if client == nil {
		message := "audit client required"
		switch row.FailureCode() {
		case "BUNDLE_AUDIT_WRITE_FAILED":
			message = "bundle audit client required"
		case "WRITE_INTENT_AUDIT_FAILED":
			message = "write audit client required"
		}
		err := errors.New(message)
		_ = row.Mirror("failed", err.Error())
		return fmt.Errorf("%s: %w", row.FailureCode(), err)
	}
	body, err := row.PostBody()
	if err != nil {
		_ = row.Mirror("failed", err.Error())
		return fmt.Errorf("%s: %w", row.FailureCode(), err)
	}

	done := make(chan auditPostResult, 1)
	go func() {
		raw, status, postErr := client.Post(row.PostPath(), body)
		done <- auditPostResult{raw: raw, status: status, err: postErr}
	}()

	var result auditPostResult
	select {
	case <-ctx.Done():
		err := ctx.Err()
		_ = row.Mirror("failed", err.Error())
		return fmt.Errorf("%s: %w", row.FailureCode(), err)
	case result = <-done:
	}

	if result.err != nil {
		_ = row.Mirror("failed", result.err.Error())
		return fmt.Errorf("%s: %w", row.FailureCode(), result.err)
	}
	if result.status < 200 || result.status > 299 {
		err := fmt.Errorf("remote returned HTTP %d: %s", result.status, string(result.raw))
		_ = row.Mirror("failed", err.Error())
		return fmt.Errorf("%s: %w", row.FailureCode(), err)
	}
	if err := row.Mirror("ok", ""); err != nil {
		return fmt.Errorf("%s: %w", row.LocalFailureCode(), err)
	}
	return nil
}

type auditPostResult struct {
	raw    json.RawMessage
	status int
	err    error
}

func warnAudit(logWarn func(format string, args ...any), format string, args ...any) {
	if logWarn != nil {
		logWarn(format, args...)
		return
	}
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func DetectHipaaMode() bool {
	return HIPAAModeFromManifest(".printing-press.json")
}

// HIPAAModeFromManifest reads the install-time provenance flag. Accepted keys
// are intentionally permissive so older scaffold manifests can opt in without
// another schema migration.
func HIPAAModeFromManifest(path string) bool {
	if strings.EqualFold(os.Getenv("SF360_HIPAA_MODE"), "true") || os.Getenv("SF360_HIPAA_MODE") == "1" {
		return true
	}
	if path == "" {
		path = ".printing-press.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false
	}
	return boolKey(manifest, "hipaa_mode") || boolKey(manifest, "hipaa") || boolKey(manifest, "sync_audit_writes")
}

func boolKey(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func GenerateTraceID() string {
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		now := time.Now().UTC().UnixNano()
		for i := range entropy {
			entropy[i] = byte(now >> (i % 8 * 8))
		}
	}

	var data [16]byte
	ms := uint64(time.Now().UTC().UnixMilli())
	data[0] = byte(ms >> 40)
	data[1] = byte(ms >> 32)
	data[2] = byte(ms >> 24)
	data[3] = byte(ms >> 16)
	data[4] = byte(ms >> 8)
	data[5] = byte(ms)
	copy(data[6:], entropy[:])
	return encodeULID(data)
}

func encodeULID(data [16]byte) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var out [26]byte
	out[0] = alphabet[(data[0]&0xE0)>>5]
	out[1] = alphabet[data[0]&0x1F]
	out[2] = alphabet[(data[1]&0xF8)>>3]
	out[3] = alphabet[((data[1]&0x07)<<2)|((data[2]&0xC0)>>6)]
	out[4] = alphabet[(data[2]&0x3E)>>1]
	out[5] = alphabet[((data[2]&0x01)<<4)|((data[3]&0xF0)>>4)]
	out[6] = alphabet[((data[3]&0x0F)<<1)|((data[4]&0x80)>>7)]
	out[7] = alphabet[(data[4]&0x7C)>>2]
	out[8] = alphabet[((data[4]&0x03)<<3)|((data[5]&0xE0)>>5)]
	out[9] = alphabet[data[5]&0x1F]
	out[10] = alphabet[(data[6]&0xF8)>>3]
	out[11] = alphabet[((data[6]&0x07)<<2)|((data[7]&0xC0)>>6)]
	out[12] = alphabet[(data[7]&0x3E)>>1]
	out[13] = alphabet[((data[7]&0x01)<<4)|((data[8]&0xF0)>>4)]
	out[14] = alphabet[((data[8]&0x0F)<<1)|((data[9]&0x80)>>7)]
	out[15] = alphabet[(data[9]&0x7C)>>2]
	out[16] = alphabet[((data[9]&0x03)<<3)|((data[10]&0xE0)>>5)]
	out[17] = alphabet[data[10]&0x1F]
	out[18] = alphabet[(data[11]&0xF8)>>3]
	out[19] = alphabet[((data[11]&0x07)<<2)|((data[12]&0xC0)>>6)]
	out[20] = alphabet[(data[12]&0x3E)>>1]
	out[21] = alphabet[((data[12]&0x01)<<4)|((data[13]&0xF0)>>4)]
	out[22] = alphabet[((data[13]&0x0F)<<1)|((data[14]&0x80)>>7)]
	out[23] = alphabet[(data[14]&0x7C)>>2]
	out[24] = alphabet[((data[14]&0x03)<<3)|((data[15]&0xE0)>>5)]
	out[25] = alphabet[data[15]&0x1F]
	return string(out[:])
}
