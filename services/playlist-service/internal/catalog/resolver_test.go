package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/playlist-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

func TestDynamicResolverMapsRulesAndTrackPage(t *testing.T) {
	trackOne := uuid.New()
	trackTwo := uuid.New()
	artistID := uuid.New()
	genreIDs := []uuid.UUID{uuid.New(), uuid.New()}
	client := &filterClientStub{response: &catalogv1.EvaluateTrackFiltersResponse{
		Tracks: []*catalogv1.Track{{Id: trackOne.String()}, {Id: trackTwo.String()}},
		Page:   &catalogv1.Page{Limit: 10, Offset: 5, Total: 9},
	}}
	resolver := NewDynamicResolver(client)
	ruleSetID := uuid.New()
	result, err := resolver.Resolve(context.Background(), rules.RuleSet{
		ID: ruleSetID, Logic: rules.LogicAnd,
		Conditions: []rules.Condition{
			{ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldTitle, Operator: rules.OperatorContains, Value: rules.TextValue("live"), Position: 1},
			{ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldArtistID, Operator: rules.OperatorEquals, Value: rules.UUIDValue(artistID), Position: 2},
			{ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldGenreID, Operator: rules.OperatorIn, Value: rules.UUIDListValue(genreIDs...), Position: 3},
			{ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldDurationMS, Operator: rules.OperatorBetween, Value: rules.IntegerRangeValue(120_000, 300_000), Position: 4},
			{ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldReleaseYear, Operator: rules.OperatorGreaterThanOrEqual, Value: rules.IntegerValue(2020), Position: 5},
			{ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldExplicit, Operator: rules.OperatorEquals, Value: rules.BooleanValue(false), Position: 6},
		},
	}, playlist.PageRequest{Limit: 10, Offset: 5})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Limit != 10 || result.Offset != 5 || result.Total != 9 ||
		len(result.Items) != 2 || result.Items[0] != (playlist.DynamicTrack{TrackID: trackOne, Position: 6}) ||
		result.Items[1] != (playlist.DynamicTrack{TrackID: trackTwo, Position: 7}) {
		t.Fatalf("Resolve() = %+v", result)
	}
	request := client.request
	if request.GetLogic() != catalogv1.FilterLogic_FILTER_LOGIC_AND ||
		request.GetSort() != catalogv1.TrackSort_TRACK_SORT_NEWEST ||
		request.GetLimit() != 10 || request.GetOffset() != 5 || len(request.GetFilters()) != 6 {
		t.Fatalf("EvaluateTrackFilters request = %+v", request)
	}
	if request.GetFilters()[0].GetValue().GetText() != "live" ||
		request.GetFilters()[1].GetValue().GetUuid() != artistID.String() ||
		!reflect.DeepEqual(request.GetFilters()[2].GetValue().GetUuidList().GetValues(),
			[]string{genreIDs[0].String(), genreIDs[1].String()}) ||
		request.GetFilters()[3].GetValue().GetIntegerRange().GetMinimum() != 120_000 ||
		request.GetFilters()[4].GetValue().GetInteger() != 2020 {
		t.Fatalf("EvaluateTrackFilters typed values = %+v", request.GetFilters())
	}
	booleanKind, ok := request.GetFilters()[5].GetValue().GetKind().(*catalogv1.TrackFilterValue_Boolean)
	if !ok || booleanKind.Boolean {
		t.Fatalf("explicit false oneof = %#v", request.GetFilters()[5].GetValue().GetKind())
	}
}

func TestDynamicResolverReturnsNonNilEmptyPage(t *testing.T) {
	client := &filterClientStub{response: &catalogv1.EvaluateTrackFiltersResponse{
		Tracks: []*catalogv1.Track{}, Page: &catalogv1.Page{Limit: 20},
	}}
	result, err := NewDynamicResolver(client).Resolve(
		context.Background(), singleConditionRuleSet(), playlist.PageRequest{Limit: 20},
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Items == nil || len(result.Items) != 0 || result.Total != 0 {
		t.Fatalf("Resolve() = %+v", result)
	}
}

func TestDynamicResolverDistinguishesTimeoutAndUnavailable(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		resolver := NewDynamicResolver(&filterClientStub{block: true})
		resolver.timeout = 10 * time.Millisecond
		_, err := resolver.Resolve(
			context.Background(), singleConditionRuleSet(), playlist.PageRequest{Limit: 20},
		)
		if !errors.Is(err, playlist.ErrCatalogTimeout) || errors.Is(err, playlist.ErrCatalogUnavailable) {
			t.Fatalf("Resolve() error = %v, want timeout only", err)
		}
	})

	t.Run("transport failure", func(t *testing.T) {
		resolver := NewDynamicResolver(&filterClientStub{err: errors.New("transport failure")})
		_, err := resolver.Resolve(
			context.Background(), singleConditionRuleSet(), playlist.PageRequest{Limit: 20},
		)
		if !errors.Is(err, playlist.ErrCatalogUnavailable) {
			t.Fatalf("Resolve() error = %v, want %v", err, playlist.ErrCatalogUnavailable)
		}
	})

	t.Run("caller cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resolver := NewDynamicResolver(&filterClientStub{block: true})
		_, err := resolver.Resolve(ctx, singleConditionRuleSet(), playlist.PageRequest{Limit: 20})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Resolve() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestDynamicResolverRejectsMalformedCatalogResponse(t *testing.T) {
	trackID := uuid.NewString()
	tests := []struct {
		name     string
		response *catalogv1.EvaluateTrackFiltersResponse
	}{
		{name: "nil response"},
		{name: "missing page", response: &catalogv1.EvaluateTrackFiltersResponse{}},
		{name: "wrong page", response: &catalogv1.EvaluateTrackFiltersResponse{
			Page: &catalogv1.Page{Limit: 99},
		}},
		{name: "invalid id", response: &catalogv1.EvaluateTrackFiltersResponse{
			Tracks: []*catalogv1.Track{{Id: "invalid"}}, Page: &catalogv1.Page{Limit: 20, Total: 1},
		}},
		{name: "duplicate id", response: &catalogv1.EvaluateTrackFiltersResponse{
			Tracks: []*catalogv1.Track{{Id: trackID}, {Id: trackID}}, Page: &catalogv1.Page{Limit: 20, Total: 2},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := NewDynamicResolver(&filterClientStub{response: test.response})
			_, err := resolver.Resolve(
				context.Background(), singleConditionRuleSet(), playlist.PageRequest{Limit: 20},
			)
			if !errors.Is(err, playlist.ErrCatalogUnavailable) {
				t.Fatalf("Resolve() error = %v, want %v", err, playlist.ErrCatalogUnavailable)
			}
		})
	}
}

func TestDynamicResolverRejectsUnsupportedStoredRules(t *testing.T) {
	tests := []rules.RuleSet{
		{Logic: "NOT", Conditions: singleConditionRuleSet().Conditions},
		{Logic: rules.LogicAnd},
		{Logic: rules.LogicAnd, Conditions: []rules.Condition{{
			Field: "unsafe", Operator: rules.OperatorEquals, Value: rules.TextValue("x"),
		}}},
	}
	for _, ruleSet := range tests {
		_, err := NewDynamicResolver(&filterClientStub{}).Resolve(
			context.Background(), ruleSet, playlist.PageRequest{Limit: 20},
		)
		if !errors.Is(err, rules.ErrInvalid) {
			t.Fatalf("Resolve(%+v) error = %v, want %v", ruleSet, err, rules.ErrInvalid)
		}
	}
}

func TestDynamicResolverMapsEveryRuleFieldAndOperator(t *testing.T) {
	fields := map[rules.Field]catalogv1.TrackFilterField{
		rules.FieldTitle:       catalogv1.TrackFilterField_TRACK_FILTER_FIELD_TITLE,
		rules.FieldArtistID:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ARTIST_ID,
		rules.FieldAlbumID:     catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ALBUM_ID,
		rules.FieldGenreID:     catalogv1.TrackFilterField_TRACK_FILTER_FIELD_GENRE_ID,
		rules.FieldMoodID:      catalogv1.TrackFilterField_TRACK_FILTER_FIELD_MOOD_ID,
		rules.FieldReleaseYear: catalogv1.TrackFilterField_TRACK_FILTER_FIELD_RELEASE_YEAR,
		rules.FieldDurationMS:  catalogv1.TrackFilterField_TRACK_FILTER_FIELD_DURATION_MS,
		rules.FieldExplicit:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_EXPLICIT,
	}
	for input, want := range fields {
		if result, err := filterField(input); err != nil || result != want {
			t.Fatalf("filterField(%q) = %v, %v; want %v", input, result, err, want)
		}
	}

	operators := map[rules.Operator]catalogv1.TrackFilterOperator{
		rules.OperatorEquals:             catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS,
		rules.OperatorContains:           catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_CONTAINS,
		rules.OperatorIn:                 catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_IN,
		rules.OperatorGreaterThan:        catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_GREATER_THAN,
		rules.OperatorGreaterThanOrEqual: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_GREATER_THAN_OR_EQUAL,
		rules.OperatorLessThan:           catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_LESS_THAN,
		rules.OperatorLessThanOrEqual:    catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_LESS_THAN_OR_EQUAL,
		rules.OperatorBetween:            catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_BETWEEN,
	}
	for input, want := range operators {
		if result, err := filterOperator(input); err != nil || result != want {
			t.Fatalf("filterOperator(%q) = %v, %v; want %v", input, result, err, want)
		}
	}
}

func singleConditionRuleSet() rules.RuleSet {
	return rules.RuleSet{
		Logic: rules.LogicAnd,
		Conditions: []rules.Condition{{
			Field: rules.FieldReleaseYear, Operator: rules.OperatorGreaterThanOrEqual,
			Value: rules.IntegerValue(2020), Position: 1,
		}},
	}
}

type filterClientStub struct {
	request  *catalogv1.EvaluateTrackFiltersRequest
	response *catalogv1.EvaluateTrackFiltersResponse
	err      error
	block    bool
}

func (stub *filterClientStub) EvaluateTrackFilters(
	ctx context.Context,
	request *catalogv1.EvaluateTrackFiltersRequest,
	_ ...grpc.CallOption,
) (*catalogv1.EvaluateTrackFiltersResponse, error) {
	stub.request = request
	if stub.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return stub.response, stub.err
}
