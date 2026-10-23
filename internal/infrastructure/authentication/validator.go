package authentication

import (
	"context"
	"errors"
	"example.com/financial-service/internal/configuration"
	"github.com/coreos/go-oidc/v3/oidc"
	"net/http"
	"time"
)

type Claims struct {
	Subject    string `json:"sub"`
	ProviderID string `json:"providerId"`
	Access     string `json:"access"`
}
type Validator struct {
	verifier *oidc.IDTokenVerifier
	client   *http.Client
}

func NewValidator(config configuration.Config) *Validator {
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := oidc.ClientContext(context.Background(), client)
	keys := oidc.NewRemoteKeySet(ctx, config.OIDCJWKS)
	verifier := oidc.NewVerifier(config.OIDCIssuer, keys, &oidc.Config{ClientID: config.OIDCAudience, SupportedSigningAlgs: []string{"RS256"}})
	return &Validator{verifier: verifier, client: client}
}
func (validator *Validator) Verify(ctx context.Context, raw string) (Claims, error) {
	token, err := validator.verifier.Verify(ctx, raw)
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	err = token.Claims(&claims)
	if err != nil {
		return claims, err
	}
	if claims.Subject == "" || (claims.Access != "internal" && claims.Access != "provider") || (claims.Access == "provider" && claims.ProviderID == "") {
		return claims, errors.New("invalid authorization claims")
	}
	return claims, nil
}
