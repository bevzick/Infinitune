package grpcapi

import (
	"context"

	signalv1 "github.com/bevzick/Infinitune/services/interaction-service/api/signal/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maximumSignalLimit = 100

type Interactions interface {
	TrackLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error)
	AlbumLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error)
	ArtistFollows(context.Context, uuid.UUID, interaction.PageRequest) (interaction.FollowPage, error)
}

type SignalServer struct {
	signalv1.UnimplementedSignalServiceServer
	interactions Interactions
}

func NewSignalServer(interactions Interactions) *SignalServer {
	return &SignalServer{interactions: interactions}
}

func (server *SignalServer) ListPreferenceSignals(
	ctx context.Context,
	request *signalv1.ListPreferenceSignalsRequest,
) (*signalv1.PreferenceSignals, error) {
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	limit := int(request.GetLimitPerType())
	if limit == 0 {
		limit = maximumSignalLimit
	}
	if limit < 1 || limit > maximumSignalLimit {
		return nil, status.Error(codes.InvalidArgument, "limit_per_type must be from 1 to 100")
	}
	page := interaction.PageRequest{Limit: limit}
	tracks, err := server.interactions.TrackLikes(ctx, userID, page)
	if err != nil {
		return nil, status.Error(codes.Internal, "track signal lookup failed")
	}
	albums, err := server.interactions.AlbumLikes(ctx, userID, page)
	if err != nil {
		return nil, status.Error(codes.Internal, "album signal lookup failed")
	}
	artists, err := server.interactions.ArtistFollows(ctx, userID, page)
	if err != nil {
		return nil, status.Error(codes.Internal, "artist signal lookup failed")
	}
	response := &signalv1.PreferenceSignals{
		LikedTrackIds:     make([]string, len(tracks.Items)),
		LikedAlbumIds:     make([]string, len(albums.Items)),
		FollowedArtistIds: make([]string, len(artists.Items)),
	}
	for index, item := range tracks.Items {
		response.LikedTrackIds[index] = item.TargetID.String()
	}
	for index, item := range albums.Items {
		response.LikedAlbumIds[index] = item.TargetID.String()
	}
	for index, item := range artists.Items {
		response.FollowedArtistIds[index] = item.ArtistID.String()
	}
	return response, nil
}
