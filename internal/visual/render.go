// Package visual renders music in the terminal: eight visualization modes,
// color themes with truecolor gradients, smoothing and automatic gain, and a
// modern HUD — using terminal characters (blocks, braille, box-drawing), no
// emojis. It produces a grid of cells that the front-end blits to the screen.
package visual

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"time"

	"github.com/madelynnblue/go-dsp/fft"
)

// fftSize is the FFT size used by the spectral modes.
const fftSize = 2048

// Cell is one terminal cell: a rune and an optional 24-bit foreground color
// (0xRRGGBB). Fg == 0 means "use the terminal default".
type Cell struct {
	R  rune
	Fg uint32
}

func rgb(r, g, b uint8) uint32 { return uint32(r)<<16 | uint32(g)<<8 | uint32(b) }

// ---------------------------------------------------------------------------
// Glyphs.
// ---------------------------------------------------------------------------

const (
	blockFull = '█'
	peakGlyph = '▔'
	zeroGlyph = '·'
)

var rampRunes = []rune("▁▂▃▄▅▆▇█")

func rampRune(v float64) rune {
	if v <= 0 {
		return ' '
	}
	i := int(v*7 + 0.5)
	if i < 0 {
		i = 0
	} else if i > 7 {
		i = 7
	}
	return rampRunes[i]
}

// ---------------------------------------------------------------------------
// Modes.
// ---------------------------------------------------------------------------

type Mode int

const (
	ModeSpectrum Mode = iota
	ModeMirror
	ModeWaveform
	ModeOscilloscope
	ModeWaterfall
	ModePhase
	ModeRadial
	ModeLevels
	ModeHalo
	ModeOrbit
	ModeNebula
	modeCount
)

func (m Mode) String() string {
	switch m {
	case ModeSpectrum:
		return "SPECTRUM"
	case ModeMirror:
		return "MIRROR"
	case ModeWaveform:
		return "WAVEFORM"
	case ModeOscilloscope:
		return "OSCILLOSCOPE"
	case ModeWaterfall:
		return "WATERFALL"
	case ModePhase:
		return "PHASE"
	case ModeRadial:
		return "RADIAL"
	case ModeLevels:
		return "LEVELS"
	case ModeHalo:
		return "HALO"
	case ModeOrbit:
		return "ORBIT"
	case ModeNebula:
		return "NEBULA"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// Themes.
// ---------------------------------------------------------------------------

type Theme struct {
	Name   string
	Colors []uint32 // RGB stops, dark (bottom) to bright (top)
}

var Themes = []Theme{
	{"mono", nil},
	{"aurora", []uint32{
		rgb(22, 22, 46), rgb(32, 44, 96), rgb(38, 82, 150), rgb(32, 138, 166),
		rgb(42, 180, 192), rgb(96, 200, 232), rgb(158, 124, 242), rgb(206, 96, 224)}},
	{"ocean", []uint32{
		rgb(10, 22, 64), rgb(20, 44, 104), rgb(22, 74, 152), rgb(30, 112, 202),
		rgb(42, 152, 222), rgb(64, 192, 234), rgb(126, 222, 244), rgb(208, 250, 255)}},
	{"ember", []uint32{
		rgb(44, 12, 12), rgb(96, 28, 12), rgb(156, 48, 12), rgb(206, 84, 18),
		rgb(234, 124, 32), rgb(250, 172, 54), rgb(255, 212, 96), rgb(255, 242, 206)}},
	{"phosphor", []uint32{
		rgb(0, 42, 0), rgb(0, 74, 0), rgb(0, 112, 20), rgb(22, 162, 30),
		rgb(62, 202, 42), rgb(132, 230, 82), rgb(190, 250, 142), rgb(232, 255, 214)}},
	{"ion", []uint32{
		rgb(0, 36, 72), rgb(0, 72, 140), rgb(0, 128, 200), rgb(0, 190, 230),
		rgb(60, 225, 250), rgb(140, 240, 255), rgb(210, 250, 255), rgb(244, 253, 255)}},
	{"ghost", []uint32{
		rgb(28, 32, 42), rgb(56, 62, 78), rgb(104, 112, 134), rgb(150, 158, 182),
		rgb(192, 200, 220), rgb(224, 230, 244), rgb(242, 246, 252), rgb(252, 253, 255)}},
	{"amethyst", []uint32{
		rgb(28, 14, 56), rgb(64, 26, 112), rgb(112, 44, 172), rgb(152, 72, 206),
		rgb(184, 116, 228), rgb(210, 166, 244), rgb(232, 210, 252), rgb(246, 238, 255)}},
}

func lerpColor(a, b uint32, t float64) uint32 {
	ar, ag, ab := (a>>16)&0xff, (a>>8)&0xff, a&0xff
	br, bg, bb := (b>>16)&0xff, (b>>8)&0xff, b&0xff
	r := uint8(float64(ar) + (float64(br)-float64(ar))*t)
	g := uint8(float64(ag) + (float64(bg)-float64(ag))*t)
	bl := uint8(float64(ab) + (float64(bb)-float64(ab))*t)
	return rgb(r, g, bl)
}

func (t Theme) colorAt(v float64) uint32 {
	if len(t.Colors) == 0 {
		return 0
	}
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	x := v * float64(len(t.Colors)-1)
	i := int(x)
	if i >= len(t.Colors)-1 {
		return t.Colors[len(t.Colors)-1]
	}
	return lerpColor(t.Colors[i], t.Colors[i+1], x-float64(i))
}

// ---------------------------------------------------------------------------
// Renderer.
// ---------------------------------------------------------------------------

type Renderer struct {
	mode      Mode
	theme     int
	mono      bool
	immersive bool
	autoGain  bool
	gain      float64

	// Smoothing / peak-hold state (per column).
	smooth []float64
	peaks  []float64
	lastT  time.Time

	// Waterfall history (newest first).
	fall  [][]float32
	fallW int

	// Level-meter peak hold.
	meterPeaks []float64

	// Glow buffer (braille resolution) and animation state for Halo/Orbit/Nebula.
	glow     []float32
	glowW    int
	glowH    int
	glowLast time.Time
	animT    float64
	parts    []particle
}

func NewRenderer() *Renderer {
	return &Renderer{
		mode:     ModeSpectrum,
		theme:    1, // aurora
		mono:     false,
		autoGain: true,
		gain:     1.0,
	}
}

func (r *Renderer) Mode() Mode     { return r.mode }
func (r *Renderer) NextMode()      { r.mode = (r.mode + 1) % modeCount }
func (r *Renderer) PrevMode()      { r.mode = (r.mode + modeCount - 1) % modeCount }
func (r *Renderer) SetMode(m Mode) { r.mode = m }
func (r *Renderer) NextTheme()     { r.theme = (r.theme + 1) % len(Themes); r.mono = false }
func (r *Renderer) SetTheme(i int) {
	if i >= 0 && i < len(Themes) {
		r.theme = i
		r.mono = i == 0
	}
}
func (r *Renderer) SetMonoForFlag(m bool) { r.mono = m }
func (r *Renderer) ToggleMono()           { r.mono = !r.mono }
func (r *Renderer) ToggleImmersive()      { r.immersive = !r.immersive }
func (r *Renderer) SetImmersive(b bool)   { r.immersive = b }
func (r *Renderer) Immersive() bool       { return r.immersive }
func (r *Renderer) ToggleAutoGain()       { r.autoGain = !r.autoGain }
func (r *Renderer) AutoGain() bool        { return r.autoGain }
func (r *Renderer) AdjustGain(d float64) {
	r.gain *= d
	if r.gain < 0.25 {
		r.gain = 0.25
	}
	if r.gain > 16 {
		r.gain = 16
	}
}

// ModeByName resolves a mode by its display name (case-insensitive).
func ModeByName(s string) (Mode, bool) {
	for m := ModeSpectrum; m < modeCount; m++ {
		if strings.EqualFold(m.String(), s) {
			return m, true
		}
	}
	return 0, false
}

// ModeByIndex resolves a 0-based mode index (1..8 keys).
func ModeByIndex(i int) (Mode, bool) {
	if i >= 0 && i < int(modeCount) {
		return Mode(i), true
	}
	return 0, false
}

// ThemeIndexByName resolves a theme by name (case-insensitive).
func ThemeIndexByName(s string) (int, bool) {
	for i, t := range Themes {
		if strings.EqualFold(t.Name, s) {
			return i, true
		}
	}
	return 0, false
}

func (r *Renderer) ThemeName() string {
	if r.mono {
		return "mono"
	}
	return Themes[r.theme].Name
}

func (r *Renderer) themeObj() Theme { return Themes[r.theme] }

// shade maps an intensity [0,1] to a cell: block ramp (mono) or a colored block.
func (r *Renderer) shade(v float64) Cell {
	if r.mono {
		return Cell{R: rampRune(v)}
	}
	return Cell{R: blockFull, Fg: r.themeObj().colorAt(v)}
}

// barCell returns a solid block whose color follows a vertical gradient
// (v = 0 at the bottom, 1 at the top).
func (r *Renderer) barCell(v float64) Cell {
	if r.mono {
		return Cell{R: blockFull}
	}
	return Cell{R: blockFull, Fg: r.themeObj().colorAt(v)}
}

// ---------------------------------------------------------------------------
// Frame.
// ---------------------------------------------------------------------------

type Frame struct {
	Samples    []float32
	SampleRate int
	Inverted   bool
	Paused     bool
	Name       string
	Elapsed    time.Duration
	Total      time.Duration
}

// ---------------------------------------------------------------------------
// Render.
// ---------------------------------------------------------------------------

func (r *Renderer) Render(f Frame, w, h int) [][]Cell {
	if w < 10 {
		w = 10
	}
	if h < 4 {
		h = 4
	}
	grid := makeGrid(h, w)

	if r.immersive {
		r.drawBody(grid, 0, h, f, w)
		return grid
	}

	bodyTop := 1
	bodyH := h - 2
	if bodyH < 1 {
		bodyH = 1
		bodyTop = 0
	}
	r.drawHeader(grid[0], f, w)
	r.drawBody(grid, bodyTop, bodyH, f, w)
	r.drawFooter(grid[h-1], f, w)
	return grid
}

func (r *Renderer) drawBody(grid [][]Cell, top, h int, f Frame, w int) {
	if h < 1 {
		return
	}
	body := grid[top : top+h]

	var mags []float64
	switch r.mode {
	case ModeSpectrum, ModeMirror, ModeRadial, ModeLevels, ModeWaterfall, ModeHalo, ModeOrbit, ModeNebula:
		mags = computeSpectrum(f.Samples, f.Inverted)
	}

	switch r.mode {
	case ModeSpectrum:
		levels, peaks := r.columnLevels(f, mags, barCount(w))
		r.drawSpectrum(body, levels, peaks, h, w)
	case ModeMirror:
		levels, peaks := r.columnLevels(f, mags, barCount(w))
		r.drawMirror(body, levels, peaks, h, w)
	case ModeWaveform:
		r.drawWaveform(body, f, w, h)
	case ModeOscilloscope:
		r.drawOscilloscope(body, f, w, h)
	case ModeWaterfall:
		r.drawWaterfall(body, f, mags, w, h)
	case ModePhase:
		r.drawPhase(body, f, w, h)
	case ModeRadial:
		levels, _ := r.columnLevels(f, mags, barCount(w))
		r.drawRadial(body, levels, w, h)
	case ModeLevels:
		r.drawLevels(body, f, mags, w, h)
	case ModeHalo:
		r.drawHalo(body, f, mags, w, h)
	case ModeOrbit:
		r.drawOrbit(body, f, mags, w, h)
	case ModeNebula:
		r.drawNebula(body, f, mags, w, h)
	}
}

// ---------------------------------------------------------------------------
// HUD.
// ---------------------------------------------------------------------------

var (
	cTitle  = rgb(210, 230, 255)
	cDim    = rgb(120, 130, 145)
	cAccent = rgb(0, 210, 220)
	cLive   = rgb(80, 220, 120)
	cPaused = rgb(240, 180, 60)
)

func (r *Renderer) drawHeader(row []Cell, f Frame, w int) {
	state := "LIVE"
	stateCol := cLive
	if f.Paused {
		state = "PAUSED"
		stateCol = cPaused
	}
	phase := "+"
	if f.Inverted {
		phase = "~"
	}
	total := formatDur(f.Total)
	if f.Total == 0 {
		total = "∞"
	}

	x := 0
	x = writeText(row, x, " antiwave", cTitle, 1)
	x = writeText(row, x+1, "["+state+"]", stateCol, 0)
	x = writeText(row, x+1, truncate(f.Name, 24), cDim, 0)
	x = writeText(row, x+1, r.mode.String(), r.accent(), 1)
	x = writeText(row, x+1, "phase:"+phase+" "+formatDur(f.Elapsed)+"/"+total, cDim, 0)

	right := fmt.Sprintf("%s · %dHz", r.ThemeName(), f.SampleRate)
	if r.autoGain {
		right += " · AUTO"
	}
	xr := w - len([]rune(right))
	if xr > x {
		writeText(row, xr, right, cDim, 0)
	}
}

func (r *Renderer) drawFooter(row []Cell, f Frame, w int) {
	peak := peakLevel(f.Samples)
	dom := 0.0
	if f.SampleRate > 0 && len(f.Samples) >= fftSize {
		mags := computeSpectrum(f.Samples, f.Inverted)
		dom = dominantFreq(mags, f.SampleRate)
	}
	domStr := "-"
	if dom > 0 {
		domStr = fmt.Sprintf("%.0fHz", dom)
	}
	left := "[Tab] mode   [t] theme   [f] focus   [a] auto   [+/-] gain   [q] quit"
	right := fmt.Sprintf("peak %s   dom %s", toDB(peak), domStr)

	x := writeText(row, 0, left, cDim, 0)
	xr := w - len([]rune(right))
	if xr > x {
		writeText(row, xr, right, cDim, 0)
	}
}

func (r *Renderer) accent() uint32 {
	if r.mono {
		return cTitle
	}
	return r.themeObj().colorAt(0.85)
}

// ---------------------------------------------------------------------------
// Text helpers.
// ---------------------------------------------------------------------------

func makeGrid(h, w int) [][]Cell {
	g := make([][]Cell, h)
	for i := range g {
		g[i] = make([]Cell, w)
	}
	return g
}

// writeText writes runes into row starting at x, returning the next column.
func writeText(row []Cell, x int, s string, fg uint32, width int) int {
	for _, r := range s {
		if x >= len(row) {
			return x
		}
		if r == ' ' && width == 0 {
			// Preserve ordinary spaces.
		}
		row[x] = Cell{R: r, Fg: fg}
		x++
	}
	return x
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// ---------------------------------------------------------------------------
// DSP.
// ---------------------------------------------------------------------------

func computeSpectrum(samples []float32, inverted bool) []float64 {
	n := len(samples)
	if n > fftSize {
		n = fftSize
	}
	buf := make([]float64, fftSize)
	start := len(samples) - n
	for i := 0; i < n; i++ {
		buf[i] = float64(samples[start+i])
	}
	if inverted {
		for i := 0; i < n; i++ {
			buf[i] = -buf[i]
		}
	}
	if n > 1 {
		for i := 0; i < n; i++ {
			buf[i] *= 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n)))
		}
	}
	spec := fft.FFTReal(buf)
	half := fftSize/2 + 1
	mags := make([]float64, half)
	for k := 0; k < half; k++ {
		mags[k] = cmplx.Abs(spec[k])
	}
	return mags
}

// columnLevels maps the spectrum to per-column levels (log frequency bins, dB
// scale), applies automatic gain and smoothing, and returns smoothed levels
// plus peak-hold values.
func (r *Renderer) columnLevels(f Frame, mags []float64, n int) (levels, peaks []float64) {
	raw := make([]float64, n)
	half := len(mags)
	norm := float64(fftSize) / 2.0
	for c := 0; c < n; c++ {
		b0, b1 := freqBins(c, n, half, f.SampleRate)
		var m float64
		for k := b0; k < b1; k++ {
			if mags[k] > m {
				m = mags[k]
			}
		}
		amp := m / norm
		// Perceptual dB scaling over a 60 dB range.
		var lvl float64
		if amp > 1e-6 {
			db := 20 * math.Log10(amp)
			lvl = (db + 60) / 60
		}
		raw[c] = lvl
	}

	// Automatic gain.
	if r.autoGain {
		var m float64
		for _, v := range raw {
			if v > m {
				m = v
			}
		}
		if m > 1e-4 {
			desired := 0.85 / m
			if desired > 6 {
				desired = 6
			}
			if desired < 0.4 {
				desired = 0.4
			}
			r.gain += (desired - r.gain) * 0.08
		}
	}
	for c := range raw {
		raw[c] *= r.gain
		if raw[c] < 0 {
			raw[c] = 0
		} else if raw[c] > 1 {
			raw[c] = 1
		}
	}

	// Time-based smoothing + peak hold.
	now := time.Now()
	dt := now.Sub(r.lastT).Seconds()
	if dt <= 0 {
		dt = 1.0 / 30
	} else if dt > 0.1 {
		dt = 0.1
	}
	r.lastT = now
	if len(r.smooth) != n {
		r.smooth = make([]float64, n)
		r.peaks = make([]float64, n)
	}
	attack := 1 - math.Exp(-dt/0.012)
	release := 1 - math.Exp(-dt/0.13)
	peakFall := 0.55
	levels = make([]float64, n)
	peaks = make([]float64, n)
	for c := 0; c < n; c++ {
		s := r.smooth[c]
		t := raw[c]
		if t >= s {
			s += (t - s) * attack
		} else {
			s += (t - s) * release
		}
		r.smooth[c] = s
		p := r.peaks[c]
		if s >= p {
			p = s
		} else {
			p -= peakFall * dt
			if p < s {
				p = s
			}
		}
		r.peaks[c] = p
		levels[c] = s
		peaks[c] = p
	}
	return levels, peaks
}

func freqBins(c, w, half, sampleRate int) (int, int) {
	fmin, fmax := 20.0, float64(sampleRate)/2.0
	if fmax <= fmin {
		fmax = fmin + 1
	}
	f0 := fmin * math.Pow(fmax/fmin, float64(c)/float64(w))
	f1 := fmin * math.Pow(fmax/fmin, float64(c+1)/float64(w))
	b0 := int(f0 * fftSize / float64(sampleRate))
	b1 := int(f1 * fftSize / float64(sampleRate))
	if b0 >= half {
		b0 = half - 1
	}
	if b1 <= b0 {
		b1 = b0 + 1
	}
	if b1 > half {
		b1 = half
	}
	return b0, b1
}

func peakLevel(samples []float32) float32 {
	var p float32
	for _, s := range samples {
		if s < 0 {
			s = -s
		}
		if s > p {
			p = s
		}
	}
	return p
}

func toDB(amp float32) string {
	if amp <= 0 {
		return "-inf"
	}
	db := 20 * math.Log10(float64(amp))
	if db < -60 {
		db = -60
	}
	return fmt.Sprintf("%.0fdB", db)
}

func dominantFreq(mags []float64, sampleRate int) float64 {
	if sampleRate <= 0 || len(mags) < 3 {
		return 0
	}
	best := 1
	for k := 2; k < len(mags); k++ {
		if mags[k] > mags[best] {
			best = k
		}
	}
	if mags[best] < 1e-6 {
		return 0
	}
	return float64(best) * float64(sampleRate) / float64(fftSize)
}

func formatDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}
