package devseed

import (
	"encoding/binary"
	"math"
)

const (
	demoDurationMS = 6000
	sampleRate     = 16000
	bitsPerSample  = 16
	channelCount   = 1
	wavHeaderSize  = 44
)

func generateWAV(melody []float64) []byte {
	sampleCount := sampleRate * demoDurationMS / 1000
	dataSize := sampleCount * channelCount * bitsPerSample / 8
	result := make([]byte, wavHeaderSize+dataSize)
	copy(result[0:4], "RIFF")
	binary.LittleEndian.PutUint32(result[4:8], uint32(36+dataSize))
	copy(result[8:12], "WAVE")
	copy(result[12:16], "fmt ")
	binary.LittleEndian.PutUint32(result[16:20], 16)
	binary.LittleEndian.PutUint16(result[20:22], 1)
	binary.LittleEndian.PutUint16(result[22:24], channelCount)
	binary.LittleEndian.PutUint32(result[24:28], sampleRate)
	byteRate := sampleRate * channelCount * bitsPerSample / 8
	binary.LittleEndian.PutUint32(result[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(result[32:34], channelCount*bitsPerSample/8)
	binary.LittleEndian.PutUint16(result[34:36], bitsPerSample)
	copy(result[36:40], "data")
	binary.LittleEndian.PutUint32(result[40:44], uint32(dataSize))

	noteSamples := sampleRate / 2
	fadeSamples := sampleRate / 100
	for sampleIndex := range sampleCount {
		notePosition := sampleIndex % noteSamples
		frequency := melody[(sampleIndex/noteSamples)%len(melody)]
		envelope := 1.0
		if notePosition < fadeSamples {
			envelope = float64(notePosition) / float64(fadeSamples)
		}
		remaining := noteSamples - notePosition - 1
		if remaining < fadeSamples {
			envelope *= float64(remaining) / float64(fadeSamples)
		}
		phase := 2 * math.Pi * frequency * float64(notePosition) / sampleRate
		sample := int16(math.Sin(phase) * envelope * 0.25 * math.MaxInt16)
		binary.LittleEndian.PutUint16(result[wavHeaderSize+sampleIndex*2:], uint16(sample))
	}
	return result
}
