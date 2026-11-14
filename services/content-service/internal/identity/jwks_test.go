package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVerifierValidatesIdentityAndRoles(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := testVerifier(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	now, userID := time.Now().UTC().Truncate(time.Second), uuid.New()
	token := signToken(t, privateKey, tokenClaims{Subject: userID.String(), Issuer: "issuer", Audience: "api", IssuedAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), Roles: []string{"moderator", "moderator"}})
	verified, err := verifier.Verify(context.Background(), token, now)
	if err != nil || verified.UserID != userID || len(verified.Roles) != 1 || verified.Roles[0] != "moderator" {
		t.Fatalf("Verify() = %+v, %v", verified, err)
	}
}

func TestVerifierRejectsMalformedRole(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := testVerifier(publicKey)
	now := time.Now().UTC().Truncate(time.Second)
	token := signToken(t, privateKey, tokenClaims{Subject: uuid.NewString(), Issuer: "issuer", Audience: "api", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), Roles: []string{" moderator "}})
	if _, err := verifier.Verify(context.Background(), token, now); err != ErrInvalidAccessToken {
		t.Fatalf("Verify() error = %v", err)
	}
}

func testVerifier(publicKey ed25519.PublicKey) (*Verifier, error) {
	document, _ := json.Marshal(keySet{Keys: []webKey{{
		KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA",
		KeyID: "key-1", X: base64.RawURLEncoding.EncodeToString(publicKey),
	}}})
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(document)))}, nil
	})}
	return NewVerifier(Config{JWKSURL: "http://auth.test/jwks", Issuer: "issuer", Audience: "api", KeyID: "key-1", RefreshInterval: time.Minute}, client)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func signToken(t *testing.T, key ed25519.PrivateKey, claims tokenClaims) string {
	t.Helper()
	encoding := base64.RawURLEncoding
	headerJSON, _ := json.Marshal(tokenHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: "key-1"})
	claimsJSON, _ := json.Marshal(claims)
	input := encoding.EncodeToString(headerJSON) + "." + encoding.EncodeToString(claimsJSON)
	return input + "." + encoding.EncodeToString(ed25519.Sign(key, []byte(input)))
}
