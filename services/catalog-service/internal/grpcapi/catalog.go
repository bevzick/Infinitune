package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/catalog-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultRequestTimeout = 3 * time.Second

type Catalog interface {
	PublishApprovedRelease(context.Context, metadata.PublicationInput) (metadata.Publication, error)
	ArchivePublishedRelease(context.Context, uuid.UUID) (metadata.Archive, error)
	PutTrackLyrics(context.Context, metadata.TrackLyricsInput) (metadata.TrackLyrics, error)
	BatchTracks(context.Context, []uuid.UUID) ([]metadata.Track, error)
	ValidateTrackIDs(context.Context, []uuid.UUID) (metadata.TrackIDValidation, error)
	ValidateAlbumIDs(context.Context, []uuid.UUID) (metadata.AlbumIDValidation, error)
	ValidateArtistIDs(context.Context, []uuid.UUID) (metadata.ArtistIDValidation, error)
	Tracks(context.Context, metadata.TrackQuery) (metadata.Page[metadata.Track], error)
	EvaluateTrackFilters(context.Context, metadata.DynamicTrackQuery) (metadata.Page[metadata.Track], error)
}

func (server *CatalogServer) PutTrackLyrics(
	ctx context.Context,
	request *catalogv1.PutTrackLyricsRequest,
) (*catalogv1.PutTrackLyricsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	id, err := requiredUUID(request.GetId(), "id")
	if err != nil {
		return nil, err
	}
	trackID, err := requiredUUID(request.GetTrackId(), "track_id")
	if err != nil {
		return nil, err
	}
	input := metadata.TrackLyricsInput{
		ID: id, TrackID: trackID, Type: metadata.LyricsType(request.GetLyricsType()),
		Language: request.GetLanguage(), Content: request.GetContent(),
		SourceType: metadata.LyricsSourceType(request.GetSourceType()),
		SourceName: request.GetSourceName(), CopyrightNotice: request.GetCopyrightNotice(),
		Official: request.GetOfficial(), RightsAuthorized: request.GetRightsAuthorized(),
		Published: request.GetPublished(),
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()
	result, err := server.catalog.PutTrackLyrics(callCtx, input)
	if err != nil {
		return nil, mapCatalogError(err, "track lyrics update failed")
	}
	return &catalogv1.PutTrackLyricsResponse{
		Id: result.ID.String(), TrackId: result.TrackID.String(),
		LyricsType: string(result.Type), Language: result.Language, Content: result.Content,
		SourceType: string(result.SourceType), SourceName: result.SourceName,
		CopyrightNotice: result.CopyrightNotice, Official: result.Official,
		RightsAuthorized: result.RightsAuthorized, Published: result.Published,
		CreatedAt: timestamppb.New(result.CreatedAt), UpdatedAt: timestamppb.New(result.UpdatedAt),
	}, nil
}

func (server *CatalogServer) ArchiveRelease(
	ctx context.Context,
	request *catalogv1.ArchiveReleaseRequest,
) (*catalogv1.ArchiveReleaseResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	sourceID, err := requiredUUID(request.GetSourceReleaseId(), "source_release_id")
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()
	result, err := server.catalog.ArchivePublishedRelease(callCtx, sourceID)
	if err != nil {
		return nil, mapCatalogError(err, "release archival failed")
	}
	return &catalogv1.ArchiveReleaseResponse{
		ArchivedAt: timestamppb.New(result.ArchivedAt),
	}, nil
}

func (server *CatalogServer) PublishRelease(
	ctx context.Context,
	request *catalogv1.PublishReleaseRequest,
) (*catalogv1.PublishReleaseResponse, error) {
	input, err := publicationInput(request)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()
	result, err := server.catalog.PublishApprovedRelease(callCtx, input)
	if err != nil {
		return nil, mapCatalogError(err, "release publication failed")
	}
	tracks := make([]*catalogv1.PublishedTrack, 0, len(result.Tracks))
	for _, track := range result.Tracks {
		tracks = append(tracks, &catalogv1.PublishedTrack{
			ContentTrackId: track.ContentTrackID.String(),
			CatalogTrackId: track.CatalogTrackID.String(),
		})
	}
	albumID := ""
	if result.AlbumID != nil {
		albumID = result.AlbumID.String()
	}
	return &catalogv1.PublishReleaseResponse{
		ArtistId: result.ArtistID.String(), AlbumId: albumID, Tracks: tracks,
		PublishedAt: timestamppb.New(result.PublishedAt),
	}, nil
}

func publicationInput(request *catalogv1.PublishReleaseRequest) (metadata.PublicationInput, error) {
	if request == nil || request.GetArtist() == nil {
		return metadata.PublicationInput{}, status.Error(codes.InvalidArgument, "artist is required")
	}
	sourceID, err := requiredUUID(request.GetSourceReleaseId(), "source_release_id")
	if err != nil {
		return metadata.PublicationInput{}, err
	}
	artistID, err := requiredUUID(request.GetArtist().GetId(), "artist.id")
	if err != nil {
		return metadata.PublicationInput{}, err
	}
	input := metadata.PublicationInput{
		SourceReleaseID: sourceID,
		Artist:          metadata.ArtistInput{ID: artistID, Name: request.GetArtist().GetName()},
	}
	if album := request.GetAlbum(); album != nil {
		albumID, parseErr := requiredUUID(album.GetId(), "album.id")
		if parseErr != nil {
			return metadata.PublicationInput{}, parseErr
		}
		releaseDate, parseErr := publicationDate(album.GetReleaseDate(), "album.release_date")
		if parseErr != nil {
			return metadata.PublicationInput{}, parseErr
		}
		input.Album = &metadata.AlbumInput{
			ID: albumID, Title: album.GetTitle(), Type: metadata.AlbumType(album.GetAlbumType()),
			ReleaseDate: releaseDate, CoverURL: album.GetCoverUrl(),
			Artists: []metadata.ArtistPositionInput{{ArtistID: artistID, Position: 1}},
		}
	}
	if len(request.GetTracks()) == 0 {
		return metadata.PublicationInput{}, status.Error(codes.InvalidArgument, "tracks must not be empty")
	}
	input.Tracks = make([]metadata.TrackInput, 0, len(request.GetTracks()))
	for index, track := range request.GetTracks() {
		if track == nil {
			return metadata.PublicationInput{}, status.Errorf(codes.InvalidArgument, "tracks[%d] is required", index)
		}
		if track.GetTrackNumber() < 1 || track.GetTrackNumber() > 32767 ||
			track.GetDiscNumber() < 1 || track.GetDiscNumber() > 32767 {
			return metadata.PublicationInput{}, status.Errorf(
				codes.InvalidArgument, "tracks[%d] track and disc numbers must be from 1 to 32767", index,
			)
		}
		trackID, parseErr := requiredUUID(track.GetId(), fmt.Sprintf("tracks[%d].id", index))
		if parseErr != nil {
			return metadata.PublicationInput{}, parseErr
		}
		contentID, parseErr := requiredUUID(track.GetContentTrackId(), fmt.Sprintf("tracks[%d].content_track_id", index))
		if parseErr != nil {
			return metadata.PublicationInput{}, parseErr
		}
		mediaID, parseErr := requiredUUID(track.GetMediaAssetId(), fmt.Sprintf("tracks[%d].media_asset_id", index))
		if parseErr != nil {
			return metadata.PublicationInput{}, parseErr
		}
		releaseDate, parseErr := publicationDate(track.GetReleaseDate(), fmt.Sprintf("tracks[%d].release_date", index))
		if parseErr != nil {
			return metadata.PublicationInput{}, parseErr
		}
		var albumID *uuid.UUID
		if input.Album != nil {
			value := input.Album.ID
			albumID = &value
		}
		input.Tracks = append(input.Tracks, metadata.TrackInput{
			ID: trackID, AlbumID: albumID, Title: track.GetTitle(),
			DurationMS: int(track.GetDurationMs()), TrackNumber: int16(track.GetTrackNumber()),
			DiscNumber: int16(track.GetDiscNumber()), Explicit: track.GetExplicit(),
			ReleaseDate: releaseDate, MediaAssetID: mediaID, ContentID: contentID,
			Artists: []metadata.TrackArtistInput{{
				ArtistID: artistID, Role: metadata.ArtistRolePrimary, Position: 1,
			}},
		})
	}
	return input, nil
}

func requiredUUID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a UUID", field)
	}
	return id, nil
}

func publicationDate(value *catalogv1.Date, field string) (time.Time, error) {
	if value == nil || value.GetYear() < 1 || value.GetYear() > 9999 ||
		value.GetMonth() < 1 || value.GetMonth() > 12 || value.GetDay() < 1 || value.GetDay() > 31 {
		return time.Time{}, status.Errorf(codes.InvalidArgument, "%s is invalid", field)
	}
	result := time.Date(
		int(value.GetYear()), time.Month(value.GetMonth()), int(value.GetDay()), 0, 0, 0, 0, time.UTC,
	)
	if result.Year() != int(value.GetYear()) || result.Month() != time.Month(value.GetMonth()) ||
		result.Day() != int(value.GetDay()) {
		return time.Time{}, status.Errorf(codes.InvalidArgument, "%s is invalid", field)
	}
	return result, nil
}

type CatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	catalog Catalog
	timeout time.Duration
}

func NewCatalogServer(catalog Catalog) *CatalogServer {
	return &CatalogServer{catalog: catalog, timeout: defaultRequestTimeout}
}

func (server *CatalogServer) GetTracks(
	ctx context.Context,
	request *catalogv1.GetTracksRequest,
) (*catalogv1.GetTracksResponse, error) {
	ids, err := parseTrackIDs(request.GetTrackIds())
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()

	tracks, err := server.catalog.BatchTracks(callCtx, ids)
	if err != nil {
		return nil, mapCatalogError(err, "track lookup failed")
	}
	return &catalogv1.GetTracksResponse{Tracks: tracksResponse(tracks)}, nil
}

func (server *CatalogServer) ValidateTrackIds(
	ctx context.Context,
	request *catalogv1.ValidateTrackIdsRequest,
) (*catalogv1.ValidateTrackIdsResponse, error) {
	ids, err := parseTrackIDs(request.GetTrackIds())
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()

	result, err := server.catalog.ValidateTrackIDs(callCtx, ids)
	if err != nil {
		return nil, mapCatalogError(err, "track validation failed")
	}
	return &catalogv1.ValidateTrackIdsResponse{
		ValidTrackIds:   uuidStrings(result.ValidIDs),
		MissingTrackIds: uuidStrings(result.MissingIDs),
	}, nil
}

func (server *CatalogServer) ValidateAlbumIds(
	ctx context.Context,
	request *catalogv1.ValidateAlbumIdsRequest,
) (*catalogv1.ValidateAlbumIdsResponse, error) {
	ids, err := parseIDs(request.GetAlbumIds(), "album_ids")
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()

	result, err := server.catalog.ValidateAlbumIDs(callCtx, ids)
	if err != nil {
		return nil, mapCatalogError(err, "album validation failed")
	}
	return &catalogv1.ValidateAlbumIdsResponse{
		ValidAlbumIds:   uuidStrings(result.ValidIDs),
		MissingAlbumIds: uuidStrings(result.MissingIDs),
	}, nil
}

func (server *CatalogServer) ValidateArtistIds(
	ctx context.Context,
	request *catalogv1.ValidateArtistIdsRequest,
) (*catalogv1.ValidateArtistIdsResponse, error) {
	ids, err := parseIDs(request.GetArtistIds(), "artist_ids")
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()

	result, err := server.catalog.ValidateArtistIDs(callCtx, ids)
	if err != nil {
		return nil, mapCatalogError(err, "artist validation failed")
	}
	return &catalogv1.ValidateArtistIdsResponse{
		ValidArtistIds:   uuidStrings(result.ValidIDs),
		MissingArtistIds: uuidStrings(result.MissingIDs),
	}, nil
}

func (server *CatalogServer) FilterTracks(
	ctx context.Context,
	request *catalogv1.FilterTracksRequest,
) (*catalogv1.FilterTracksResponse, error) {
	query, err := filterQuery(request)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()

	result, err := server.catalog.Tracks(callCtx, query)
	if err != nil {
		return nil, mapCatalogError(err, "track filtering failed")
	}
	return &catalogv1.FilterTracksResponse{
		Tracks: tracksResponse(result.Items),
		Page: &catalogv1.Page{
			Limit: int32(result.Limit), Offset: int32(result.Offset), Total: result.Total,
		},
	}, nil
}

func (server *CatalogServer) EvaluateTrackFilters(
	ctx context.Context,
	request *catalogv1.EvaluateTrackFiltersRequest,
) (*catalogv1.EvaluateTrackFiltersResponse, error) {
	query, err := dynamicTrackQuery(request)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, server.timeout)
	defer cancel()

	result, err := server.catalog.EvaluateTrackFilters(callCtx, query)
	if err != nil {
		return nil, mapCatalogError(err, "dynamic track filtering failed")
	}
	return &catalogv1.EvaluateTrackFiltersResponse{
		Tracks: tracksResponse(result.Items),
		Page: &catalogv1.Page{
			Limit: int32(result.Limit), Offset: int32(result.Offset), Total: result.Total,
		},
	}, nil
}

func parseTrackIDs(values []string) ([]uuid.UUID, error) {
	return parseIDs(values, "track_ids")
}

func parseIDs(values []string, field string) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s must not be empty", field)
	}
	if len(values) > 100 {
		return nil, status.Errorf(codes.InvalidArgument, "%s must not contain more than 100 ids", field)
	}
	ids := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return nil, status.Errorf(codes.InvalidArgument, "%s must contain UUIDs", field)
		}
		if _, exists := seen[id]; exists {
			return nil, status.Errorf(codes.InvalidArgument, "%s must not contain duplicates", field)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func filterQuery(request *catalogv1.FilterTracksRequest) (metadata.TrackQuery, error) {
	if request == nil {
		request = &catalogv1.FilterTracksRequest{}
	}
	artistID, err := optionalUUID(request.GetArtistId(), "artist_id")
	if err != nil {
		return metadata.TrackQuery{}, err
	}
	albumID, err := optionalUUID(request.GetAlbumId(), "album_id")
	if err != nil {
		return metadata.TrackQuery{}, err
	}
	genreID, err := optionalUUID(request.GetGenreId(), "genre_id")
	if err != nil {
		return metadata.TrackQuery{}, err
	}
	moodID, err := optionalUUID(request.GetMoodId(), "mood_id")
	if err != nil {
		return metadata.TrackQuery{}, err
	}
	sort, err := trackSort(request.GetSort())
	if err != nil {
		return metadata.TrackQuery{}, err
	}
	limit := int(request.GetLimit())
	if limit == 0 {
		limit = 20
	}
	var releaseYear *int
	if request.ReleaseYear != nil {
		value := int(request.GetReleaseYear())
		releaseYear = &value
	}
	var explicit *bool
	if request.Explicit != nil {
		value := request.GetExplicit()
		explicit = &value
	}
	return metadata.TrackQuery{
		Page:  metadata.PageRequest{Limit: limit, Offset: int(request.GetOffset())},
		Title: request.GetTitle(), ArtistID: artistID, AlbumID: albumID,
		GenreID: genreID, MoodID: moodID, ReleaseYear: releaseYear,
		Explicit: explicit, Sort: sort,
	}, nil
}

func dynamicTrackQuery(
	request *catalogv1.EvaluateTrackFiltersRequest,
) (metadata.DynamicTrackQuery, error) {
	if request == nil {
		request = &catalogv1.EvaluateTrackFiltersRequest{}
	}
	logic, err := filterLogic(request.GetLogic())
	if err != nil {
		return metadata.DynamicTrackQuery{}, err
	}
	if len(request.GetFilters()) == 0 {
		return metadata.DynamicTrackQuery{}, status.Error(codes.InvalidArgument, "filters must not be empty")
	}
	filters := make([]metadata.TrackFilter, 0, len(request.GetFilters()))
	for index, filter := range request.GetFilters() {
		mapped, mapErr := dynamicTrackFilter(filter, index)
		if mapErr != nil {
			return metadata.DynamicTrackQuery{}, mapErr
		}
		filters = append(filters, mapped)
	}
	sort, err := trackSort(request.GetSort())
	if err != nil {
		return metadata.DynamicTrackQuery{}, err
	}
	limit := int(request.GetLimit())
	if limit == 0 {
		limit = 20
	}
	return metadata.DynamicTrackQuery{
		Page:  metadata.PageRequest{Limit: limit, Offset: int(request.GetOffset())},
		Logic: logic, Filters: filters, Sort: sort,
	}, nil
}

func filterLogic(value catalogv1.FilterLogic) (metadata.FilterLogic, error) {
	switch value {
	case catalogv1.FilterLogic_FILTER_LOGIC_AND:
		return metadata.FilterLogicAnd, nil
	case catalogv1.FilterLogic_FILTER_LOGIC_OR:
		return metadata.FilterLogicOr, nil
	default:
		return "", status.Error(codes.InvalidArgument, "logic must be AND or OR")
	}
}

func dynamicTrackFilter(value *catalogv1.TrackFilter, index int) (metadata.TrackFilter, error) {
	if value == nil || value.GetValue() == nil {
		return metadata.TrackFilter{}, status.Errorf(
			codes.InvalidArgument, "filters[%d].value is required", index,
		)
	}
	field, err := dynamicTrackFilterField(value.GetField(), index)
	if err != nil {
		return metadata.TrackFilter{}, err
	}
	operator, err := dynamicTrackFilterOperator(value.GetOperator(), index)
	if err != nil {
		return metadata.TrackFilter{}, err
	}
	mappedValue, err := dynamicTrackFilterValue(value.GetValue(), index)
	if err != nil {
		return metadata.TrackFilter{}, err
	}
	return metadata.TrackFilter{Field: field, Operator: operator, Value: mappedValue}, nil
}

func dynamicTrackFilterField(
	value catalogv1.TrackFilterField,
	index int,
) (metadata.TrackFilterField, error) {
	switch value {
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_TITLE:
		return metadata.TrackFilterTitle, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ARTIST_ID:
		return metadata.TrackFilterArtistID, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ALBUM_ID:
		return metadata.TrackFilterAlbumID, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_GENRE_ID:
		return metadata.TrackFilterGenreID, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_MOOD_ID:
		return metadata.TrackFilterMoodID, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_RELEASE_YEAR:
		return metadata.TrackFilterReleaseYear, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_DURATION_MS:
		return metadata.TrackFilterDurationMS, nil
	case catalogv1.TrackFilterField_TRACK_FILTER_FIELD_EXPLICIT:
		return metadata.TrackFilterExplicit, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "filters[%d].field is unsupported", index)
	}
}

func dynamicTrackFilterOperator(
	value catalogv1.TrackFilterOperator,
	index int,
) (metadata.TrackFilterOperator, error) {
	switch value {
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS:
		return metadata.TrackFilterEquals, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_CONTAINS:
		return metadata.TrackFilterContains, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_IN:
		return metadata.TrackFilterIn, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_GREATER_THAN:
		return metadata.TrackFilterGreaterThan, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_GREATER_THAN_OR_EQUAL:
		return metadata.TrackFilterGreaterThanOrEqual, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_LESS_THAN:
		return metadata.TrackFilterLessThan, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_LESS_THAN_OR_EQUAL:
		return metadata.TrackFilterLessThanOrEqual, nil
	case catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_BETWEEN:
		return metadata.TrackFilterBetween, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "filters[%d].operator is unsupported", index)
	}
}

func dynamicTrackFilterValue(
	value *catalogv1.TrackFilterValue,
	index int,
) (metadata.TrackFilterValue, error) {
	switch kind := value.GetKind().(type) {
	case *catalogv1.TrackFilterValue_Text:
		return metadata.TextFilterValue(kind.Text), nil
	case *catalogv1.TrackFilterValue_Uuid:
		id, err := uuid.Parse(kind.Uuid)
		if err != nil || id == uuid.Nil {
			return metadata.TrackFilterValue{}, status.Errorf(
				codes.InvalidArgument, "filters[%d].value must be a UUID", index,
			)
		}
		return metadata.UUIDFilterValue(id), nil
	case *catalogv1.TrackFilterValue_UuidList:
		if kind.UuidList == nil {
			return metadata.TrackFilterValue{}, status.Errorf(
				codes.InvalidArgument, "filters[%d].value must contain UUIDs", index,
			)
		}
		ids, err := parseUUIDList(kind.UuidList.GetValues(), fmt.Sprintf("filters[%d].value", index))
		if err != nil {
			return metadata.TrackFilterValue{}, err
		}
		return metadata.UUIDListFilterValue(ids...), nil
	case *catalogv1.TrackFilterValue_Integer:
		return metadata.IntegerFilterValue(int(kind.Integer)), nil
	case *catalogv1.TrackFilterValue_IntegerRange:
		if kind.IntegerRange == nil {
			return metadata.TrackFilterValue{}, status.Errorf(
				codes.InvalidArgument, "filters[%d].value must contain an integer range", index,
			)
		}
		return metadata.IntegerRangeFilterValue(
			int(kind.IntegerRange.GetMinimum()), int(kind.IntegerRange.GetMaximum()),
		), nil
	case *catalogv1.TrackFilterValue_Boolean:
		return metadata.BooleanFilterValue(kind.Boolean), nil
	default:
		return metadata.TrackFilterValue{}, status.Errorf(
			codes.InvalidArgument, "filters[%d].value is required", index,
		)
	}
}

func parseUUIDList(values []string, field string) ([]uuid.UUID, error) {
	if len(values) == 0 || len(values) > 100 {
		return nil, status.Errorf(codes.InvalidArgument, "%s must contain from 1 to 100 UUIDs", field)
	}
	ids := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return nil, status.Errorf(codes.InvalidArgument, "%s must contain UUIDs", field)
		}
		if _, exists := seen[id]; exists {
			return nil, status.Errorf(codes.InvalidArgument, "%s must not contain duplicates", field)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func optionalUUID(value, field string) (*uuid.UUID, error) {
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s must be a UUID", field)
	}
	return &id, nil
}

func trackSort(value catalogv1.TrackSort) (metadata.TrackSort, error) {
	switch value {
	case catalogv1.TrackSort_TRACK_SORT_UNSPECIFIED, catalogv1.TrackSort_TRACK_SORT_NEWEST:
		return metadata.TrackSortNewest, nil
	case catalogv1.TrackSort_TRACK_SORT_OLDEST:
		return metadata.TrackSortOldest, nil
	case catalogv1.TrackSort_TRACK_SORT_TITLE_ASC:
		return metadata.TrackSortTitleAsc, nil
	case catalogv1.TrackSort_TRACK_SORT_TITLE_DESC:
		return metadata.TrackSortTitleDesc, nil
	default:
		return "", status.Error(codes.InvalidArgument, "sort is unsupported")
	}
}

func mapCatalogError(err error, internalMessage string) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "catalog request timed out")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "catalog request was canceled")
	case errors.Is(err, metadata.ErrInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, metadata.ErrConflict):
		return status.Error(codes.AlreadyExists, "catalog data conflicts with an existing item")
	case errors.Is(err, metadata.ErrInvalidReference):
		return status.Error(codes.FailedPrecondition, "catalog references are invalid")
	case errors.Is(err, metadata.ErrNotFound):
		return status.Error(codes.NotFound, "catalog item was not found")
	default:
		return status.Error(codes.Internal, internalMessage)
	}
}

func tracksResponse(tracks []metadata.Track) []*catalogv1.Track {
	result := make([]*catalogv1.Track, 0, len(tracks))
	for _, track := range tracks {
		result = append(result, trackResponse(track))
	}
	return result
}

func trackResponse(track metadata.Track) *catalogv1.Track {
	albumID := ""
	if track.AlbumID != nil {
		albumID = track.AlbumID.String()
	}
	artists := make([]*catalogv1.ArtistCredit, 0, len(track.Artists))
	for _, credit := range track.Artists {
		artists = append(artists, &catalogv1.ArtistCredit{
			Artist: &catalogv1.Artist{
				Id: credit.Artist.ID.String(), Name: credit.Artist.Name,
				ImageUrl: credit.Artist.ImageURL,
			},
			Role: artistRole(credit.Role), Position: int32(credit.Position),
		})
	}
	genres := make([]*catalogv1.Genre, 0, len(track.Genres))
	for _, genre := range track.Genres {
		genres = append(genres, &catalogv1.Genre{
			Id: genre.ID.String(), Name: genre.Name, Slug: genre.Slug,
		})
	}
	moods := make([]*catalogv1.TrackMood, 0, len(track.Moods))
	for _, mood := range track.Moods {
		moods = append(moods, &catalogv1.TrackMood{
			Mood: &catalogv1.Mood{
				Id: mood.Mood.ID.String(), Name: mood.Mood.Name, Slug: mood.Mood.Slug,
			},
			Weight: mood.Weight,
		})
	}
	return &catalogv1.Track{
		Id: track.ID.String(), AlbumId: albumID, Title: track.Title,
		DurationMs: int32(track.DurationMS), TrackNumber: int32(track.TrackNumber),
		DiscNumber: int32(track.DiscNumber), Explicit: track.Explicit,
		ReleaseDate: &catalogv1.Date{
			Year: int32(track.ReleaseDate.Year()), Month: int32(track.ReleaseDate.Month()),
			Day: int32(track.ReleaseDate.Day()),
		},
		MediaAssetId: track.MediaAssetID.String(), ContentId: track.ContentID.String(),
		Artists: artists, Genres: genres, Moods: moods,
		CreatedAt: timestamppb.New(track.CreatedAt), UpdatedAt: timestamppb.New(track.UpdatedAt),
	}
}

func artistRole(role metadata.ArtistRole) catalogv1.ArtistRole {
	switch role {
	case metadata.ArtistRolePrimary:
		return catalogv1.ArtistRole_ARTIST_ROLE_PRIMARY
	case metadata.ArtistRoleFeatured:
		return catalogv1.ArtistRole_ARTIST_ROLE_FEATURED
	case metadata.ArtistRoleProducer:
		return catalogv1.ArtistRole_ARTIST_ROLE_PRODUCER
	case metadata.ArtistRoleComposer:
		return catalogv1.ArtistRole_ARTIST_ROLE_COMPOSER
	default:
		return catalogv1.ArtistRole_ARTIST_ROLE_UNSPECIFIED
	}
}

func uuidStrings(ids []uuid.UUID) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, id.String())
	}
	return result
}
