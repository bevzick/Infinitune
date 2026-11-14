package devseed

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestGenerateWAVCreatesDeterministicPlayablePCM(t *testing.T) {
	melody := []float64{220, 277.18, 329.63}
	first := generateWAV(melody)
	second := generateWAV(melody)
	wantSize := wavHeaderSize + sampleRate*demoDurationMS/1000*bitsPerSample/8
	if len(first) != wantSize || !bytes.Equal(first, second) {
		t.Fatalf("generated WAV size = %d, want %d; deterministic = %t",
			len(first), wantSize, bytes.Equal(first, second))
	}
	if string(first[0:4]) != "RIFF" || string(first[8:12]) != "WAVE" ||
		string(first[36:40]) != "data" {
		t.Fatalf("invalid WAV header: %q %q %q", first[0:4], first[8:12], first[36:40])
	}
	if binary.LittleEndian.Uint32(first[24:28]) != sampleRate ||
		binary.LittleEndian.Uint16(first[34:36]) != bitsPerSample ||
		int(binary.LittleEndian.Uint32(first[40:44])) != len(first)-wavHeaderSize {
		t.Fatalf("invalid WAV format fields")
	}
}
