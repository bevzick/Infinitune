package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVerifierAcceptsValidSubjectAndRejectsExpiredToken(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		body, marshalErr := json.Marshal(keySet{Keys: []webKey{{KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA", KeyID: "auth-key-1", X: base64.RawURLEncoding.EncodeToString(publicKey)}}})
		if marshalErr != nil {
			t.Fatalf("marshal JWKS: %v", marshalErr)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	verifier, err := NewVerifier(Config{JWKSURL: "https://auth.example.test/v1/auth/jwks", Issuer: "infinitune-auth", Audience: "infinitune-api", KeyID: "auth-key-1", RefreshInterval: 5 * time.Minute}, client)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	verified, err := verifier.Verify(context.Background(), signToken(t, privateKey, userID, now, now.Add(15*time.Minute)), now)
	if err != nil || verified.UserID != userID || len(verified.Roles) != 1 || verified.Roles[0] != "moderator" {
		t.Fatalf("Verify(valid) = %+v, %v", verified, err)
	}
	_, err = verifier.Verify(context.Background(), signToken(t, privateKey, userID, now.Add(-time.Hour), now.Add(-time.Minute)), now)
	if err != ErrInvalidAccessToken {
		t.Fatalf("Verify(expired) error = %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func signToken(t *testing.T, privateKey ed25519.PrivateKey, userID uuid.UUID, issuedAt, expiresAt time.Time) string {
	t.Helper()
	header, _ := json.Marshal(tokenHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: "auth-key-1"})
	claims, _ := json.Marshal(tokenClaims{Subject: userID.String(), Issuer: "infinitune-auth", Audience: "infinitune-api", IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix(), Roles: []string{"moderator", "moderator"}})
	encoding := base64.RawURLEncoding
	input := encoding.EncodeToString(header) + "." + encoding.EncodeToString(claims)
	return input + "." + encoding.EncodeToString(ed25519.Sign(privateKey, []byte(input)))
}
