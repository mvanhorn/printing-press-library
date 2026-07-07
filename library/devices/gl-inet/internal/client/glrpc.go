// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// GL.iNet JSON-RPC transport. The GL firmware 4.x local API is JSON-RPC 2.0
// over POST /rpc with a challenge/crypt/hash session handshake — neither a
// stock REST shape nor a stock auth mode — so it is hand-built here as an
// extension of the generated client.Client rather than emitted by the
// generator. See manuscripts device-probe for the confirmed algorithm.

package client

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/md5_crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
)

// rpcRequest is the JSON-RPC 2.0 envelope sent to /rpc.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// rpcResponse is the JSON-RPC 2.0 reply.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("GL RPC error %d: %s", e.Code, e.Message) }

// challengeResult is the response of the no-auth `challenge` method.
type challengeResult struct {
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Alg        int    `json:"alg"`
	HashMethod string `json:"hash-method"`
}

// GLAuthError signals an authentication failure (bad/expired session or wrong
// password) so commands can map it to a typed exit code.
type GLAuthError struct{ msg string }

func (e *GLAuthError) Error() string { return e.msg }

// GLUsername returns the RPC username (always "root" on stock GL firmware; the
// web-UI "admin" account IS root). Overridable for non-standard builds.
func GLUsername() string {
	if u := strings.TrimSpace(os.Getenv("GL_INET_USER")); u != "" {
		return u
	}
	return "root"
}

// passwordOverride holds a --password flag value for the current process. It is
// never persisted to any artifact; only the derived session id is cached.
var passwordOverride string

// SetPasswordOverride records a router admin password supplied via the
// --password flag, taking precedence over GL_INET_PASSWORD for this process.
func SetPasswordOverride(p string) { passwordOverride = p }

// GLPassword resolves the router admin password from the --password override or
// the environment. Exported so the SSH layer can reuse the same resolution.
func GLPassword() (string, error) {
	if p := strings.TrimSpace(passwordOverride); p != "" {
		return p, nil
	}
	if p := os.Getenv("GL_INET_PASSWORD"); p != "" {
		return p, nil
	}
	return "", &GLAuthError{msg: "no router password set: export GL_INET_PASSWORD (the router admin/web-UI password) or pass --password"}
}

func (c *Client) glPassword() (string, error) { return GLPassword() }

func (c *Client) rpcURL() string {
	return strings.TrimRight(c.BaseURL, "/") + "/rpc"
}

// rpcPost sends a single JSON-RPC request and returns the parsed response.
// It bypasses the generated REST do() machinery (no Authorization header, no
// query params, no response cache) because GL auth rides in params, not headers.
func (c *Client) rpcPost(ctx context.Context, method string, params any) (*rpcResponse, error) {
	reqBody, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 0, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("marshaling rpc request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL(), strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("creating rpc request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	c.limiter.Wait()
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("rpc %s: %w", method, err)
	}
	defer resp.Body.Close()
	var rpcResp rpcResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decoding rpc response for %s: %w", method, err)
	}
	return &rpcResp, nil
}

// glLogin performs the full challenge/crypt/hash handshake and returns a sid.
// The algorithm is read from the live challenge (alg selects the crypt variant,
// hash-method selects the final digest) so it adapts across firmware versions.
func (c *Client) glLogin(ctx context.Context) (string, error) {
	user := GLUsername()
	password, err := c.glPassword()
	if err != nil {
		return "", err
	}
	chResp, err := c.rpcPost(ctx, "challenge", map[string]string{"username": user})
	if err != nil {
		return "", err
	}
	if chResp.Error != nil {
		return "", &GLAuthError{msg: "challenge rejected: " + chResp.Error.Message}
	}
	var ch challengeResult
	if err := json.Unmarshal(chResp.Result, &ch); err != nil {
		return "", fmt.Errorf("parsing challenge: %w", err)
	}
	cipher, err := cryptPassword(password, ch.Alg, ch.Salt)
	if err != nil {
		return "", err
	}
	loginHash := hashLogin(ch.HashMethod, user, cipher, ch.Nonce)
	loginResp, err := c.rpcPost(ctx, "login", map[string]string{"username": user, "hash": loginHash})
	if err != nil {
		return "", err
	}
	if loginResp.Error != nil {
		return "", &GLAuthError{msg: "login failed (check the router admin password): " + loginResp.Error.Message}
	}
	var lr struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(loginResp.Result, &lr); err != nil {
		return "", fmt.Errorf("parsing login result: %w", err)
	}
	if lr.SID == "" {
		return "", &GLAuthError{msg: "login returned no session id (check the router admin password)"}
	}
	return lr.SID, nil
}

// cryptPassword produces the crypt(3) ciphertext ($1$/$5$/$6$ per alg) the GL
// firmware expects as the middle term of the login hash.
func cryptPassword(password string, alg int, salt string) (string, error) {
	var crypter crypt.Crypter
	var magic string
	switch alg {
	case 1:
		crypter = crypt.MD5.New()
		magic = "$1$"
	case 5:
		crypter = crypt.SHA256.New()
		magic = "$5$"
	case 6:
		crypter = crypt.SHA512.New()
		magic = "$6$"
	default:
		return "", fmt.Errorf("unsupported crypt alg %d from router challenge", alg)
	}
	cipher, err := crypter.Generate([]byte(password), []byte(magic+salt))
	if err != nil {
		return "", fmt.Errorf("computing crypt hash: %w", err)
	}
	return cipher, nil
}

// hashLogin computes <hash-method>_hex(user:cipher:nonce). hash-method comes
// from the challenge (sha256 on 4.8.x, md5 on older firmware).
func hashLogin(method, user, cipher, nonce string) string {
	payload := user + ":" + cipher + ":" + nonce
	var h hash.Hash
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	case "md5", "":
		h = md5.New()
	default:
		h = md5.New()
	}
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// --- session caching -------------------------------------------------------

type glSession struct {
	SID       string    `json:"sid"`
	BaseURL   string    `json:"base_url"`
	User      string    `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) sessionFile() string {
	dir, err := cliutil.CacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	// Key the session file by host so multiple routers don't collide.
	host := strings.NewReplacer("http://", "", "https://", "", "/", "_", ":", "_").Replace(c.BaseURL)
	return filepath.Join(dir, "gl-session-"+host+".json")
}

func (c *Client) loadSession() string {
	data, err := os.ReadFile(c.sessionFile())
	if err != nil {
		return ""
	}
	var s glSession
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	if s.BaseURL != c.BaseURL || s.User != GLUsername() {
		return ""
	}
	// GL idle timeout is undocumented; treat sessions older than 4 minutes as
	// stale and re-login proactively. A still-valid older sid is also handled
	// by the re-login-on-auth-error retry in Call.
	if time.Since(s.CreatedAt) > 4*time.Minute {
		return ""
	}
	return s.SID
}

func (c *Client) saveSession(sid string) {
	s := glSession{SID: sid, BaseURL: c.BaseURL, User: GLUsername(), CreatedAt: time.Now()}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = cliutil.AtomicWritePrivateFile(c.sessionFile(), data, 0o600, 0o700)
}

func (c *Client) clearSession() { _ = os.Remove(c.sessionFile()) }

// isAuthRPCError reports whether a JSON-RPC error indicates an invalid or
// expired session (so Call should re-login and retry).
func isAuthRPCError(e *rpcError) bool {
	if e == nil {
		return false
	}
	if e.Code == -32000 {
		return true
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "access") || strings.Contains(msg, "denied") ||
		strings.Contains(msg, "unauthor") || strings.Contains(msg, "login")
}

// Call invokes a GL module.function over the JSON-RPC `call` envelope with the
// session id in params[0], auto-authenticating and retrying once on an expired
// session. args may be nil (omitted from params) or a map/struct of arguments.
//
// pp:client-call
func (c *Client) Call(ctx context.Context, module, function string, args any) (json.RawMessage, error) {
	// Verify-mode safety: under PRINTING_PRESS_VERIFY without the live-HTTP
	// opt-in, return a benign synthetic result so the verifier never dials the
	// real router. Live dogfood and workflow-verify strip the var.
	if cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv() {
		return verifyShortCircuitEnvelope("CALL", module+"."+function), nil
	}
	if c.DryRun {
		c.dryRunCall(module, function, args)
		return json.RawMessage(`{"dry_run": true}`), nil
	}

	sid := c.loadSession()
	for attempt := 0; attempt < 2; attempt++ {
		if sid == "" {
			newSID, err := c.glLogin(ctx)
			if err != nil {
				return nil, err
			}
			sid = newSID
			c.saveSession(sid)
		}
		params := buildCallParams(sid, module, function, args)
		resp, err := c.rpcPost(ctx, "call", params)
		if err != nil {
			return nil, err
		}
		if resp.Error != nil {
			if isAuthRPCError(resp.Error) && attempt == 0 {
				c.clearSession()
				sid = ""
				continue
			}
			return nil, fmt.Errorf("%s.%s: %w", module, function, resp.Error)
		}
		return resp.Result, nil
	}
	return nil, &GLAuthError{msg: "authentication failed after retry"}
}

// buildCallParams assembles [sid, module, function, args?]. The args element is
// omitted when nil so argument-less methods match the firmware's expectation.
func buildCallParams(sid, module, function string, args any) []any {
	if args == nil {
		return []any{sid, module, function}
	}
	return []any{sid, module, function, args}
}

func (c *Client) dryRunCall(module, function string, args any) {
	fmt.Fprintf(os.Stderr, "POST %s/rpc\n  call %s.%s", c.BaseURL, module, function)
	if args != nil {
		if b, err := json.Marshal(args); err == nil {
			fmt.Fprintf(os.Stderr, " %s", string(b))
		}
	}
	fmt.Fprintf(os.Stderr, "\n(dry run - no request sent)\n")
}
