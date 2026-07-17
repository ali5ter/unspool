package cmd

import (
	"context"
	"fmt"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/auth"
)

func runLogin(cfg *config.Config) error {
	return auth.Login(context.Background(), cfg.OAuthClientSecretFile)
}

func runLogout() error {
	if err := auth.Logout(); err != nil {
		return err
	}
	fmt.Println("Logged out — stored credentials removed from the system keychain.")
	return nil
}
