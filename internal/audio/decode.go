// Package audio decodes audio files (WAV/MP3) and abstracts playback (real via
// speakers or "silent" for headless visualization), plus system audio capture.
package audio

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Decoded holds mono float32 samples in [-1, 1] and the original sample rate.
type Decoded struct {
	Samples    []float32
	SampleRate int
}

// Decode opens the file at path and decodes it to mono float32.
// Supported formats: .wav and .mp3.
func Decode(path string) (*Decoded, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav", ".wave":
		return decodeWAV(path)
	case ".mp3":
		return decodeMP3(path)
	default:
		return nil, fmt.Errorf("unsupported format: %q (only .wav and .mp3)", ext)
	}
}
