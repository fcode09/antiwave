package visual

import (
	"math"
	"strings"
	"testing"
	"time"
)

func synthSamples(n, sr int) []float32 {
	s := make([]float32, n)
	for i := range s {
		t := float64(i) / float64(sr)
		s[i] = float32(0.7*math.Sin(2*math.Pi*440*t) + 0.3*math.Sin(2*math.Pi*220*t))
	}
	return s
}

func testFrame() Frame {
	return Frame{
		Samples:    synthSamples(fftSize*2, 44100),
		SampleRate: 44100,
		Name:       "test.wav",
		Elapsed:    time.Second,
		Total:      2 * time.Second,
	}
}

func TestRenderAllModes(t *testing.T) {
	r := NewRenderer()
	frame := testFrame()

	for m := ModeWaveform; m <= ModeXY; m++ {
		r.mode = m
		out := r.Render(frame, 80, 24)
		if !strings.Contains(out, "┌") || !strings.Contains(out, "└") {
			t.Errorf("mode %v: missing HUD frame", m)
		}
		if !strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
			t.Errorf("mode %v: does not draw blocks", m)
		}
	}
}

func TestRenderColorTheme(t *testing.T) {
	r := NewRenderer()
	r.mono = false
	r.theme = 1 // termal
	out := r.Render(testFrame(), 80, 24)
	if !strings.Contains(out, "\x1b[38;5;") {
		t.Errorf("color theme should emit ANSI color sequences")
	}
}

func TestRenderInverted(t *testing.T) {
	r := NewRenderer()
	frame := testFrame()
	frame.Inverted = true
	out := r.Render(frame, 80, 24)
	if !strings.Contains(out, "phase:~") {
		t.Errorf("HUD should indicate inverted phase (~)")
	}
}
