package coverupload

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maximumOriginalNameLength = 255

var allowedMIMEByExtension = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

type validatedInput struct {
	ReleaseID      uuid.UUID
	IdempotencyKey uuid.UUID
	OriginalName   string
	Extension      string
	MIMEType       string
	SizeBytes      int64
}

func validateCreateInput(input CreateInput) (validatedInput, error) {
	if input.ReleaseID == uuid.Nil {
		return validatedInput{}, invalidField("release_id", "must be a UUID")
	}
	if input.IdempotencyKey == uuid.Nil {
		return validatedInput{}, invalidField("idempotency_key", "must be a UUID")
	}
	name := strings.TrimSpace(input.OriginalName)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > maximumOriginalNameLength {
		return validatedInput{}, invalidField("original_name", "must contain 1 to 255 characters")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return validatedInput{}, invalidField("original_name", "must not contain a path")
	}
	extension := strings.ToLower(filepath.Ext(name))
	expectedMIME, exists := allowedMIMEByExtension[extension]
	if !exists {
		return validatedInput{}, invalidField("original_name", "must use jpg, jpeg, png, or webp")
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(input.MIMEType))
	if err != nil || len(params) != 0 || strings.ToLower(mediaType) != expectedMIME {
		return validatedInput{}, invalidField("mime_type", "must match the image file extension")
	}
	if input.SizeBytes < 1 || input.SizeBytes > MaximumCoverSizeBytes {
		return validatedInput{}, invalidField("size_bytes", fmt.Sprintf("must be between 1 and %d", MaximumCoverSizeBytes))
	}
	return validatedInput{
		ReleaseID: input.ReleaseID, IdempotencyKey: input.IdempotencyKey,
		OriginalName: name, Extension: extension, MIMEType: expectedMIME, SizeBytes: input.SizeBytes,
	}, nil
}

func invalidField(field, reason string) error { return &ValidationError{Field: field, Reason: reason} }
