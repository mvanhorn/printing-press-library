// Package cli wires the amazon-pp-cli cobra command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/store"
)

// Exit codes.
const (
	ExitOK        = 0
	ExitUsage     = 2
	ExitAuth      = 3
	ExitNotFound  = 4
	ExitConflict  = 5
	ExitTransient = 7
	ExitConfirm   = 10 // checkout invoked without --yes
	ExitLoose     = 11 // loose match resolved but --allow-loose not passed
	ExitManual    = 9  // helper hit a CAPTCHA / sign-in gate; manual completion required
)

// CodedError carries an exit code through error returns.
type CodedError struct {
	msg  string
	code int
}

func (e CodedError) Error() string { return e.msg }
func (e CodedError) Code() int     { return e.code }

func coded(code int, format string, args ...any) CodedError {
	return CodedError{msg: fmt.Sprintf(format, args...), code: code}
}

// ExitCodeFor maps an error returned by cobra back to an exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var ce CodedError
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return 1
}

// AppContext is passed to every command's RunE body.
type AppContext struct {
	Ctx     context.Context
	Cfg     *config.Config
	Profile config.Profile
	Store   *store.Store
	Session *auth.Session // nil until RequireSession is called
	JSON    bool
	DryRun  bool
}

func newAppContext(cmd *cobra.Command) (*AppContext, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if profileName, _ := cmd.Flags().GetString("profile"); profileName != "" {
		if err := cfg.SetActiveOverride(profileName); err != nil {
			return nil, coded(ExitNotFound, "%v", err)
		}
	}
	prof, err := cfg.ActiveProfile()
	if err != nil {
		return nil, coded(ExitUsage, "%v", err)
	}
	if err := os.MkdirAll(cfg.ProfileDir(prof.Name), 0o700); err != nil {
		return nil, fmt.Errorf("ensure profile dir: %w", err)
	}
	st, err := store.Open(cfg.DBPath(prof.Name))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	ctx, cancel := context.WithCancel(cmd.Context())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	jsonOut, _ := cmd.Flags().GetBool("json")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	return &AppContext{
		Ctx:     ctx,
		Cfg:     cfg,
		Profile: prof,
		Store:   st,
		JSON:    jsonOut,
		DryRun:  dryRun,
	}, nil
}

// RequireSession lazy-loads cookies.json for the active profile.
func (a *AppContext) RequireSession() error {
	if a.Session != nil {
		return nil
	}
	path := a.Cfg.CookiesPath(a.Profile.Name)
	sess, err := auth.Load(path)
	if err != nil {
		return coded(ExitAuth, "no session loaded for profile %q (run `amazon-pp-cli --profile %s auth login`): %v",
			a.Profile.Name, a.Profile.Name, err)
	}
	a.Session = sess
	return nil
}

// Version is overridden at build time.
var Version = "0.1.0"

// Root returns the configured cobra command tree.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "amazon-pp-cli",
		Short: "Agent-native Amazon CLI for history-first repurchases.",
		Long: strings.TrimSpace(`
amazon-pp-cli replays HTTP against amazon.com using your Chrome session cookies.
It is repurchase-only: 'add' refuses to write to your cart unless the item is
already in your local order history. Multi-account via --profile.
`),
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
	root.PersistentFlags().Bool("dry-run", false, "Preview side effects without performing them")
	root.PersistentFlags().String("profile", "", "Named profile to act on (overrides the active profile for this call)")
	root.AddCommand(
		newDoctorCmd(),
		newAuthCmd(),
		newProfilesCmd(),
		newHistoryCmd(),
		newCartCmd(),
		newAddCmd(),
		newReorderLastCmd(),
		newCheckoutCmd(),
		newDefaultsCmd(),
	)
	return root
}
