package grpcapi

import (
	"context"
	"errors"
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
	BatchTracks(context.Context, []uuid.UUID) ([]metadata.Track, error)
	ValidateTrackIDs(context.Context, []uuid.UUID) (metadata.TrackIDValidation, error)
	Tracks(context.Context, metadata.TrackQuery) (metadata.Page[metadata.Track], error)
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

func parseTrackIDs(values []string) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return nil, status.Error(codes.InvalidArgument, "track_ids must not be empty")
	}
	if len(values) > 100 {
		return nil, status.Error(codes.InvalidArgument, "track_ids must not contain more than 100 ids")
	}
	ids := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "track_ids must contain UUIDs")
		}
		if _, exists := seen[id]; exists {
			return nil, status.Error(codes.InvalidArgument, "track_ids must not contain duplicates")
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
	case errors.Is(err, metadata.ErrNotFound):
		return status.Error(codes.NotFound, "one or more tracks were not found")
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
