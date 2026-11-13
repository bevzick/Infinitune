package correlation

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryClientInterceptorPropagatesCorrelationID(t *testing.T) {
	ctx := WithID(context.Background(), "request-42")
	interceptor := UnaryClientInterceptor()
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			outgoing, _ := metadata.FromOutgoingContext(ctx)
			values := outgoing.Get(MetadataKey)
			if len(values) != 1 || values[0] != "request-42" {
				t.Fatalf("outgoing %s = %v", MetadataKey, values)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
}

func TestUnaryServerInterceptorReadsCorrelationIDAndLogsIt(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	interceptor := UnaryServerInterceptor(logger)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, "request-84"))

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		func(ctx context.Context, _ any) (any, error) {
			if got := ID(ctx); got != "request-84" {
				t.Fatalf("correlation ID = %q, want request-84", got)
			}
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	for _, expected := range []string{
		"\"event\":\"grpc_request\"",
		"\"request_id\":\"request-84\"",
		"\"method\":\"/test.Service/Method\"",
	} {
		if !strings.Contains(logs.String(), expected) {
			t.Fatalf("log output %q does not contain %q", logs.String(), expected)
		}
	}
}

func TestNormalizeReplacesUnsafeID(t *testing.T) {
	id := Normalize("unsafe request id")
	if !Valid(id) || id == "unsafe request id" {
		t.Fatalf("Normalize returned %q", id)
	}
}
