package coverrpc

import (
	"context"
	"errors"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const ServiceName = "infinitune.content.cover.v1.CoverUploadService"

const (
	GetSessionMethod = "/" + ServiceName + "/GetSession"
	TransitionMethod = "/" + ServiceName + "/Transition"
	CompleteMethod   = "/" + ServiceName + "/Complete"
)

type Sessions interface {
	SessionForUser(context.Context, uuid.UUID, uuid.UUID) (coverupload.Session, error)
	Transition(context.Context, uuid.UUID, coverupload.Status) (coverupload.Session, error)
	Complete(context.Context, uuid.UUID) (coverupload.Session, error)
}

type Server struct{ sessions Sessions }

func Register(registrar grpc.ServiceRegistrar, sessions Sessions) {
	registrar.RegisterService(&grpc.ServiceDesc{
		ServiceName: ServiceName,
		HandlerType: (*api)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "GetSession", Handler: serveGetSession},
			{MethodName: "Transition", Handler: serveTransition},
			{MethodName: "Complete", Handler: serveComplete},
		},
		Metadata: "api/cover/v1/cover.proto",
	}, &Server{sessions: sessions})
}

type api interface {
	GetSession(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Transition(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Complete(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

func serveGetSession(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	return dispatch(srv, ctx, dec, interceptor, GetSessionMethod, func(ctx context.Context, req *structpb.Struct) (*structpb.Struct, error) {
		return srv.(api).GetSession(ctx, req)
	})
}
func serveTransition(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	return dispatch(srv, ctx, dec, interceptor, TransitionMethod, func(ctx context.Context, req *structpb.Struct) (*structpb.Struct, error) {
		return srv.(api).Transition(ctx, req)
	})
}
func serveComplete(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	return dispatch(srv, ctx, dec, interceptor, CompleteMethod, func(ctx context.Context, req *structpb.Struct) (*structpb.Struct, error) {
		return srv.(api).Complete(ctx, req)
	})
}
func dispatch(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor,
	method string, call func(context.Context, *structpb.Struct) (*structpb.Struct, error)) (any, error) {
	req := new(structpb.Struct)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return call(ctx, req)
	}
	return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: method},
		func(ctx context.Context, value any) (any, error) { return call(ctx, value.(*structpb.Struct)) })
}

func field(req *structpb.Struct, name string) string {
	if req == nil || req.Fields == nil {
		return ""
	}
	value := req.Fields[name]
	if value == nil {
		return ""
	}
	return value.GetStringValue()
}
func uuidField(req *structpb.Struct, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(field(req, name))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a UUID", name)
	}
	return id, nil
}

func (s *Server) GetSession(ctx context.Context, request *structpb.Struct) (*structpb.Struct, error) {
	uploadID, err := uuidField(request, "upload_id")
	if err != nil {
		return nil, err
	}
	userID, err := uuidField(request, "user_id")
	if err != nil {
		return nil, err
	}
	session, err := s.sessions.SessionForUser(ctx, userID, uploadID)
	if err != nil {
		return nil, mapError(err)
	}
	return encode(session)
}
func (s *Server) Transition(ctx context.Context, request *structpb.Struct) (*structpb.Struct, error) {
	uploadID, err := uuidField(request, "upload_id")
	if err != nil {
		return nil, err
	}
	next := coverupload.Status(field(request, "target_status"))
	if next != coverupload.StatusUploading && next != coverupload.StatusFailed {
		return nil, status.Error(codes.InvalidArgument, "invalid target_status")
	}
	session, err := s.sessions.Transition(ctx, uploadID, next)
	if err != nil {
		return nil, mapError(err)
	}
	return encode(session)
}
func (s *Server) Complete(ctx context.Context, request *structpb.Struct) (*structpb.Struct, error) {
	uploadID, err := uuidField(request, "upload_id")
	if err != nil {
		return nil, err
	}
	session, err := s.sessions.Complete(ctx, uploadID)
	if err != nil {
		return nil, mapError(err)
	}
	return encode(session)
}
func encode(s coverupload.Session) (*structpb.Struct, error) {
	return structpb.NewStruct(map[string]any{
		"id": s.ID.String(), "release_id": s.ReleaseID.String(),
		"requested_by_user_id": s.RequestedByUserID.String(), "object_key": s.ObjectKey,
		"mime_type": s.MIMEType, "size_bytes": s.SizeBytes, "status": string(s.Status),
	})
}
func mapError(err error) error {
	switch {
	case errors.Is(err, creator.ErrForbidden):
		return status.Error(codes.PermissionDenied, "cover session not available")
	case errors.Is(err, coverupload.ErrInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, coverupload.ErrNotFound):
		return status.Error(codes.NotFound, "cover session not found")
	case errors.Is(err, coverupload.ErrReleaseLocked), errors.Is(err, coverupload.ErrInvalidTransition):
		return status.Error(codes.FailedPrecondition, "cover session state does not permit upload")
	case errors.Is(err, coverupload.ErrConflict):
		return status.Error(codes.Aborted, "cover session changed")
	default:
		return status.Error(codes.Internal, "cover session operation failed")
	}
}
