package lastfm

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

const (
	// AuthCallbackPort is the port used for the local OAuth callback server.
	AuthCallbackPort = 9847
)

// AuthServer handles the OAuth callback flow.
type AuthServer struct {
	server    *http.Server
	listener  net.Listener
	tokenChan chan string
	done      chan struct{}
}

// StartAuthServer starts a local HTTP server to receive the OAuth callback.
// Returns a channel that will receive the token when authorization completes.
func StartAuthServer() (*AuthServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", AuthCallbackPort))
	if err != nil {
		return nil, fmt.Errorf("listen on port %d: %w", AuthCallbackPort, err)
	}

	tokenChan := make(chan string, 1)
	done := make(chan struct{})

	mux := http.NewServeMux()
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	as := &AuthServer{
		server:    server,
		listener:  listener,
		tokenChan: tokenChan,
		done:      done,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Last.fm redirects here after user authorizes with token in query params.
		token := r.URL.Query().Get("token")

		// Send success response to browser
		w.Header().Set("Content-Type", "text/html")
		if token != "" {
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Waves - Last.fm Authorization</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>Authorization Successful!</h1>
<p>You can close this window and return to Waves.</p>
</body>
</html>`)
		} else {
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Waves - Last.fm Authorization</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>Authorization Failed</h1>
<p>No token received. Please try again.</p>
</body>
</html>`)
		}

		// Send token to channel (non-blocking)
		select {
		case tokenChan <- token:
		default:
		}
	})

	go func() {
		_ = server.Serve(listener)
		close(done)
	}()

	return as, nil
}

// TokenChan returns the channel that receives the auth token.
func (as *AuthServer) TokenChan() <-chan string {
	return as.tokenChan
}

// Shutdown stops the auth server.
func (as *AuthServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = as.server.Shutdown(ctx)
	<-as.done
}

// OpenBrowser opens the given URL in the default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
