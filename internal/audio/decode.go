// Package audio decodifica archivos de audio (WAV/MP3) y abstrae la
// reproducción (real por parlantes o "silenciosa" para visualizar sin sonido).
package audio

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Decoded contiene las muestras de audio en mono float32 dentro de [-1, 1]
// y la frecuencia de muestreo original.
type Decoded struct {
	Samples    []float32
	SampleRate int
}

// Decode abre el archivo en path y lo decodifica a mono float32.
// Soportados: .wav y .mp3.
func Decode(path string) (*Decoded, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav", ".wave":
		return decodeWAV(path)
	case ".mp3":
		return decodeMP3(path)
	default:
		return nil, fmt.Errorf("formato no soportado: %q (solo .wav y .mp3)", ext)
	}
}
