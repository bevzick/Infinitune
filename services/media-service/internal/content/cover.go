package content

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

const coverRPCPrefix = "/infinitune.content.cover.v1.CoverUploadService/"

type CoverSession struct {
	ID, ReleaseID, RequestedByUserID uuid.UUID
	ObjectKey, MIMEType              string
	SizeBytes                        int64
	Status                           Status
}

type CoverClient struct{ conn grpc.ClientConnInterface }

func NewCoverClient(conn grpc.ClientConnInterface) *CoverClient { return &CoverClient{conn: conn} }

func (c *CoverClient) Session(ctx context.Context, actorID, uploadID uuid.UUID) (CoverSession, error) {
	return c.call(ctx, "GetSession", map[string]any{"user_id": actorID.String(), "upload_id": uploadID.String()})
}
func (c *CoverClient) Transition(ctx context.Context, uploadID uuid.UUID, next Status) (CoverSession, error) {
	return c.call(ctx, "Transition", map[string]any{"upload_id": uploadID.String(), "target_status": string(next)})
}
func (c *CoverClient) Complete(ctx context.Context, uploadID uuid.UUID) (CoverSession, error) {
	return c.call(ctx, "Complete", map[string]any{"upload_id": uploadID.String()})
}
func (c *CoverClient) call(parent context.Context, method string, data map[string]any) (CoverSession, error) {
	req, err := structpb.NewStruct(data)
	if err != nil {
		return CoverSession{}, fmt.Errorf("serialize cover request: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	result := new(structpb.Struct)
	if err := c.conn.Invoke(ctx, coverRPCPrefix+method, req, result); err != nil {
		return CoverSession{}, mapError(parent, err)
	}
	id, errID := parseCoverID(result, "id")
	releaseID, errRelease := parseCoverID(result, "release_id")
	userID, errUser := parseCoverID(result, "requested_by_user_id")
	if errID != nil || errRelease != nil || errUser != nil {
		return CoverSession{}, ErrUnavailable
	}
	objectKey := coverString(result, "object_key")
	mime := coverString(result, "mime_type")
	size := result.GetFields()["size_bytes"].GetNumberValue()
	status := Status(coverString(result, "status"))
	if strings.TrimSpace(objectKey) == "" || !strings.HasPrefix(objectKey, "creator-content/") ||
		size < 1 || size > 10*1024*1024 || float64(int64(size)) != size ||
		(mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") ||
		(status != StatusPending && status != StatusUploading && status != StatusCompleted && status != StatusFailed) {
		return CoverSession{}, fmt.Errorf("malformed cover session response: %w", ErrUnavailable)
	}
	return CoverSession{ID: id, ReleaseID: releaseID, RequestedByUserID: userID, ObjectKey: objectKey, MIMEType: mime, SizeBytes: int64(size), Status: status}, nil
}
func coverString(data *structpb.Struct, name string) string {
	if data == nil || data.Fields == nil || data.Fields[name] == nil {
		return ""
	}
	return data.Fields[name].GetStringValue()
}
func parseCoverID(data *structpb.Struct, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(coverString(data, name))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, ErrUnavailable
	}
	return id, nil
}
