package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/playlist-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const resolutionTimeout = 3 * time.Second

type filterClient interface {
	EvaluateTrackFilters(
		context.Context,
		*catalogv1.EvaluateTrackFiltersRequest,
		...grpc.CallOption,
	) (*catalogv1.EvaluateTrackFiltersResponse, error)
}

type DynamicResolver struct {
	client  filterClient
	timeout time.Duration
}

func NewDynamicResolver(client filterClient) *DynamicResolver {
	return &DynamicResolver{client: client, timeout: resolutionTimeout}
}

func (resolver *DynamicResolver) Resolve(
	ctx context.Context,
	ruleSet rules.RuleSet,
	page playlist.PageRequest,
) (playlist.DynamicTrackPage, error) {
	request, err := filterRequest(ruleSet, page)
	if err != nil {
		return playlist.DynamicTrackPage{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, resolver.timeout)
	defer cancel()
	response, err := resolver.client.EvaluateTrackFilters(callCtx, request)
	if err != nil {
		if ctx.Err() != nil {
			return playlist.DynamicTrackPage{}, ctx.Err()
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) ||
			status.Code(err) == codes.DeadlineExceeded {
			return playlist.DynamicTrackPage{}, fmt.Errorf(
				"%w: EvaluateTrackFilters", playlist.ErrCatalogTimeout,
			)
		}
		return playlist.DynamicTrackPage{}, fmt.Errorf(
			"%w: EvaluateTrackFilters: %v", playlist.ErrCatalogUnavailable, err,
		)
	}
	return dynamicTrackPage(response, page)
}

func filterRequest(
	ruleSet rules.RuleSet,
	page playlist.PageRequest,
) (*catalogv1.EvaluateTrackFiltersRequest, error) {
	logic, err := filterLogic(ruleSet.Logic)
	if err != nil {
		return nil, err
	}
	filters := make([]*catalogv1.TrackFilter, 0, len(ruleSet.Conditions))
	for _, condition := range ruleSet.Conditions {
		filter, mapErr := dynamicFilter(condition)
		if mapErr != nil {
			return nil, mapErr
		}
		filters = append(filters, filter)
	}
	if len(filters) == 0 {
		return nil, fmt.Errorf("dynamic rule set has no conditions: %w", rules.ErrInvalid)
	}
	return &catalogv1.EvaluateTrackFiltersRequest{
		Logic: logic, Filters: filters,
		Sort:  catalogv1.TrackSort_TRACK_SORT_NEWEST,
		Limit: int32(page.Limit), Offset: int32(page.Offset),
	}, nil
}

func filterLogic(value rules.LogicOperator) (catalogv1.FilterLogic, error) {
	switch value {
	case rules.LogicAnd:
		return catalogv1.FilterLogic_FILTER_LOGIC_AND, nil
	case rules.LogicOr:
		return catalogv1.FilterLogic_FILTER_LOGIC_OR, nil
	default:
		return catalogv1.FilterLogic_FILTER_LOGIC_UNSPECIFIED,
			fmt.Errorf("unsupported rule logic %q: %w", value, rules.ErrInvalid)
	}
}

func dynamicFilter(condition rules.Condition) (*catalogv1.TrackFilter, error) {
	if _, err := rules.EncodeValue(condition.Field, condition.Operator, condition.Value); err != nil {
		return nil, err
	}
	field, err := filterField(condition.Field)
	if err != nil {
		return nil, err
	}
	operator, err := filterOperator(condition.Operator)
	if err != nil {
		return nil, err
	}
	value, err := filterValue(condition.Value)
	if err != nil {
		return nil, err
	}
	return &catalogv1.TrackFilter{Field: field, Operator: operator, Value: value}, nil
}

func filterField(value rules.Field) (catalogv1.TrackFilterField, error) {
	switch value {
	case rules.FieldTitle:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_TITLE, nil
	case rules.FieldArtistID:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ARTIST_ID, nil
	case rules.FieldAlbumID:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ALBUM_ID, nil
	case rules.FieldGenreID:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_GENRE_ID, nil
	case rules.FieldMoodID:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_MOOD_ID, nil
	case rules.FieldReleaseYear:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_RELEASE_YEAR, nil
	case rules.FieldDurationMS:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_DURATION_MS, nil
	case rules.FieldExplicit:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_EXPLICIT, nil
	default:
		return catalogv1.TrackFilterField_TRACK_FILTER_FIELD_UNSPECIFIED,
			fmt.Errorf("unsupported rule field %q: %w", value, rules.ErrInvalid)
	}
}

func filterOperator(value rules.Operator) (catalogv1.TrackFilterOperator, error) {
	switch value {
	case rules.OperatorEquals:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS, nil
	case rules.OperatorContains:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_CONTAINS, nil
	case rules.OperatorIn:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_IN, nil
	case rules.OperatorGreaterThan:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_GREATER_THAN, nil
	case rules.OperatorGreaterThanOrEqual:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_GREATER_THAN_OR_EQUAL, nil
	case rules.OperatorLessThan:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_LESS_THAN, nil
	case rules.OperatorLessThanOrEqual:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_LESS_THAN_OR_EQUAL, nil
	case rules.OperatorBetween:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_BETWEEN, nil
	default:
		return catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_UNSPECIFIED,
			fmt.Errorf("unsupported rule operator %q: %w", value, rules.ErrInvalid)
	}
}

func filterValue(value rules.Value) (*catalogv1.TrackFilterValue, error) {
	result := &catalogv1.TrackFilterValue{}
	switch value.Kind {
	case rules.ValueText:
		result.Kind = &catalogv1.TrackFilterValue_Text{Text: value.Text}
	case rules.ValueUUID:
		result.Kind = &catalogv1.TrackFilterValue_Uuid{Uuid: value.UUID.String()}
	case rules.ValueUUIDList:
		values := make([]string, 0, len(value.UUIDs))
		for _, id := range value.UUIDs {
			values = append(values, id.String())
		}
		result.Kind = &catalogv1.TrackFilterValue_UuidList{
			UuidList: &catalogv1.UUIDList{Values: values},
		}
	case rules.ValueInteger:
		result.Kind = &catalogv1.TrackFilterValue_Integer{Integer: int32(value.Integer)}
	case rules.ValueIntegerRange:
		result.Kind = &catalogv1.TrackFilterValue_IntegerRange{
			IntegerRange: &catalogv1.IntegerRange{
				Minimum: int32(value.IntegerRange.Minimum),
				Maximum: int32(value.IntegerRange.Maximum),
			},
		}
	case rules.ValueBoolean:
		result.Kind = &catalogv1.TrackFilterValue_Boolean{Boolean: value.Boolean}
	default:
		return nil, fmt.Errorf("unsupported rule value %q: %w", value.Kind, rules.ErrInvalid)
	}
	return result, nil
}

func dynamicTrackPage(
	response *catalogv1.EvaluateTrackFiltersResponse,
	request playlist.PageRequest,
) (playlist.DynamicTrackPage, error) {
	if response == nil || response.GetPage() == nil {
		return playlist.DynamicTrackPage{}, malformedCatalogResponse("page is missing")
	}
	page := response.GetPage()
	if page.GetLimit() != int32(request.Limit) || page.GetOffset() != int32(request.Offset) ||
		page.GetTotal() < 0 || len(response.GetTracks()) > request.Limit {
		return playlist.DynamicTrackPage{}, malformedCatalogResponse("page metadata is inconsistent")
	}
	if len(response.GetTracks()) > 0 &&
		page.GetTotal() < int64(request.Offset+len(response.GetTracks())) {
		return playlist.DynamicTrackPage{}, malformedCatalogResponse("page total is inconsistent")
	}

	items := make([]playlist.DynamicTrack, 0, len(response.GetTracks()))
	seen := make(map[uuid.UUID]struct{}, len(response.GetTracks()))
	for index, track := range response.GetTracks() {
		if track == nil {
			return playlist.DynamicTrackPage{}, malformedCatalogResponse("track is missing")
		}
		id, err := uuid.Parse(track.GetId())
		if err != nil || id == uuid.Nil {
			return playlist.DynamicTrackPage{}, malformedCatalogResponse("track id is invalid")
		}
		if _, exists := seen[id]; exists {
			return playlist.DynamicTrackPage{}, malformedCatalogResponse("track ids are duplicated")
		}
		seen[id] = struct{}{}
		items = append(items, playlist.DynamicTrack{
			TrackID: id, Position: request.Offset + index + 1,
		})
	}
	return playlist.DynamicTrackPage{
		Items: items, Limit: int(page.GetLimit()), Offset: int(page.GetOffset()), Total: page.GetTotal(),
	}, nil
}

func malformedCatalogResponse(reason string) error {
	return fmt.Errorf("%w: malformed EvaluateTrackFilters response: %s",
		playlist.ErrCatalogUnavailable, reason)
}

var _ playlist.DynamicTrackResolver = (*DynamicResolver)(nil)
