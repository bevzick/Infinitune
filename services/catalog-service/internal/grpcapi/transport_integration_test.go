//go:build integration

package grpcapi

import (
	"context"
	"os"
	"testing"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/catalog-service/api/catalog/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCatalogServiceInternalTransport(t *testing.T) {
	target := os.Getenv("CATALOG_GRPC_TEST_TARGET")
	if target == "" {
		t.Skip("set CATALOG_GRPC_TEST_TARGET to run the live gRPC integration test")
	}

	connection, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create Catalog gRPC client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := catalogv1.NewCatalogServiceClient(connection)
	missingID := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.ValidateTrackIds(ctx, &catalogv1.ValidateTrackIdsRequest{
		TrackIds: []string{missingID},
	})
	if err != nil {
		t.Fatalf("ValidateTrackIds() error = %v", err)
	}
	if len(response.GetValidTrackIds()) != 0 || len(response.GetMissingTrackIds()) != 1 ||
		response.GetMissingTrackIds()[0] != missingID {
		t.Fatalf("ValidateTrackIds() = %+v", response)
	}

	filtered, err := client.EvaluateTrackFilters(ctx, &catalogv1.EvaluateTrackFiltersRequest{
		Logic: catalogv1.FilterLogic_FILTER_LOGIC_AND,
		Filters: []*catalogv1.TrackFilter{{
			Field:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_TITLE,
			Operator: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS,
			Value: &catalogv1.TrackFilterValue{Kind: &catalogv1.TrackFilterValue_Text{
				Text: "missing-" + missingID,
			}},
		}},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("EvaluateTrackFilters() error = %v", err)
	}
	if len(filtered.GetTracks()) != 0 || filtered.GetPage() == nil ||
		filtered.GetPage().GetTotal() != 0 || filtered.GetPage().GetLimit() != 10 {
		t.Fatalf("EvaluateTrackFilters() = %+v", filtered)
	}
}
