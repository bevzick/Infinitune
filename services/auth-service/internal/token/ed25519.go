package token

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
)

const (
	minimumAccessTokenTTL = time.Minute
	maximumAccessTokenTTL = time.Hour
)

type Config struct {
	Issuer           string
	Audience         string
	KeyID            string
	AccessTokenTTL   time.Duration
	PrivateKeyBase64 string
}

type Ed25519Issuer struct {
	issuer         string
	audience       string
	keyID          string
	accessTokenTTL time.Duration
	privateKey     ed25519.PrivateKey
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type claims struct {
	Subject   string   `json:"sub"`
	Issuer    string   `json:"iss"`
	Audience  string   `json:"aud"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	TokenID   string   `json:"jti"`
	Roles     []string `json:"roles"`
}

func NewEd25519Issuer(config Config) (*Ed25519Issuer, error) {
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("JWT issuer is required")
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, errors.New("JWT audience is required")
	}
	if strings.TrimSpace(config.KeyID) == "" {
		return nil, errors.New("JWT key id is required")
	}
	if config.AccessTokenTTL < minimumAccessTokenTTL || config.AccessTokenTTL > maximumAccessTokenTTL {
		return nil, fmt.Errorf(
			"JWT access token TTL must be between %s and %s",
			minimumAccessTokenTTL,
			maximumAccessTokenTTL,
		)
	}

	privateKey, err := parsePrivateKey(config.PrivateKeyBase64)
	if err != nil {
		return nil, err
	}

	return &Ed25519Issuer{
		issuer:         strings.TrimSpace(config.Issuer),
		audience:       strings.TrimSpace(config.Audience),
		keyID:          strings.TrimSpace(config.KeyID),
		accessTokenTTL: config.AccessTokenTTL,
		privateKey:     privateKey,
	}, nil
}

func (i *Ed25519Issuer) IssueAccessToken(
	userID uuid.UUID,
	roles []string,
	issuedAt time.Time,
) (auth.AccessToken, error) {
	issuedAt = issuedAt.UTC().Truncate(time.Second)

	tokenID, err := uuid.NewRandom()
	if err != nil {
		return auth.AccessToken{}, fmt.Errorf("generate JWT id: %w", err)
	}

	expiresAt := issuedAt.Add(i.accessTokenTTL)
	headerJSON, err := json.Marshal(header{
		Algorithm: "EdDSA",
		Type:      "JWT",
		KeyID:     i.keyID,
	})
	if err != nil {
		return auth.AccessToken{}, fmt.Errorf("encode JWT header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims{
		Subject:   userID.String(),
		Issuer:    i.issuer,
		Audience:  i.audience,
		IssuedAt:  issuedAt.Unix(),
		ExpiresAt: expiresAt.Unix(),
		TokenID:   tokenID.String(),
		Roles:     append([]string(nil), roles...),
	})
	if err != nil {
		return auth.AccessToken{}, fmt.Errorf("encode JWT claims: %w", err)
	}

	encoding := base64.RawURLEncoding
	signingInput := encoding.EncodeToString(headerJSON) + "." + encoding.EncodeToString(claimsJSON)
	signature := ed25519.Sign(i.privateKey, []byte(signingInput))

	return auth.AccessToken{
		Value:     signingInput + "." + encoding.EncodeToString(signature),
		Type:      "Bearer",
		ExpiresAt: expiresAt,
	}, nil
}

func (i *Ed25519Issuer) PublicKey() ed25519.PublicKey {
	publicKey := i.privateKey.Public().(ed25519.PublicKey)
	return append(ed25519.PublicKey(nil), publicKey...)
}

func (i *Ed25519Issuer) KeyID() string {
	return i.keyID
}

func parsePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	keyDER, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("decode JWT private key: expected strict Base64")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(keyDER)
	if err != nil {
		return nil, errors.New("parse JWT private key: expected PKCS#8 DER")
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("parse JWT private key: expected Ed25519 key")
	}

	return append(ed25519.PrivateKey(nil), privateKey...), nil
}
