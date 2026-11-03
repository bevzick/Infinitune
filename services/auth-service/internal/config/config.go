package config

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/database"
	"github.com/bevzick/Infinitune/services/auth-service/internal/password"
	"github.com/bevzick/Infinitune/services/auth-service/internal/token"
)

const (
	defaultHTTPHost     = "0.0.0.0"
	defaultHTTPPort     = "8080"
	defaultPostgresHost = "127.0.0.1"
	defaultPostgresPort = "5432"
	defaultRedisHost    = "127.0.0.1"
	defaultRedisPort    = "6379"
	defaultJWTIssuer    = "infinitune-auth"
	defaultJWTAudience  = "infinitune-api"
	defaultJWTKeyID     = "auth-key-1"
	defaultJWTAccessTTL = 15 * time.Minute
	defaultRefreshTTL   = 30 * 24 * time.Hour
	minimumRefreshTTL   = time.Hour
	maximumRefreshTTL   = 90 * 24 * time.Hour
	defaultCookieSecure = true
	defaultCookiePolicy = "lax"
)

type SessionCookieConfig struct {
	Secure   bool
	SameSite http.SameSite
}

type Config struct {
	HTTPAddress string
	Database    database.Config
	Redis       database.RedisConfig
	Password    password.Params
	Token       token.Config
	RefreshTTL  time.Duration
	Cookie      SessionCookieConfig
	Origins     []string
}

func Load() (Config, error) {
	host := envOrDefault("AUTH_HTTP_HOST", defaultHTTPHost)
	port := envOrDefault("AUTH_HTTP_PORT", defaultHTTPPort)

	if _, err := parsePort("AUTH_HTTP_PORT", port); err != nil {
		return Config{}, err
	}

	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	redisConfig, err := loadRedisConfig()
	if err != nil {
		return Config{}, err
	}
	passwordParams, err := loadPasswordParams()
	if err != nil {
		return Config{}, err
	}
	tokenConfig, err := loadTokenConfig()
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := loadRefreshTokenTTL()
	if err != nil {
		return Config{}, err
	}
	cookieConfig, origins, err := loadSessionTransportConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress: net.JoinHostPort(host, port),
		Database:    databaseConfig,
		Redis:       redisConfig,
		Password:    passwordParams,
		Token:       tokenConfig,
		RefreshTTL:  refreshTTL,
		Cookie:      cookieConfig,
		Origins:     origins,
	}, nil
}

func loadSessionTransportConfig() (SessionCookieConfig, []string, error) {
	secureValue := envOrDefault("AUTH_SESSION_COOKIE_SECURE", strconv.FormatBool(defaultCookieSecure))
	secure, err := strconv.ParseBool(secureValue)
	if err != nil {
		return SessionCookieConfig{}, nil, fmt.Errorf(
			"invalid AUTH_SESSION_COOKIE_SECURE %q: expected a boolean",
			secureValue,
		)
	}

	sameSiteValue := strings.ToLower(envOrDefault("AUTH_SESSION_COOKIE_SAME_SITE", defaultCookiePolicy))
	var sameSite http.SameSite
	switch sameSiteValue {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "lax":
		sameSite = http.SameSiteLaxMode
	case "none":
		if !secure {
			return SessionCookieConfig{}, nil, errors.New("SameSite=None requires AUTH_SESSION_COOKIE_SECURE=true")
		}
		sameSite = http.SameSiteNoneMode
	default:
		return SessionCookieConfig{}, nil, fmt.Errorf(
			"invalid AUTH_SESSION_COOKIE_SAME_SITE %q: expected strict, lax, or none",
			sameSiteValue,
		)
	}

	origins, err := parseAllowedOrigins(envOrDefault("AUTH_ALLOWED_ORIGINS", ""))
	if err != nil {
		return SessionCookieConfig{}, nil, err
	}

	return SessionCookieConfig{Secure: secure, SameSite: sameSite}, origins, nil
}

func parseAllowedOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	origins := make([]string, 0)
	for rawOrigin := range strings.SplitSeq(value, ",") {
		origin := strings.TrimSpace(rawOrigin)
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid AUTH_ALLOWED_ORIGINS entry %q", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	return origins, nil
}

func loadRedisConfig() (database.RedisConfig, error) {
	port, err := parsePort("REDIS_PORT", envOrDefault("REDIS_PORT", defaultRedisPort))
	if err != nil {
		return database.RedisConfig{}, err
	}
	databaseNumber, err := parseUintEnv("REDIS_AUTH_DB", 0, 31)
	if err != nil {
		return database.RedisConfig{}, err
	}

	return database.RedisConfig{
		Host:     envOrDefault("REDIS_HOST", defaultRedisHost),
		Port:     port,
		Password: envOrDefault("REDIS_PASSWORD", ""),
		DB:       int(databaseNumber),
	}, nil
}

func loadRefreshTokenTTL() (time.Duration, error) {
	value := envOrDefault("AUTH_REFRESH_TOKEN_TTL", defaultRefreshTTL.String())
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid AUTH_REFRESH_TOKEN_TTL %q: %w", value, err)
	}
	if ttl < minimumRefreshTTL || ttl > maximumRefreshTTL {
		return 0, fmt.Errorf(
			"AUTH_REFRESH_TOKEN_TTL must be between %s and %s",
			minimumRefreshTTL,
			maximumRefreshTTL,
		)
	}

	return ttl, nil
}

func loadTokenConfig() (token.Config, error) {
	privateKey, err := requiredEnv("AUTH_JWT_PRIVATE_KEY_BASE64")
	if err != nil {
		return token.Config{}, err
	}

	ttlValue := envOrDefault("AUTH_JWT_ACCESS_TTL", defaultJWTAccessTTL.String())
	ttl, err := time.ParseDuration(ttlValue)
	if err != nil {
		return token.Config{}, fmt.Errorf("invalid AUTH_JWT_ACCESS_TTL %q: %w", ttlValue, err)
	}

	return token.Config{
		Issuer:           envOrDefault("AUTH_JWT_ISSUER", defaultJWTIssuer),
		Audience:         envOrDefault("AUTH_JWT_AUDIENCE", defaultJWTAudience),
		KeyID:            envOrDefault("AUTH_JWT_KEY_ID", defaultJWTKeyID),
		AccessTokenTTL:   ttl,
		PrivateKeyBase64: privateKey,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func loadDatabaseConfig() (database.Config, error) {
	port, err := parsePort("POSTGRES_PORT", envOrDefault("POSTGRES_PORT", defaultPostgresPort))
	if err != nil {
		return database.Config{}, err
	}

	user, err := requiredEnv("POSTGRES_USER")
	if err != nil {
		return database.Config{}, err
	}
	password, err := requiredEnv("POSTGRES_PASSWORD")
	if err != nil {
		return database.Config{}, err
	}
	name, err := requiredEnv("POSTGRES_AUTH_DB")
	if err != nil {
		return database.Config{}, err
	}

	return database.Config{
		Host:     envOrDefault("POSTGRES_HOST", defaultPostgresHost),
		Port:     port,
		User:     user,
		Password: password,
		Name:     name,
	}, nil
}

func requiredEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}

	return value, nil
}

func loadPasswordParams() (password.Params, error) {
	params := password.DefaultParams()

	memory, err := parseUintEnv("AUTH_PASSWORD_MEMORY_KIB", uint64(params.MemoryKiB), 32)
	if err != nil {
		return password.Params{}, err
	}
	iterations, err := parseUintEnv("AUTH_PASSWORD_ITERATIONS", uint64(params.Iterations), 32)
	if err != nil {
		return password.Params{}, err
	}
	parallelism, err := parseUintEnv("AUTH_PASSWORD_PARALLELISM", uint64(params.Parallelism), 8)
	if err != nil {
		return password.Params{}, err
	}

	params.MemoryKiB = uint32(memory)
	params.Iterations = uint32(iterations)
	params.Parallelism = uint8(parallelism)
	if err = params.Validate(); err != nil {
		return password.Params{}, fmt.Errorf("invalid password hashing configuration: %w", err)
	}

	return params, nil
}

func parseUintEnv(key string, fallback uint64, bitSize int) (uint64, error) {
	value := envOrDefault(key, strconv.FormatUint(fallback, 10))
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: expected an unsigned integer", key, value)
	}

	return parsed, nil
}

func parsePort(key, port string) (uint16, error) {
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return 0, fmt.Errorf("invalid %s %q: expected a number from 1 to 65535", key, port)
	}

	return uint16(portNumber), nil
}
