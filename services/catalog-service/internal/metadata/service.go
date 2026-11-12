package metadata

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Store interface {
	ArtistByID(context.Context, uuid.UUID) (Artist, error)
	ListArtists(context.Context, PageRequest) (Page[Artist], error)
	ListArtistDiscography(context.Context, uuid.UUID, PageRequest) (Page[DiscographyItem], error)
	ExistingArtistIDs(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)
	SaveArtist(context.Context, ArtistInput) (Artist, error)
	AlbumByID(context.Context, uuid.UUID) (Album, error)
	ListAlbums(context.Context, PageRequest) (Page[Album], error)
	ExistingAlbumIDs(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)
	SaveAlbum(context.Context, AlbumInput) (Album, error)
	TrackByID(context.Context, uuid.UUID) (Track, error)
	TracksByIDs(context.Context, []uuid.UUID) ([]Track, error)
	ExistingTrackIDs(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)
	SearchTracks(context.Context, TrackQuery) (Page[Track], error)
	EvaluateTrackFilters(context.Context, DynamicTrackQuery) (Page[Track], error)
	SaveTrack(context.Context, TrackInput) (Track, error)
	PublicTrackLyrics(context.Context, uuid.UUID) ([]TrackLyrics, error)
	SaveTrackLyrics(context.Context, TrackLyricsInput) (TrackLyrics, error)
	TrackLyricsTiming(context.Context, uuid.UUID) (TrackLyrics, int, error)
	ReplaceLyricsLines(context.Context, uuid.UUID, []LyricsLine) ([]LyricsLine, error)
	PublishRelease(context.Context, PublicationInput, string) (Publication, error)
	ArchiveRelease(context.Context, uuid.UUID) (Archive, error)
	GenreByID(context.Context, uuid.UUID) (Genre, error)
	ListGenres(context.Context, PageRequest) (Page[Genre], error)
	SaveGenre(context.Context, GenreInput) (Genre, error)
	MoodByID(context.Context, uuid.UUID) (Mood, error)
	ListMoods(context.Context, PageRequest) (Page[Mood], error)
	SaveMood(context.Context, MoodInput) (Mood, error)
}

func (service *Service) ArchivePublishedRelease(
	ctx context.Context,
	sourceReleaseID uuid.UUID,
) (Archive, error) {
	if err := validateID("source_release_id", sourceReleaseID); err != nil {
		return Archive{}, err
	}
	result, err := service.store.ArchiveRelease(ctx, sourceReleaseID)
	if err != nil {
		return Archive{}, fmt.Errorf("archive published release: %w", err)
	}
	return result, nil
}

func (service *Service) PublishApprovedRelease(
	ctx context.Context,
	input PublicationInput,
) (Publication, error) {
	canonical, fingerprint, err := validatePublication(input)
	if err != nil {
		return Publication{}, err
	}
	result, err := service.store.PublishRelease(ctx, canonical, fingerprint)
	if err != nil {
		return Publication{}, fmt.Errorf("publish approved release: %w", err)
	}
	return result, nil
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) Artist(ctx context.Context, id uuid.UUID) (Artist, error) {
	if err := validateID("artist_id", id); err != nil {
		return Artist{}, err
	}
	result, err := service.store.ArtistByID(ctx, id)
	if err != nil {
		return Artist{}, fmt.Errorf("read artist: %w", err)
	}
	return result, nil
}

func (service *Service) Artists(ctx context.Context, page PageRequest) (Page[Artist], error) {
	if err := validatePage(page); err != nil {
		return Page[Artist]{}, err
	}
	result, err := service.store.ListArtists(ctx, page)
	if err != nil {
		return Page[Artist]{}, fmt.Errorf("list artists: %w", err)
	}
	return result, nil
}

func (service *Service) ArtistDiscography(
	ctx context.Context,
	artistID uuid.UUID,
	page PageRequest,
) (Page[DiscographyItem], error) {
	if err := validateID("artist_id", artistID); err != nil {
		return Page[DiscographyItem]{}, err
	}
	if err := validatePage(page); err != nil {
		return Page[DiscographyItem]{}, err
	}
	if _, err := service.store.ArtistByID(ctx, artistID); err != nil {
		return Page[DiscographyItem]{}, fmt.Errorf("read discography artist: %w", err)
	}
	result, err := service.store.ListArtistDiscography(ctx, artistID, page)
	if err != nil {
		return Page[DiscographyItem]{}, fmt.Errorf("list artist discography: %w", err)
	}
	return result, nil
}

func (service *Service) ValidateArtistIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (ArtistIDValidation, error) {
	if err := validateIDBatch("artist_ids", ids); err != nil {
		return ArtistIDValidation{}, err
	}
	existing, err := service.store.ExistingArtistIDs(ctx, ids)
	if err != nil {
		return ArtistIDValidation{}, fmt.Errorf("validate artist ids: %w", err)
	}
	result := ArtistIDValidation{
		ValidIDs: make([]uuid.UUID, 0, len(ids)), MissingIDs: make([]uuid.UUID, 0),
	}
	for _, id := range ids {
		if _, exists := existing[id]; exists {
			result.ValidIDs = append(result.ValidIDs, id)
		} else {
			result.MissingIDs = append(result.MissingIDs, id)
		}
	}
	return result, nil
}

func (service *Service) Album(ctx context.Context, id uuid.UUID) (Album, error) {
	if err := validateID("album_id", id); err != nil {
		return Album{}, err
	}
	result, err := service.store.AlbumByID(ctx, id)
	if err != nil {
		return Album{}, fmt.Errorf("read album: %w", err)
	}
	return result, nil
}

func (service *Service) Albums(ctx context.Context, page PageRequest) (Page[Album], error) {
	if err := validatePage(page); err != nil {
		return Page[Album]{}, err
	}
	result, err := service.store.ListAlbums(ctx, page)
	if err != nil {
		return Page[Album]{}, fmt.Errorf("list albums: %w", err)
	}
	return result, nil
}

func (service *Service) ValidateAlbumIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (AlbumIDValidation, error) {
	if err := validateIDBatch("album_ids", ids); err != nil {
		return AlbumIDValidation{}, err
	}
	existing, err := service.store.ExistingAlbumIDs(ctx, ids)
	if err != nil {
		return AlbumIDValidation{}, fmt.Errorf("validate album ids: %w", err)
	}
	result := AlbumIDValidation{
		ValidIDs: make([]uuid.UUID, 0, len(ids)), MissingIDs: make([]uuid.UUID, 0),
	}
	for _, id := range ids {
		if _, exists := existing[id]; exists {
			result.ValidIDs = append(result.ValidIDs, id)
		} else {
			result.MissingIDs = append(result.MissingIDs, id)
		}
	}
	return result, nil
}

func (service *Service) Track(ctx context.Context, id uuid.UUID) (Track, error) {
	if err := validateID("track_id", id); err != nil {
		return Track{}, err
	}
	result, err := service.store.TrackByID(ctx, id)
	if err != nil {
		return Track{}, fmt.Errorf("read track: %w", err)
	}
	return result, nil
}

func (service *Service) TrackLyrics(ctx context.Context, trackID uuid.UUID) ([]TrackLyrics, error) {
	if err := validateID("track_id", trackID); err != nil {
		return nil, err
	}
	result, err := service.store.PublicTrackLyrics(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("read track lyrics: %w", err)
	}
	public := make([]TrackLyrics, 0, len(result))
	for _, value := range result {
		if value.Published && value.RightsAuthorized &&
			(value.Type != LyricsTypeSynchronized || len(value.Lines) > 0) {
			public = append(public, value)
		}
	}
	return public, nil
}

func (service *Service) PutTrackLyrics(
	ctx context.Context,
	input TrackLyricsInput,
) (TrackLyrics, error) {
	canonical, err := validateTrackLyrics(input)
	if err != nil {
		return TrackLyrics{}, err
	}
	result, err := service.store.SaveTrackLyrics(ctx, canonical)
	if err != nil {
		return TrackLyrics{}, fmt.Errorf("save track lyrics: %w", err)
	}
	return result, nil
}

func (service *Service) ReplaceTrackLyricsLines(
	ctx context.Context,
	lyricsID uuid.UUID,
	lines []LyricsLine,
) ([]LyricsLine, error) {
	if err := validateID("lyrics_id", lyricsID); err != nil {
		return nil, err
	}
	lyrics, durationMS, err := service.store.TrackLyricsTiming(ctx, lyricsID)
	if err != nil {
		return nil, fmt.Errorf("read track lyrics timing: %w", err)
	}
	if lyrics.Type != LyricsTypeSynchronized {
		return nil, validationError("lyrics_id", "must reference synchronized lyrics")
	}
	canonical, err := validateLyricsLines(lines, durationMS)
	if err != nil {
		return nil, err
	}
	result, err := service.store.ReplaceLyricsLines(ctx, lyricsID, canonical)
	if err != nil {
		return nil, fmt.Errorf("replace synchronized lyrics lines: %w", err)
	}
	return result, nil
}

func (service *Service) BatchTracks(ctx context.Context, ids []uuid.UUID) ([]Track, error) {
	if err := validateTrackIDs(ids); err != nil {
		return nil, err
	}
	result, err := service.store.TracksByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("read tracks by ids: %w", err)
	}
	if len(result) != len(ids) {
		return nil, ErrNotFound
	}
	return result, nil
}

func (service *Service) ValidateTrackIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (TrackIDValidation, error) {
	if err := validateTrackIDs(ids); err != nil {
		return TrackIDValidation{}, err
	}
	existing, err := service.store.ExistingTrackIDs(ctx, ids)
	if err != nil {
		return TrackIDValidation{}, fmt.Errorf("validate track ids: %w", err)
	}
	result := TrackIDValidation{
		ValidIDs: make([]uuid.UUID, 0, len(ids)), MissingIDs: make([]uuid.UUID, 0),
	}
	for _, id := range ids {
		if _, exists := existing[id]; exists {
			result.ValidIDs = append(result.ValidIDs, id)
		} else {
			result.MissingIDs = append(result.MissingIDs, id)
		}
	}
	return result, nil
}

func (service *Service) Tracks(ctx context.Context, query TrackQuery) (Page[Track], error) {
	canonical, err := validateTrackQuery(query)
	if err != nil {
		return Page[Track]{}, err
	}
	result, err := service.store.SearchTracks(ctx, canonical)
	if err != nil {
		return Page[Track]{}, fmt.Errorf("list tracks: %w", err)
	}
	return result, nil
}

func (service *Service) EvaluateTrackFilters(
	ctx context.Context,
	query DynamicTrackQuery,
) (Page[Track], error) {
	canonical, err := validateDynamicTrackQuery(query)
	if err != nil {
		return Page[Track]{}, err
	}
	result, err := service.store.EvaluateTrackFilters(ctx, canonical)
	if err != nil {
		return Page[Track]{}, fmt.Errorf("evaluate track filters: %w", err)
	}
	return result, nil
}

func (service *Service) Genre(ctx context.Context, id uuid.UUID) (Genre, error) {
	if err := validateID("genre_id", id); err != nil {
		return Genre{}, err
	}
	result, err := service.store.GenreByID(ctx, id)
	if err != nil {
		return Genre{}, fmt.Errorf("read genre: %w", err)
	}
	return result, nil
}

func (service *Service) Genres(ctx context.Context, page PageRequest) (Page[Genre], error) {
	if err := validatePage(page); err != nil {
		return Page[Genre]{}, err
	}
	result, err := service.store.ListGenres(ctx, page)
	if err != nil {
		return Page[Genre]{}, fmt.Errorf("list genres: %w", err)
	}
	return result, nil
}

func (service *Service) Mood(ctx context.Context, id uuid.UUID) (Mood, error) {
	if err := validateID("mood_id", id); err != nil {
		return Mood{}, err
	}
	result, err := service.store.MoodByID(ctx, id)
	if err != nil {
		return Mood{}, fmt.Errorf("read mood: %w", err)
	}
	return result, nil
}

func (service *Service) Moods(ctx context.Context, page PageRequest) (Page[Mood], error) {
	if err := validatePage(page); err != nil {
		return Page[Mood]{}, err
	}
	result, err := service.store.ListMoods(ctx, page)
	if err != nil {
		return Page[Mood]{}, fmt.Errorf("list moods: %w", err)
	}
	return result, nil
}

// StoreApprovedArtist is an internal write path for metadata that has passed
// the Content Service approval workflow. It is intentionally not exposed by HTTP.
func (service *Service) StoreApprovedArtist(
	ctx context.Context,
	input ArtistInput,
) (Artist, error) {
	canonical, err := validateArtist(input)
	if err != nil {
		return Artist{}, err
	}
	result, err := service.store.SaveArtist(ctx, canonical)
	if err != nil {
		return Artist{}, fmt.Errorf("store approved artist: %w", err)
	}
	return result, nil
}

// StoreApprovedAlbum is an internal write path for approved release metadata.
func (service *Service) StoreApprovedAlbum(
	ctx context.Context,
	input AlbumInput,
) (Album, error) {
	canonical, err := validateAlbum(input)
	if err != nil {
		return Album{}, err
	}
	result, err := service.store.SaveAlbum(ctx, canonical)
	if err != nil {
		return Album{}, fmt.Errorf("store approved album: %w", err)
	}
	return result, nil
}

// StoreApprovedTrack is an internal write path for approved track metadata.
func (service *Service) StoreApprovedTrack(
	ctx context.Context,
	input TrackInput,
) (Track, error) {
	canonical, err := validateTrack(input)
	if err != nil {
		return Track{}, err
	}
	result, err := service.store.SaveTrack(ctx, canonical)
	if err != nil {
		return Track{}, fmt.Errorf("store approved track: %w", err)
	}
	return result, nil
}

// StoreGenre is reserved for trusted internal taxonomy management.
func (service *Service) StoreGenre(ctx context.Context, input GenreInput) (Genre, error) {
	canonical, err := validateGenre(input)
	if err != nil {
		return Genre{}, err
	}
	result, err := service.store.SaveGenre(ctx, canonical)
	if err != nil {
		return Genre{}, fmt.Errorf("store genre: %w", err)
	}
	return result, nil
}

// StoreMood is reserved for trusted internal taxonomy management.
func (service *Service) StoreMood(ctx context.Context, input MoodInput) (Mood, error) {
	canonical, err := validateMood(input)
	if err != nil {
		return Mood{}, err
	}
	result, err := service.store.SaveMood(ctx, canonical)
	if err != nil {
		return Mood{}, fmt.Errorf("store mood: %w", err)
	}
	return result, nil
}
