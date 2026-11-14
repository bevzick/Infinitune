package processing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type audioProfile struct {
	bitrateKbps int
	sampleRate  int
	channels    int
}

var defaultProfiles = []audioProfile{
	{bitrateKbps: 128, sampleRate: 48000, channels: 2},
	{bitrateKbps: 320, sampleRate: 48000, channels: 2},
}

type FFmpegTranscoder struct {
	ffmpegPath  string
	ffprobePath string
}

func NewFFmpegTranscoder(ffmpegPath, ffprobePath string) *FFmpegTranscoder {
	return &FFmpegTranscoder{ffmpegPath: ffmpegPath, ffprobePath: ffprobePath}
}

func (transcoder *FFmpegTranscoder) Transcode(
	ctx context.Context,
	source io.Reader,
) (TranscodeResult, error) {
	workingDirectory, err := os.MkdirTemp("", "infinitune-media-")
	if err != nil {
		return TranscodeResult{}, fmt.Errorf("create transcoding workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workingDirectory) }
	sourcePath := filepath.Join(workingDirectory, "source")
	if err = copySource(sourcePath, source); err != nil {
		cleanup()
		return TranscodeResult{}, err
	}
	durationMS, err := transcoder.probe(ctx, sourcePath)
	if err != nil {
		cleanup()
		return TranscodeResult{}, err
	}
	result := TranscodeResult{DurationMS: durationMS, close: func() error {
		return os.RemoveAll(workingDirectory)
	}}
	for _, profile := range defaultProfiles {
		variant, transcodeErr := transcoder.transcodeProfile(
			ctx, workingDirectory, sourcePath, profile,
		)
		if transcodeErr != nil {
			_ = result.Close()
			return TranscodeResult{}, transcodeErr
		}
		result.Variants = append(result.Variants, variant)
	}
	return result, nil
}

func copySource(path string, source io.Reader) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create local source audio: %w", err)
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("copy source audio: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close source audio: %w", closeErr)
	}
	return nil
}

func (transcoder *FFmpegTranscoder) probe(ctx context.Context, sourcePath string) (int, error) {
	command := exec.CommandContext(ctx, transcoder.ffprobePath,
		"-v", "error", "-select_streams", "a:0",
		"-show_entries", "format=duration", "-of", "json", sourcePath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return 0, commandError("probe source audio", err, output)
	}
	var response struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err = json.Unmarshal(output, &response); err != nil {
		return 0, fmt.Errorf("decode ffprobe output: %w", err)
	}
	durationSeconds, err := strconv.ParseFloat(response.Format.Duration, 64)
	if err != nil || durationSeconds <= 0 {
		return 0, fmt.Errorf("source audio has invalid duration %q", response.Format.Duration)
	}
	durationMS := int(math.Round(durationSeconds * 1000))
	if durationMS < 1 {
		return 0, fmt.Errorf("source audio duration is too short")
	}
	return durationMS, nil
}

func (transcoder *FFmpegTranscoder) transcodeProfile(
	ctx context.Context,
	workingDirectory string,
	sourcePath string,
	profile audioProfile,
) (GeneratedVariant, error) {
	outputPath := filepath.Join(workingDirectory,
		fmt.Sprintf("mp3-%d.mp3", profile.bitrateKbps))
	command := exec.CommandContext(ctx, transcoder.ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", sourcePath, "-map", "0:a:0", "-map_metadata", "-1", "-vn",
		"-c:a", "libmp3lame", "-b:a", fmt.Sprintf("%dk", profile.bitrateKbps),
		"-ar", strconv.Itoa(profile.sampleRate), "-ac", strconv.Itoa(profile.channels),
		"-threads", "2",
		outputPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return GeneratedVariant{}, commandError(
			fmt.Sprintf("transcode %d kbps variant", profile.bitrateKbps), err, output,
		)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return GeneratedVariant{}, fmt.Errorf("stat transcoded variant: %w", err)
	}
	if info.Size() <= 0 {
		return GeneratedVariant{}, fmt.Errorf("transcoded variant is empty")
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return GeneratedVariant{}, fmt.Errorf("open transcoded variant: %w", err)
	}
	return GeneratedVariant{
		Body: file, Codec: "mp3", Container: "mp3", ContentType: "audio/mpeg",
		BitrateKbps: profile.bitrateKbps, SampleRate: profile.sampleRate,
		Channels: profile.channels, SizeBytes: info.Size(),
	}, nil
}

func commandError(operation string, err error, output []byte) error {
	detail := strings.TrimSpace(string(output))
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	if detail == "" {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: %w: %s", operation, err, detail)
}
