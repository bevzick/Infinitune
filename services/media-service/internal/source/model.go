package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MaximumExternalIDLength  = 512
	MaximumMetadataEntries   = 32
	MaximumMetadataKeySize   = 64
	MaximumMetadataValueSize = 1024
	MaximumAudioSizeBytes    = 500 * 1024 * 1024
)

var (
	ErrInvalid             = errors.New("invalid media source")
	ErrUnsupportedProvider = errors.New("unsupported media source provider")
	ErrProviderConflict    = errors.New("media source provider is already registered")
	ErrInvalidStream       = errors.New("invalid provider audio stream")
	ErrNotFound            = errors.New("media source not found")
	ErrConflict            = errors.New("media asset source conflicts with stored provenance")
)

type Kind string

const (
	KindLocalDevice      Kind = "local_device"
	KindServerStorage    Kind = "server_storage"
	KindExternalProvider Kind = "external_provider"
)

type ProviderName string

const (
	ProviderJamendo         ProviderName = "jamendo"
	ProviderInternetArchive ProviderName = "internet_archive"
)

type License struct {
	Name                 string
	URL                  string
	Attribution          string
	AllowsStreaming      bool
	AllowsRedistribution bool
}

type Reference struct {
	Kind       Kind
	Provider   ProviderName
	ExternalID string
	SourceURL  string
	License    License
	Metadata   map[string]string
}

type Record struct {
	MediaAssetID uuid.UUID
	Reference
	CreatedAt time.Time
}

type Track struct {
	ExternalID string
	Title      string
	Artist     string
	PageURL    string
	License    License
	Metadata   map[string]string
}

type AudioStream struct {
	Body      io.ReadCloser
	Filename  string
	MIMEType  string
	SizeBytes int64
}

type Provider interface {
	Name() ProviderName
	Resolve(context.Context, string) (Track, error)
	Open(context.Context, string) (AudioStream, error)
}

type Store interface {
	Create(context.Context, Record) (Record, bool, error)
	ByMediaAsset(context.Context, uuid.UUID) (Record, error)
}

func ValidateReference(reference Reference) error {
	if reference.Kind != KindLocalDevice &&
		reference.Kind != KindServerStorage &&
		reference.Kind != KindExternalProvider {
		return validationError("kind", "is unsupported")
	}
	if err := validateMetadata(reference.Metadata); err != nil {
		return err
	}
	if reference.Kind != KindExternalProvider {
		if reference.Provider != "" || reference.ExternalID != "" ||
			reference.SourceURL != "" || reference.License != (License{}) {
			return validationError(
				"provider",
				"is only allowed for an external_provider source",
			)
		}
		return nil
	}
	if !reference.Provider.Supported() {
		return fmt.Errorf("%w: %s", ErrUnsupportedProvider, reference.Provider)
	}
	if !validExternalID(reference.ExternalID) {
		return validationError("external_id", "must be a non-blank provider identifier")
	}
	if err := validateHTTPSURL("source_url", reference.SourceURL); err != nil {
		return err
	}
	if strings.TrimSpace(reference.License.Name) == "" {
		return validationError("license.name", "must not be blank")
	}
	if err := validateHTTPSURL("license.url", reference.License.URL); err != nil {
		return err
	}
	return nil
}

func (provider ProviderName) Supported() bool {
	return provider == ProviderJamendo || provider == ProviderInternetArchive
}

func ValidateTrack(track Track) error {
	if !validExternalID(track.ExternalID) {
		return validationError("external_id", "must be a non-blank provider identifier")
	}
	if strings.TrimSpace(track.Title) == "" {
		return validationError("title", "must not be blank")
	}
	if strings.TrimSpace(track.Artist) == "" {
		return validationError("artist", "must not be blank")
	}
	if err := validateHTTPSURL("page_url", track.PageURL); err != nil {
		return err
	}
	if strings.TrimSpace(track.License.Name) == "" {
		return validationError("license.name", "must not be blank")
	}
	if err := validateHTTPSURL("license.url", track.License.URL); err != nil {
		return err
	}
	return validateMetadata(track.Metadata)
}

func ValidateAudioStream(stream AudioStream) error {
	if stream.Body == nil {
		return fmt.Errorf("%w: body is required", ErrInvalidStream)
	}
	if strings.TrimSpace(stream.Filename) == "" {
		return fmt.Errorf("%w: filename is required", ErrInvalidStream)
	}
	if strings.ContainsAny(stream.Filename, "/\\\r\n\t") {
		return fmt.Errorf("%w: filename must not contain a path", ErrInvalidStream)
	}
	switch strings.ToLower(strings.TrimSpace(stream.MIMEType)) {
	case "audio/wav", "audio/x-wav", "audio/flac", "audio/x-flac",
		"audio/mpeg", "audio/mp4", "audio/x-m4a":
	default:
		return fmt.Errorf("%w: MIME type %q is unsupported", ErrInvalidStream, stream.MIMEType)
	}
	if stream.SizeBytes < 1 || stream.SizeBytes > MaximumAudioSizeBytes {
		return fmt.Errorf("%w: size must be between 1 and %d bytes",
			ErrInvalidStream, MaximumAudioSizeBytes)
	}
	return nil
}

func validateMetadata(metadata map[string]string) error {
	if len(metadata) > MaximumMetadataEntries {
		return validationError("metadata", "contains too many entries")
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" || len(key) > MaximumMetadataKeySize ||
			strings.ContainsAny(key, "\r\n\t") {
			return validationError("metadata", "contains an invalid key")
		}
		if len(value) > MaximumMetadataValueSize {
			return validationError("metadata."+key, "value is too long")
		}
	}
	return nil
}

func validateHTTPSURL(field, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || parsed.Fragment != "" {
		return validationError(field, "must be an HTTPS URL without credentials or fragment")
	}
	return nil
}

func validExternalID(value string) bool {
	return value == strings.TrimSpace(value) && value != "" &&
		len(value) <= MaximumExternalIDLength &&
		!strings.ContainsAny(value, "\r\n\t")
}

func validationError(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalid, field, reason)
}
