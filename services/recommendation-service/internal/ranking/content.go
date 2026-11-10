package ranking

import (
	"context"

	contentv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/content/v1"
	"github.com/google/uuid"
)

type ContentClient struct {
	client contentv1.ContentServiceClient
}

func NewContentClient(client contentv1.ContentServiceClient) *ContentClient {
	return &ContentClient{client: client}
}
func (client *ContentClient) Allowed(ctx context.Context, contentID uuid.UUID) (bool, error) {
	response, err := client.client.CheckTrackPlayback(ctx, &contentv1.CheckTrackPlaybackRequest{ReleaseTrackId: contentID.String()})
	if err != nil {
		return false, err
	}
	return response.GetAllowed(), nil
}
