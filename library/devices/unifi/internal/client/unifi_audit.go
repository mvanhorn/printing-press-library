// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output.

package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/secretfield"
)

const (
	auditFileName    = "audit.jsonl"
	auditBodyMaxByte = 64 * 1024
	scrubbedValue    = secretfield.ScrubbedValue
)

// AuditRecord is one attempted mutation. Attempts are logged, not only
// successes, because a timed-out write may have committed and the log is where
// an operator looks to find out what was tried.
type AuditRecord struct {
	Time      string          `json:"ts"`
	Method    string          `json:"method"`
	Path      string          `json:"path"`
	Console   string          `json:"console,omitempty"`
	Transport string          `json:"transport,omitempty"`
	Status    int             `json:"status"`
	Outcome   string          `json:"outcome"`
	Params    map[string]any  `json:"params,omitempty"`
	Body      json.RawMessage `json:"body,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// IsSecretField reports whether a field name is one the scrubber hides.
func IsSecretField(name string) bool { return secretfield.IsSecretField(name) }

// ScrubSecrets returns v with secrets replaced, walking nested objects and arrays.
// Exported so the change journal uses the same rules. The rules themselves live in
// secretfield, so the mirror and the journal cannot disagree about what is a secret.
func ScrubSecrets(v any) any {
	out, _ := secretfield.Walk(v, func(any) any { return scrubbedValue })
	return out
}

// ScrubJSON applies ScrubSecrets to a JSON document. An unparseable body is
// dropped rather than logged, because it cannot be proven free of secrets.
func ScrubJSON(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return json.RawMessage(`"[unparseable body omitted]"`)
	}
	out, err := json.Marshal(ScrubSecrets(decoded))
	if err != nil {
		return json.RawMessage(`"[unparseable body omitted]"`)
	}
	return out
}

// AuditLogPath is the JSONL file under the CLI state directory.
func AuditLogPath() (string, error) {
	dir, err := cliutil.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, auditFileName), nil
}

var auditWriteMu sync.Mutex

// appendAuditRecord never fails the caller: the mutation has already happened
// and an unwritable log must not turn its result into an error. It warns instead.
func appendAuditRecord(rec AuditRecord) {
	path, err := AuditLogPath()
	if err == nil {
		err = writeAuditLine(path, rec)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log not written: %v\n", err)
	}
}

func writeAuditLine(path string, rec AuditRecord) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	auditWriteMu.Lock()
	defer auditWriteMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- state dir plus a constant name; never set by an API response or MCP argument
	if err != nil {
		return err
	}
	defer f.Close()
	// OpenFile's mode only applies on creation, and this file names every write made.
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// ReadAuditLog returns records, oldest first. A missing file is an empty log. A
// torn line from a crash is skipped so one bad write cannot hide the rest.
func ReadAuditLog(path string) ([]AuditRecord, error) {
	f, err := os.Open(path) // #nosec G304 -- state dir plus a constant name; never set by an API response or MCP argument
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditRecord{}, nil
		}
		return nil, err
	}
	defer f.Close()
	records := make([]AuditRecord, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*auditBodyMaxByte)
	for scanner.Scan() {
		var rec AuditRecord
		if json.Unmarshal(scanner.Bytes(), &rec) == nil {
			records = append(records, rec)
		}
	}
	return records, scanner.Err()
}

func auditOutcome(status int, err error) string {
	switch {
	case err == nil:
		return "applied"
	case NotSent(err):
		return "refused"
	case status == 0 || status == 408 || status >= 500:
		return "ambiguous"
	default:
		return "refused"
	}
}

// decodedBody normalizes a request body to generic JSON values, so the secret walker sees
// the same shape whatever Go type the caller built it from.
func decodedBody(body any) any {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	var decoded any
	if json.Unmarshal(encoded, &decoded) != nil {
		return nil
	}
	return decoded
}

func (c *Client) recordMutation(method, path string, params map[string]string, body any, status int, err error) {
	rec := AuditRecord{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Method:  strings.ToUpper(method),
		Path:    path,
		Status:  status,
		Outcome: auditOutcome(status, err),
	}
	if len(params) > 0 {
		asAny := make(map[string]any, len(params))
		for k, v := range params {
			asAny[k] = v
		}
		if scrubbed, ok := ScrubSecrets(asAny).(map[string]any); ok {
			rec.Params = scrubbed
		}
	}
	if body != nil {
		if raw, mErr := json.Marshal(body); mErr == nil {
			rec.Body = ScrubJSON(raw)
			if len(rec.Body) > auditBodyMaxByte {
				rec.Body = json.RawMessage(`"[body over 64KB omitted]"`)
			}
		}
	}
	if err != nil {
		rec.Error = c.maskCredentialText(err.Error())
		// An API often quotes a rejected value back, and this file outlives the session.
		// The values are known here, so they can be removed by value.
		for _, secret := range secretfield.StringValues(decodedBody(body)) {
			rec.Error = strings.ReplaceAll(rec.Error, secret, scrubbedValue)
		}
	}
	// Reads already-resolved state and never dials; a failed resolution still
	// logs, because that attempt is exactly what someone may look for later.
	c.transport.mu.Lock()
	c.loadSelectionLocked()
	rec.Console = c.transport.console
	if rec.Console == "" {
		rec.Console = c.transport.autoSelected
	}
	c.transport.mu.Unlock()
	if IsConsoleHostedPath(path) {
		if rec.Console == "" {
			c.transport.mu.Lock()
			if c.transport.gateway != "" {
				// The normalized key, because the raw value may be a URL with a password.
				rec.Console = GatewayMirrorKey(c.transport.gateway)
				if rec.Console == "" {
					rec.Console = "gateway:[invalid]"
				}
			}
			c.transport.mu.Unlock()
		}
		rec.Transport = string(c.DescribeTransport().Mode)
	} else {
		rec.Transport = string(TransportSiteManager)
		rec.Console = ""
	}
	appendAuditRecord(rec)
}
