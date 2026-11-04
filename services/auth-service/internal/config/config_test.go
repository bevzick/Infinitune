package config

import (
	"net/http"
	"slices"
	"testing"
)

func TestParseAllowedOrigins(t *testing.T) {
	t.Parallel()

	origins, err := parseAllowedOrigins(
		"https://app.example.com, http://localhost:5173, https://app.example.com",
	)
	if err != nil {
		t.Fatalf("parseAllowedOrigins() error = %v", err)
	}
	want := []string{"https://app.example.com", "http://localhost:5173"}
	if !slices.Equal(origins, want) {
		t.Fatalf("parseAllowedOrigins() = %v, want %v", origins, want)
	}
}

func TestParseAllowedOriginsRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	values := []string{
		"javascript:alert(1)",
		"https://user@example.com",
		"https://example.com/path",
		"https://example.com?query=value",
		"https://example.com#fragment",
		"example.com",
	}
	for _, value := range values {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := parseAllowedOrigins(value); err == nil {
				t.Fatal("parseAllowedOrigins() error = nil")
			}
		})
	}
}

func TestLoadSessionTransportConfig(t *testing.T) {
	t.Setenv("AUTH_SESSION_COOKIE_SECURE", "true")
	t.Setenv("AUTH_SESSION_COOKIE_SAME_SITE", "none")
	t.Setenv("AUTH_ALLOWED_ORIGINS", "https://app.example.com")

	cookie, origins, err := loadSessionTransportConfig()
	if err != nil {
		t.Fatalf("loadSessionTransportConfig() error = %v", err)
	}
	if !cookie.Secure || cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("cookie config = %+v", cookie)
	}
	if !slices.Equal(origins, []string{"https://app.example.com"}) {
		t.Fatalf("origins = %v", origins)
	}
}

func TestLoadSessionTransportConfigRejectsInsecureSameSiteNone(t *testing.T) {
	t.Setenv("AUTH_SESSION_COOKIE_SECURE", "false")
	t.Setenv("AUTH_SESSION_COOKIE_SAME_SITE", "none")
	t.Setenv("AUTH_ALLOWED_ORIGINS", "")

	if _, _, err := loadSessionTransportConfig(); err == nil {
		t.Fatal("loadSessionTransportConfig() error = nil")
	}
}

func TestParsePort(t *testing.T) {
	t.Parallel()

	if port, err := parsePort("PORT", "8080"); err != nil || port != 8080 {
		t.Fatalf("parsePort(8080) = %d, %v", port, err)
	}
	for _, value := range []string{"0", "65536", "invalid"} {
		if _, err := parsePort("PORT", value); err == nil {
			t.Fatalf("parsePort(%q) error = nil", value)
		}
	}
}
