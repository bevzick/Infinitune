package coveringest

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os/exec"
	"strconv"
	"time"
)

const maxCoverEdge = 8192
const maxCoverPixels = 16_000_000

func validateImage(data []byte, expected string) error {
	sniffed := ""
	switch {
	case len(data) > 3 && bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		sniffed = "image/jpeg"
	case len(data) > 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}):
		sniffed = "image/png"
	case len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		sniffed = "image/webp"
	}
	if expected != sniffed {
		return ErrInvalid
	}
	if expected == "image/webp" {
		return validateWebP(data)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (expected == "image/jpeg" && format != "jpeg") || (expected == "image/png" && format != "png") {
		return ErrInvalid
	}
	if !validDimensions(config.Width, config.Height) {
		return ErrInvalid
	}
	// Decode after checking dimensions so truncated and forged images are rejected.
	_, _, err = image.Decode(bytes.NewReader(data))
	if err != nil {
		return ErrInvalid
	}
	return nil
}

func validDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= maxCoverEdge && height <= maxCoverEdge &&
		width <= maxCoverPixels/height
}

func validateWebP(data []byte) error {
	// ffprobe/ffmpeg are already part of the Media Service image and can
	// validate both lossy and lossless WebP without a new Go dependency.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-f", "image2pipe",
		"-select_streams", "v:0", "-show_entries", "stream=codec_name,width,height",
		"-of", "json", "-i", "pipe:0")
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.Output()
	if err != nil {
		return ErrInvalid
	}
	var report struct {
		Streams []struct {
			Codec  string      `json:"codec_name"`
			Width  json.Number `json:"width"`
			Height json.Number `json:"height"`
		} `json:"streams"`
	}
	dec := json.NewDecoder(bytes.NewReader(output))
	dec.UseNumber()
	if err = dec.Decode(&report); err != nil || len(report.Streams) != 1 || report.Streams[0].Codec != "webp" {
		return ErrInvalid
	}
	width, e1 := strconv.Atoi(string(report.Streams[0].Width))
	height, e2 := strconv.Atoi(string(report.Streams[0].Height))
	if e1 != nil || e2 != nil || !validDimensions(width, height) {
		return ErrInvalid
	}
	// Decode one frame with the format forced, blocking URL/protocol injection.
	decodeCtx, decodeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer decodeCancel()
	decode := exec.CommandContext(decodeCtx, "ffmpeg", "-v", "error", "-f", "image2pipe",
		"-i", "pipe:0", "-frames:v", "1", "-f", "null", "-")
	decode.Stdin = bytes.NewReader(data)
	if err := decode.Run(); err != nil {
		return ErrInvalid
	}
	return nil
}
