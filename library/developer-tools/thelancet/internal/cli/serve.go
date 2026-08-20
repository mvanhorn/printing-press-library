// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

// serveShutdownGrace bounds how long in-flight requests get to finish after
// SIGINT/SIGTERM before the listener is torn down.
const serveShutdownGrace = 5 * time.Second

// newServeCmd exposes the two hand-authored Lancet analytics (rank-authors and
// affiliation-growth) as a local read-only JSON API so the Lancet web portal
// can query them over HTTP instead of shelling out to the CLI. stdlib
// net/http only — no framework, no new dependencies.
func newServeCmd(flags *rootFlags) *cobra.Command {
	var listen string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the Lancet analytics as a local read-only JSON API",
		Long: "Start a local HTTP server exposing the analytics engine as JSON endpoints:\n" +
			"GET /affiliations mirrors 'affiliation-growth' and GET /authors mirrors\n" +
			"'rank-authors', with the same query parameters, defaults, and JSON shapes\n" +
			"as the CLI commands. Reads the local mirror; run 'thelancet-pp-cli refresh'\n" +
			"first. Binds to loopback (127.0.0.1) by default and never mutates data.",
		Example: "  thelancet-pp-cli serve\n" +
			"  thelancet-pp-cli serve --listen 127.0.0.1:9090\n" +
			"  curl 'http://127.0.0.1:8080/affiliations?journal=lancet-neurology&years=5'\n" +
			"  curl 'http://127.0.0.1:8080/authors?institution=Oxford&limit=10'",
		// mcp:hidden — serve blocks until signalled; mirroring it as an MCP
		// tool would hang the tool call. The cobratree walker skips hidden
		// commands (internal/mcp/cobratree/classify.go).
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := ensureLancetStore(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			reqTimeout := flags.timeout
			if reqTimeout <= 0 {
				reqTimeout = 60 * time.Second
			}
			mux := newServeMux(st.DB(), reqTimeout, cmd.ErrOrStderr())

			srv := &http.Server{
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
			}

			ln, err := net.Listen("tcp", listen)
			if err != nil {
				return fmt.Errorf("listening on %s: %w", listen, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "thelancet analytics API listening on http://%s (GET /affiliations, GET /authors; Ctrl+C to stop)\n", ln.Addr().String())

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve(ln) }()

			select {
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			case <-ctx.Done():
				fmt.Fprintln(cmd.ErrOrStderr(), "shutting down...")
				shutCtx, cancel := context.WithTimeout(context.Background(), serveShutdownGrace)
				defer cancel()
				if serr := srv.Shutdown(shutCtx); serr != nil {
					return serr
				}
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:8080", "Address to bind the API server to (loopback by default; do not expose publicly)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}

// serveErrorBody is the structured JSON error shape for every non-2xx
// response from the analytics API.
type serveErrorBody struct {
	Error string `json:"error"`
}

// lancetAPI carries the shared read handle and the per-request timeout into
// the handlers. Handlers stay thin: decode/validate params, call the engine
// function, encode JSON.
type lancetAPI struct {
	db      *sql.DB
	timeout time.Duration
	logW    io.Writer
}

// newServeMux builds the analytics API handler. Split out from the cobra
// command so tests can drive it via httptest without binding a port.
func newServeMux(db *sql.DB, timeout time.Duration, logW io.Writer) http.Handler {
	api := &lancetAPI{db: db, timeout: timeout, logW: logW}
	mux := http.NewServeMux()
	// Method+path patterns (Go 1.22+): non-GET methods on these paths get an
	// automatic 405, unknown paths a 404.
	mux.HandleFunc("GET /affiliations", api.handleAffiliations)
	mux.HandleFunc("GET /authors", api.handleAuthors)
	return mux
}

// handleAffiliations mirrors the affiliation-growth command:
// GET /affiliations?journal=<slug>&years=5&threshold=2&limit=25
func (a *lancetAPI) handleAffiliations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	issn, err := resolveJournalISSN(q.Get("journal"))
	if err != nil {
		writeServeError(w, http.StatusBadRequest, err.Error())
		return
	}
	years, err := serveQueryInt(q.Get("years"), 5)
	if err != nil {
		writeServeError(w, http.StatusBadRequest, "invalid years: must be an integer")
		return
	}
	// Mirror the CLI: affiliation-growth coerces years < 1 back to the
	// default window instead of erroring.
	if years < 1 {
		years = 5
	}
	threshold, err := serveQueryInt(q.Get("threshold"), 2)
	if err != nil {
		writeServeError(w, http.StatusBadRequest, "invalid threshold: must be an integer")
		return
	}
	limit, err := serveQueryInt(q.Get("limit"), 25)
	if err != nil {
		writeServeError(w, http.StatusBadRequest, "invalid limit: must be an integer")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	rows, err := lancet.AffiliationGrowth(ctx, a.db, issn, years, threshold, limit)
	if err != nil {
		a.logf("affiliations query failed: %v", err)
		writeServeError(w, http.StatusInternalServerError, "computing affiliation growth failed")
		return
	}
	if rows == nil {
		rows = []lancet.InstGrowth{}
	}
	writeServeJSON(w, rows)
}

// handleAuthors mirrors the rank-authors command:
// GET /authors?journal=<slug>&institution=<substring>&limit=25
func (a *lancetAPI) handleAuthors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	issn, err := resolveJournalISSN(q.Get("journal"))
	if err != nil {
		writeServeError(w, http.StatusBadRequest, err.Error())
		return
	}
	institution := q.Get("institution")
	limit, err := serveQueryInt(q.Get("limit"), 25)
	if err != nil {
		writeServeError(w, http.StatusBadRequest, "invalid limit: must be an integer")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	rows, err := lancet.RankAuthors(ctx, a.db, issn, institution, limit)
	if err != nil {
		a.logf("authors query failed: %v", err)
		writeServeError(w, http.StatusInternalServerError, "ranking authors failed")
		return
	}
	if rows == nil {
		rows = []lancet.AuthorRank{}
	}
	writeServeJSON(w, rows)
}

// logf writes server-side diagnostics (which may carry engine detail) to the
// command's stderr. Response bodies never carry the raw engine error — no
// file paths, SQL, or keys leak to HTTP clients.
func (a *lancetAPI) logf(format string, args ...any) {
	if a.logW == nil {
		return
	}
	fmt.Fprintf(a.logW, format+"\n", args...)
}

// serveQueryInt parses an optional integer query parameter, falling back to
// def when the parameter is absent or empty — the same semantics as the
// corresponding cobra flag defaults.
func serveQueryInt(raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}

func writeServeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeServeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(serveErrorBody{Error: msg})
}
