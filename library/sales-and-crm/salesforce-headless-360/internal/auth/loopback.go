package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type ListenFunc func(network, address string) (net.Listener, error)

type CallbackResult struct {
	Code string
}

type LoopbackOptions struct {
	State   string
	Timeout time.Duration
	Listen  ListenFunc
}

type LoopbackServer struct {
	RedirectURI string
	Results     <-chan CallbackResult
	Errors      <-chan error
	close       func() error
}

func (s *LoopbackServer) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

type BindError struct {
	Attempts int
	Err      error
}

func (e *BindError) Error() string {
	return fmt.Sprintf("binding OAuth loopback listener failed after %d attempts: %v", e.Attempts, e.Err)
}

func (e *BindError) Unwrap() error { return e.Err }

func StartLoopback(ctx context.Context, opts LoopbackOptions) (*LoopbackServer, error) {
	if opts.State == "" {
		return nil, &Error{Kind: "oauth_state", Message: "state is required"}
	}
	listen := opts.Listen
	if listen == nil {
		listen = net.Listen
	}
	var (
		ln  net.Listener
		err error
	)
	for i := 0; i < 3; i++ {
		const addr = "127.0.0.1:0"
		if addr[:9] != "127.0.0.1" {
			panic("OAuth loopback listener must bind 127.0.0.1 only")
		}
		ln, err = listen("tcp4", addr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, &BindError{Attempts: 3, Err: err}
	}
	host, _, splitErr := net.SplitHostPort(ln.Addr().String())
	if splitErr != nil || host != "127.0.0.1" {
		ln.Close()
		panic("OAuth loopback listener bound non-loopback address")
	}

	resultCh := make(chan CallbackResult, 1)
	errCh := make(chan error, 1)
	server := &http.Server{}
	mux := http.NewServeMux()
	server.Handler = mux
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("state") != opts.State {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		if msg := query.Get("error"); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			select {
			case errCh <- &Error{Kind: "oauth_callback", Message: "authorization server returned error", Err: fmt.Errorf("%s", msg)}:
			default:
			}
			return
		}
		code := query.Get("code")
		if code == "" {
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authentication successful</h2><p>You can close this tab.</p></body></html>")
		select {
		case resultCh <- CallbackResult{Code: code}:
		default:
		}
		go server.Shutdown(context.Background())
	})
	go func() {
		if serveErr := server.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			select {
			case errCh <- serveErr:
			default:
			}
		}
	}()
	go func() {
		if opts.Timeout == 0 {
			opts.Timeout = 2 * time.Minute
		}
		select {
		case <-ctx.Done():
			server.Shutdown(context.Background())
			select {
			case errCh <- ctx.Err():
			default:
			}
		case <-time.After(opts.Timeout):
			server.Shutdown(context.Background())
			select {
			case errCh <- &Error{Kind: "oauth_timeout", Message: "authentication timed out"}:
			default:
			}
		}
	}()
	return &LoopbackServer{
		RedirectURI: "http://" + ln.Addr().String() + "/callback",
		Results:     resultCh,
		Errors:      errCh,
		close:       func() error { return server.Shutdown(context.Background()) },
	}, nil
}
