package configuration

import (
	"encoding/json"
	"errors"
	"os"
)

type Config struct {
	DatabaseURL, HTTPAddress, OIDCIssuer, OIDCJWKS, OIDCAudience, SQSEndpoint, AWSRegion, AWSKey, AWSSecret, QueueURL, EventsURL string
	ProviderSenders                                                                                                              map[string]string
}

func value(name, fallback string) string {
	result := os.Getenv(name)
	if result == "" {
		return fallback
	}
	return result
}
func Load() (Config, error) {
	config := Config{DatabaseURL: os.Getenv("DATABASE_URL"), HTTPAddress: value("HTTP_ADDRESS", ":8080"), OIDCIssuer: os.Getenv("OIDC_ISSUER"), OIDCJWKS: os.Getenv("OIDC_JWKS_URL"), OIDCAudience: value("OIDC_AUDIENCE", "financial-api"), SQSEndpoint: value("SQS_ENDPOINT", "http://localhost:4566"), AWSRegion: value("AWS_REGION", "us-east-1"), AWSKey: value("AWS_ACCESS_KEY_ID", "test"), AWSSecret: value("AWS_SECRET_ACCESS_KEY", "test"), QueueURL: os.Getenv("SQS_QUEUE_URL"), EventsURL: os.Getenv("SQS_EVENTS_URL")}
	if raw := os.Getenv("SQS_PROVIDER_SENDERS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &config.ProviderSenders); err != nil {
			return config, errors.New("invalid SQS_PROVIDER_SENDERS JSON")
		}
	}
	if config.DatabaseURL == "" || config.OIDCIssuer == "" || config.OIDCJWKS == "" {
		return config, errors.New("DATABASE_URL, OIDC_ISSUER and OIDC_JWKS_URL are required")
	}
	return config, nil
}
