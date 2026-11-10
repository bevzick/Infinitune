package grpcapi

import (
	"context"
	"testing"

	signalv1 "github.com/bevzick/Infinitune/services/history-service/api/signal/v1"
	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
)

func TestListListeningSignals(t *testing.T) {
	trackID := uuid.New()
	server := NewSignalServer(historyStub{trackID: trackID})
	response, err := server.ListListeningSignals(context.Background(), &signalv1.ListListeningSignalsRequest{UserId: uuid.NewString(), Limit: 25})
	if err != nil {
		t.Fatalf("ListListeningSignals() error = %v", err)
	}
	if response.GetItems()[0].GetTrackId() != trackID.String() || !response.GetItems()[0].GetCompleted() {
		t.Fatalf("ListListeningSignals() = %+v", response)
	}
}

type historyStub struct{ trackID uuid.UUID }

func (stub historyStub) List(context.Context, uuid.UUID, history.ListRequest) (history.Page, error) {
	return history.Page{Items: []history.Event{{TrackID: stub.trackID, ListenedMS: 45000, Completed: true}}}, nil
}
