package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SourceRepository struct {
	database mediaDatabase
}

var _ source.Store = (*SourceRepository)(nil)

func NewSourceRepository(database mediaDatabase) *SourceRepository {
	return &SourceRepository{database: database}
}

func (repository *SourceRepository) Create(
	ctx context.Context,
	record source.Record,
) (source.Record, bool, error) {
	if record.MediaAssetID == uuid.Nil {
		return source.Record{}, false, fmt.Errorf("%w: media_asset_id must be a UUID", source.ErrInvalid)
	}
	if err := source.ValidateReference(record.Reference); err != nil {
		return source.Record{}, false, err
	}
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return source.Record{}, false, fmt.Errorf("encode media source metadata: %w", err)
	}
	provider, externalID, sourceURL, licenseName, licenseURL, attribution,
		allowsStreaming, allowsRedistribution :=
		sourceColumns(record.Reference)
	stored, err := scanSource(repository.database.QueryRow(ctx, `
		INSERT INTO media.asset_sources (
			media_asset_id, source_kind, provider, external_id, source_url,
			license_name, license_url, attribution, allows_streaming,
			allows_redistribution, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (media_asset_id) DO NOTHING
		RETURNING media_asset_id, source_kind, provider, external_id, source_url,
		          license_name, license_url, attribution, allows_streaming,
		          allows_redistribution, metadata, created_at
	`, record.MediaAssetID, record.Kind, provider, externalID, sourceURL,
		licenseName, licenseURL, attribution, allowsStreaming,
		allowsRedistribution, metadata))
	if err == nil {
		return stored, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return source.Record{}, false, fmt.Errorf("create media asset source: %w", err)
	}
	stored, err = repository.ByMediaAsset(ctx, record.MediaAssetID)
	if err != nil {
		return source.Record{}, false, err
	}
	if !sameSourceReference(stored.Reference, record.Reference) {
		return source.Record{}, false, source.ErrConflict
	}
	return stored, false, nil
}

func (repository *SourceRepository) ByMediaAsset(
	ctx context.Context,
	mediaAssetID uuid.UUID,
) (source.Record, error) {
	if mediaAssetID == uuid.Nil {
		return source.Record{}, fmt.Errorf("%w: media_asset_id must be a UUID", source.ErrInvalid)
	}
	record, err := scanSource(repository.database.QueryRow(ctx, `
		SELECT media_asset_id, source_kind, provider, external_id, source_url,
		       license_name, license_url, attribution, allows_streaming,
		       allows_redistribution, metadata, created_at
		FROM media.asset_sources
		WHERE media_asset_id = $1
	`, mediaAssetID))
	if errors.Is(err, pgx.ErrNoRows) {
		return source.Record{}, source.ErrNotFound
	}
	if err != nil {
		return source.Record{}, fmt.Errorf("query media asset source: %w", err)
	}
	return record, nil
}

func scanSource(row pgx.Row) (source.Record, error) {
	var (
		record                                source.Record
		provider, externalID, sourceURL       *string
		licenseName, licenseURL, attribution  *string
		allowsStreaming, allowsRedistribution *bool
		metadata                              []byte
	)
	err := row.Scan(
		&record.MediaAssetID, &record.Kind, &provider, &externalID, &sourceURL,
		&licenseName, &licenseURL, &attribution, &allowsStreaming,
		&allowsRedistribution, &metadata, &record.CreatedAt,
	)
	if err != nil {
		return source.Record{}, err
	}
	if provider != nil {
		record.Provider = source.ProviderName(*provider)
	}
	if externalID != nil {
		record.ExternalID = *externalID
	}
	if sourceURL != nil {
		record.SourceURL = *sourceURL
	}
	if licenseName != nil {
		record.License.Name = *licenseName
	}
	if licenseURL != nil {
		record.License.URL = *licenseURL
	}
	if attribution != nil {
		record.License.Attribution = *attribution
	}
	if allowsStreaming != nil {
		record.License.AllowsStreaming = *allowsStreaming
	}
	if allowsRedistribution != nil {
		record.License.AllowsRedistribution = *allowsRedistribution
	}
	if err = json.Unmarshal(metadata, &record.Metadata); err != nil {
		return source.Record{}, fmt.Errorf("decode media source metadata: %w", err)
	}
	return record, nil
}

func sourceColumns(reference source.Reference) (
	provider, externalID, sourceURL, licenseName, licenseURL, attribution,
	allowsStreaming, allowsRedistribution any,
) {
	if reference.Kind != source.KindExternalProvider {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}
	return string(reference.Provider), reference.ExternalID, reference.SourceURL,
		reference.License.Name, reference.License.URL,
		nullableString(reference.License.Attribution),
		reference.License.AllowsStreaming, reference.License.AllowsRedistribution
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func sameSourceReference(left, right source.Reference) bool {
	return left.Kind == right.Kind &&
		left.Provider == right.Provider &&
		left.ExternalID == right.ExternalID &&
		left.SourceURL == right.SourceURL &&
		left.License == right.License &&
		maps.Equal(left.Metadata, right.Metadata)
}
