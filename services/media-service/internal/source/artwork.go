package source

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// ArtworkFromHTTP copies a bounded provider response into memory so its actual
// byte count and MIME type are known before Content creates an upload session.
func ArtworkFromHTTP(response *http.Response, filenameBase string) (ArtworkStream, error) {
	if response == nil || response.Body == nil {
		return ArtworkStream{}, fmt.Errorf("%w: empty artwork response", ErrProviderUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ArtworkStream{}, fmt.Errorf("%w: artwork returned %s", ErrProviderUnavailable, response.Status)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return ArtworkStream{}, fmt.Errorf("%w: invalid artwork content type", ErrProviderUnavailable)
	}
	extension := map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp",
	}[mediaType]
	if extension == "" {
		return ArtworkStream{}, fmt.Errorf("%w: unsupported artwork content type %q", ErrInvalidStream, mediaType)
	}
	if response.ContentLength > MaximumArtworkSizeBytes {
		return ArtworkStream{}, fmt.Errorf("%w: artwork is too large", ErrInvalidStream)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, MaximumArtworkSizeBytes+1))
	if err != nil {
		return ArtworkStream{}, fmt.Errorf("%w: read artwork: %v", ErrProviderUnavailable, err)
	}
	stream := ArtworkStream{
		Body: io.NopCloser(bytes.NewReader(payload)), Filename: filenameBase + extension,
		MIMEType: mediaType, SizeBytes: int64(len(payload)),
	}
	if err = ValidateArtworkStream(stream); err != nil {
		_ = stream.Body.Close()
		return ArtworkStream{}, err
	}
	return stream, nil
}
