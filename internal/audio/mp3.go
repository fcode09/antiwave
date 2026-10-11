package audio

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/hajimehoshi/go-mp3"
)

// decodeMP3 decodes an MP3 to mono float32.
// go-mp3 always outputs 16-bit little-endian stereo (2 channels) PCM.
func decodeMP3(path string) (*Decoded, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, err
	}

	sampleRate := d.SampleRate()

	pcm, err := io.ReadAll(d)
	if err != nil {
		return nil, err
	}

	const channels = 2
	samples := make([]float32, 0, len(pcm)/2/channels)
	for i := 0; i+2*channels <= len(pcm); i += 2 * channels {
		var sum float64
		for ch := 0; ch < channels; ch++ {
			off := i + ch*2
			v := int16(binary.LittleEndian.Uint16(pcm[off : off+2]))
			sum += float64(v) / 32768.0
		}
		samples = append(samples, float32(sum/float64(channels)))
	}

	return &Decoded{Samples: samples, SampleRate: sampleRate}, nil
}
