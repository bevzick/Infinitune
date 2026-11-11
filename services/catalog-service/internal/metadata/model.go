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

type LyricsType string

const (
	LyricsTypePlain        LyricsType = "plain"
	LyricsTypeSynchronized LyricsType = "synchronized"
)

type LyricsSourceType string

const (
	LyricsSourceArtist           LyricsSourceType = "artist"
	LyricsSourcePublisher        LyricsSourceType = "publisher"
	LyricsSourceLicensedProvider LyricsSourceType = "licensed_provider"
	LyricsSourcePublicDomain     LyricsSourceType = "public_domain"
)

type TrackLyrics struct {
	ID               uuid.UUID
	TrackID          uuid.UUID
	Type             LyricsType
	Language         string
	Content          string
	SourceType       LyricsSourceType
	SourceName       string
	CopyrightNotice  string
	Official         bool
	RightsAuthorized bool
	Published        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TrackLyricsInput struct {
	ID               uuid.UUID
	TrackID          uuid.UUID
	Type             LyricsType
	Language         string
	Content          string
	SourceType       LyricsSourceType
	SourceName       string
	CopyrightNotice  string
	Official         bool
	RightsAuthorized bool
	Published        bool
}

type PublicationInput struct {
	SourceReleaseID uuid.UUID
	Artist          ArtistInput
	Album           *AlbumInput
	Tracks          []TrackInput
}

type PublishedTrack struct {
	ContentTrackID uuid.UUID
	CatalogTrackID uuid.UUID
}

type Publication struct {
	SourceReleaseID uuid.UUID
	ArtistID        uuid.UUID
	AlbumID         *uuid.UUID
	Tracks          []PublishedTrack
	PublishedAt     time.Time
}

type Archive struct {
	SourceReleaseID uuid.UUID
	ArchivedAt      time.Time
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

type FilterLogic string

const (
	FilterLogicAnd FilterLogic = "AND"
	FilterLogicOr  FilterLogic = "OR"
)

type TrackFilterField string

const (
	TrackFilterTitle       TrackFilterField = "title"
	TrackFilterArtistID    TrackFilterField = "artist_id"
	TrackFilterAlbumID     TrackFilterField = "album_id"
	TrackFilterGenreID     TrackFilterField = "genre_id"
	TrackFilterMoodID      TrackFilterField = "mood_id"
	TrackFilterReleaseYear TrackFilterField = "release_year"
	TrackFilterDurationMS  TrackFilterField = "duration_ms"
	TrackFilterExplicit    TrackFilterField = "explicit"
)

type TrackFilterOperator string

const (
	TrackFilterEquals             TrackFilterOperator = "equals"
	TrackFilterContains           TrackFilterOperator = "contains"
	TrackFilterIn                 TrackFilterOperator = "in"
	TrackFilterGreaterThan        TrackFilterOperator = "greater_than"
	TrackFilterGreaterThanOrEqual TrackFilterOperator = "greater_than_or_equal"
	TrackFilterLessThan           TrackFilterOperator = "less_than"
	TrackFilterLessThanOrEqual    TrackFilterOperator = "less_than_or_equal"
	TrackFilterBetween            TrackFilterOperator = "between"
)

type TrackFilterValueKind string

const (
	TrackFilterValueText         TrackFilterValueKind = "text"
	TrackFilterValueUUID         TrackFilterValueKind = "uuid"
	TrackFilterValueUUIDList     TrackFilterValueKind = "uuid_list"
	TrackFilterValueInteger      TrackFilterValueKind = "integer"
	TrackFilterValueIntegerRange TrackFilterValueKind = "integer_range"
	TrackFilterValueBoolean      TrackFilterValueKind = "boolean"
)

type IntegerRange struct {
	Minimum int
	Maximum int
}

type TrackFilterValue struct {
	Kind         TrackFilterValueKind
	Text         string
	UUID         uuid.UUID
	UUIDs        []uuid.UUID
	Integer      int
	IntegerRange IntegerRange
	Boolean      bool
}

func TextFilterValue(value string) TrackFilterValue {
	return TrackFilterValue{Kind: TrackFilterValueText, Text: value}
}

func UUIDFilterValue(value uuid.UUID) TrackFilterValue {
	return TrackFilterValue{Kind: TrackFilterValueUUID, UUID: value}
}

func UUIDListFilterValue(values ...uuid.UUID) TrackFilterValue {
	return TrackFilterValue{
		Kind: TrackFilterValueUUIDList, UUIDs: append([]uuid.UUID(nil), values...),
	}
}

func IntegerFilterValue(value int) TrackFilterValue {
	return TrackFilterValue{Kind: TrackFilterValueInteger, Integer: value}
}

func IntegerRangeFilterValue(minimum, maximum int) TrackFilterValue {
	return TrackFilterValue{
		Kind:         TrackFilterValueIntegerRange,
		IntegerRange: IntegerRange{Minimum: minimum, Maximum: maximum},
	}
}

func BooleanFilterValue(value bool) TrackFilterValue {
	return TrackFilterValue{Kind: TrackFilterValueBoolean, Boolean: value}
}

type TrackFilter struct {
	Field    TrackFilterField
	Operator TrackFilterOperator
	Value    TrackFilterValue
}

type DynamicTrackQuery struct {
	Page    PageRequest
	Logic   FilterLogic
	Filters []TrackFilter
	Sort    TrackSort
}

type TrackIDValidation struct {
	ValidIDs   []uuid.UUID
	MissingIDs []uuid.UUID
}

type AlbumIDValidation struct {
	ValidIDs   []uuid.UUID
	MissingIDs []uuid.UUID
}

type ArtistIDValidation struct {
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
