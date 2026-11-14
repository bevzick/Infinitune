//go:build integration

package grpcapi

import (
	"context"
	"os"
	"testing"
	"time"

	mediav1 "github.com/bevzick/Infinitune/services/media-service/api/media/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestMediaGRPCTransportReturnsMissingAsset(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	target := os.Getenv("MEDIA_GRPC_TEST_TARGET")
	if target == "" {
		t.Fatal("MEDIA_GRPC_TEST_TARGET is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect to media gRPC: %v", err)
	}
	defer connection.Close()

	_, err = mediav1.NewMediaServiceClient(connection).GetMediaAsset(ctx,
		&mediav1.GetMediaAssetRequest{MediaAssetId: uuid.NewString()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetMediaAsset() code = %v, want %v", status.Code(err), codes.NotFound)
	}
}
