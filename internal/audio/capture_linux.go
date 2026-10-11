//go:build linux

package audio

import (
	"fmt"

	"github.com/jfreymuth/pulse"
)

func newSystemCapture(push func([]float32)) (*captureHandle, error) {
	c, err := pulse.NewClient(pulse.ClientApplicationName("antiwave"))
	if err != nil {
		return nil, fmt.Errorf("conectar a PulseAudio/PipeWire: %w", err)
	}

	sink, err := c.DefaultSink()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("sink por defecto: %w", err)
	}

	sr := sink.SampleRate()
	if sr <= 0 {
		sr = 44100
	}

	stream, err := c.NewRecord(
		pulse.Float32Writer(func(b []float32) (int, error) {
			push(b)
			return len(b), nil
		}),
		pulse.RecordMonitor(sink),
		pulse.RecordMono,
		pulse.RecordSampleRate(sr),
		pulse.RecordLatency(0.02),
	)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("abrir flujo de grabación: %w", err)
	}

	// Usar la frecuencia negociada real.
	actualRate := stream.SampleRate()
	if actualRate <= 0 {
		actualRate = sr
	}

	stream.Start()

	stop := func() {
		stream.Stop()
		stream.Close()
		c.Close()
	}

	return &captureHandle{
		sampleRate: actualRate,
		name:       sink.Name(),
		stop:       stop,
	}, nil
}
