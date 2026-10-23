package authentication

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"example.com/financial-service/internal/configuration"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type keyHandler struct{ key *rsa.PrivateKey }

func (handler keyHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(writer).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(handler.key.N.Bytes()), "e": "AQAB"}}})
	if err != nil {
		http.Error(writer, "encoding failed", 500)
	}
}
func signedToken(key *rsa.PrivateKey, issuer, audience string, expiry time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{"iss": issuer, "aud": audience, "sub": "service-account", "exp": expiry.Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "providerId": "provider-a", "access": "provider"})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	hash := sha256.Sum256([]byte(payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return payload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
func TestOIDCValidatesSignatureIssuerAudienceAndExpiry(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(keyHandler{key: key})
	defer server.Close()
	validator := NewValidator(configuration.Config{OIDCIssuer: server.URL, OIDCJWKS: server.URL, OIDCAudience: "financial-api"})
	valid, err := signedToken(key, server.URL, "financial-api", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := validator.Verify(context.Background(), valid)
	if err != nil || claims.ProviderID != "provider-a" {
		t.Fatalf("valid token rejected: %v", err)
	}
	cases := []struct {
		issuer, audience string
		expiry           time.Time
	}{{server.URL, "financial-api", time.Now().Add(-time.Hour)}, {server.URL, "another-api", time.Now().Add(time.Minute)}, {"https://another-issuer", "financial-api", time.Now().Add(time.Minute)}}
	for _, testCase := range cases {
		invalid, signErr := signedToken(key, testCase.issuer, testCase.audience, testCase.expiry)
		if signErr != nil {
			t.Fatal(signErr)
		}
		_, verifyErr := validator.Verify(context.Background(), invalid)
		if verifyErr == nil {
			t.Fatal("invalid claims accepted")
		}
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := signedToken(otherKey, server.URL, "financial-api", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, err = validator.Verify(context.Background(), invalid)
	if err == nil {
		t.Fatal("forged signature accepted")
	}
}
