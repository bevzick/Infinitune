package correlation

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	MetadataKey  = "x-request-id"
	maximumIDLen = 128
)

type contextKey struct{}

func WithID(ctx context.Context, id string) context.Context {
	if !Valid(id) {
		id = uuid.NewString()
	}
	return context.WithValue(ctx, contextKey{}, id)
}

func ID(ctx context.Context) string {
	value, _ := ctx.Value(contextKey{}).(string)
	return value
}

func Normalize(value string) string {
	if Valid(value) {
		return value
	}
	return uuid.NewString()
}

func Valid(value string) bool {
	if value == "" || len(value) > maximumIDLen {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' ||
			character == '.' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		request, reply any,
		connection *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		options ...grpc.CallOption,
	) error {
		id := ID(ctx)
		if id == "" {
			id = uuid.NewString()
			ctx = WithID(ctx, id)
		}
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataKey, id)
		return invoker(ctx, method, request, reply, connection, options...)
	}
}

func UnaryServerInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		id := incomingID(ctx)
		ctx = WithID(ctx, id)
		startedAt := time.Now()
		response, err := handler(ctx, request)
		code := status.Code(err)
		attributes := []any{
			"event", "grpc_request",
			"method", info.FullMethod,
			"status", code.String(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
		}
		requestLogger := logger.With("request_id", id)
		switch {
		case err == nil:
			requestLogger.Info("gRPC request completed", attributes...)
		case code == codes.Internal || code == codes.Unavailable || code == codes.Unknown:
			requestLogger.Error("gRPC request completed", append(attributes, "error", err)...)
		default:
			requestLogger.Warn("gRPC request completed", append(attributes, "error", err)...)
		}
		return response, err
	}
}

func incomingID(ctx context.Context) string {
	values := metadata.ValueFromIncomingContext(ctx, MetadataKey)
	if len(values) == 1 && Valid(strings.TrimSpace(values[0])) {
		return strings.TrimSpace(values[0])
	}
	return uuid.NewString()
}
