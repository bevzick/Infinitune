package grpcapi

import (
	"context"
	"testing"

	signalv1 "github.com/bevzick/Infinitune/services/interaction-service/api/signal/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
)

func TestListPreferenceSignals(t *testing.T) {
	trackID, albumID, artistID := uuid.New(), uuid.New(), uuid.New()
	server := NewSignalServer(interactionsStub{trackID: trackID, albumID: albumID, artistID: artistID})
	response, err := server.ListPreferenceSignals(context.Background(), &signalv1.ListPreferenceSignalsRequest{UserId: uuid.NewString(), LimitPerType: 25})
	if err != nil {
		t.Fatalf("ListPreferenceSignals() error = %v", err)
	}
	if response.GetLikedTrackIds()[0] != trackID.String() || response.GetLikedAlbumIds()[0] != albumID.String() || response.GetFollowedArtistIds()[0] != artistID.String() {
		t.Fatalf("ListPreferenceSignals() = %+v", response)
	}
}

type interactionsStub struct{ trackID, albumID, artistID uuid.UUID }

func (stub interactionsStub) TrackLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error) {
	return interaction.LikePage{Items: []interaction.Like{{TargetID: stub.trackID}}}, nil
}
func (stub interactionsStub) AlbumLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error) {
	return interaction.LikePage{Items: []interaction.Like{{TargetID: stub.albumID}}}, nil
}
func (stub interactionsStub) ArtistFollows(context.Context, uuid.UUID, interaction.PageRequest) (interaction.FollowPage, error) {
	return interaction.FollowPage{Items: []interaction.Follow{{ArtistID: stub.artistID}}}, nil
}
