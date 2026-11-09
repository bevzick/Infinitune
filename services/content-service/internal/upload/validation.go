package upload

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maximumOriginalNameLength = 255

var allowedMIMETypesByExtension = map[string]map[string]struct{}{
	".wav":  {"audio/wav": {}, "audio/x-wav": {}},
	".flac": {"audio/flac": {}, "audio/x-flac": {}},
	".mp3":  {"audio/mpeg": {}},
	".m4a":  {"audio/mp4": {}, "audio/x-m4a": {}},
}

type validatedInput struct {
	ReleaseTrackID uuid.UUID
	IdempotencyKey uuid.UUID
	OriginalName   string
	Extension      string
	MIMEType       string
	SizeBytes      int64
}

func validateCreateInput(input CreateInput) (validatedInput, error) {
	if input.ReleaseTrackID == uuid.Nil {
		return validatedInput{}, validationError("release_track_id", "must be a UUID")
	}
	if input.IdempotencyKey == uuid.Nil {
		return validatedInput{}, validationError("idempotency_key", "must be a UUID")
	}
	name := strings.TrimSpace(input.OriginalName)
	if name == "" {
		return validatedInput{}, validationError("original_name", "must not be blank")
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > maximumOriginalNameLength {
		return validatedInput{}, validationError("original_name", "must contain at most 255 characters")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return validatedInput{}, validationError("original_name", "must not contain a path")
	}
	extension := strings.ToLower(filepath.Ext(name))
	allowedMIMETypes, exists := allowedMIMETypesByExtension[extension]
	if !exists {
		return validatedInput{}, validationError("original_name", "must use wav, flac, mp3, or m4a")
	}
	mediaType, parameters, err := mime.ParseMediaType(strings.TrimSpace(input.MIMEType))
	if err != nil || len(parameters) != 0 {
		return validatedInput{}, validationError("mime_type", "must be a supported audio media type")
	}
	mediaType = strings.ToLower(mediaType)
	if _, exists = allowedMIMETypes[mediaType]; !exists {
		return validatedInput{}, validationError(
			"mime_type", fmt.Sprintf("does not match the %s extension", extension),
		)
	}
	if input.SizeBytes < 1 || input.SizeBytes > MaximumAudioSizeBytes {
		return validatedInput{}, validationError(
			"size_bytes", fmt.Sprintf("must be between 1 and %d", MaximumAudioSizeBytes),
		)
	}
	return validatedInput{
		ReleaseTrackID: input.ReleaseTrackID, IdempotencyKey: input.IdempotencyKey,
		OriginalName: name, Extension: extension, MIMEType: mediaType,
		SizeBytes: input.SizeBytes,
	}, nil
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
