// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// SSH transport for the OpenWrt/UCI layer beneath GL firmware. The GL JSON-RPC
// API exposes no whole-config backup/restore and ubus-over-HTTP is not proxied
// on stock GL firmware, so the config-snapshot engine and version detection
// reach the router's UCI tree over SSH (dropbear, port 22). Go's crypto/ssh
// does password auth natively — no external sshpass needed.
package glssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/client"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Config describes how to reach the router over SSH.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	KeyPath  string
	Timeout  time.Duration
}

// ResolveConfig derives SSH settings from the router base URL and environment.
// Host comes from baseURL; user defaults to root; password reuses the router
// admin password (client.GLPassword); an optional key path overrides password
// auth (GL_INET_SSH_KEY).
func ResolveConfig(baseURL string) (Config, error) {
	host := baseURL
	for _, p := range []string{"https://", "http://"} {
		host = strings.TrimPrefix(host, p)
	}
	host = strings.TrimRight(host, "/")
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if h := strings.TrimSpace(os.Getenv("GL_INET_SSH_HOST")); h != "" {
		host = h
	}
	if host == "" {
		return Config{}, fmt.Errorf("could not determine router host from base URL %q", baseURL)
	}
	port := 22
	if p := strings.TrimSpace(os.Getenv("GL_INET_SSH_PORT")); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	user := "root"
	if u := strings.TrimSpace(os.Getenv("GL_INET_SSH_USER")); u != "" {
		user = u
	}
	cfg := Config{Host: host, Port: port, User: user, Timeout: 15 * time.Second}
	if kp := strings.TrimSpace(os.Getenv("GL_INET_SSH_KEY")); kp != "" {
		cfg.KeyPath = kp
		return cfg, nil
	}
	pw, err := client.GLPassword()
	if err != nil {
		return Config{}, err
	}
	cfg.Password = pw
	return cfg, nil
}

func (c Config) authMethods() ([]ssh.AuthMethod, error) {
	if c.KeyPath != "" {
		key, err := os.ReadFile(c.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading SSH key %s: %w", c.KeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parsing SSH key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}
	// Password auth; the router accepts keyboard-interactive too, so offer both.
	pw := c.Password
	return []ssh.AuthMethod{
		ssh.Password(pw),
		ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = pw
			}
			return answers, nil
		}),
	}, nil
}

// knownHostsPath returns the file where router host keys are pinned.
// Overridable with GL_INET_KNOWN_HOSTS.
func knownHostsPath() string {
	if p := strings.TrimSpace(os.Getenv("GL_INET_KNOWN_HOSTS")); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "gl-inet", "known_hosts")
}

// tofuHostKeyCallback returns a trust-on-first-use host-key verifier. An unknown
// host is pinned on first connect; a host whose key later differs from the pin
// is rejected (possible MITM). This replaces InsecureIgnoreHostKey, which never
// detects a key change — meaningful for a tool used on untrusted venue networks.
func tofuHostKeyCallback() (ssh.HostKeyCallback, error) {
	path := knownHostsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating known_hosts dir: %w", err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("creating known_hosts: %w", err)
		}
		_ = f.Close()
	}
	verify, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("loading known_hosts: %w", err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				// Unknown host: trust on first use and pin it.
				return appendKnownHost(path, hostname, key)
			}
			// Known host whose key changed: refuse.
			return fmt.Errorf("SSH host key for %s does not match the pinned key (possible MITM). "+
				"If you intentionally reset or replaced the router, remove its entry from %s and retry", hostname, path)
		}
		return err
	}, nil
}

// appendKnownHost pins a host key by appending a known_hosts line.
func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("pinning host key: %w", err)
	}
	defer f.Close()
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("pinning host key: %w", err)
	}
	return nil
}

// Run opens a session, executes a single command, and returns combined stdout.
// stderr is returned in the error when the command exits non-zero.
func Run(ctx context.Context, cfg Config, command string) (string, error) {
	auth, err := cfg.authMethods()
	if err != nil {
		return "", err
	}
	hostKeyCb, err := tofuHostKeyCallback()
	if err != nil {
		return "", err
	}
	sshCfg := &ssh.ClientConfig{
		User: cfg.User,
		Auth: auth,
		// Trust-on-first-use: there is no CA for a LAN router, but a one-time
		// pin still protects the travel use case — if the key first seen at
		// home later changes on a hostile venue network, the connection is
		// rejected instead of silently handing the admin password to an
		// impersonator.
		HostKeyCallback: hostKeyCb,
		Timeout:         cfg.Timeout,
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("ssh handshake (check SSH is enabled and the password/key): %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return stdout.String(), ctx.Err()
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return stdout.String(), fmt.Errorf("remote command failed: %s", msg)
		}
	}
	return stdout.String(), nil
}

// UCIExport returns the full `uci export` text (or one config when name is set):
// the canonical, restorable representation of the router configuration.
func UCIExport(ctx context.Context, cfg Config, name string) (string, error) {
	cmd := "uci export"
	if strings.TrimSpace(name) != "" {
		cmd += " " + shellQuote(name)
	}
	return Run(ctx, cfg, cmd)
}

// UCIShow returns the flat dotted `uci show` form, ideal for option-level diffs.
func UCIShow(ctx context.Context, cfg Config, name string) (string, error) {
	cmd := "uci show"
	if strings.TrimSpace(name) != "" {
		cmd += " " + shellQuote(name)
	}
	return Run(ctx, cfg, cmd)
}

// shellQuote single-quotes a UCI config/section name for safe shell embedding.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
