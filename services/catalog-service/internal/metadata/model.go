package metadata

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound         = errors.New("catalog metadata not found")
	ErrConflict         = errors.New("catalog metadata conflicts with existing data")
	ErrInvalid          = errors.New("catalog metadata is invalid")
	ErrInvalidReference = errors.New("catalog metadata references are invalid")
)

type AlbumType string

const (
	AlbumTypeAlbum       AlbumType = "album"
	AlbumTypeSingle      AlbumType = "single"
	AlbumTypeEP          AlbumType = "ep"
	AlbumTypeCompilation AlbumType = "compilation"
)

type ArtistRole string

const (
	ArtistRolePrimary  ArtistRole = "primary"
	ArtistRoleFeatured ArtistRole = "featured"
	ArtistRoleProducer ArtistRole = "producer"
	ArtistRoleComposer ArtistRole = "composer"
)

type Artist struct {
	ID          uuid.UUID
	Name        string
	Description string
	ImageURL    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ArtistInput struct {
	ID          uuid.UUID
	Name        string
	Description string
	ImageURL    string
}

type Genre struct {
	ID   uuid.UUID
	Name string
	Slug string
}

type GenreInput struct {
	ID   uuid.UUID
	Name string
	Slug string
}

type Mood struct {
	ID   uuid.UUID
	Name string
	Slug string
}

type MoodInput struct {
	ID   uuid.UUID
	Name string
	Slug string
}

type Album struct {
	ID          uuid.UUID
	Title       string
	Type        AlbumType
	ReleaseDate time.Time
	CoverURL    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Artists     []AlbumArtist
}

type AlbumArtist struct {
	Artist   Artist
	Position int16
}

type ArtistPositionInput struct {
	ArtistID uuid.UUID
	Position int16
}

type AlbumInput struct {
	ID          uuid.UUID
	Title       string
	Type        AlbumType
	ReleaseDate time.Time
	CoverURL    string
	Artists     []ArtistPositionInput
}

type Track struct {
	ID           uuid.UUID
	AlbumID      *uuid.UUID
	Title        string
	DurationMS   int
	TrackNumber  int16
	DiscNumber   int16
	Explicit     bool
	ReleaseDate  time.Time
	MediaAssetID uuid.UUID
	ContentID    uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Artists      []TrackArtist
	Genres       []Genre
	Moods        []TrackMood
}

type TrackArtist struct {
	Artist   Artist
	Role     ArtistRole
	Position int16
}

type TrackArtistInput struct {
	ArtistID uuid.UUID
	Role     ArtistRole
	Position int16
}

type TrackMood struct {
	Mood   Mood
	Weight float64
}

type TrackMoodInput struct {
	MoodID uuid.UUID
	Weight float64
}

type TrackInput struct {
	ID           uuid.UUID
	AlbumID      *uuid.UUID
	Title        string
	DurationMS   int
	TrackNumber  int16
	DiscNumber   int16
	Explicit     bool
	ReleaseDate  time.Time
	MediaAssetID uuid.UUID
	ContentID    uuid.UUID
	Artists      []TrackArtistInput
	GenreIDs     []uuid.UUID
	Moods        []TrackMoodInput
}

type PageRequest struct {
	Limit  int
	Offset int
}

type Page[T any] struct {
	Items  []T
	Limit  int
	Offset int
	Total  int64
}

type TrackSort string

const (
	TrackSortNewest    TrackSort = "newest"
	TrackSortOldest    TrackSort = "oldest"
	TrackSortTitleAsc  TrackSort = "title_asc"
	TrackSortTitleDesc TrackSort = "title_desc"
)

type TrackQuery struct {
	Page        PageRequest
	Title       string
	ArtistID    *uuid.UUID
	AlbumID     *uuid.UUID
	GenreID     *uuid.UUID
	MoodID      *uuid.UUID
	ReleaseYear *int
	Explicit    *bool
	Sort        TrackSort
}

type TrackIDValidation struct {
	ValidIDs   []uuid.UUID
	MissingIDs []uuid.UUID
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string {
	return err.Field + ": " + err.Reason
}

func (err *ValidationError) Unwrap() error {
	return ErrInvalid
}
