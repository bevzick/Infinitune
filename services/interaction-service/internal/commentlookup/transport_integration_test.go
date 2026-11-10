//go:build integration

package commentlookup

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	commentv1 "github.com/bevzick/Infinitune/services/interaction-service/api/comment/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCommentServiceTransportTreatsUnknownCommentAsUnavailable(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	address := net.JoinHostPort(envOrDefault("COMMENT_GRPC_HOST", "127.0.0.1"),
		envOrDefault("COMMENT_GRPC_PORT", "9090"))
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("configure Comment gRPC client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = NewValidator(commentv1.NewCommentServiceClient(connection)).ValidateComment(ctx, uuid.New())
	if !errors.Is(err, interaction.ErrCommentNotFound) {
		t.Fatalf("ValidateComment(unknown) error = %v, want %v", err, interaction.ErrCommentNotFound)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
