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
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		requests.Add(1)
		body, marshalErr := json.Marshal(keySet{Keys: []webKey{{
			KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA",
			KeyID: "auth-key-1", X: base64.RawURLEncoding.EncodeToString(publicKey),
		}}})
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	verifier, err := NewVerifier(Config{
		JWKSURL: "https://auth.example.test/v1/auth/jwks", Issuer: "infinitune-auth", Audience: "infinitune-api",
		KeyID: "auth-key-1", RefreshInterval: 5 * time.Minute,
	}, client)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	userID := uuid.New()
	now := time.Date(2026, time.November, 5, 8, 0, 0, 0, time.UTC)
	token := signToken(t, privateKey, userID, "infinitune-auth", "infinitune-api", now, now.Add(15*time.Minute))

	for range 2 {
		verified, verifyErr := verifier.Verify(context.Background(), token, now.Add(time.Minute))
		if verifyErr != nil {
			t.Fatalf("Verify() error = %v", verifyErr)
		}
		if verified.UserID != userID {
			t.Fatalf("Verify().UserID = %s, want %s", verified.UserID, userID)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("JWKS requests = %d, want 1", requests.Load())
	}
}

func TestVerifierRejectsInvalidClaimsAndSignature(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	_, otherPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}
	client := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		body, marshalErr := json.Marshal(keySet{Keys: []webKey{{
			KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA",
			KeyID: "auth-key-1", X: base64.RawURLEncoding.EncodeToString(publicKey),
		}}})
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	verifier, err := NewVerifier(Config{
		JWKSURL: "https://auth.example.test/v1/auth/jwks", Issuer: "infinitune-auth", Audience: "infinitune-api",
		KeyID: "auth-key-1", RefreshInterval: 5 * time.Minute,
	}, client)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	now := time.Date(2026, time.November, 5, 8, 0, 0, 0, time.UTC)
	userID := uuid.New()

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "wrong audience",
			token: signToken(t, privateKey, userID, "infinitune-auth", "another-api", now, now.Add(15*time.Minute)),
		},
		{
			name:  "expired",
			token: signToken(t, privateKey, userID, "infinitune-auth", "infinitune-api", now.Add(-time.Hour), now.Add(-time.Minute)),
		},
		{
			name:  "wrong signature",
			token: signToken(t, otherPrivateKey, userID, "infinitune-auth", "infinitune-api", now, now.Add(15*time.Minute)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, verifyErr := verifier.Verify(context.Background(), test.token, now); verifyErr != ErrInvalidAccessToken {
				t.Fatalf("Verify() error = %v, want %v", verifyErr, ErrInvalidAccessToken)
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func signToken(
	t *testing.T,
	privateKey ed25519.PrivateKey,
	userID uuid.UUID,
	issuer string,
	audience string,
	issuedAt time.Time,
	expiresAt time.Time,
) string {
	t.Helper()
	headerJSON, err := json.Marshal(tokenHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: "auth-key-1"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(tokenClaims{
		Subject: userID.String(), Issuer: issuer, Audience: audience,
		IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encoding := base64.RawURLEncoding
	signingInput := encoding.EncodeToString(headerJSON) + "." + encoding.EncodeToString(claimsJSON)
	return signingInput + "." + encoding.EncodeToString(ed25519.Sign(privateKey, []byte(signingInput)))
}
