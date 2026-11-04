package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEd25519IssuerIssueVerifyAndPublishKey(t *testing.T) {
	t.Parallel()

	privateKey := generateTestPrivateKey(t)
	issuer := newTestIssuer(t, privateKey, "infinitune-auth", "infinitune-api", "auth-key-1")
	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	issuedAt := time.Date(2026, time.November, 4, 12, 0, 0, 0, time.UTC)

	accessToken, err := issuer.IssueAccessToken(userID, []string{"listener", "creator"}, issuedAt)
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	if accessToken.Type != "Bearer" {
		t.Fatalf("IssueAccessToken() type = %q, want Bearer", accessToken.Type)
	}
	if !accessToken.ExpiresAt.Equal(issuedAt.Add(time.Minute)) {
		t.Fatalf("IssueAccessToken() expires at = %v", accessToken.ExpiresAt)
	}

	identity, err := issuer.VerifyAccessToken(accessToken.Value, issuedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	if identity.UserID != userID {
		t.Errorf("VerifyAccessToken() user id = %v, want %v", identity.UserID, userID)
	}
	if !slices.Equal(identity.Roles, []string{"listener", "creator"}) {
		t.Errorf("VerifyAccessToken() roles = %v", identity.Roles)
	}
	if identity.TokenID == uuid.Nil {
		t.Error("VerifyAccessToken() returned a nil token id")
	}
	if !identity.IssuedAt.Equal(issuedAt) || !identity.ExpiresAt.Equal(issuedAt.Add(time.Minute)) {
		t.Errorf("VerifyAccessToken() returned unexpected timestamps: %+v", identity)
	}

	keySet := issuer.JSONWebKeySet()
	if len(keySet.Keys) != 1 {
		t.Fatalf("JSONWebKeySet() key count = %d, want 1", len(keySet.Keys))
	}
	key := keySet.Keys[0]
	if key.KeyType != "OKP" || key.Curve != "Ed25519" || key.Algorithm != "EdDSA" ||
		key.Use != "sig" || key.KeyID != "auth-key-1" {
		t.Errorf("JSONWebKeySet() key = %+v", key)
	}
	decodedPublicKey, err := base64.RawURLEncoding.Strict().DecodeString(key.X)
	if err != nil {
		t.Fatalf("decode JWK x: %v", err)
	}
	if !slices.Equal(decodedPublicKey, privateKey.Public().(ed25519.PublicKey)) {
		t.Error("JSONWebKeySet() published a different public key")
	}
}

func TestEd25519IssuerRejectsInvalidAccessTokens(t *testing.T) {
	t.Parallel()

	privateKey := generateTestPrivateKey(t)
	issuer := newTestIssuer(t, privateKey, "infinitune-auth", "infinitune-api", "auth-key-1")
	now := time.Date(2026, time.November, 4, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	valid := issueTestAccessToken(t, issuer, userID, []string{"listener"}, now)
	tamperedSegments := strings.Split(valid, ".")
	tamperedSignature, err := base64.RawURLEncoding.Strict().DecodeString(tamperedSegments[2])
	if err != nil {
		t.Fatalf("decode test signature: %v", err)
	}
	tamperedSignature[0] ^= 0xff
	tamperedSegments[2] = base64.RawURLEncoding.EncodeToString(tamperedSignature)

	wrongAudienceIssuer := newTestIssuer(t, privateKey, "infinitune-auth", "another-api", "auth-key-1")
	wrongKeyIDIssuer := newTestIssuer(t, privateKey, "infinitune-auth", "infinitune-api", "other-key")

	tests := map[string]string{
		"malformed":        "not-a-jwt",
		"tampered":         strings.Join(tamperedSegments, "."),
		"wrong audience":   issueTestAccessToken(t, wrongAudienceIssuer, userID, []string{"listener"}, now),
		"wrong key id":     issueTestAccessToken(t, wrongKeyIDIssuer, userID, []string{"listener"}, now),
		"expired":          issueTestAccessToken(t, issuer, userID, []string{"listener"}, now.Add(-2*time.Minute)),
		"future issued at": issueTestAccessToken(t, issuer, userID, []string{"listener"}, now.Add(time.Minute)),
		"duplicate roles":  issueTestAccessToken(t, issuer, userID, []string{"listener", "listener"}, now),
	}

	for name, encodedToken := range tests {
		encodedToken := encodedToken
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, verifyErr := issuer.VerifyAccessToken(encodedToken, now.Add(time.Second)); !errors.Is(verifyErr, errInvalidAccessToken) {
				t.Fatalf("VerifyAccessToken() error = %v, want invalid access token", verifyErr)
			}
		})
	}
}

func TestNewEd25519IssuerValidatesConfiguration(t *testing.T) {
	t.Parallel()

	privateKey := generateTestPrivateKey(t)
	encodedPrivateKey := encodeTestPrivateKey(t, privateKey)
	base := Config{
		Issuer:           "infinitune-auth",
		Audience:         "infinitune-api",
		KeyID:            "auth-key-1",
		AccessTokenTTL:   time.Minute,
		PrivateKeyBase64: encodedPrivateKey,
	}

	tests := map[string]func(*Config){
		"missing issuer":   func(config *Config) { config.Issuer = " " },
		"missing audience": func(config *Config) { config.Audience = "" },
		"missing key id":   func(config *Config) { config.KeyID = "" },
		"short ttl":        func(config *Config) { config.AccessTokenTTL = time.Second },
		"long ttl":         func(config *Config) { config.AccessTokenTTL = 2 * time.Hour },
		"invalid key":      func(config *Config) { config.PrivateKeyBase64 = "invalid" },
	}

	for name, mutate := range tests {
		mutate := mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := base
			mutate(&config)
			if _, err := NewEd25519Issuer(config); err == nil {
				t.Fatal("NewEd25519Issuer() error = nil")
			}
		})
	}
}

func generateTestPrivateKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate Ed25519 key: %v", err)
	}
	return privateKey
}

func encodeTestPrivateKey(t *testing.T, privateKey ed25519.PrivateKey) string {
	t.Helper()

	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("encode Ed25519 private key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

func newTestIssuer(
	t *testing.T,
	privateKey ed25519.PrivateKey,
	issuer string,
	audience string,
	keyID string,
) *Ed25519Issuer {
	t.Helper()

	tokenIssuer, err := NewEd25519Issuer(Config{
		Issuer:           issuer,
		Audience:         audience,
		KeyID:            keyID,
		AccessTokenTTL:   time.Minute,
		PrivateKeyBase64: encodeTestPrivateKey(t, privateKey),
	})
	if err != nil {
		t.Fatalf("NewEd25519Issuer() error = %v", err)
	}
	return tokenIssuer
}

func issueTestAccessToken(
	t *testing.T,
	issuer *Ed25519Issuer,
	userID uuid.UUID,
	roles []string,
	issuedAt time.Time,
) string {
	t.Helper()

	accessToken, err := issuer.IssueAccessToken(userID, roles, issuedAt)
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	return accessToken.Value
}
