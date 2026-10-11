package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// decodeWAV parses a WAV file (PCM 8/16/24/32 bits or IEEE float 32/64) and
// converts it to mono float32.
func decodeWAV(path string) (*Decoded, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var riffID, waveID [4]byte
	var riffSize uint32
	if err := binary.Read(f, binary.LittleEndian, &riffID); err != nil {
		return nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &riffSize); err != nil {
		return nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &waveID); err != nil {
		return nil, err
	}
	if string(riffID[:]) != "RIFF" || string(waveID[:]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV (missing RIFF/WAVE header)")
	}

	var (
		audioFormat   uint16
		numChannels   int
		sampleRate    int
		bitsPerSample int
		data          []byte
	)

	for {
		var chunkID [4]byte
		var chunkSize uint32
		if err := binary.Read(f, binary.LittleEndian, &chunkID); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &chunkSize); err != nil {
			return nil, err
		}

		switch string(chunkID[:]) {
		case "fmt ":
			var hdr struct {
				AudioFormat   uint16
				NumChannels   uint16
				SampleRate    uint32
				ByteRate      uint32
				BlockAlign    uint16
				BitsPerSample uint16
			}
			if err := binary.Read(f, binary.LittleEndian, &hdr); err != nil {
				return nil, err
			}
			audioFormat = hdr.AudioFormat
			numChannels = int(hdr.NumChannels)
			sampleRate = int(hdr.SampleRate)
			bitsPerSample = int(hdr.BitsPerSample)
			// Skip extra fmt chunk bytes (WAVE_FORMAT_EXTENSIBLE, cbSize, ...).
			if chunkSize > 16 {
				if _, err := io.CopyN(io.Discard, f, int64(chunkSize-16)); err != nil {
					return nil, err
				}
			}
		case "data":
			data = make([]byte, chunkSize)
			if _, err := io.ReadFull(f, data); err != nil {
				return nil, err
			}
		default:
			if _, err := io.CopyN(io.Discard, f, int64(chunkSize)); err != nil {
				return nil, err
			}
		}

		// WAV chunks are word-aligned (pad byte if the size is odd).
		if chunkSize%2 == 1 {
			if _, err := io.CopyN(io.Discard, f, 1); err != nil {
				return nil, err
			}
		}
	}

	if sampleRate == 0 || numChannels == 0 || bitsPerSample == 0 || len(data) == 0 {
		return nil, fmt.Errorf("incomplete WAV or no audio data")
	}

	bytesPerSample := bitsPerSample / 8
	if bytesPerSample == 0 {
		return nil, fmt.Errorf("invalid bits per sample: %d", bitsPerSample)
	}
	blockAlign := numChannels * bytesPerSample
	frames := len(data) / blockAlign

	samples := make([]float32, 0, frames)
	for i := 0; i+blockAlign <= len(data); i += blockAlign {
		var mono float64
		for ch := 0; ch < numChannels; ch++ {
			off := i + ch*bytesPerSample
			mono += sampleToFloat(data[off:off+bytesPerSample], audioFormat, bitsPerSample)
		}
		mono /= float64(numChannels)
		samples = append(samples, float32(mono))
	}

	return &Decoded{Samples: samples, SampleRate: sampleRate}, nil
}

// sampleToFloat converts a raw sample to a float64 in [-1, 1].
func sampleToFloat(b []byte, format uint16, bits int) float64 {
	// IEEE float
	if format == 3 {
		switch bits {
		case 32:
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
		case 64:
			return math.Float64frombits(binary.LittleEndian.Uint64(b))
		}
	}
	// Integer PCM
	switch bits {
	case 8: // unsigned
		return float64(int(b[0])-128) / 128.0
	case 16:
		return float64(int16(binary.LittleEndian.Uint16(b))) / 32768.0
	case 24: // signed 24-bit little-endian
		u := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
		if u&0x800000 != 0 {
			u |= 0xFF000000
		}
		return float64(int32(u)) / 8388608.0
	case 32:
		return float64(int32(binary.LittleEndian.Uint32(b))) / 2147483648.0
	}
	return 0
}
