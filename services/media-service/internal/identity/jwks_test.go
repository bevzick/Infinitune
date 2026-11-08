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
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVerifierValidatesAndCachesAuthJWKS(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		body, marshalErr := json.Marshal(keySet{Keys: []webKey{{
			KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA",
			KeyID: "auth-key-1", X: base64.RawURLEncoding.EncodeToString(publicKey),
		}}})
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK",
			Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	verifier, err := NewVerifier(Config{
		JWKSURL: "https://auth.example.test/v1/auth/jwks", Issuer: "infinitune-auth",
		Audience: "infinitune-api", KeyID: "auth-key-1", RefreshInterval: 5 * time.Minute,
	}, client)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	token := signToken(t, privateKey, userID, now, now.Add(15*time.Minute))
	for range 2 {
		verified, verifyErr := verifier.Verify(context.Background(), token, now.Add(time.Minute))
		if verifyErr != nil || verified.UserID != userID {
			t.Fatalf("Verify() = %+v, %v", verified, verifyErr)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("JWKS requests = %d, want 1", requests.Load())
	}
}

func TestVerifierRejectsExpiredToken(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	client := jwksClient(t, publicKey)
	verifier, err := NewVerifier(Config{
		JWKSURL: "https://auth.example.test/v1/auth/jwks", Issuer: "infinitune-auth",
		Audience: "infinitune-api", KeyID: "auth-key-1", RefreshInterval: time.Minute,
	}, client)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	token := signToken(t, privateKey, uuid.New(), now.Add(-time.Hour), now.Add(-time.Minute))
	if _, err = verifier.Verify(context.Background(), token, now); err != ErrInvalidAccessToken {
		t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidAccessToken)
	}
}

func jwksClient(t *testing.T, publicKey ed25519.PublicKey) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		body, err := json.Marshal(keySet{Keys: []webKey{{
			KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA",
			KeyID: "auth-key-1", X: base64.RawURLEncoding.EncodeToString(publicKey),
		}}})
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK",
			Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func signToken(
	t *testing.T,
	privateKey ed25519.PrivateKey,
	userID uuid.UUID,
	issuedAt time.Time,
	expiresAt time.Time,
) string {
	t.Helper()
	headerJSON, err := json.Marshal(tokenHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: "auth-key-1"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(tokenClaims{
		Subject: userID.String(), Issuer: "infinitune-auth", Audience: "infinitune-api",
		IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encoding := base64.RawURLEncoding
	signingInput := encoding.EncodeToString(headerJSON) + "." + encoding.EncodeToString(claimsJSON)
	return signingInput + "." + encoding.EncodeToString(ed25519.Sign(privateKey, []byte(signingInput)))
}
