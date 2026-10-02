// Traveloka shopper workflows use scoped private sessions and normalized public snapshots.
package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type travelokaShopperContextKey struct{}

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		var sessionFile, market, locale, currency string
		root.PersistentFlags().StringVar(&sessionFile, "session-file", "", "Private Traveloka-only cookie and request-profile session file")
		root.PersistentFlags().StringVar(&f.dataSource, "data-source", "auto", "Command source strategy: auto, local or live; incompatible modes return errors")
		root.PersistentFlags().StringVar(&market, "market", "SG", "Traveloka shopper market country code, such as SG")
		root.PersistentFlags().StringVar(&locale, "locale", "en-SG", "Traveloka shopper language-region locale, such as en-SG")
		root.PersistentFlags().StringVar(&currency, "currency", "SGD", "Requested source price currency, such as SGD or USD")
		before := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if before != nil {
				if err := before(cmd, args); err != nil {
					return err
				}
			}
			if dryRunOK(f) {
				return nil
			}
			shop, err := travelokaShopper(cmd)
			if err != nil {
				return travelokaFail(cmd, f, err)
			}
			cmd.SetContext(context.WithValue(cmd.Context(), travelokaShopperContextKey{}, shop))
			file, _ := cmd.Flags().GetString("session-file")
			if file != "" {
				if err := os.Setenv("TRAVELOKA_SESSION_FILE", file); err != nil {
					return configErr(err)
				}
			} else if os.Getenv("TRAVELOKA_SESSION_FILE") == "" {
				p := travelokaDefaultSessionFile()
				if _, err := os.Stat(p); err == nil {
					if err = os.Setenv("TRAVELOKA_SESSION_FILE", p); err != nil {
						return configErr(err)
					}
				}
			}
			return nil
		}
	})
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.DryRun || travelokaVerifierMock(c.BaseURL) {
			return nil
		}
		if c.Config == nil || c.HTTPClient == nil {
			return authErr(&traveloka.APIError{Code: "AUTH_REQUIRED", Message: "A scoped Traveloka session client is required"})
		}
		file := c.Config.TravelokaSessionFile
		if file == "" {
			file = travelokaDefaultSessionFile()
			if _, err := os.Lstat(file); os.IsNotExist(err) {
				file = ""
			}
		}
		var src *traveloka.Client
		if file != "" {
			var err error
			src, err = traveloka.NewClient(file)
			if err != nil {
				return authErr(err)
			}
		}
		original := c.HTTPClient.Transport
		if src != nil {
			src.SetHTTPTransport(original)
		}
		c.HTTPClient.Jar = nil
		c.NoCache = true
		c.HTTPClient.Transport = &travelokaReplayTransport{source: src, fallback: original}
		return nil
	})
	doctorAuthConfiguredHook = func() (bool, string) {
		p := os.Getenv("TRAVELOKA_SESSION_FILE")
		if p == "" {
			p = travelokaDefaultSessionFile()
		}
		_, e := traveloka.NewClient(p)
		return e == nil, "scoped_private_session"
	}
}
func travelokaDefaultSessionFile() string {
	d, e := cliutil.StateDir()
	if e != nil {
		return ""
	}
	return filepath.Join(d, "traveloka-session.json")
}
func travelokaSessionFile(cmd *cobra.Command) string {
	p, _ := cmd.Flags().GetString("session-file")
	if p != "" {
		return p
	}
	if p = os.Getenv("TRAVELOKA_SESSION_FILE"); p != "" {
		return p
	}
	return travelokaDefaultSessionFile()
}
func travelokaShopper(cmd *cobra.Command) (traveloka.Shopper, error) {
	m, _ := cmd.Flags().GetString("market")
	l, _ := cmd.Flags().GetString("locale")
	c, _ := cmd.Flags().GetString("currency")
	s := traveloka.Shopper{Market: m, Locale: l, Currency: c}
	return s, traveloka.ValidateShopper(s)
}
func newTravelokaClient(cmd *cobra.Command, f *rootFlags) (*traveloka.Client, error) {
	s, e := traveloka.NewClient(travelokaSessionFile(cmd))
	if e != nil {
		return nil, e
	}
	cfg, e := config.Load(f.configPath)
	if e != nil {
		return nil, e
	}
	native := client.New(cfg, f.timeout, f.rateLimit)
	s.SetHTTPTransport(native.HTTPClient.Transport)
	if e = s.SetRateLimit(f.rateLimit); e != nil {
		return nil, e
	}
	return s, nil
}
func travelokaDB(ctx context.Context, path string) (*store.Store, error) {
	if path == "" {
		path = defaultDBPath("traveloka-pp-cli")
	}
	return store.OpenWithContext(ctx, path)
}
func travelokaSave(ctx context.Context, cmd *cobra.Command, f *rootFlags, s *traveloka.Snapshot, dbPath, file string) error {
	db, e := travelokaDB(ctx, dbPath)
	if e != nil {
		return e
	}
	defer db.Close()
	if e = traveloka.SaveSnapshot(ctx, db, s); e != nil {
		return e
	}
	if file != "" {
		if e = traveloka.SaveSnapshotFile(file, s); e != nil {
			return e
		}
	}
	return nil
}
func travelokaLoad(ctx context.Context, dbPath, file, id, kind string) (*traveloka.Snapshot, error) {
	if file != "" {
		return traveloka.ReadSnapshotFile(file)
	}
	if dbPath == "" {
		dbPath = defaultDBPath("traveloka-pp-cli")
	}
	if _, e := os.Stat(dbPath); os.IsNotExist(e) {
		if id != "" {
			return nil, &traveloka.APIError{Code: "NOT_FOUND", Message: "--snapshot-id was not found in the local history"}
		}
		return nil, nil
	}
	db, e := store.OpenReadOnlyContext(ctx, dbPath)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	s, e := traveloka.LoadSnapshot(ctx, db, id, kind)
	if errors.Is(e, sql.ErrNoRows) {
		if id != "" {
			return nil, &traveloka.APIError{Code: "NOT_FOUND", Message: "--snapshot-id was not found in the local history"}
		}
		return nil, nil
	}
	return s, e
}
func travelokaMode(f *rootFlags, want string) error {
	if f.dataSource != "" && f.dataSource != "auto" && f.dataSource != want {
		return &traveloka.APIError{Code: "UNSUPPORTED_OPERATION", Message: "--data-source must be " + want + " or auto for this command"}
	}
	return nil
}
func travelokaFail(cmd *cobra.Command, f *rootFlags, err error) error {
	if err == nil {
		return nil
	}
	code := "UPSTREAM_ERROR"
	status := 0
	retry := false
	message := err.Error()
	wrapped := apiErr(err)
	var ae *traveloka.APIError
	var re *cliutil.RateLimitError
	var pe *os.PathError
	if errors.As(err, &re) {
		code, status, retry = "RATE_LIMITED", 429, true
		wrapped = rateLimitErr(err)
	} else if errors.As(err, &ae) {
		code, status, retry, message = ae.Code, ae.Status, ae.Retryable, ae.Message
		switch ae.Code {
		case "INVALID_INPUT", "UNSUPPORTED_OPERATION":
			wrapped = usageErr(err)
		case "AUTH_REQUIRED", "ACCESS_BLOCKED":
			wrapped = authErr(err)
		case "NOT_FOUND":
			wrapped = notFoundErr(err)
		case "RATE_LIMITED":
			wrapped = rateLimitErr(err)
		}
	} else if errors.As(err, &pe) {
		code = "LOCAL_IO_ERROR"
		wrapped = &cliError{code: 6, err: err}
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "TIMEOUT"
		retry = true
	}
	if !wantsHumanTable(cmd.OutOrStdout(), f) {
		if printErr := f.printJSON(cmd, map[string]any{"error": map[string]any{"code": code, "message": message, "status": status, "retryable": retry}}); printErr != nil {
			return errors.Join(wrapped, printErr)
		}
	}
	return wrapped
}
func travelokaAnnotations(source, fixture string) map[string]string {
	return map[string]string{"pp:data-source": source, "mcp:read-only": "false", "pp:happy-args": fixture}
}
func travelokaBareHelp(cmd *cobra.Command, args []string) bool {
	return len(args) == 0 && cmd.Flags().NFlag() == 0
}
func travelokaBounds(limit, max int) error {
	if limit < 1 || limit > max {
		return &traveloka.APIError{Code: "INVALID_INPUT", Message: fmt.Sprintf("--limit must be between 1 and %d (CLI output bound)", max)}
	}
	return nil
}

// The generated endpoint mirror shares the exact scoped session transport, while
// clean shopper commands use the same source client directly. No credential path
// is sent as Authorization/Cookie; the private file is interpreted locally.
// The verifier may use its explicit loopback mock; a real Traveloka origin
// must keep the scoped replay boundary even while verifier flags are set.
func travelokaVerifierMock(baseURL string) bool {
	if !cliutil.IsVerifyEnv() || !cliutil.IsVerifyLiveHTTPEnv() {
		return false
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type travelokaReplayTransport struct {
	source   *traveloka.Client
	fallback http.RoundTripper
}

func (t *travelokaReplayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != "www.traveloka.com" {
		return nil, &traveloka.APIError{Code: "UNSUPPORTED_OPERATION", Message: "Traveloka session replay is pinned to https://www.traveloka.com"}
	}
	if req.Method == http.MethodGet && (req.URL.Path == "/" || req.URL.Path == "/en-sg") {
		copy := req.Clone(req.Context())
		copy.Header.Del("Authorization")
		copy.Header.Del("Cookie")
		rt := t.fallback
		if rt == nil {
			rt = http.DefaultTransport
		}
		return rt.RoundTrip(copy)
	}
	if req.Method != http.MethodPost {
		return nil, &traveloka.APIError{Code: "UNSUPPORTED_OPERATION", Message: "Only observed read-only Traveloka POST searches are supported"}
	}
	if t.source == nil {
		// A local 401 keeps generated CLI/MCP auth classification intact without
		// allowing a sessionless request to reach the fallback transport.
		return travelokaReplayResponse(req, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "AUTH_REQUIRED", "message": "Import or capture a scoped Traveloka session before searching"}})
	}
	var envelope map[string]any
	if req.Body != nil {
		defer req.Body.Close()
		dec := json.NewDecoder(io.LimitReader(req.Body, 2<<20))
		dec.UseNumber()
		if e := dec.Decode(&envelope); e != nil {
			return nil, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Source search body must be a JSON object"}
		}
	}
	data, _ := envelope["data"].(map[string]any)
	if data == nil {
		return nil, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use the clean resolve/flights/hotels command, or supply source --data JSON"}
	}
	shop, contextual := req.Context().Value(travelokaShopperContextKey{}).(traveloka.Shopper)
	if !contextual {
		shop = traveloka.Shopper{Market: req.Header.Get("tv-country"), Locale: req.Header.Get("tv-language"), Currency: req.Header.Get("tv-currency")}
	}
	if shop.Market == "" {
		shop.Market = "SG"
	}
	if shop.Locale == "" {
		shop.Locale = "en-SG"
	}
	if shop.Currency == "" {
		shop.Currency = "SGD"
	}
	overrides := map[string]any{}
	for _, key := range []string{"fields", "clientInterface"} {
		if value, present := envelope[key]; present {
			overrides[key] = value
		}
	}
	value, e := t.source.PostWithEnvelope(req.Context(), req.URL.Path, data, shop, overrides)
	if e != nil {
		var source *traveloka.APIError
		if errors.As(e, &source) {
			switch source.Code {
			case "AUTH_REQUIRED":
				return travelokaReplayResponse(req, http.StatusUnauthorized, map[string]any{"error": source})
			case "ACCESS_BLOCKED":
				// Source protection can return HTTP202. Expose a bridge HTTP403 so
				// the generated client keeps auth classification and does not retry
				// it as a transport failure; the public error retains source status.
				return travelokaReplayResponse(req, http.StatusForbidden, map[string]any{"error": source})
			}
		}
		return nil, e
	}
	return travelokaReplayResponse(req, http.StatusOK, value)
}
func travelokaReplayResponse(req *http.Request, status int, value map[string]any) (*http.Response, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(b)), Request: req, ContentLength: int64(len(b))}, nil
}
func travelokaValidateSession(ctx context.Context, cmd *cobra.Command, f *rootFlags, path string) error {
	src, e := traveloka.NewClient(path)
	if e != nil {
		return e
	}
	cfg, e := config.Load(f.configPath)
	if e != nil {
		return e
	}
	cfg.TravelokaSessionFile = path
	native := client.New(cfg, f.timeout, f.rateLimit)
	src.SetHTTPTransport(native.HTTPClient.Transport)
	shop, e := travelokaShopper(cmd)
	if e != nil {
		return e
	}
	data, e := src.TemplateData("/api/v2/airport/search-nexus")
	if e != nil {
		return e
	}
	data["query"] = "Singapore"
	reply, e := src.Post(ctx, "/api/v2/airport/search-nexus", data, shop)
	if e != nil {
		return e
	}
	if e := travelokaCheckAirportValidationReply(reply); e != nil {
		return e
	}
	credentialFingerprint := fingerprintCredential(cfg.AuthHeader())
	proof := browserSessionProof{APIName: "traveloka", CookieDomain: ".traveloka.com", ValidationMethod: "POST", ValidationPath: "/api/v2/airport/search-nexus", StatusCode: 200, AuthSource: "scoped_private_session", CredentialFingerprint: credentialFingerprint, VerifiedAt: time.Now().UTC().Format(time.RFC3339)}
	b, e := json.MarshalIndent(proof, "", "  ")
	if e != nil {
		return e
	}
	return writeBrowserSessionProof(cfg, append(b, '\n'))
}

func travelokaCheckAirportValidationReply(reply map[string]any) error {
	data, err := traveloka.ResponseData(reply)
	if err != nil {
		return err
	}
	for _, object := range []map[string]any{reply, data} {
		if success, present := object["success"]; present {
			_, valid := success.(bool)
			if !valid {
				return &traveloka.APIError{Code: "MALFORMED_RESPONSE", Message: "Session validation returned a malformed source success flag", Status: http.StatusOK}
			}
		}
	}
	sections, ok := data["sections"].([]any)
	if !ok {
		return &traveloka.APIError{Code: "MALFORMED_RESPONSE", Message: "Session validation did not return ranked airport sections", Status: http.StatusOK}
	}
	for _, raw := range sections {
		section, ok := raw.(map[string]any)
		if !ok {
			return &traveloka.APIError{Code: "MALFORMED_RESPONSE", Message: "Session validation returned a malformed airport section", Status: http.StatusOK}
		}
		if _, ok := section["results"].([]any); !ok {
			return &traveloka.APIError{Code: "MALFORMED_RESPONSE", Message: "Session validation returned an airport section without results", Status: http.StatusOK}
		}
	}
	return nil
}
