// Package visual dibuja la música en la terminal: ondas (waveform) y barras
// (espectro FFT), usando solo secuencias ANSI y caracteres de bloque Unicode.
package visual

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"

	"github.com/madelynnblue/go-dsp/fft"
)

// View es el modo de visualización.
type View int

const (
	Waveform View = iota
	Spectrum
)

const block = '█'

// Params es la entrada de cada frame.
type Params struct {
	Samples    []float32
	SampleRate int
	Played     int // índice de la muestra actual (playhead)
	Inverted   bool
	Paused     bool
	View       View
	Total      int
}

// Render devuelve un frame completo listo para imprimir en la terminal.
// w y h son el ancho y alto en caracteres.
func Render(p Params, w, h int) string {
	if w < 10 {
		w = 10
	}
	if h < 4 {
		h = 4
	}
	bodyH := h - 2
	if bodyH < 1 {
		bodyH = 1
	}

	var b strings.Builder
	b.WriteString("\x1b[H") // cursor arriba

	// Encabezado.
	b.WriteString(header(p))
	b.WriteString("\x1b[K\r\n")

	// Cuerpo (grid de runas).
	grid := make([][]rune, bodyH)
	for i := range grid {
		grid[i] = make([]rune, w)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	if p.View == Spectrum {
		drawSpectrum(grid, p, w, bodyH)
	} else {
		drawWaveform(grid, p, w, bodyH)
	}

	for _, row := range grid {
		b.WriteString(string(row))
		b.WriteString("\x1b[K\r\n")
	}

	// Pie con ayuda.
	b.WriteString(" [espacio] pausa   [o] opuesta   [b] barras   [w] ondas   [q] salir ")
	b.WriteString("\x1b[K")
	b.WriteString("\x1b[J") // limpiar resto

	return b.String()
}

func header(p Params) string {
	state := "▶"
	if p.Paused {
		state = "⏸"
	}
	view := "ONDAS"
	if p.View == Spectrum {
		view = "BARRAS"
	}
	phase := "\x1b[90mnormal\x1b[0m"
	if p.Inverted {
		phase = "\x1b[1;31mOPUESTA\x1b[0m"
	}
	t := formatTime(p.Played, p.SampleRate)
	total := formatTime(p.Total, p.SampleRate)
	return fmt.Sprintf(" %s antiwave  |  %s  |  %s/%s  |  fase: %s ",
		state, view, t, total, phase)
}

func drawWaveform(grid [][]rune, p Params, w, h int) {
	center := h / 2
	for c := 0; c < w; c++ {
		grid[center][c] = '·'
	}

	spc := p.SampleRate / 50
	if spc < 1 {
		spc = 1
	}

	for c := 0; c < w; c++ {
		colEnd := p.Played - (w-1-c)*spc
		colStart := colEnd - spc
		if colEnd <= 0 {
			continue
		}
		if colStart < 0 {
			colStart = 0
		}
		mn, mx := minMax(p.Samples, colStart, colEnd)
		if p.Inverted {
			mn, mx = -mx, -mn
		}
		top := rowFor(mx, h)
		bot := rowFor(mn, h)
		if top > bot {
			top, bot = bot, top
		}
		for r := top; r <= bot; r++ {
			if r >= 0 && r < h {
				grid[r][c] = block
			}
		}
	}
}

func drawSpectrum(grid [][]rune, p Params, w, h int) {
	const fftSize = 2048

	end := p.Played
	start := end - fftSize
	if start < 0 {
		start = 0
	}

	buf := make([]float64, fftSize)
	n := end - start
	for i := 0; i < n; i++ {
		buf[i] = float64(p.Samples[start+i])
	}
	if p.Inverted {
		for i := range buf {
			buf[i] = -buf[i]
		}
	}
	// Ventana de Hann para reducir fugas espectrales.
	for i := 0; i < n; i++ {
		buf[i] *= 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n)))
	}

	spec := fft.FFTReal(buf)

	half := fftSize/2 + 1
	fmin := 20.0
	fmax := float64(p.SampleRate) / 2.0
	if fmax <= fmin {
		fmax = fmin + 1
	}
	norm := float64(fftSize) / 2.0

	for c := 0; c < w; c++ {
		// Bandas logarítmicas de frecuencia.
		f0 := fmin * math.Pow(fmax/fmin, float64(c)/float64(w))
		f1 := fmin * math.Pow(fmax/fmin, float64(c+1)/float64(w))
		bin0 := int(f0 * fftSize / float64(p.SampleRate))
		bin1 := int(f1 * fftSize / float64(p.SampleRate))
		if bin0 >= half {
			break
		}
		if bin1 <= bin0 {
			bin1 = bin0 + 1
		}
		if bin1 > half {
			bin1 = half
		}

		var mag float64
		for k := bin0; k < bin1; k++ {
			if m := cmplx.Abs(spec[k]); m > mag {
				mag = m
			}
		}

		height := barHeight(mag, norm, h)
		for r := h - 1; r >= h-height; r-- {
			grid[r][c] = block
		}
	}
}

// barHeight convierte una magnitud FFT a un número de filas (escala perceptual).
func barHeight(mag, norm float64, h int) int {
	v := mag / norm
	if v > 1 {
		v = 1
	}
	if v < 0 {
		v = 0
	}
	v = math.Sqrt(v) // realza los niveles bajos
	return int(math.Round(v * float64(h)))
}

func rowFor(a float32, h int) int {
	n := float64(a+1) / 2.0 // [0,1]
	r := int(math.Round(n * float64(h-1)))
	return h - 1 - r
}

func minMax(s []float32, a, b int) (float32, float32) {
	mn, mx := s[a], s[a]
	for i := a + 1; i < b; i++ {
		if v := s[i]; v < mn {
			mn = v
		} else if v > mx {
			mx = v
		}
	}
	return mn, mx
}

func formatTime(samples, sr int) string {
	if sr <= 0 {
		return "0:00"
	}
	sec := samples / sr
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}
