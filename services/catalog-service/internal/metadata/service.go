package metadata

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Store interface {
	ArtistByID(context.Context, uuid.UUID) (Artist, error)
	SaveArtist(context.Context, ArtistInput) (Artist, error)
	AlbumByID(context.Context, uuid.UUID) (Album, error)
	SaveAlbum(context.Context, AlbumInput) (Album, error)
	TrackByID(context.Context, uuid.UUID) (Track, error)
	SaveTrack(context.Context, TrackInput) (Track, error)
	GenreByID(context.Context, uuid.UUID) (Genre, error)
	SaveGenre(context.Context, GenreInput) (Genre, error)
	MoodByID(context.Context, uuid.UUID) (Mood, error)
	SaveMood(context.Context, MoodInput) (Mood, error)
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
