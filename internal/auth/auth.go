// Package auth handles the YouTube Data API v3 OAuth 2.0 installed-app
// loopback flow and system-keychain token storage.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

// loopbackTimeout bounds how long --login waits for the browser callback.
// A headless/SSH session with no local browser will hit this and fail
// explicitly, rather than hang — see docs/SETUP.md for the fallbacks.
const loopbackTimeout = 3 * time.Minute

// scopes requests read access plus the write access needed to manage
// playlists and rate videos.
var scopes = []string{
	youtube.YoutubeScope,
	youtube.YoutubeForceSslScope,
}

// configFromFile loads the OAuth client config from the downloaded Google
// Cloud client secret JSON at path (see docs/SETUP.md).
func configFromFile(path string) (*oauth2.Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read OAuth client secret file %q: %w\n\nRun through docs/SETUP.md to create a Google Cloud OAuth client, "+
				"download its JSON, and save it to this path (or set oauth_client_secret_file in config.toml)",
			path, err,
		)
	}
	cfg, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parse OAuth client secret file %q: %w", path, err)
	}
	return cfg, nil
}

// Login runs the installed-app loopback OAuth flow: starts a local server on
// a random port, opens (or prints) the consent URL, waits for the callback,
// exchanges the code for a token, and stores it in the system keychain.
func Login(ctx context.Context, clientSecretFile string) error {
	cfg, err := configFromFile(clientSecretFile)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	state, err := randomState()
	if err != nil {
		return fmt.Errorf("generate OAuth state: %w", err)
	}

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("OAuth state mismatch — possible CSRF, aborting")}
			return
		}
		if errMsg := q.Get("error"); errMsg != "" {
			http.Error(w, "authorization denied", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("authorization denied: %s", errMsg)}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("no authorization code in callback")}
			return
		}
		fmt.Fprintln(w, "unspool: authenticated — you can close this tab and return to the terminal.")
		resultCh <- result{code: code}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	fmt.Println("Opening browser for YouTube authorization...")
	fmt.Println("If it doesn't open automatically, visit:")
	fmt.Println()
	fmt.Println("  " + authURL)
	fmt.Println()
	openBrowser(authURL)

	select {
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		tok, err := cfg.Exchange(ctx, res.code)
		if err != nil {
			return fmt.Errorf("exchange authorization code: %w", err)
		}
		if err := saveToken(tok); err != nil {
			return err
		}
		fmt.Println("Logged in — refresh token stored in the system keychain.")
		return nil

	case <-time.After(loopbackTimeout):
		return fmt.Errorf(
			"timed out waiting for the browser callback (%s)\n\n"+
				"This usually means no browser is available on this host (e.g. over SSH). Either:\n"+
				"  - SSH port-forward the loopback port and retry, or\n"+
				"  - run 'unspool --login' once on a machine with a browser, then sync the token\n"+
				"    via store_dir to this host",
			loopbackTimeout,
		)

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Client returns an HTTP client that authenticates YouTube Data API v3
// requests using the stored token, transparently refreshing (and
// re-persisting) it as needed.
func Client(ctx context.Context, clientSecretFile string) (*http.Client, error) {
	cfg, err := configFromFile(clientSecretFile)
	if err != nil {
		return nil, err
	}
	tok, err := loadToken()
	if err != nil {
		return nil, err
	}
	src := &persistingTokenSource{base: cfg.TokenSource(ctx, tok), last: tok.AccessToken}
	return oauth2.NewClient(ctx, src), nil
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// openBrowser best-effort opens url in the default browser. Failure is not
// fatal — the URL is always printed so the user can open it manually.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
