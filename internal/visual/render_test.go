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

func anyNonSpace(g [][]Cell) bool {
	for _, row := range g {
		for _, c := range row {
			if c.R != 0 && c.R != ' ' {
				return true
			}
		}
	}
	return false
}

func anyColor(g [][]Cell) bool {
	for _, row := range g {
		for _, c := range row {
			if c.Fg != 0 {
				return true
			}
		}
	}
	return false
}

func rowString(row []Cell) string {
	b := make([]rune, len(row))
	for i, c := range row {
		if c.R == 0 {
			b[i] = ' '
		} else {
			b[i] = c.R
		}
	}
	return string(b)
}

func TestRenderAllModes(t *testing.T) {
	r := NewRenderer()
	frame := testFrame()
	for m := ModeSpectrum; m < modeCount; m++ {
		r.mode = m
		g := r.Render(frame, 80, 24)
		if !anyNonSpace(g[1 : len(g)-1]) {
			t.Errorf("mode %v: body is empty", m)
		}
	}
}

func TestRenderImmersiveHidesHUD(t *testing.T) {
	r := NewRenderer()
	r.immersive = true
	g := r.Render(testFrame(), 80, 24)
	if strings.Contains(rowString(g[0]), "antiwave") {
		t.Errorf("immersive mode should hide the header")
	}
}

func TestRenderColorTheme(t *testing.T) {
	r := NewRenderer()
	r.mono = false
	r.theme = 1
	g := r.Render(testFrame(), 80, 24)
	if !anyColor(g) {
		t.Errorf("color theme should set foreground colors")
	}
}

func TestModeByName(t *testing.T) {
	if m, ok := ModeByName("waterfall"); !ok || m != ModeWaterfall {
		t.Errorf("ModeByName(waterfall) = %v, %v", m, ok)
	}
}
