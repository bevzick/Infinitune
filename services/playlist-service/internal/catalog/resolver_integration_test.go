//go:build integration

package catalog

import (
	"context"
	"os"
	"testing"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/playlist-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestDynamicResolverCatalogTransport(t *testing.T) {
	target := os.Getenv("CATALOG_GRPC_TEST_TARGET")
	if target == "" {
		t.Skip("set CATALOG_GRPC_TEST_TARGET to run the live Catalog gRPC integration test")
	}
	connection, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create Catalog gRPC client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	resolver := NewDynamicResolver(catalogv1.NewCatalogServiceClient(connection))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := resolver.Resolve(ctx, rules.RuleSet{
		Logic: rules.LogicAnd,
		Conditions: []rules.Condition{{
			Field: rules.FieldTitle, Operator: rules.OperatorEquals,
			Value: rules.TextValue("First Light Engine"), Position: 1,
		}},
	}, playlist.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Resolve() seeded track error = %v", err)
	}
	wantTrackID := uuid.MustParse("30000000-0000-4000-8000-000000000001")
	if result.Total != 1 || result.Limit != 10 || len(result.Items) != 1 ||
		result.Items[0].TrackID != wantTrackID || result.Items[0].Position != 1 {
		t.Fatalf("Resolve() seeded track = %+v", result)
	}

	empty, err := resolver.Resolve(ctx, rules.RuleSet{
		Logic: rules.LogicAnd,
		Conditions: []rules.Condition{{
			Field: rules.FieldTitle, Operator: rules.OperatorEquals,
			Value: rules.TextValue("missing-" + uuid.NewString()), Position: 1,
		}},
	}, playlist.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Resolve() empty result error = %v", err)
	}
	if empty.Items == nil || len(empty.Items) != 0 || empty.Total != 0 || empty.Limit != 10 {
		t.Fatalf("Resolve() empty result = %+v", empty)
	}
}
