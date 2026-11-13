package jamendo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/source"
)

const (
	defaultBaseURL  = "https://api.jamendo.com/v3.0"
	defaultTimeout  = 15 * time.Second
	maximumResponse = 1024 * 1024
)

var (
	ErrUnavailable        = errors.New("Jamendo API unavailable")
	ErrDownloadForbidden  = errors.New("Jamendo track download is not allowed")
	ErrUnsupportedLicense = errors.New("Jamendo track license is unsupported")
)

type Config struct {
	ClientID      string
	BaseURL       string
	Timeout       time.Duration
	DownloadHosts []string
}

type Client struct {
	client        *http.Client
	baseURL       *url.URL
	clientID      string
	downloadHosts []string
}

var _ source.Provider = (*Client)(nil)

func New(config Config, httpClient *http.Client) (*Client, error) {
	config.ClientID = strings.TrimSpace(config.ClientID)
	if config.ClientID == "" {
		return nil, fmt.Errorf("Jamendo client ID is required")
	}
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil || baseURL.Scheme != "https" || baseURL.Host == "" ||
		baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("Jamendo API URL must be an HTTPS URL without credentials, query, or fragment")
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	downloadHosts := append([]string(nil), config.DownloadHosts...)
	if len(downloadHosts) == 0 {
		downloadHosts = []string{"storage.jamendo.com"}
	}
	for index, host := range downloadHosts {
		downloadHosts[index] = strings.ToLower(strings.TrimSpace(host))
		if downloadHosts[index] == "" || strings.ContainsAny(downloadHosts[index], "/:@") {
			return nil, fmt.Errorf("invalid Jamendo download host %q", host)
		}
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	clientCopy := *httpClient
	clientCopy.Timeout = config.Timeout
	client := &Client{
		client: &clientCopy, baseURL: baseURL, clientID: config.ClientID,
		downloadHosts: append([]string(nil), downloadHosts...),
	}
	previousRedirectPolicy := clientCopy.CheckRedirect
	client.client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := client.validateHTTPURL(request.URL); err != nil {
			return err
		}
		if previousRedirectPolicy != nil {
			return previousRedirectPolicy(request, via)
		}
		if len(via) >= 5 {
			return errors.New("stopped after 5 redirects")
		}
		return nil
	}
	return client, nil
}

func (client *Client) Name() source.ProviderName {
	return source.ProviderJamendo
}

func (client *Client) Resolve(ctx context.Context, externalID string) (source.Track, error) {
	result, err := client.track(ctx, externalID)
	if err != nil {
		return source.Track{}, err
	}
	license, err := licenseFromURL(result.LicenseURL, result.ArtistName, result.DownloadAllowed)
	if err != nil {
		return source.Track{}, err
	}
	metadata := map[string]string{
		"artist_id":    result.ArtistID,
		"duration_sec": strconv.Itoa(result.Duration),
		"release_date": result.ReleaseDate,
	}
	if result.AlbumID != "" {
		metadata["album_id"] = result.AlbumID
	}
	if result.AlbumName != "" {
		metadata["album_name"] = result.AlbumName
	}
	return source.Track{
		ExternalID: result.ID,
		Title:      result.Name,
		Artist:     result.ArtistName,
		PageURL:    result.ShareURL,
		License:    license,
		Metadata:   metadata,
	}, nil
}

func (client *Client) Open(ctx context.Context, externalID string) (source.AudioStream, error) {
	result, err := client.track(ctx, externalID)
	if err != nil {
		return source.AudioStream{}, err
	}
	if !result.DownloadAllowed || result.DownloadURL == "" {
		return source.AudioStream{}, ErrDownloadForbidden
	}
	downloadURL, err := url.Parse(result.DownloadURL)
	if err != nil {
		return source.AudioStream{}, fmt.Errorf("%w: invalid download URL", ErrUnavailable)
	}
	if err = client.validateDownloadURL(downloadURL); err != nil {
		return source.AudioStream{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return source.AudioStream{}, fmt.Errorf("create Jamendo download request: %w", err)
	}
	request.Header.Set("Accept", "audio/mpeg")
	response, err := client.client.Do(request)
	if err != nil {
		return source.AudioStream{}, fmt.Errorf("%w: download track: %v", ErrUnavailable, err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return source.AudioStream{}, fmt.Errorf("%w: download returned %s", ErrUnavailable, response.Status)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		_ = response.Body.Close()
		return source.AudioStream{}, fmt.Errorf("%w: invalid download content type", ErrUnavailable)
	}
	if response.ContentLength < 1 || response.ContentLength > source.MaximumAudioSizeBytes {
		_ = response.Body.Close()
		return source.AudioStream{}, fmt.Errorf("%w: invalid download size %d",
			ErrUnavailable, response.ContentLength)
	}
	stream := source.AudioStream{
		Body: response.Body, Filename: "jamendo-" + result.ID + extensionFor(mediaType),
		MIMEType: mediaType, SizeBytes: response.ContentLength,
	}
	if err = source.ValidateAudioStream(stream); err != nil {
		_ = response.Body.Close()
		return source.AudioStream{}, err
	}
	return stream, nil
}

func (client *Client) track(ctx context.Context, externalID string) (trackResult, error) {
	if !validTrackID(externalID) {
		return trackResult{}, fmt.Errorf("%w: Jamendo track ID must contain only digits", source.ErrInvalid)
	}
	endpoint := *client.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/tracks/"
	query := endpoint.Query()
	query.Set("client_id", client.clientID)
	query.Set("format", "json")
	query.Set("limit", "1")
	query.Set("id", externalID)
	query.Set("include", "licenses")
	query.Set("audioformat", "mp32")
	query.Set("audiodlformat", "mp32")
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return trackResult{}, fmt.Errorf("create Jamendo track request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return trackResult{}, fmt.Errorf("%w: resolve track: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return trackResult{}, fmt.Errorf("%w: tracks endpoint returned %s", ErrUnavailable, response.Status)
	}
	var document tracksResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maximumResponse))
	if err = decoder.Decode(&document); err != nil {
		return trackResult{}, fmt.Errorf("%w: decode tracks response: %v", ErrUnavailable, err)
	}
	if document.Headers.Status != "success" || document.Headers.Code != 0 {
		return trackResult{}, fmt.Errorf("%w: API code %d: %s",
			ErrUnavailable, document.Headers.Code, document.Headers.ErrorMessage)
	}
	if len(document.Results) == 0 {
		return trackResult{}, source.ErrNotFound
	}
	result := document.Results[0]
	if result.ID != externalID {
		return trackResult{}, fmt.Errorf("%w: API returned track %q for %q",
			ErrUnavailable, result.ID, externalID)
	}
	return result, nil
}

func (client *Client) validateDownloadURL(downloadURL *url.URL) error {
	if err := validateHTTPSURL(downloadURL); err != nil {
		return fmt.Errorf("%w: download URL is not safe", ErrUnavailable)
	}
	host := strings.ToLower(downloadURL.Hostname())
	for _, allowed := range client.downloadHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: download host %q is not allowed", ErrUnavailable, host)
}

func (client *Client) validateHTTPURL(target *url.URL) error {
	if err := validateHTTPSURL(target); err != nil {
		return fmt.Errorf("%w: redirect URL is not safe", ErrUnavailable)
	}
	if strings.EqualFold(target.Hostname(), client.baseURL.Hostname()) {
		return nil
	}
	return client.validateDownloadURL(target)
}

func validateHTTPSURL(target *url.URL) error {
	if target == nil || target.Scheme != "https" || target.Host == "" ||
		target.User != nil || target.Fragment != "" {
		return errors.New("URL must use HTTPS without credentials or a fragment")
	}
	return nil
}

func licenseFromURL(rawURL, artist string, downloadAllowed bool) (source.License, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		!creativeCommonsHost(parsed.Hostname()) {
		return source.License{}, ErrUnsupportedLicense
	}
	segments := strings.Split(strings.Trim(path.Clean(parsed.Path), "/"), "/")
	if len(segments) < 3 || segments[0] != "licenses" {
		return source.License{}, ErrUnsupportedLicense
	}
	code := strings.ToUpper(segments[1])
	version := segments[2]
	if !supportedLicenseCode(code) || version == "" {
		return source.License{}, ErrUnsupportedLicense
	}
	parsed.Scheme = "https"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return source.License{
		Name: "CC " + code + " " + version,
		URL:  parsed.String(), Attribution: artist,
		AllowsStreaming: true, AllowsRedistribution: downloadAllowed,
	}, nil
}

func creativeCommonsHost(host string) bool {
	return strings.EqualFold(host, "creativecommons.org") ||
		strings.EqualFold(host, "www.creativecommons.org")
}

func supportedLicenseCode(code string) bool {
	switch code {
	case "BY", "BY-SA", "BY-ND", "BY-NC", "BY-NC-SA", "BY-NC-ND":
		return true
	default:
		return false
	}
}

func validTrackID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func extensionFor(mediaType string) string {
	switch mediaType {
	case "audio/flac", "audio/x-flac":
		return ".flac"
	default:
		return ".mp3"
	}
}

type tracksResponse struct {
	Headers responseHeaders `json:"headers"`
	Results []trackResult   `json:"results"`
}

type responseHeaders struct {
	Status       string `json:"status"`
	Code         int    `json:"code"`
	ErrorMessage string `json:"error_message"`
}

type trackResult struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Duration        int    `json:"duration"`
	ArtistID        string `json:"artist_id"`
	ArtistName      string `json:"artist_name"`
	AlbumID         string `json:"album_id"`
	AlbumName       string `json:"album_name"`
	ReleaseDate     string `json:"releasedate"`
	LicenseURL      string `json:"license_ccurl"`
	ShareURL        string `json:"shareurl"`
	DownloadURL     string `json:"audiodownload"`
	DownloadAllowed bool   `json:"audiodownload_allowed"`
}
