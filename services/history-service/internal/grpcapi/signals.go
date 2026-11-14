package grpcapi

import (
	"context"

	signalv1 "github.com/bevzick/Infinitune/services/history-service/api/signal/v1"
	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maximumSignalLimit = 100

type Histories interface {
	List(context.Context, uuid.UUID, history.ListRequest) (history.Page, error)
}

type SignalServer struct {
	signalv1.UnimplementedSignalServiceServer
	histories Histories
}

func NewSignalServer(histories Histories) *SignalServer { return &SignalServer{histories: histories} }

func (server *SignalServer) ListListeningSignals(
	ctx context.Context,
	request *signalv1.ListListeningSignalsRequest,
) (*signalv1.ListeningSignals, error) {
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil || userID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	limit := int(request.GetLimit())
	if limit == 0 {
		limit = maximumSignalLimit
	}
	if limit < 1 || limit > maximumSignalLimit {
		return nil, status.Error(codes.InvalidArgument, "limit must be from 1 to 100")
	}
	page, err := server.histories.List(ctx, userID, history.ListRequest{Limit: limit})
	if err != nil {
		return nil, status.Error(codes.Internal, "listening signal lookup failed")
	}
	response := &signalv1.ListeningSignals{Items: make([]*signalv1.ListeningSignal, len(page.Items))}
	for index, item := range page.Items {
		response.Items[index] = &signalv1.ListeningSignal{
			TrackId: item.TrackID.String(), ListenedMs: int32(item.ListenedMS), Completed: item.Completed,
		}
	}
	return response, nil
}
