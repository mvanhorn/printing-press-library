// tesla auth fleet-template scaffolds a Vercel-ready public-key host into a
// user-chosen directory and, optionally, generates a fresh EC P256 keypair
// whose public half lands in the template's .well-known path. This removes
// the biggest UX cliff in Tesla Fleet API setup: standing up a domain that
// hosts the partner-app public key.
//
// Hand-coded; lives outside the generator's emit set so it survives
// `printing-press generate --force` regens.
package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed all:templates/vercel-tesla-keys
var fleetTemplateFS embed.FS

const fleetTemplateRoot = "templates/vercel-tesla-keys"

func newFleetTemplateCmd(flags *rootFlags) *cobra.Command {
	var dest string
	var genKey bool
	var force bool
	cmd := &cobra.Command{
		Use:   "fleet-template",
		Short: "Scaffold a Vercel-ready public-key host for Tesla Fleet API",
		Long: `Scaffold a Vercel-deployable static site that hosts your partner-app
public key at the path Tesla scans during partner_accounts registration
(.well-known/appspecific/com.tesla.3p.public-key.pem). Optionally generates a
fresh EC P256 keypair; the private half lands in ~/.tesla/<dest-basename>-private.pem
(mode 600), the public half is embedded into the template's .well-known path.

After scaffolding:
  cd <dest> && vercel deploy --prod

Then pass the resulting hostname to ` + "`tesla auth fleet-register --public-key-domain`" + `.
`,
		Annotations: map[string]string{
			"mcp:read-only": "false", // writes to disk; not read-only
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if isVerify() {
				fmt.Fprintf(cmd.OutOrStdout(), "would scaffold Vercel template at %s (gen-key=%v force=%v)\n", dest, genKey, force)
				return nil
			}
			return runFleetTemplate(cmd, dest, genKey, force)
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "./tesla-keys-host", "destination directory for the scaffolded template")
	cmd.Flags().BoolVar(&genKey, "gen-key", false, "also generate a fresh EC P256 keypair and write the public half into the template")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite template files in dest if it already exists; never overwrites an existing private key")
	return cmd
}

func runFleetTemplate(cmd *cobra.Command, dest string, genKey, force bool) error {
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return usageErr(fmt.Errorf("resolve dest: %w", err))
	}
	// Refuse to write into a non-empty dest unless --force.
	if entries, statErr := os.ReadDir(absDest); statErr == nil && len(entries) > 0 && !force {
		return usageErr(fmt.Errorf("dest %s exists and is non-empty; pass --force to overwrite template files", absDest))
	}
	if err := os.MkdirAll(absDest, 0o755); err != nil {
		return fmt.Errorf("create dest: %w", err)
	}

	// Copy the embedded template tree into dest.
	if err := copyEmbeddedTree(fleetTemplateFS, fleetTemplateRoot, absDest); err != nil {
		return fmt.Errorf("copy template: %w", err)
	}

	if genKey {
		baseName := filepath.Base(absDest)
		if baseName == "." || baseName == "/" {
			baseName = "tesla-keys-host"
		}
		privPath := filepath.Join(homeOrDot(), ".tesla", baseName+"-private.pem")
		pubPath := filepath.Join(absDest, "public", ".well-known", "appspecific", "com.tesla.3p.public-key.pem")

		if _, statErr := os.Stat(privPath); statErr == nil {
			// Existing private key. Do NOT overwrite; reuse the matching
			// public key if we can derive it. The simplest path is to
			// leave the example placeholder in dest and tell the user.
			fmt.Fprintf(cmd.OutOrStdout(), "Private key already exists at %s; not regenerating.\n", privPath)
			fmt.Fprintf(cmd.OutOrStdout(), "If the .well-known/appspecific/com.tesla.3p.public-key.pem file in %s needs the matching public key, copy it from your records.\n", absDest)
		} else {
			if err := os.MkdirAll(filepath.Dir(privPath), 0o700); err != nil {
				return fmt.Errorf("create ~/.tesla: %w", err)
			}
			pubPEM, err := genFleetKeypair(privPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
				return fmt.Errorf("write public key: %w", err)
			}
			// Remove the placeholder .example file if present.
			_ = os.Remove(pubPath + ".example")
			fmt.Fprintf(cmd.OutOrStdout(), "Generated EC P256 keypair.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Private key: %s (mode 600)\n", privPath)
			fmt.Fprintf(cmd.OutOrStdout(), "  Public key:  %s\n", pubPath)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Template scaffolded without --gen-key; the .well-known/appspecific/com.tesla.3p.public-key.pem file is a placeholder. Replace it with your real public key before deploying.\n")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nNext step:\n  cd %s && vercel deploy --prod\n\nThen pass the resulting hostname to:\n  tesla auth fleet-register --public-key-domain <hostname> --client-id ... --client-secret ...\n", absDest)
	return nil
}

// copyEmbeddedTree walks the embedded src subtree and mirrors it into dest.
// Re-applies the same directory structure; files are written with mode 0644,
// directories with 0755. The .example placeholder for the public key is
// renamed to its non-.example form ONLY when the caller is going to write
// real key bytes over it; otherwise we keep the .example suffix so users
// know to replace it before deploy.
func copyEmbeddedTree(fsys embed.FS, src, dest string) error {
	return fs.WalkDir(fsys, src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := fsys.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}

// genFleetKeypair generates an EC P256 keypair, writes the private key to
// privPath in PKCS#8 PEM (mode 600), and returns the public key in SPKI PEM
// (the format Tesla expects at .well-known/appspecific/com.tesla.3p.public-key.pem).
func genFleetKeypair(privPath string) ([]byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate EC P256: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), nil
}

// homeOrDot returns the user's home dir, falling back to "." so the scaffolder
// degrades to a relative key location on systems where UserHomeDir fails.
func homeOrDot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return home
}

// isVerify reports whether the printing-press verify harness short-circuit
// is active. Mirrors cliutil.IsVerifyEnv() without taking a dependency on
// the cliutil package from this file (kept independent for testability).
func isVerify() bool {
	for _, k := range []string{"PRINTING_PRESS_VERIFY", "PRINTING_PRESS_VERIFY_LIVE_HTTP"} {
		if v := os.Getenv(k); v != "" && !strings.EqualFold(v, "false") && v != "0" {
			return true
		}
	}
	return false
}

// errFleetTemplate signals a usage error from this unit so callers can
// distinguish a misuse from an environment error. Currently unused but kept
// for parity with other auth subcommands; remove if no caller picks it up
// during U2.
var errFleetTemplate = errors.New("fleet-template usage error")
