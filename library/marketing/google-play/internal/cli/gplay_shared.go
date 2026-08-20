// Hand-authored shared helpers for the Google Play commands: building the
// gplay client from root flags, registering the common --country/--lang flags,
// and emitting Go-typed results through the generated output pipeline so
// --json/--select/--compact/--csv all work for free.
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/gplay"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

// localeCountry / localeLang back the persistent --country / --lang root flags
// (registered in newRootCmd). Hand-authored locale flags live here rather than
// in the generated rootFlags struct so a regen cannot drop them.
var (
	localeCountry string
	localeLang    string
)

// localeOf reads --country/--lang with sane defaults. It reads the parsed flag
// values via the command (which includes inherited persistent flags at RunE
// time) and falls back to the package vars / "us"/"en" when unset.
func localeOf(cmd *cobra.Command) (country, lang string) {
	country, _ = cmd.Flags().GetString("country")
	lang, _ = cmd.Flags().GetString("lang")
	if country == "" {
		country = localeCountry
	}
	if lang == "" {
		lang = localeLang
	}
	if country == "" {
		country = "us"
	}
	if lang == "" {
		lang = "en"
	}
	return country, lang
}

// newGplayClient builds a Play client honoring root --rate-limit and --timeout
// plus the command's --country/--lang. Under live dogfood it tightens the
// timeout so the matrix's per-command budget is respected.
func newGplayClient(cmd *cobra.Command, flags *rootFlags) *gplay.Client {
	country, lang := localeOf(cmd)
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return gplay.New(gplay.Options{
		Lang:     lang,
		Country:  country,
		RatePerS: flags.rateLimit,
		Timeout:  timeout,
	})
}

// emit writes a Go value through the generated output pipeline so global
// output flags apply. It marshals v to JSON then routes through
// printJSONFiltered, falling back to a plain encode on error.
func emit(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// classifyGplayErr maps a gplay error to the right typed CLI exit code.
func classifyGplayErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*cliutil.RateLimitError); ok {
		return rateLimitErr(err)
	}
	return apiErr(err)
}

// openStoreFor opens the snapshot store at the resolved db path, creating the
// gplay schema. Callers are responsible for Close.
func openStoreFor(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("google-play-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := s.EnsureGplaySchema(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// dbFileExists reports whether the local snapshot db file is present. Local
// commands use this to emit an identifying empty-state view (with a sync hint)
// rather than a bare [] that would lose the queried subject.
func dbFileExists(dbPath string) bool {
	if dbPath == "" {
		dbPath = defaultDBPath("google-play-pp-cli")
	}
	_, err := os.Stat(dbPath)
	return err == nil
}

// hintStderr writes a sync hint to stderr (stdout stays clean for machine output).
func hintStderr(cmd *cobra.Command, dbPath, hint string) {
	if dbPath == "" {
		dbPath = defaultDBPath("google-play-pp-cli")
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "no local data at %s\n%s\n", dbPath, hint)
}

// nowUnix returns the current unix time; isolated so tests can keep snapshots
// deterministic via the store layer.
func nowUnix() int64 { return store.NowUnix() }

// resolveDBFlag returns the --db value if set, else the default path.
func resolveDBFlag(cmd *cobra.Command) string {
	db, _ := cmd.Flags().GetString("db")
	if db == "" {
		return defaultDBPath("google-play-pp-cli")
	}
	return db
}
