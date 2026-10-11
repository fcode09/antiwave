//go:build !windows && !linux

package audio

import "fmt"

func newSystemCapture(push func([]float32)) (*captureHandle, error) {
	return nil, fmt.Errorf("system audio capture is not supported on this operating system")
}
