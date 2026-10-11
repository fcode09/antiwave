//go:build !windows && !linux

package audio

import "fmt"

func newSystemCapture(push func([]float32)) (*captureHandle, error) {
	return nil, fmt.Errorf("captura de audio del sistema no soportada en este sistema operativo")
}
