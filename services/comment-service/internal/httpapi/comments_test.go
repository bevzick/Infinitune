package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/bevzick/Infinitune/services/comment-service/internal/identity"
	"github.com/google/uuid"
)

func TestCreateCommentUsesVerifiedIdentity(t *testing.T) {
	userID, trackID := uuid.New(), uuid.New()
	comments := &commentsStub{}
	router := NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, Comments: comments,
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/comments",
		strings.NewReader(`{"track_id":"`+trackID.String()+`","content":"Hello"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || comments.userID != userID || comments.input.TrackID != trackID {
		t.Fatalf("status = %d, owner = %s, input = %+v; body = %s", response.Code, comments.userID, comments.input, response.Body.String())
	}
}

func TestCreateCommentRejectsSpoofedAuthor(t *testing.T) {
	userID := uuid.New()
	comments := &commentsStub{}
	router := NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, Comments: comments,
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/comments", strings.NewReader(
		`{"track_id":"`+uuid.NewString()+`","user_id":"`+uuid.NewString()+`","content":"Spoof"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || comments.calls != 0 {
		t.Fatalf("status = %d, calls = %d; body = %s", response.Code, comments.calls, response.Body.String())
	}
}

func TestListCommentsIsPublicAndPaginated(t *testing.T) {
	trackID := uuid.New()
	comments := &commentsStub{}
	router := NewRouter(Dependencies{Database: databaseStub{}, Comments: comments})
	request := httptest.NewRequest(http.MethodGet,
		"/v1/tracks/"+trackID.String()+"/comments?limit=7&offset=2", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || comments.trackID != trackID ||
		comments.page != (comment.PageRequest{Limit: 7, Offset: 2}) {
		t.Fatalf("status = %d, track = %s, page = %+v; body = %s", response.Code, comments.trackID, comments.page, response.Body.String())
	}
}

func TestProtectedCommentMutationMapsOwnershipAndRateLimit(t *testing.T) {
	userID, commentID := uuid.New(), uuid.New()
	tests := []struct {
		name, method, path string
		domainErr          error
		want               int
	}{
		{name: "foreign edit", method: http.MethodPatch, path: "/v1/comments/" + commentID.String(), domainErr: comment.ErrForbidden, want: http.StatusForbidden},
		{name: "rate limit", method: http.MethodPost, path: "/v1/comments", domainErr: comment.ErrRateLimited, want: http.StatusTooManyRequests},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			comments := &commentsStub{err: test.domainErr}
			router := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, Comments: comments})
			body := `{"content":"Updated"}`
			if test.method == http.MethodPost {
				body = `{"track_id":"` + uuid.NewString() + `","content":"New"}`
			}
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

type tokenStub struct {
	identity identity.Identity
	err      error
}

func (stub tokenStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	if stub.err != nil {
		return identity.Identity{}, stub.err
	}
	if stub.identity.UserID == uuid.Nil {
		return identity.Identity{}, errors.New("identity unavailable")
	}
	return stub.identity, nil
}

type commentsStub struct {
	userID    uuid.UUID
	trackID   uuid.UUID
	commentID uuid.UUID
	input     comment.CreateInput
	page      comment.PageRequest
	err       error
	calls     int
}

func (stub *commentsStub) Create(_ context.Context, userID uuid.UUID, input comment.CreateInput) (comment.Comment, error) {
	stub.userID, stub.trackID, stub.input = userID, input.TrackID, input
	stub.calls++
	return comment.Comment{ID: uuid.New(), UserID: userID, TrackID: input.TrackID, ParentID: input.ParentID,
		Content: input.Content, CreatedAt: time.Now(), UpdatedAt: time.Now()}, stub.err
}
func (stub *commentsStub) List(_ context.Context, trackID uuid.UUID, page comment.PageRequest) (comment.Page, error) {
	stub.trackID, stub.page = trackID, page
	stub.calls++
	return comment.Page{Items: []comment.Comment{}, Limit: page.Limit, Offset: page.Offset}, stub.err
}
func (stub *commentsStub) Update(_ context.Context, userID, id uuid.UUID, content string) (comment.Comment, error) {
	stub.userID, stub.commentID, stub.calls = userID, id, stub.calls+1
	return comment.Comment{ID: id, UserID: userID, Content: content, CreatedAt: time.Now(), UpdatedAt: time.Now()}, stub.err
}
func (stub *commentsStub) Delete(_ context.Context, userID, id uuid.UUID) error {
	stub.userID, stub.commentID, stub.calls = userID, id, stub.calls+1
	return stub.err
}
