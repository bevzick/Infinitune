package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogv1 "github.com/bevzick/Infinitune/services/history-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const lookupTimeout = 3 * time.Second

type trackClient interface {
	GetTracks(context.Context, *catalogv1.GetTracksRequest, ...grpc.CallOption) (*catalogv1.GetTracksResponse, error)
}

type Client struct {
	client  trackClient
	timeout time.Duration
}

func NewClient(client trackClient) *Client { return &Client{client: client, timeout: lookupTimeout} }

func (client *Client) Duration(ctx context.Context, trackID uuid.UUID) (int, error) {
	callCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	response, err := client.client.GetTracks(callCtx, &catalogv1.GetTracksRequest{TrackIds: []string{trackID.String()}})
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if status.Code(err) == codes.NotFound {
			return 0, history.ErrTrackNotFound
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			return 0, fmt.Errorf("%w: GetTracks", history.ErrCatalogTimeout)
		}
		return 0, fmt.Errorf("%w: GetTracks: %v", history.ErrCatalogUnavailable, err)
	}
	if len(response.GetTracks()) != 1 || response.GetTracks()[0].GetId() != trackID.String() ||
		response.GetTracks()[0].GetDurationMs() <= 0 {
		return 0, fmt.Errorf("%w: Catalog returned an invalid track", history.ErrCatalogUnavailable)
	}
	return int(response.GetTracks()[0].GetDurationMs()), nil
}

var _ history.TrackCatalog = (*Client)(nil)
