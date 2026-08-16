// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) Chrome-session login: reads WeWork's logged-in
// auth0 session from a Chrome-family browser's on-disk Local Storage and
// persists it. Chrome's LevelDB is copied to a private temporary snapshot so a
// running browser need not be stopped and no debug/browser window is required.
// Reading real LevelDB records keeps origin, access token, rotating refresh
// token, account UUID, and member type bound together.

package cli

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/config"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const weworkOrigin = "https://members.wework.com"
const weworkLocalStoragePrefix = "_" + weworkOrigin + "\x00\x01"
const chromeSnapshotAttempts = 4

type chromeLevelDBSnapshotter func(string) (string, func(), error)

type chromeDiskSession struct {
	Token   string
	Refresh string
	UUID    string
	Member  string
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var useChrome bool
	var useCDP bool
	var cdpPort int
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Show headless auth setup or explicitly capture a browser session",
		Long: "The recommended setup is a complete renewable session imported over stdin.\n" +
			"Run 'wework-pp-cli auth session-import --help' locally, or 'wework-pp-cli auth\n" +
			"handoff --ssh-target user@booking-host' when the CLI runs remotely. After\n" +
			"that one-time bootstrap, normal commands use direct HTTP and refresh the\n" +
			"stored session without a browser.\n\n" +
			"Explicit browser bootstrap fallbacks:\n\n" +
			"  --cdp   Connects to a Chrome running with\n" +
			"          --remote-debugging-port and reads the LIVE, in-memory session — access\n" +
			"          token, refresh token, uuid, and member type — from the members.wework.com\n" +
			"          tab. Because it captures the refresh token, the CLI then refreshes its own\n" +
			"          access token via auth0 and can run headless afterward. Use a dedicated\n" +
			"          agent Chrome (not your personal browser): auth0 rotates refresh tokens, so\n" +
			"          whichever session refreshes invalidates the other's.\n\n" +
			"  --chrome  Reads a complete session from a private snapshot of Chrome's on-disk\n" +
			"          Local Storage without opening or debugging a browser. Chrome flushes lazily,\n" +
			"          so the access token may be stale; its matching refresh token renews it.\n",
		Example: strings.Trim(`
  wework-pp-cli auth login --cdp --cdp-port 9222
  wework-pp-cli auth login --chrome`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !useCDP && !useChrome {
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, "Recommended browser-free runtime bootstrap:")
				fmt.Fprintln(w, "  wework-pp-cli auth session-import --help")
				fmt.Fprintln(w, "  pbpaste | wework-pp-cli auth session-import --stdin")
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "For a remote booking host:")
				fmt.Fprintln(w, "  wework-pp-cli auth handoff --ssh-target user@booking-host")
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Browser capture is an explicit one-time fallback only: add --cdp or --chrome.")
				return nil
			}
			if useCDP {
				return runCDPLogin(cmd, flags, cdpPort)
			}
			sess, scanned, err := readChromeSessionFromDisk()
			if err != nil {
				e := cmd.ErrOrStderr()
				fmt.Fprintf(e, "Could not read a WeWork session from Chrome: %v\n\n", err)
				fmt.Fprintln(e, "Make sure you're logged in to members.wework.com in Chrome, then retry.")
				fmt.Fprintln(e, "Or import manually: see 'wework-pp-cli auth session-import --help'.")
				return authErr(err)
			}
			cfg, cerr := config.Load(flags.configPath)
			if cerr != nil {
				return configErr(cerr)
			}
			if serr := cfg.SaveWeworkAuth(sess.Token, sess.Refresh, sess.UUID, sess.Member); serr != nil {
				return configErr(fmt.Errorf("saving credentials: %w", serr))
			}
			hasT, hasU, hasM, exp := cfg.ComposedAuthStatus()
			expired := !exp.IsZero() && time.Now().After(exp)
			if flags.asJSON {
				out := map[string]any{
					"saved": true, "source": "chrome", "profiles_scanned": scanned,
					"token": hasT, "uuid": hasU, "member_type": hasM,
					"refresh_token": sess.Refresh != "", "config_path": cfg.Path,
				}
				if !exp.IsZero() {
					out["token_expires"] = exp.UTC().Format("2006-01-02T15:04:05Z07:00")
					out["token_expired"] = expired
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Imported WeWork token from Chrome (scanned %d profile store(s)).\n", scanned)
			fmt.Fprintf(w, "  token: %s   uuid: %s   member-type: %s   refresh-token: %s\n",
				presence(hasT), presence(hasU), presence(hasM), presence(sess.Refresh != ""))
			if !exp.IsZero() {
				fmt.Fprintf(w, "  token expires: %s (%s)\n", exp.Local().Format("2006-01-02 15:04 MST"), humanUntil(exp))
			}
			if expired {
				fmt.Fprintln(w, "\nwarning: the token Chrome has on disk is EXPIRED. Chrome keeps the current")
				fmt.Fprintln(w, "token in memory and flushes to disk lazily, so the live token may not be saved yet.")
				fmt.Fprintln(w, "For a guaranteed-fresh token, use the DevTools snippet with 'auth session-import' instead")
				fmt.Fprintln(w, "(see 'wework-pp-cli auth session-import --help').")
			}
			if !hasU || !hasM {
				fmt.Fprintln(w, "\nThe Chrome snapshot did not contain the account uuid + member type:")
				fmt.Fprintln(w, "  wework-pp-cli auth session-import --uuid <CurrentAccountUUID> --member-type <WWMemberType>")
				fmt.Fprintln(w, "(grab them from members.wework.com DevTools: localStorage.CurrentAccountUUID / .WWMemberType)")
			} else {
				fmt.Fprintln(w, "\nReady for API calls.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Explicitly read the token from Chrome's on-disk Local Storage (may be stale)")
	cmd.Flags().BoolVar(&useCDP, "cdp", false, "Explicitly read the live session via CDP from a Chrome started with --remote-debugging-port")
	cmd.Flags().IntVar(&cdpPort, "cdp-port", 9222, "Chrome remote debugging port for --cdp")
	return cmd
}

// runCDPLogin reads the live session from a debug Chrome and persists token +
// refresh token + uuid + member type, enabling headless auto-refresh afterward.
func runCDPLogin(cmd *cobra.Command, flags *rootFlags, port int) error {
	sess, err := readChromeSessionCDP(port)
	if err != nil {
		e := cmd.ErrOrStderr()
		fmt.Fprintf(e, "CDP login failed: %v\n\n", err)
		fmt.Fprintf(e, "Start the agent Chrome with a debug port and log in to members.wework.com, e.g.:\n")
		fmt.Fprintf(e, "  \"Google Chrome\" --remote-debugging-port=%d --user-data-dir=/tmp/wework-agent-chrome\n", port)
		fmt.Fprintln(e, "Or seed manually with 'wework-pp-cli auth session-import' (see --help).")
		return authErr(err)
	}
	cfg, cerr := config.Load(flags.configPath)
	if cerr != nil {
		return configErr(cerr)
	}
	if serr := cfg.SaveWeworkAuth(sess.Token, sess.Refresh, sess.UUID, sess.Member); serr != nil {
		return configErr(fmt.Errorf("saving credentials: %w", serr))
	}
	hasT, hasU, hasM, exp := cfg.ComposedAuthStatus()
	if flags.asJSON {
		out := map[string]any{
			"saved": true, "source": "chrome-cdp", "port": port,
			"token": hasT, "uuid": hasU, "member_type": hasM,
			"refresh_token": sess.Refresh != "", "config_path": cfg.Path,
		}
		if !exp.IsZero() {
			out["token_expires"] = exp.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Imported live WeWork session from Chrome (CDP, port %d).\n", port)
	fmt.Fprintf(w, "  token: %s   uuid: %s   member-type: %s   refresh-token: %s\n",
		presence(hasT), presence(hasU), presence(hasM), presence(sess.Refresh != ""))
	if !exp.IsZero() {
		fmt.Fprintf(w, "  token expires: %s (%s)\n", exp.Local().Format("2006-01-02 15:04 MST"), humanUntil(exp))
	}
	if sess.Refresh != "" {
		fmt.Fprintln(w, "\nRefresh token stored — the CLI will refresh its own access token from here (headless).")
	}
	return nil
}

// readChromeSessionFromDisk reads Chrome-family Local Storage databases for a
// complete WeWork session. It uses actual LevelDB records rather than scanning
// shared .ldb bytes, which could otherwise cross origin/record boundaries.
func readChromeSessionFromDisk() (session chromeDiskSession, scanned int, err error) {
	return readChromeSessionFromDirs(chromeLevelDBDirs())
}

func readChromeSessionFromDirs(dirs []string) (session chromeDiskSession, scanned int, err error) {
	if len(dirs) == 0 {
		return chromeDiskSession{}, 0, errors.New("no Chrome-family Local Storage directory found")
	}
	var sessions []chromeDiskSession
	var lastReadErr error
	for _, dir := range dirs {
		dirSessions, uuid, member, originSeen, readErr := readChromeProfileSession(dir)
		if readErr != nil {
			lastReadErr = readErr
			continue
		}
		if !originSeen {
			continue
		}
		scanned++
		for i := range dirSessions {
			dirSessions[i].UUID = uuid
			dirSessions[i].Member = member
		}
		sessions = append(sessions, dirSessions...)
	}
	session = pickFreshestChromeSession(sessions)
	if session.Token == "" {
		if lastReadErr != nil {
			return chromeDiskSession{}, scanned, fmt.Errorf("could not read a coherent Chrome Local Storage snapshot: %w", lastReadErr)
		}
		return chromeDiskSession{}, scanned, errors.New("no WeWork session token found in Chrome (found the site in " +
			itoaScan(scanned) + " store(s) but no token) — are you logged in to members.wework.com?")
	}
	return session, scanned, nil
}

func readChromeProfileSession(dir string) (sessions []chromeDiskSession, uuid, member string, originSeen bool, err error) {
	return readChromeProfileSessionWithSnapshotter(dir, snapshotChromeLevelDB)
}

func readChromeProfileSessionWithSnapshotter(dir string, snapshotter chromeLevelDBSnapshotter) (sessions []chromeDiskSession, uuid, member string, originSeen bool, err error) {
	var lastErr error
	for attempt := 0; attempt < chromeSnapshotAttempts; attempt++ {
		sessions, uuid, member, originSeen, err = readChromeProfileSessionOnce(dir, snapshotter)
		if err == nil {
			return sessions, uuid, member, originSeen, nil
		}
		lastErr = err
	}
	return nil, "", "", false, fmt.Errorf("snapshot remained inconsistent after %d attempts: %w", chromeSnapshotAttempts, lastErr)
}

func readChromeProfileSessionOnce(dir string, snapshotter chromeLevelDBSnapshotter) (sessions []chromeDiskSession, uuid, member string, originSeen bool, err error) {
	snapshot, cleanup, err := snapshotter(dir)
	if err != nil {
		return nil, "", "", false, err
	}
	defer cleanup()
	db, err := leveldb.OpenFile(snapshot, &opt.Options{ReadOnly: true})
	if err != nil {
		return nil, "", "", false, fmt.Errorf("opening Chrome Local Storage snapshot: %w", err)
	}
	defer db.Close()

	iterator := db.NewIterator(util.BytesPrefix([]byte(weworkLocalStoragePrefix)), nil)
	defer iterator.Release()
	for iterator.Next() {
		originSeen = true
		name := strings.TrimPrefix(string(iterator.Key()), weworkLocalStoragePrefix)
		value, decodeErr := decodeChromeLocalStorageValue(iterator.Value())
		if decodeErr != nil {
			continue
		}
		switch name {
		case "CurrentAccountUUID":
			uuid = chromeLocalStorageScalar(value)
		case "WWMemberType":
			member = chromeLocalStorageScalar(value)
		default:
			if strings.HasPrefix(name, "@@auth0spajs@@") {
				if candidate, ok := parseChromeAuthCache(value); ok && tokenIsWeWork(candidate.Token) {
					sessions = append(sessions, candidate)
				}
			}
		}
	}
	if err := iterator.Error(); err != nil {
		return nil, "", "", originSeen, fmt.Errorf("reading Chrome Local Storage snapshot: %w", err)
	}
	return sessions, uuid, member, originSeen, nil
}

func parseChromeAuthCache(value string) (chromeDiskSession, bool) {
	var cached struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Body         struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(value), &cached); err != nil {
		return chromeDiskSession{}, false
	}
	access, refresh := cached.Body.AccessToken, cached.Body.RefreshToken
	if access == "" {
		access, refresh = cached.AccessToken, cached.RefreshToken
	}
	return chromeDiskSession{Token: access, Refresh: refresh}, access != ""
}

func decodeChromeLocalStorageValue(value []byte) (string, error) {
	if len(value) == 0 {
		return "", errors.New("empty Chrome Local Storage value")
	}
	switch value[0] {
	case 1: // Latin-1/UTF-8 string storage (the normal JSON/scalar path).
		return string(value[1:]), nil
	case 0: // UTF-16LE string storage.
		encoded := value[1:]
		if len(encoded)%2 != 0 {
			return "", errors.New("invalid UTF-16 Chrome Local Storage value")
		}
		units := make([]uint16, len(encoded)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(encoded[i*2:])
		}
		return string(utf16.Decode(units)), nil
	default:
		return "", fmt.Errorf("unknown Chrome Local Storage encoding %d", value[0])
	}
}

func chromeLocalStorageScalar(value string) string {
	value = strings.TrimSpace(value)
	var decoded string
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return strings.TrimSpace(decoded)
	}
	return strings.Trim(value, `"`)
}

func snapshotChromeLevelDB(source string) (snapshot string, cleanup func(), err error) {
	root, err := os.MkdirTemp("", "wework-pp-chrome-leveldb-")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating private Chrome snapshot: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(root) }
	snapshot = filepath.Join(root, "leveldb")
	if err := os.Mkdir(snapshot, 0o700); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("creating Chrome snapshot directory: %w", err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "LOCK" {
			continue
		}
		if err := copyPrivateFile(filepath.Join(source, entry.Name()), filepath.Join(snapshot, entry.Name())); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}
	return snapshot, cleanup, nil
}

func copyPrivateFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func itoaScan(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// tokenIsWeWork accepts only a renewable Auth0 token whose issuer is WeWork and
// whose payload names the public SPA client. Generic claim-name heuristics are
// unsafe because unrelated services can carry the same names.
func tokenIsWeWork(token string) bool {
	return config.ValidateWeworkRenewableAccessToken(token) == nil
}

func pickFreshestChromeSession(sessions []chromeDiskSession) chromeDiskSession {
	best := chromeDiskSession{}
	bestExp := time.Time{}
	for _, session := range sessions {
		if session.Token == "" {
			continue
		}
		exp := config.JWTExpiry(session.Token)
		if best.Token == "" || exp.After(bestExp) {
			best, bestExp = session, exp
		}
	}
	return best
}

// chromeLevelDBDirs returns candidate "Local Storage/leveldb" dirs across
// Chrome-family browsers and all their profiles.
func chromeLevelDBDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var bases []string
	switch runtime.GOOS {
	case "darwin":
		as := filepath.Join(home, "Library", "Application Support")
		bases = []string{
			filepath.Join(as, "Google", "Chrome"),
			filepath.Join(as, "Google", "Chrome Beta"),
			filepath.Join(as, "Chromium"),
			filepath.Join(as, "BraveSoftware", "Brave-Browser"),
			filepath.Join(as, "Microsoft Edge"),
		}
	case "linux":
		cfg := filepath.Join(home, ".config")
		bases = []string{
			filepath.Join(cfg, "google-chrome"),
			filepath.Join(cfg, "chromium"),
			filepath.Join(cfg, "BraveSoftware", "Brave-Browser"),
			filepath.Join(cfg, "microsoft-edge"),
		}
	case "windows":
		la := os.Getenv("LOCALAPPDATA")
		if la == "" {
			return nil
		}
		bases = []string{
			filepath.Join(la, "Google", "Chrome", "User Data"),
			filepath.Join(la, "Chromium", "User Data"),
			filepath.Join(la, "BraveSoftware", "Brave-Browser", "User Data"),
			filepath.Join(la, "Microsoft", "Edge", "User Data"),
		}
	default:
		return nil
	}
	var dirs []string
	for _, base := range bases {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			n := e.Name()
			if n == "Default" || n == "Guest Profile" || strings.HasPrefix(n, "Profile ") {
				d := filepath.Join(base, n, "Local Storage", "leveldb")
				if fi, err := os.Stat(d); err == nil && fi.IsDir() {
					dirs = append(dirs, d)
				}
			}
		}
	}
	return dirs
}
