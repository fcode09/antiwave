// Package visual dibuja la música en la terminal: múltiples modos de
// visualización (ondas, barras, cascada, VU, X-Y), temas de color y un HUD con
// caracteres de terminal (box-drawing + bloques, sin emojis).
package visual

import (
	"fmt"
	"math"
	"math/cmplx"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/madelynnblue/go-dsp/fft"
)

// fftSize es el tamaño de la FFT usada por los modos espectrales.
const fftSize = 2048

// ---------------------------------------------------------------------------
// Glifos de terminal (sin emojis).
// ---------------------------------------------------------------------------

const (
	ramp      = "▁▂▃▄▅▆▇█"
	blockFull = '█'
	peakGlyph = '▔'
	zeroGlyph = '·'
)

var rampRunes = []rune(ramp)

// cell es una celda de la terminal: una runa y un color ANSI-256 (0 = default).
type cell struct {
	r rune
	c int
}

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
// Modos de visualización.
// ---------------------------------------------------------------------------

type Mode int

const (
	ModeWaveform Mode = iota
	ModeSpectrum
	ModeWaterfall
	ModeVU
	ModeXY
)

func (m Mode) String() string {
	switch m {
	case ModeWaveform:
		return "ONDAS"
	case ModeSpectrum:
		return "BARRAS"
	case ModeWaterfall:
		return "CASCADA"
	case ModeVU:
		return "VU"
	case ModeXY:
		return "X-Y"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// Temas de color.
// ---------------------------------------------------------------------------

type Theme struct {
	Name   string
	Colors []int // índices ANSI-256, de baja a alta intensidad
}

var Themes = []Theme{
	{"mono", nil},
	{"termal", []int{232, 88, 160, 196, 202, 208, 214, 220, 226, 231}},
	{"aurora", []int{232, 17, 25, 39, 45, 50, 79, 85, 159, 195}},
	{"matriz", []int{232, 22, 28, 34, 40, 46, 82, 118, 154, 190}},
	{"oceano", []int{232, 17, 18, 19, 20, 21, 33, 45, 51, 87}},
}

func (t Theme) colorIndex(v float64) int {
	if len(t.Colors) == 0 {
		return 0
	}
	i := int(v*float64(len(t.Colors)-1) + 0.5)
	if i < 0 {
		i = 0
	} else if i >= len(t.Colors) {
		i = len(t.Colors) - 1
	}
	return t.Colors[i]
}

// ---------------------------------------------------------------------------
// Renderer (estado persistente entre frames).
// ---------------------------------------------------------------------------

type Renderer struct {
	mode    Mode
	theme   int
	mono    bool
	fall    [][]float32 // historial de la cascada (nuevo primero)
	fallW   int
	vuPeaks []float32
}

func NewRenderer() *Renderer {
	return &Renderer{mode: ModeWaveform, theme: 0, mono: true}
}

func (r *Renderer) Mode() Mode { return r.mode }
func (r *Renderer) NextMode() {
	r.mode = (r.mode + 1) % 5
}
func (r *Renderer) NextTheme() {
	r.theme = (r.theme + 1) % len(Themes)
	r.mono = false
}
func (r *Renderer) ToggleMono() { r.mono = !r.mono }
func (r *Renderer) ThemeName() string {
	if r.mono {
		return "mono"
	}
	return Themes[r.theme].Name
}

func (r *Renderer) themeObj() Theme { return Themes[r.theme] }

// shade convierte una intensidad [0,1] en una celda: rampa de bloques (mono) o
// bloque con color. Pensado para mapas de calor (cascada).
func (r *Renderer) shade(v float64) cell {
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	if r.mono {
		return cell{r: rampRune(v)}
	}
	return cell{r: blockFull, c: r.themeObj().colorIndex(v)}
}

// bar devuelve una celda de barra sólida: bloque lleno (la altura transmite la
// amplitud), con color según v en los temas de color.
func (r *Renderer) bar(v float64) cell {
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	if r.mono {
		return cell{r: blockFull}
	}
	return cell{r: blockFull, c: r.themeObj().colorIndex(v)}
}

// ---------------------------------------------------------------------------
// Frame: entrada de cada frame.
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

func (r *Renderer) Render(f Frame, w, h int) string {
	if w < 20 {
		w = 20
	}
	if h < 6 {
		h = 6
	}
	bodyH := h - 2
	if bodyH < 1 {
		bodyH = 1
	}

	grid := makeGrid(bodyH, w)

	var mags []float64
	switch r.mode {
	case ModeWaveform:
		r.drawWaveform(grid, f, w, bodyH)
	case ModeSpectrum:
		mags = computeSpectrum(f.Samples, f.Inverted)
		r.drawSpectrum(grid, f, mags, w, bodyH)
	case ModeWaterfall:
		mags = computeSpectrum(f.Samples, f.Inverted)
		r.drawWaterfall(grid, f, mags, w, bodyH)
	case ModeVU:
		mags = computeSpectrum(f.Samples, f.Inverted)
		r.drawVU(grid, f, mags, w, bodyH)
	case ModeXY:
		r.drawXY(grid, f, w, bodyH)
	}

	peak := peakLevel(f.Samples)
	var dom float64
	if mags != nil {
		dom = dominantFreq(mags, f.SampleRate)
	}

	var b strings.Builder
	b.WriteString("\x1b[H")
	b.WriteString(boxLine('┌', '┐', r.header(f), w))
	b.WriteString("\x1b[K\r\n")
	for _, row := range grid {
		b.WriteString(serializeRow(row))
		b.WriteString("\x1b[K\r\n")
	}
	b.WriteString(boxLine('└', '┘', r.footer(f, peak, dom), w))
	b.WriteString("\x1b[K")
	b.WriteString("\x1b[J")
	return b.String()
}

func (r *Renderer) header(f Frame) string {
	state := ">"
	if f.Paused {
		state = "||"
	}
	phase := "+"
	if f.Inverted {
		phase = "~"
	}
	total := formatDur(f.Total)
	if f.Total == 0 {
		total = "∞"
	}
	mode := colorStr("1;36", r.mode.String()) // cian
	return fmt.Sprintf(" antiwave · [%s] %s · %dHz · %s · fase:%s · %s/%s ",
		state, f.Name, f.SampleRate, mode, phase, formatDur(f.Elapsed), total)
}

func (r *Renderer) footer(f Frame, peak float32, dom float64) string {
	domStr := "-"
	if dom > 0 {
		domStr = fmt.Sprintf("%.0fHz", dom)
	}
	peakStr := toDB(peak)
	return fmt.Sprintf(" [Tab]modo [t]tema:%s [m]mono [o]fase [espacio]pausa [q]salir · pico:%s dom:%s ",
		r.ThemeName(), peakStr, domStr)
}

// ---------------------------------------------------------------------------
// Helpers de render.
// ---------------------------------------------------------------------------

func makeGrid(h, w int) [][]cell {
	g := make([][]cell, h)
	for i := range g {
		g[i] = make([]cell, w)
		for j := range g[i] {
			g[i][j] = cell{r: ' '}
		}
	}
	return g
}

func serializeRow(row []cell) string {
	var b strings.Builder
	prev := -1
	for _, c := range row {
		if c.c != prev {
			if c.c == 0 {
				b.WriteString("\x1b[0m")
			} else {
				b.WriteString("\x1b[38;5;" + strconv.Itoa(c.c) + "m")
			}
			prev = c.c
		}
		b.WriteRune(c.r)
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

func boxLine(l, rr rune, content string, w int) string {
	inner := w - 2
	fill := inner - visibleLen(content)
	if fill < 0 {
		fill = 0
	}
	return string(l) + content + strings.Repeat("─", fill) + string(rr)
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visibleLen(s string) int {
	return utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
}

func colorStr(code, s string) string {
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func formatDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

// ---------------------------------------------------------------------------
// Análisis de señal.
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
