package processing

import (
	"bytes"
	"encoding/binary"
	"os/exec"
	"testing"
)

func TestFFmpegTranscoderProducesPlaybackProfiles(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}
	result, err := NewFFmpegTranscoder("ffmpeg", "ffprobe").Transcode(
		t.Context(), bytes.NewReader(testWAV(t)),
	)
	if err != nil {
		t.Fatalf("Transcode() error = %v", err)
	}
	defer result.Close()
	if result.DurationMS < 990 || result.DurationMS > 1010 || len(result.Variants) != 2 {
		t.Fatalf("Transcode() = duration %d, variants %d",
			result.DurationMS, len(result.Variants))
	}
	for index, bitrate := range []int{128, 320} {
		variant := result.Variants[index]
		if variant.BitrateKbps != bitrate || variant.Codec != "mp3" ||
			variant.Container != "mp3" || variant.SampleRate != 48000 ||
			variant.Channels != 2 || variant.SizeBytes <= 0 {
			t.Fatalf("variant %d = %+v", index, variant)
		}
	}
}

func testWAV(t *testing.T) []byte {
	t.Helper()
	const sampleRate = 8000
	const samples = sampleRate
	dataSize := samples * 2
	buffer := new(bytes.Buffer)
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(36+dataSize))
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(2))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(16))
	buffer.WriteString("data")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(dataSize))
	buffer.Write(make([]byte, dataSize))
	return buffer.Bytes()
}
