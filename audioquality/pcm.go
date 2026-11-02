package audioquality

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// MeasurePCM16LE measures levels; it does NOT estimate SNR, speech presence or intelligibility.
func MeasurePCM16LE(samples []byte, sampleRate int) (Signal, error) {
	if sampleRate < 8000 || sampleRate > 192000 {
		return Signal{}, errors.New("unsupported sample rate")
	}
	if len(samples) == 0 || len(samples)%2 != 0 {
		return Signal{}, errors.New("empty or truncated PCM16 data")
	}
	var sum float64
	var clipped int
	for i := 0; i < len(samples); i += 2 {
		n := int16(binary.LittleEndian.Uint16(samples[i : i+2]))
		v := float64(n) / 32768
		sum += v * v
		if n >= 32760 || n <= -32760 {
			clipped++
		}
	}
	total := len(samples) / 2
	rms := math.Sqrt(sum / float64(total))
	rmsDB := -200.0
	if rms > 0 {
		rmsDB = 20 * math.Log10(rms)
	}
	return Signal{RMSdBFS: rmsDB, ClippedFraction: float64(clipped) / float64(total), DurationSeconds: float64(total) / float64(sampleRate)}, nil
}

// WAVPCM16Mono reads a conventional little-endian RIFF/WAVE file. Cap caller's bytes.
func WAVPCM16Mono(src io.Reader) (Signal, error) {
	b, err := io.ReadAll(io.LimitReader(src, 20<<20))
	if err != nil {
		return Signal{}, err
	}
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return Signal{}, errors.New("not a RIFF WAVE file")
	}
	var sampleRate uint32
	var data []byte
	for i := 12; i+8 <= len(b); {
		n := uint64(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		i += 8
		if n > uint64(len(b)-i) {
			return Signal{}, errors.New("truncated WAV chunk")
		}
		chunk := b[i : i+int(n)]
		switch string(b[i-8 : i-4]) {
		case "fmt ":
			if len(chunk) < 16 {
				return Signal{}, errors.New("short WAV format chunk")
			}
			if binary.LittleEndian.Uint16(chunk[0:2]) != 1 || binary.LittleEndian.Uint16(chunk[2:4]) != 1 || binary.LittleEndian.Uint16(chunk[14:16]) != 16 {
				return Signal{}, errors.New("requires mono PCM16 WAV")
			}
			sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
		case "data":
			data = chunk
		}
		i += int(n)
		if n%2 == 1 {
			i++
		}
	}
	if sampleRate == 0 || len(data) == 0 {
		return Signal{}, fmt.Errorf("missing WAV format or data")
	}
	return MeasurePCM16LE(data, int(sampleRate))
}
