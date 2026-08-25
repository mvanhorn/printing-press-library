package bambu

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed bambu_printer_ca.pem
var caPEM []byte

func TLSConfig(serial string) (*tls.Config, error) {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("load embedded Bambu printer CA bundle")
	}
	wanted := strings.ToUpper(strings.TrimSpace(serial))
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            roots,
		ClientSessionCache: tls.NewLRUClientSessionCache(16),
		// The serial provides a stable cache key so FTPS data channels resume
		// the control-channel session, which Bambu printers require (FTP 522).
		ServerName: wanted,
		// Printer certificates identify the device by serial CN rather than LAN IP.
		// VerifyConnection performs both chain and serial identity verification.
		InsecureSkipVerify: true, // #nosec G402 -- VerifyConnection performs CA-chain and printer-serial identity verification below.
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("Bambu TLS peer sent no certificate")
			}
			intermediates := x509.NewCertPool()
			for _, cert := range state.PeerCertificates[1:] {
				intermediates.AddCert(cert)
			}
			if _, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{
				Roots: roots, Intermediates: intermediates,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}); err != nil {
				return fmt.Errorf("verify Bambu certificate chain: %w", err)
			}
			actual := strings.ToUpper(strings.TrimSpace(state.PeerCertificates[0].Subject.CommonName))
			if err := verifyPrinterIdentity(actual, wanted); err != nil {
				return err
			}
			return nil
		},
	}, nil
}

func verifyPrinterIdentity(actual, wanted string) error {
	if actual != wanted {
		return fmt.Errorf("Bambu certificate identity does not match configured printer")
	}
	return nil
}
