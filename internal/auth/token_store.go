package auth

import (
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	serviceName = "unspool"
	keyToken    = "token"
)

// loadToken reads the stored OAuth token from the system keychain.
func loadToken() (*oauth2.Token, error) {
	raw, err := keyring.Get(serviceName, keyToken)
	if err != nil {
		return nil, fmt.Errorf("no stored credentials — run 'unspool --login' to authenticate")
	}
	var tok oauth2.Token
	if err := json.Unmarshal([]byte(raw), &tok); err != nil {
		return nil, fmt.Errorf("stored credentials are corrupt — run 'unspool --login' again: %w", err)
	}
	return &tok, nil
}

// saveToken writes tok to the system keychain.
func saveToken(tok *oauth2.Token) error {
	raw, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}
	if err := keyring.Set(serviceName, keyToken, string(raw)); err != nil {
		return fmt.Errorf("store token in keychain: %w", err)
	}
	return nil
}

// Logout removes the stored token from the system keychain.
func Logout() error {
	if err := keyring.Delete(serviceName, keyToken); err != nil && err != keyring.ErrNotFound {
		return fmt.Errorf("remove stored credentials: %w", err)
	}
	return nil
}

// persistingTokenSource wraps an oauth2.TokenSource and writes each newly
// minted token back to the keychain, since golang.org/x/oauth2 refreshes
// tokens transparently but never persists them itself.
type persistingTokenSource struct {
	base oauth2.TokenSource
	last string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != p.last {
		if err := saveToken(tok); err != nil {
			return tok, err
		}
		p.last = tok.AccessToken
	}
	return tok, nil
}
