package grpcapi

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/catalog-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGetTracksReturnsInternalDTOs(t *testing.T) {
	t.Parallel()

	track := grpcTestTrack()
	server := NewCatalogServer(&catalogStub{tracks: []metadata.Track{track}})
	response, err := server.GetTracks(context.Background(), &catalogv1.GetTracksRequest{
		TrackIds: []string{track.ID.String()},
	})
	if err != nil {
		t.Fatalf("GetTracks() error = %v", err)
	}
	if len(response.GetTracks()) != 1 {
		t.Fatalf("GetTracks().Tracks = %d", len(response.GetTracks()))
	}
	result := response.GetTracks()[0]
	if result.GetId() != track.ID.String() || result.GetAlbumId() != track.AlbumID.String() ||
		result.GetMediaAssetId() != track.MediaAssetID.String() ||
		result.GetContentId() != track.ContentID.String() ||
		result.GetReleaseDate().GetYear() != 2026 ||
		result.GetArtists()[0].GetRole() != catalogv1.ArtistRole_ARTIST_ROLE_PRIMARY ||
		!result.GetCreatedAt().AsTime().Equal(track.CreatedAt) {
		t.Fatalf("GetTracks() track = %+v", result)
	}
}

func TestPublishReleaseMapsApprovedMetadata(t *testing.T) {
	t.Parallel()
	sourceID, artistID, albumID := uuid.New(), uuid.New(), uuid.New()
	contentTrackID, catalogTrackID, mediaID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	stub := &catalogStub{publication: metadata.Publication{
		SourceReleaseID: sourceID, ArtistID: artistID, AlbumID: &albumID,
		Tracks: []metadata.PublishedTrack{{
			ContentTrackID: contentTrackID, CatalogTrackID: catalogTrackID,
		}}, PublishedAt: now,
	}}
	response, err := NewCatalogServer(stub).PublishRelease(
		context.Background(), &catalogv1.PublishReleaseRequest{
			SourceReleaseId: sourceID.String(),
			Artist:          &catalogv1.PublishArtist{Id: artistID.String(), Name: "Artist"},
			Album: &catalogv1.PublishAlbum{
				Id: albumID.String(), Title: "Release", AlbumType: "album",
				ReleaseDate: &catalogv1.Date{Year: 2027, Month: 2, Day: 3},
			},
			Tracks: []*catalogv1.PublishTrack{{
				Id: catalogTrackID.String(), ContentTrackId: contentTrackID.String(),
				Title: "Track", DurationMs: 180000, TrackNumber: 1, DiscNumber: 1,
				ReleaseDate:  &catalogv1.Date{Year: 2027, Month: 2, Day: 3},
				MediaAssetId: mediaID.String(),
			}},
		},
	)
	if err != nil || response.GetAlbumId() != albumID.String() ||
		len(response.GetTracks()) != 1 ||
		response.GetTracks()[0].GetCatalogTrackId() != catalogTrackID.String() {
		t.Fatalf("PublishRelease() = %+v, %v", response, err)
	}
	input := stub.publicationInput
	if input.SourceReleaseID != sourceID || input.Artist.ID != artistID ||
		input.Album == nil || input.Album.ID != albumID || len(input.Tracks) != 1 ||
		input.Tracks[0].ContentID != contentTrackID || input.Tracks[0].MediaAssetID != mediaID {
		t.Fatalf("publication input = %+v", input)
	}
}

func TestPublishReleaseRejectsInvalidContract(t *testing.T) {
	t.Parallel()
	_, err := NewCatalogServer(&catalogStub{}).PublishRelease(
		context.Background(), &catalogv1.PublishReleaseRequest{},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("PublishRelease() code = %s; error = %v", status.Code(err), err)
	}
}

func TestValidateTrackIdsSeparatesValidAndMissing(t *testing.T) {
	t.Parallel()

	validID := uuid.New()
	missingID := uuid.New()
	server := NewCatalogServer(&catalogStub{validation: metadata.TrackIDValidation{
		ValidIDs: []uuid.UUID{validID}, MissingIDs: []uuid.UUID{missingID},
	}})
	response, err := server.ValidateTrackIds(
		context.Background(),
		&catalogv1.ValidateTrackIdsRequest{TrackIds: []string{validID.String(), missingID.String()}},
	)
	if err != nil {
		t.Fatalf("ValidateTrackIds() error = %v", err)
	}
	if len(response.GetValidTrackIds()) != 1 || response.GetValidTrackIds()[0] != validID.String() ||
		len(response.GetMissingTrackIds()) != 1 || response.GetMissingTrackIds()[0] != missingID.String() {
		t.Fatalf("ValidateTrackIds() = %+v", response)
	}
}

func TestFilterTracksMapsFiltersAndPagination(t *testing.T) {
	t.Parallel()

	artistID := uuid.New()
	albumID := uuid.New()
	genreID := uuid.New()
	moodID := uuid.New()
	releaseYear := int32(2026)
	notExplicit := false
	stub := &catalogStub{page: metadata.Page[metadata.Track]{
		Items: []metadata.Track{grpcTestTrack()}, Limit: 7, Offset: 14, Total: 21,
	}}
	server := NewCatalogServer(stub)
	response, err := server.FilterTracks(context.Background(), &catalogv1.FilterTracksRequest{
		Title: "night", ArtistId: artistID.String(), AlbumId: albumID.String(),
		GenreId: genreID.String(), MoodId: moodID.String(), ReleaseYear: &releaseYear,
		Explicit: &notExplicit, Sort: catalogv1.TrackSort_TRACK_SORT_TITLE_ASC,
		Limit: 7, Offset: 14,
	})
	if err != nil {
		t.Fatalf("FilterTracks() error = %v", err)
	}
	query := stub.lastQuery
	if query.Title != "night" || query.ArtistID == nil || *query.ArtistID != artistID ||
		query.AlbumID == nil || *query.AlbumID != albumID ||
		query.GenreID == nil || *query.GenreID != genreID ||
		query.MoodID == nil || *query.MoodID != moodID ||
		query.ReleaseYear == nil || *query.ReleaseYear != 2026 ||
		query.Explicit == nil || *query.Explicit || query.Sort != metadata.TrackSortTitleAsc ||
		query.Page != (metadata.PageRequest{Limit: 7, Offset: 14}) {
		t.Fatalf("FilterTracks() query = %+v", query)
	}
	if len(response.GetTracks()) != 1 || response.GetPage().GetTotal() != 21 ||
		response.GetPage().GetLimit() != 7 || response.GetPage().GetOffset() != 14 {
		t.Fatalf("FilterTracks() = %+v", response)
	}
}

func TestEvaluateTrackFiltersMapsTypedFilters(t *testing.T) {
	t.Parallel()

	genreOne := uuid.New()
	genreTwo := uuid.New()
	stub := &catalogStub{page: metadata.Page[metadata.Track]{
		Items: []metadata.Track{grpcTestTrack()}, Limit: 5, Offset: 10, Total: 11,
	}}
	server := NewCatalogServer(stub)
	response, err := server.EvaluateTrackFilters(
		context.Background(),
		&catalogv1.EvaluateTrackFiltersRequest{
			Logic: catalogv1.FilterLogic_FILTER_LOGIC_AND,
			Filters: []*catalogv1.TrackFilter{
				{
					Field:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_GENRE_ID,
					Operator: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_IN,
					Value: &catalogv1.TrackFilterValue{Kind: &catalogv1.TrackFilterValue_UuidList{
						UuidList: &catalogv1.UUIDList{Values: []string{genreOne.String(), genreTwo.String()}},
					}},
				},
				{
					Field:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_DURATION_MS,
					Operator: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_BETWEEN,
					Value: &catalogv1.TrackFilterValue{Kind: &catalogv1.TrackFilterValue_IntegerRange{
						IntegerRange: &catalogv1.IntegerRange{Minimum: 120_000, Maximum: 300_000},
					}},
				},
				{
					Field:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_EXPLICIT,
					Operator: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS,
					Value: &catalogv1.TrackFilterValue{Kind: &catalogv1.TrackFilterValue_Boolean{
						Boolean: false,
					}},
				},
			},
			Sort: catalogv1.TrackSort_TRACK_SORT_TITLE_DESC, Limit: 5, Offset: 10,
		},
	)
	if err != nil {
		t.Fatalf("EvaluateTrackFilters() error = %v", err)
	}
	query := stub.lastDynamicQuery
	if query.Logic != metadata.FilterLogicAnd || query.Sort != metadata.TrackSortTitleDesc ||
		query.Page != (metadata.PageRequest{Limit: 5, Offset: 10}) || len(query.Filters) != 3 ||
		!reflect.DeepEqual(query.Filters[0].Value.UUIDs, []uuid.UUID{genreOne, genreTwo}) ||
		query.Filters[1].Value.IntegerRange != (metadata.IntegerRange{Minimum: 120_000, Maximum: 300_000}) ||
		query.Filters[2].Value.Kind != metadata.TrackFilterValueBoolean || query.Filters[2].Value.Boolean {
		t.Fatalf("EvaluateTrackFilters() query = %+v", query)
	}
	if len(response.GetTracks()) != 1 || response.GetPage().GetTotal() != 11 ||
		response.GetPage().GetLimit() != 5 || response.GetPage().GetOffset() != 10 {
		t.Fatalf("EvaluateTrackFilters() response = %+v", response)
	}
}

func TestEvaluateTrackFiltersRejectsInvalidContract(t *testing.T) {
	t.Parallel()

	tests := []*catalogv1.EvaluateTrackFiltersRequest{
		{},
		{Logic: catalogv1.FilterLogic_FILTER_LOGIC_AND},
		{Logic: catalogv1.FilterLogic_FILTER_LOGIC_AND, Filters: []*catalogv1.TrackFilter{nil}},
		{Logic: catalogv1.FilterLogic_FILTER_LOGIC_AND, Filters: []*catalogv1.TrackFilter{{
			Field:    catalogv1.TrackFilterField_TRACK_FILTER_FIELD_ARTIST_ID,
			Operator: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS,
			Value:    &catalogv1.TrackFilterValue{Kind: &catalogv1.TrackFilterValue_Uuid{Uuid: "invalid"}},
		}}},
		{Logic: catalogv1.FilterLogic_FILTER_LOGIC_AND, Filters: []*catalogv1.TrackFilter{{
			Field:    catalogv1.TrackFilterField(99),
			Operator: catalogv1.TrackFilterOperator_TRACK_FILTER_OPERATOR_EQUALS,
			Value:    &catalogv1.TrackFilterValue{Kind: &catalogv1.TrackFilterValue_Text{Text: "x"}},
		}}},
	}
	for _, request := range tests {
		_, err := NewCatalogServer(&catalogStub{}).EvaluateTrackFilters(context.Background(), request)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("EvaluateTrackFilters(%+v) code = %s; error = %v", request, status.Code(err), err)
		}
	}
}

func TestCatalogServerMapsErrors(t *testing.T) {
	t.Parallel()

	validID := uuid.NewString()
	tests := []struct {
		name     string
		request  *catalogv1.GetTracksRequest
		catalog  Catalog
		wantCode codes.Code
	}{
		{name: "empty ids", request: &catalogv1.GetTracksRequest{}, catalog: &catalogStub{}, wantCode: codes.InvalidArgument},
		{name: "invalid id", request: &catalogv1.GetTracksRequest{TrackIds: []string{"invalid"}}, catalog: &catalogStub{}, wantCode: codes.InvalidArgument},
		{name: "duplicate id", request: &catalogv1.GetTracksRequest{TrackIds: []string{validID, validID}}, catalog: &catalogStub{}, wantCode: codes.InvalidArgument},
		{name: "not found", request: &catalogv1.GetTracksRequest{TrackIds: []string{validID}}, catalog: &catalogStub{err: metadata.ErrNotFound}, wantCode: codes.NotFound},
		{name: "repository unavailable", request: &catalogv1.GetTracksRequest{TrackIds: []string{validID}}, catalog: &catalogStub{err: errors.New("database unavailable")}, wantCode: codes.Internal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCatalogServer(test.catalog).GetTracks(context.Background(), test.request)
			if status.Code(err) != test.wantCode {
				t.Fatalf("GetTracks() code = %s, want %s; error = %v", status.Code(err), test.wantCode, err)
			}
		})
	}
}

func TestFilterTracksRejectsInvalidContractValues(t *testing.T) {
	t.Parallel()

	tests := []*catalogv1.FilterTracksRequest{
		{ArtistId: "invalid"},
		{Sort: catalogv1.TrackSort(99)},
	}
	for _, request := range tests {
		_, err := NewCatalogServer(&catalogStub{}).FilterTracks(context.Background(), request)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("FilterTracks(%+v) code = %s; error = %v", request, status.Code(err), err)
		}
	}
}

func TestCatalogServerEnforcesTimeout(t *testing.T) {
	t.Parallel()

	server := NewCatalogServer(blockingCatalogStub{})
	server.timeout = 10 * time.Millisecond
	_, err := server.GetTracks(context.Background(), &catalogv1.GetTracksRequest{
		TrackIds: []string{uuid.NewString()},
	})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("GetTracks() code = %s, want %s; error = %v", status.Code(err), codes.DeadlineExceeded, err)
	}
}

func TestCatalogGRPCTransport(t *testing.T) {
	t.Parallel()

	validID := uuid.New()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(grpcServer, NewCatalogServer(&catalogStub{
		validation: metadata.TrackIDValidation{ValidIDs: []uuid.UUID{validID}},
	}))
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := catalogv1.NewCatalogServiceClient(connection)

	response, err := client.ValidateTrackIds(
		context.Background(),
		&catalogv1.ValidateTrackIdsRequest{TrackIds: []string{validID.String()}},
	)
	if err != nil {
		t.Fatalf("ValidateTrackIds() transport error = %v", err)
	}
	if len(response.GetValidTrackIds()) != 1 || response.GetValidTrackIds()[0] != validID.String() {
		t.Fatalf("ValidateTrackIds() = %+v", response)
	}
}

func grpcTestTrack() metadata.Track {
	albumID := uuid.New()
	createdAt := time.Date(2026, time.November, 8, 10, 0, 0, 0, time.UTC)
	return metadata.Track{
		ID: uuid.New(), AlbumID: &albumID, Title: "Internal Track", DurationMS: 180000,
		TrackNumber: 1, DiscNumber: 1, ReleaseDate: createdAt,
		MediaAssetID: uuid.New(), ContentID: uuid.New(), CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Hour),
		Artists: []metadata.TrackArtist{{
			Artist: metadata.Artist{ID: uuid.New(), Name: "Artist"},
			Role:   metadata.ArtistRolePrimary, Position: 1,
		}},
		Genres: []metadata.Genre{{ID: uuid.New(), Name: "Electronic", Slug: "electronic"}},
		Moods: []metadata.TrackMood{{
			Mood: metadata.Mood{ID: uuid.New(), Name: "Focused", Slug: "focused"}, Weight: 0.8,
		}},
	}
}

type catalogStub struct {
	publication      metadata.Publication
	publicationInput metadata.PublicationInput
	tracks           []metadata.Track
	validation       metadata.TrackIDValidation
	page             metadata.Page[metadata.Track]
	err              error
	lastQuery        metadata.TrackQuery
	lastDynamicQuery metadata.DynamicTrackQuery
}

func (stub *catalogStub) PublishApprovedRelease(
	_ context.Context,
	input metadata.PublicationInput,
) (metadata.Publication, error) {
	stub.publicationInput = input
	return stub.publication, stub.err
}

func (stub *catalogStub) BatchTracks(context.Context, []uuid.UUID) ([]metadata.Track, error) {
	return stub.tracks, stub.err
}

func (stub *catalogStub) ValidateTrackIDs(
	context.Context,
	[]uuid.UUID,
) (metadata.TrackIDValidation, error) {
	return stub.validation, stub.err
}

func (stub *catalogStub) Tracks(
	_ context.Context,
	query metadata.TrackQuery,
) (metadata.Page[metadata.Track], error) {
	stub.lastQuery = query
	return stub.page, stub.err
}

func (stub *catalogStub) EvaluateTrackFilters(
	_ context.Context,
	query metadata.DynamicTrackQuery,
) (metadata.Page[metadata.Track], error) {
	stub.lastDynamicQuery = query
	return stub.page, stub.err
}

type blockingCatalogStub struct{}

func (blockingCatalogStub) PublishApprovedRelease(
	ctx context.Context,
	_ metadata.PublicationInput,
) (metadata.Publication, error) {
	<-ctx.Done()
	return metadata.Publication{}, ctx.Err()
}

func (blockingCatalogStub) BatchTracks(ctx context.Context, _ []uuid.UUID) ([]metadata.Track, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingCatalogStub) ValidateTrackIDs(
	ctx context.Context,
	_ []uuid.UUID,
) (metadata.TrackIDValidation, error) {
	<-ctx.Done()
	return metadata.TrackIDValidation{}, ctx.Err()
}

func (blockingCatalogStub) Tracks(
	ctx context.Context,
	_ metadata.TrackQuery,
) (metadata.Page[metadata.Track], error) {
	<-ctx.Done()
	return metadata.Page[metadata.Track]{}, ctx.Err()
}

func (blockingCatalogStub) EvaluateTrackFilters(
	ctx context.Context,
	_ metadata.DynamicTrackQuery,
) (metadata.Page[metadata.Track], error) {
	<-ctx.Done()
	return metadata.Page[metadata.Track]{}, ctx.Err()
}
