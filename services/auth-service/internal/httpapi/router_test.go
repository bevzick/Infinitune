package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/token"
	"github.com/google/uuid"
)

func TestIdentityUsesVerifiedTokenInsteadOfClientHeaders(t *testing.T) {
	verifiedUserID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	router, _, _ := newHTTPTestRouter(auth.Identity{
		UserID:    verifiedUserID,
		Roles:     []string{auth.RoleListener},
		TokenID:   uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		IssuedAt:  time.Now().Add(-time.Minute),
		ExpiresAt: time.Now().Add(time.Minute),
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/identity", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("X-User-ID", "99999999-9999-4999-8999-999999999999")
	request.Header.Set("X-User-Roles", auth.RoleAdmin)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET identity status = %d, body = %s", response.Code, response.Body.String())
	}
	var body identityResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode identity response: %v", err)
	}
	if body.UserID != verifiedUserID || !slices.Equal(body.Roles, []string{auth.RoleListener}) {
		t.Fatalf("GET identity body = %+v", body)
	}
}

func TestProtectedEndpointRejectsMissingOrInvalidToken(t *testing.T) {
	router, tokens, _ := newHTTPTestRouter(auth.Identity{})
	tokens.verifyError = errors.New("invalid token")

	tests := []struct {
		name          string
		authorization string
	}{
		{name: "missing"},
		{name: "invalid", authorization: "Bearer invalid-token"},
		{name: "wrong scheme", authorization: "Basic credentials"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v1/auth/identity", nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("WWW-Authenticate = %q", response.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestJWKSIsPublic(t *testing.T) {
	router, _, _ := newHTTPTestRouter(auth.Identity{})
	request := httptest.NewRequest(http.MethodGet, "/v1/auth/jwks", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET jwks status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"kid":"auth-key-1"`) {
		t.Fatalf("GET jwks body = %s", response.Body.String())
	}
}

func TestRoleEndpointsMapAuthorizationErrors(t *testing.T) {
	identity := auth.Identity{
		UserID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		Roles:  []string{auth.RoleListener},
	}
	router, _, roles := newHTTPTestRouter(identity)
	roles.err = auth.ErrForbidden
	targetID := "22222222-2222-4222-8222-222222222222"

	request := httptest.NewRequest(http.MethodPut, "/v1/auth/users/"+targetID+"/roles/creator", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("PUT role status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"code":"insufficient_permissions"`) {
		t.Fatalf("PUT role body = %s", response.Body.String())
	}
}

func TestRoleEndpointRejectsInvalidUserID(t *testing.T) {
	router, _, roles := newHTTPTestRouter(auth.Identity{
		UserID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		Roles:  []string{auth.RoleAdmin},
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/users/not-a-uuid/roles", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || roles.calls != 0 {
		t.Fatalf("GET roles status = %d, calls = %d, body = %s", response.Code, roles.calls, response.Body.String())
	}
}

func TestCORSAllowsAuthorizationAndRoleMethods(t *testing.T) {
	router, _, _ := newHTTPTestRouter(auth.Identity{})
	request := httptest.NewRequest(
		http.MethodOptions,
		"/v1/auth/users/22222222-2222-4222-8222-222222222222/roles/creator",
		nil,
	)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodPut)
	request.Header.Set("Access-Control-Request-Headers", "Authorization")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), http.MethodPut) ||
		!strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete) {
		t.Fatalf("Access-Control-Allow-Methods = %q", response.Header().Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("Access-Control-Allow-Headers = %q", response.Header().Get("Access-Control-Allow-Headers"))
	}
}

func newHTTPTestRouter(identity auth.Identity) (http.Handler, *identityTokensFake, *roleAssignmentsFake) {
	tokens := &identityTokensFake{identity: identity}
	roles := &roleAssignmentsFake{roles: []string{auth.RoleListener}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewRouter(Dependencies{
		Database:     healthCheckFake{},
		Redis:        healthCheckFake{},
		Registration: registrationFake{},
		Login:        loginFake{},
		Sessions:     sessionsFake{},
		Tokens:       tokens,
		Roles:        roles,
		Cookies:      SessionCookieConfig{SameSite: http.SameSiteLaxMode},
		Origins:      []string{"http://localhost:5173"},
		Logger:       logger,
	}), tokens, roles
}

type healthCheckFake struct{}

func (healthCheckFake) Ping(context.Context) error { return nil }

type registrationFake struct{}

func (registrationFake) Register(context.Context, auth.RegisterInput) (auth.RegistrationResult, error) {
	return auth.RegistrationResult{}, nil
}

type loginFake struct{}

func (loginFake) Login(context.Context, auth.LoginInput) (auth.SessionTokens, error) {
	return auth.SessionTokens{}, nil
}

type sessionsFake struct{}

func (sessionsFake) Refresh(context.Context, string) (auth.SessionTokens, error) {
	return auth.SessionTokens{}, nil
}

func (sessionsFake) Logout(context.Context, string) (bool, error) { return true, nil }

type identityTokensFake struct {
	identity    auth.Identity
	verifyError error
}

func (f *identityTokensFake) VerifyAccessToken(encoded string, _ time.Time) (auth.Identity, error) {
	if f.verifyError != nil {
		return auth.Identity{}, f.verifyError
	}
	if encoded != "valid-token" {
		return auth.Identity{}, errors.New("invalid token")
	}
	return f.identity, nil
}

func (f *identityTokensFake) JSONWebKeySet() token.JSONWebKeySet {
	return token.JSONWebKeySet{Keys: []token.JSONWebKey{{
		KeyType:   "OKP",
		Curve:     "Ed25519",
		Use:       "sig",
		Algorithm: "EdDSA",
		KeyID:     "auth-key-1",
		X:         "public-key",
	}}}
}

type roleAssignmentsFake struct {
	roles []string
	err   error
	calls int
}

func (f *roleAssignmentsFake) ListUserRoles(
	context.Context,
	auth.Identity,
	uuid.UUID,
) ([]string, error) {
	f.calls++
	return append([]string(nil), f.roles...), f.err
}

func (f *roleAssignmentsFake) AssignRole(
	context.Context,
	auth.Identity,
	uuid.UUID,
	string,
) ([]string, error) {
	f.calls++
	return append([]string(nil), f.roles...), f.err
}

func (f *roleAssignmentsFake) RemoveRole(
	context.Context,
	auth.Identity,
	uuid.UUID,
	string,
) ([]string, error) {
	f.calls++
	return append([]string(nil), f.roles...), f.err
}
