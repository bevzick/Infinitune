package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	allowedClockSkew = 30 * time.Second
	maximumTokenTTL  = time.Hour
	maximumJWKSSize  = 64 * 1024
)

var ErrInvalidAccessToken = errors.New("invalid access token")

type Config struct {
	JWKSURL         string
	Issuer          string
	Audience        string
	KeyID           string
	RefreshInterval time.Duration
}

type Identity struct {
	UserID uuid.UUID
}

type Verifier struct {
	config    Config
	client    *http.Client
	mutex     sync.Mutex
	key       ed25519.PublicKey
	refreshAt time.Time
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type tokenClaims struct {
	Subject   string `json:"sub"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type keySet struct {
	Keys []webKey `json:"keys"`
}

type webKey struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	X         string `json:"x"`
}

func NewVerifier(config Config, client *http.Client) (*Verifier, error) {
	parsedURL, err := url.Parse(config.JWKSURL)
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil ||
		(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, errors.New("JWKS URL must be an absolute HTTP or HTTPS URL without credentials")
	}
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("JWT issuer is required")
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, errors.New("JWT audience is required")
	}
	if strings.TrimSpace(config.KeyID) == "" {
		return nil, errors.New("JWT key id is required")
	}
	if config.RefreshInterval <= 0 {
		return nil, errors.New("JWKS refresh interval must be positive")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	return &Verifier{config: config, client: client}, nil
}

func (verifier *Verifier) Verify(
	ctx context.Context,
	encodedToken string,
	now time.Time,
) (Identity, error) {
	header, claimsSegment, signingInput, signature, err := parseToken(encodedToken)
	if err != nil || header.Algorithm != "EdDSA" || header.Type != "JWT" ||
		header.KeyID != verifier.config.KeyID {
		return Identity{}, ErrInvalidAccessToken
	}

	key, err := verifier.publicKey(ctx, now, false)
	if err != nil {
		return Identity{}, fmt.Errorf("load JWT verification key: %w", err)
	}
	if !ed25519.Verify(key, []byte(signingInput), signature) {
		key, err = verifier.publicKey(ctx, now, true)
		if err != nil || !ed25519.Verify(key, []byte(signingInput), signature) {
			return Identity{}, ErrInvalidAccessToken
		}
	}

	claimsJSON, err := base64.RawURLEncoding.Strict().DecodeString(claimsSegment)
	if err != nil {
		return Identity{}, ErrInvalidAccessToken
	}
	var claims tokenClaims
	if err = json.Unmarshal(claimsJSON, &claims); err != nil ||
		claims.Issuer != verifier.config.Issuer ||
		claims.Audience != verifier.config.Audience ||
		claims.IssuedAt <= 0 || claims.ExpiresAt <= 0 {
		return Identity{}, ErrInvalidAccessToken
	}

	issuedAt := time.Unix(claims.IssuedAt, 0).UTC()
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	now = now.UTC()
	if !expiresAt.After(now) || issuedAt.After(now.Add(allowedClockSkew)) ||
		!expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > maximumTokenTTL {
		return Identity{}, ErrInvalidAccessToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return Identity{}, ErrInvalidAccessToken
	}

	return Identity{UserID: userID}, nil
}

func parseToken(encodedToken string) (tokenHeader, string, string, []byte, error) {
	if encodedToken == "" || strings.TrimSpace(encodedToken) != encodedToken {
		return tokenHeader{}, "", "", nil, ErrInvalidAccessToken
	}
	segments := strings.Split(encodedToken, ".")
	if len(segments) != 3 {
		return tokenHeader{}, "", "", nil, ErrInvalidAccessToken
	}

	encoding := base64.RawURLEncoding.Strict()
	headerJSON, err := encoding.DecodeString(segments[0])
	if err != nil {
		return tokenHeader{}, "", "", nil, ErrInvalidAccessToken
	}
	var header tokenHeader
	if err = json.Unmarshal(headerJSON, &header); err != nil {
		return tokenHeader{}, "", "", nil, ErrInvalidAccessToken
	}
	signature, err := encoding.DecodeString(segments[2])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return tokenHeader{}, "", "", nil, ErrInvalidAccessToken
	}

	return header, segments[1], segments[0] + "." + segments[1], signature, nil
}

func (verifier *Verifier) publicKey(
	ctx context.Context,
	now time.Time,
	forceRefresh bool,
) (ed25519.PublicKey, error) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if !forceRefresh && len(verifier.key) == ed25519.PublicKeySize && now.Before(verifier.refreshAt) {
		return append(ed25519.PublicKey(nil), verifier.key...), nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, verifier.config.JWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create JWKS request: %w", err)
	}
	response, err := verifier.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request JWKS: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil, fmt.Errorf("JWKS returned %s", response.Status)
	}

	var document keySet
	decoder := json.NewDecoder(io.LimitReader(response.Body, maximumJWKSSize))
	if err = decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("JWKS must contain one JSON document")
	}

	for _, candidate := range document.Keys {
		if candidate.KeyID != verifier.config.KeyID || candidate.KeyType != "OKP" ||
			candidate.Curve != "Ed25519" || candidate.Use != "sig" ||
			candidate.Algorithm != "EdDSA" {
			continue
		}
		decoded, decodeErr := base64.RawURLEncoding.Strict().DecodeString(candidate.X)
		if decodeErr != nil || len(decoded) != ed25519.PublicKeySize {
			return nil, errors.New("JWKS contains an invalid Ed25519 public key")
		}
		verifier.key = append(ed25519.PublicKey(nil), decoded...)
		verifier.refreshAt = now.Add(verifier.config.RefreshInterval)
		return append(ed25519.PublicKey(nil), verifier.key...), nil
	}

	return nil, fmt.Errorf("JWKS does not contain key %q", verifier.config.KeyID)
}
