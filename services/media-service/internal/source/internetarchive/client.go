package internetarchive

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
	defaultMetadataURL = "https://archive.org/metadata"
	defaultDownloadURL = "https://archive.org/download"
	defaultTimeout     = 20 * time.Second
	maximumResponse    = 8 * 1024 * 1024
)

var ErrUnsupportedLicense = fmt.Errorf("Internet Archive item license is unsupported: %w", source.ErrInvalid)

type Config struct {
	MetadataURL  string
	DownloadURL  string
	Timeout      time.Duration
	AllowedHosts []string
}

type Client struct {
	client       *http.Client
	metadataURL  *url.URL
	downloadURL  *url.URL
	allowedHosts []string
}

var _ source.Provider = (*Client)(nil)

func New(config Config, httpClient *http.Client) (*Client, error) {
	if config.MetadataURL == "" {
		config.MetadataURL = defaultMetadataURL
	}
	if config.DownloadURL == "" {
		config.DownloadURL = defaultDownloadURL
	}
	metadataURL, err := parseBaseURL("Internet Archive metadata URL", config.MetadataURL)
	if err != nil {
		return nil, err
	}
	downloadURL, err := parseBaseURL("Internet Archive download URL", config.DownloadURL)
	if err != nil {
		return nil, err
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	hosts := append([]string(nil), config.AllowedHosts...)
	if len(hosts) == 0 {
		hosts = []string{"archive.org", metadataURL.Hostname(), downloadURL.Hostname()}
	}
	for index, host := range hosts {
		hosts[index] = strings.ToLower(strings.TrimSpace(host))
		if hosts[index] == "" || strings.ContainsAny(hosts[index], "/:@") {
			return nil, fmt.Errorf("invalid Internet Archive host %q", host)
		}
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	clientCopy := *httpClient
	clientCopy.Timeout = config.Timeout
	client := &Client{
		client: &clientCopy, metadataURL: metadataURL, downloadURL: downloadURL,
		allowedHosts: hosts,
	}
	previousPolicy := clientCopy.CheckRedirect
	client.client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := client.validateURL(request.URL); err != nil {
			return err
		}
		if previousPolicy != nil {
			return previousPolicy(request, via)
		}
		if len(via) >= 5 {
			return errors.New("stopped after 5 redirects")
		}
		return nil
	}
	return client, nil
}

func (client *Client) Name() source.ProviderName { return source.ProviderInternetArchive }

func (client *Client) Resolve(ctx context.Context, externalID string) (source.Track, error) {
	item, file, err := client.resolve(ctx, externalID)
	if err != nil {
		return source.Track{}, err
	}
	license, err := archiveLicense(item.Metadata.LicenseURL.First(), item.artist())
	if err != nil {
		return source.Track{}, err
	}
	title := file.Title
	if strings.TrimSpace(title) == "" {
		title = item.Metadata.Title.First()
	}
	artist := file.Artist
	if strings.TrimSpace(artist) == "" {
		artist = item.artist()
	}
	metadata := map[string]string{
		"item_identifier": item.Metadata.Identifier,
		"file_name":       file.Name,
		"format":          file.Format,
	}
	if date := item.Metadata.Date.First(); date != "" {
		metadata["date"] = date
	}
	if file.Track != "" {
		metadata["track_number"] = file.Track
	}
	return source.Track{
		ExternalID: externalID, Title: title, Artist: artist,
		PageURL: "https://archive.org/details/" + url.PathEscape(item.Metadata.Identifier),
		License: license, Metadata: metadata,
	}, nil
}

func (client *Client) Open(ctx context.Context, externalID string) (source.AudioStream, error) {
	item, file, err := client.resolve(ctx, externalID)
	if err != nil {
		return source.AudioStream{}, err
	}
	if _, err = archiveLicense(item.Metadata.LicenseURL.First(), item.artist()); err != nil {
		return source.AudioStream{}, err
	}
	mediaType, extension, err := audioFormat(file)
	if err != nil {
		return source.AudioStream{}, err
	}
	size, err := strconv.ParseInt(file.Size, 10, 64)
	if err != nil || size < 1 || size > source.MaximumAudioSizeBytes {
		return source.AudioStream{}, fmt.Errorf("%w: invalid Internet Archive file size", source.ErrInvalid)
	}
	downloadURL := *client.downloadURL
	downloadURL.Path = strings.TrimRight(downloadURL.Path, "/") + "/" +
		item.Metadata.Identifier + "/" + file.Name
	if err = client.validateURL(&downloadURL); err != nil {
		return source.AudioStream{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return source.AudioStream{}, fmt.Errorf("create Internet Archive download request: %w", err)
	}
	request.Header.Set("Accept", mediaType)
	response, err := client.client.Do(request)
	if err != nil {
		return source.AudioStream{}, fmt.Errorf("%w: download Internet Archive file: %v", source.ErrProviderUnavailable, err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return source.AudioStream{}, fmt.Errorf("%w: download returned %s", source.ErrProviderUnavailable, response.Status)
	}
	if response.ContentLength != size {
		_ = response.Body.Close()
		return source.AudioStream{}, fmt.Errorf("%w: download size changed", source.ErrProviderUnavailable)
	}
	if received, _, parseErr := mime.ParseMediaType(response.Header.Get("Content-Type")); parseErr == nil && received != "application/octet-stream" && received != mediaType {
		_ = response.Body.Close()
		return source.AudioStream{}, fmt.Errorf("%w: download content type changed", source.ErrProviderUnavailable)
	}
	stream := source.AudioStream{
		Body: response.Body, Filename: "internet-archive-" + item.Metadata.Identifier + extension,
		MIMEType: mediaType, SizeBytes: size,
	}
	if err = source.ValidateAudioStream(stream); err != nil {
		_ = response.Body.Close()
		return source.AudioStream{}, err
	}
	return stream, nil
}

func (client *Client) resolve(
	ctx context.Context,
	externalID string,
) (itemDocument, fileRecord, error) {
	identifier, fileName, err := parseExternalID(externalID)
	if err != nil {
		return itemDocument{}, fileRecord{}, err
	}
	endpoint := *client.metadataURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + url.PathEscape(identifier)
	query := endpoint.Query()
	query.Set("extended_err", "1")
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return itemDocument{}, fileRecord{}, fmt.Errorf("create Internet Archive metadata request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return itemDocument{}, fileRecord{}, fmt.Errorf("%w: read Internet Archive metadata: %v", source.ErrProviderUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return itemDocument{}, fileRecord{}, fmt.Errorf("%w: metadata returned %s", source.ErrProviderUnavailable, response.Status)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(payload) > maximumResponse {
		return itemDocument{}, fileRecord{}, fmt.Errorf("%w: invalid metadata response", source.ErrProviderUnavailable)
	}
	if strings.TrimSpace(string(payload)) == "[]" {
		return itemDocument{}, fileRecord{}, source.ErrNotFound
	}
	var item itemDocument
	if err = json.Unmarshal(payload, &item); err != nil {
		return itemDocument{}, fileRecord{}, fmt.Errorf("%w: decode Internet Archive metadata: %v", source.ErrProviderUnavailable, err)
	}
	if item.Error != "" {
		if item.ErrorCode == 104 {
			return itemDocument{}, fileRecord{}, source.ErrNotFound
		}
		return itemDocument{}, fileRecord{}, fmt.Errorf("%w: metadata error %d", source.ErrProviderUnavailable, item.ErrorCode)
	}
	if item.Metadata.Identifier != identifier ||
		(item.Metadata.MediaType != "audio" && item.Metadata.MediaType != "etree") {
		return itemDocument{}, fileRecord{}, fmt.Errorf("%w: item is not public audio", source.ErrInvalid)
	}
	for _, file := range item.Files {
		if file.Name == fileName {
			if file.Source != "original" {
				return itemDocument{}, fileRecord{}, fmt.Errorf("%w: selected file is not original", source.ErrInvalid)
			}
			if _, _, formatErr := audioFormat(file); formatErr != nil {
				return itemDocument{}, fileRecord{}, formatErr
			}
			return item, file, nil
		}
	}
	return itemDocument{}, fileRecord{}, source.ErrNotFound
}

func parseExternalID(value string) (string, string, error) {
	identifier, fileName, found := strings.Cut(value, "/")
	if !found || !validIdentifier(identifier) || !validFileName(fileName) {
		return "", "", fmt.Errorf("%w: expected item-identifier/file-name", source.ErrInvalid)
	}
	return identifier, fileName, nil
}

func validFileName(value string) bool {
	if value == "" || len(value) > 1024 || strings.ContainsAny(value, "\\\r\n\t") ||
		path.Clean(value) != value || strings.HasPrefix(value, "/") {
		return false
	}
	for segment := range strings.SplitSeq(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 100 || !asciiAlphaNumeric(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !asciiAlphaNumeric(character) && character != '_' && character != '-' && character != '.' {
			return false
		}
	}
	return true
}

func asciiAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func audioFormat(file fileRecord) (string, string, error) {
	extension := strings.ToLower(path.Ext(file.Name))
	format := strings.ToLower(file.Format)
	switch {
	case extension == ".mp3" && strings.Contains(format, "mp3"):
		return "audio/mpeg", ".mp3", nil
	case extension == ".flac" && strings.Contains(format, "flac"):
		return "audio/flac", ".flac", nil
	case extension == ".wav" && (strings.Contains(format, "wave") || strings.Contains(format, "wav")):
		return "audio/wav", ".wav", nil
	case extension == ".m4a" && (strings.Contains(format, "mpeg-4") ||
		strings.Contains(format, "mpeg4") || strings.Contains(format, "m4a")):
		return "audio/mp4", ".m4a", nil
	default:
		return "", "", fmt.Errorf("%w: Internet Archive file format is unsupported", source.ErrInvalid)
	}
}

func archiveLicense(rawURL, attribution string) (source.License, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		(!strings.EqualFold(parsed.Hostname(), "creativecommons.org") &&
			!strings.EqualFold(parsed.Hostname(), "www.creativecommons.org")) {
		return source.License{}, ErrUnsupportedLicense
	}
	segments := strings.Split(strings.Trim(path.Clean(parsed.Path), "/"), "/")
	name := ""
	if len(segments) >= 3 && segments[0] == "licenses" && supportedCCCode(segments[1]) {
		name = "CC " + strings.ToUpper(segments[1]) + " " + segments[2]
	} else if len(segments) >= 3 && segments[0] == "publicdomain" && segments[1] == "zero" {
		name = "CC0 " + segments[2]
	} else if len(segments) >= 3 && segments[0] == "publicdomain" && segments[1] == "mark" {
		name = "Public Domain Mark " + segments[2]
	} else {
		return source.License{}, ErrUnsupportedLicense
	}
	parsed.Scheme = "https"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return source.License{
		Name: name, URL: parsed.String(), Attribution: attribution,
		AllowsStreaming: true, AllowsRedistribution: true,
	}, nil
}

func supportedCCCode(code string) bool {
	switch strings.ToLower(code) {
	case "by", "by-sa", "by-nd", "by-nc", "by-nc-sa", "by-nc-nd":
		return true
	default:
		return false
	}
}

func parseBaseURL(name, value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must be HTTPS without credentials, query, or fragment", name)
	}
	return parsed, nil
}

func (client *Client) validateURL(target *url.URL) error {
	if target == nil || target.Scheme != "https" || target.Host == "" ||
		target.User != nil || target.Fragment != "" {
		return fmt.Errorf("%w: unsafe Internet Archive URL", source.ErrProviderUnavailable)
	}
	host := strings.ToLower(target.Hostname())
	for _, allowed := range client.allowedHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: Internet Archive host %q is not allowed", source.ErrProviderUnavailable, host)
}

type itemDocument struct {
	Metadata  itemMetadata `json:"metadata"`
	Files     []fileRecord `json:"files"`
	Error     string       `json:"error"`
	ErrorCode int          `json:"errcode"`
}

func (item itemDocument) artist() string { return item.Metadata.Creator.First() }

type itemMetadata struct {
	Identifier string    `json:"identifier"`
	MediaType  string    `json:"mediatype"`
	Title      textValue `json:"title"`
	Creator    textValue `json:"creator"`
	LicenseURL textValue `json:"licenseurl"`
	Date       textValue `json:"date"`
}

type fileRecord struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Format string `json:"format"`
	Size   string `json:"size"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Track  string `json:"track"`
}

type textValue []string

func (value *textValue) UnmarshalJSON(data []byte) error {
	var single string
	if json.Unmarshal(data, &single) == nil {
		*value = []string{single}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return err
	}
	*value = multiple
	return nil
}

func (value textValue) First() string {
	for _, entry := range value {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
