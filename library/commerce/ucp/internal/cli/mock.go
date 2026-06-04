package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/mock"
	"github.com/spf13/cobra"
)

func newMockCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "mock",
		Short:  "Internal UCP fixture-merchant for CI/conformance testing (hidden)",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMockServeCmd(flags))
	return cmd
}

func newMockServeCmd(flags *rootFlags) *cobra.Command {
	var port int
	var addr string

	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Start the bundled pure-Go reference UCP merchant on a local port",
		Hidden:  true,
		Example: `  ucp-pp-cli mock serve --port 8080`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			listenAddr := fmt.Sprintf("%s:%d", addr, port)
			srv, err := mock.Start(listenAddr)
			if err != nil {
				return fmt.Errorf("start mock server: %w", err)
			}
			fmt.Fprintf(os.Stderr, "mock UCP merchant listening on http://%s/.well-known/ucp\n", listenAddr)

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			fmt.Fprintln(os.Stderr, "shutting down mock server...")
			return srv.Shutdown(context.Background())
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1", "Address to bind")

	// suppress the unused import warning
	_ = http.StatusOK
	return cmd
}
