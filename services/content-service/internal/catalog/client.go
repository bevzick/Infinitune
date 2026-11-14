package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/content-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrUnavailable = errors.New("catalog service unavailable")
	ErrConflict    = errors.New("catalog publication conflict")
	ErrInvalid     = errors.New("invalid catalog response")
	ErrNotApproved = errors.New("catalog has not observed release approval")
)

const requestTimeout = 5 * time.Second

type PublicationInput struct {
	SourceReleaseID uuid.UUID
	ArtistID        uuid.UUID
	ArtistName      string
	AlbumID         *uuid.UUID
	Title           string
	Type            release.Type
	ReleaseDate     time.Time
	CoverURL        string
	Tracks          []TrackInput
}

type TrackInput struct {
	ID             uuid.UUID
	ContentTrackID uuid.UUID
	Title          string
	DurationMS     int
	TrackNumber    int32
	DiscNumber     int32
	Explicit       bool
	MediaAssetID   uuid.UUID
}

type PublishedTrack struct {
	ContentTrackID uuid.UUID
	CatalogTrackID uuid.UUID
}

type Publication struct {
	ArtistID    uuid.UUID
	AlbumID     *uuid.UUID
	Tracks      []PublishedTrack
	PublishedAt time.Time
}

type Client struct {
	client catalogv1.CatalogServiceClient
}

func NewClient(client catalogv1.CatalogServiceClient) *Client {
	return &Client{client: client}
}

func (client *Client) Archive(ctx context.Context, sourceReleaseID uuid.UUID) error {
	if sourceReleaseID == uuid.Nil {
		return ErrInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := client.client.ArchiveRelease(
		callCtx, &catalogv1.ArchiveReleaseRequest{SourceReleaseId: sourceReleaseID.String()},
	)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return ErrInvalid
		case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
			return fmt.Errorf("archive release: %w", ErrUnavailable)
		default:
			return fmt.Errorf("archive release: %w", err)
		}
	}
	if response == nil || response.GetArchivedAt() == nil ||
		!response.GetArchivedAt().IsValid() {
		return ErrInvalid
	}
	return nil
}

func (client *Client) Publish(
	ctx context.Context,
	input PublicationInput,
) (Publication, error) {
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request := &catalogv1.PublishReleaseRequest{
		SourceReleaseId: input.SourceReleaseID.String(),
		Artist:          &catalogv1.PublishArtist{Id: input.ArtistID.String(), Name: input.ArtistName},
		Tracks:          make([]*catalogv1.PublishTrack, 0, len(input.Tracks)),
	}
	date := publicationDate(input.ReleaseDate)
	if input.AlbumID != nil {
		request.Album = &catalogv1.PublishAlbum{
			Id: input.AlbumID.String(), Title: input.Title, AlbumType: string(input.Type),
			ReleaseDate: date, CoverUrl: input.CoverURL,
		}
	}
	for _, track := range input.Tracks {
		request.Tracks = append(request.Tracks, &catalogv1.PublishTrack{
			Id: track.ID.String(), ContentTrackId: track.ContentTrackID.String(),
			Title: track.Title, DurationMs: int32(track.DurationMS),
			TrackNumber: track.TrackNumber, DiscNumber: track.DiscNumber,
			Explicit: track.Explicit, ReleaseDate: date,
			MediaAssetId: track.MediaAssetID.String(),
		})
	}
	response, err := client.client.PublishRelease(callCtx, request)
	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists, codes.Aborted:
			return Publication{}, ErrConflict
		case codes.InvalidArgument:
			return Publication{}, ErrInvalid
		case codes.FailedPrecondition:
			return Publication{}, ErrNotApproved
		case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
			return Publication{}, fmt.Errorf("publish release: %w", ErrUnavailable)
		default:
			return Publication{}, fmt.Errorf("publish release: %w", err)
		}
	}
	return parsePublication(response)
}

func parsePublication(response *catalogv1.PublishReleaseResponse) (Publication, error) {
	if response == nil || response.GetPublishedAt() == nil ||
		!response.GetPublishedAt().IsValid() {
		return Publication{}, ErrInvalid
	}
	artistID, err := parseID(response.GetArtistId())
	if err != nil {
		return Publication{}, ErrInvalid
	}
	result := Publication{ArtistID: artistID, PublishedAt: response.GetPublishedAt().AsTime()}
	if response.GetAlbumId() != "" {
		albumID, parseErr := parseID(response.GetAlbumId())
		if parseErr != nil {
			return Publication{}, ErrInvalid
		}
		result.AlbumID = &albumID
	}
	result.Tracks = make([]PublishedTrack, 0, len(response.GetTracks()))
	seen := make(map[uuid.UUID]struct{}, len(response.GetTracks()))
	for _, track := range response.GetTracks() {
		if track == nil {
			return Publication{}, ErrInvalid
		}
		contentID, contentErr := parseID(track.GetContentTrackId())
		catalogID, catalogErr := parseID(track.GetCatalogTrackId())
		if contentErr != nil || catalogErr != nil {
			return Publication{}, ErrInvalid
		}
		if _, exists := seen[contentID]; exists {
			return Publication{}, ErrInvalid
		}
		seen[contentID] = struct{}{}
		result.Tracks = append(result.Tracks, PublishedTrack{
			ContentTrackID: contentID, CatalogTrackID: catalogID,
		})
	}
	return result, nil
}

func parseID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, ErrInvalid
	}
	return id, nil
}

func publicationDate(value time.Time) *catalogv1.Date {
	return &catalogv1.Date{
		Year: int32(value.Year()), Month: int32(value.Month()), Day: int32(value.Day()),
	}
}
