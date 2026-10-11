package visual

import (
	"math"
)

// rowFor mapea una amplitud [-1,1] a una fila (0 arriba, h-1 abajo).
func rowFor(a float32, h int) int {
	n := float64(a+1) / 2.0
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

// freqBins devuelve el rango de bins [b0,b1) para una columna c de w, en escala
// logarítmica de frecuencias.
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

// ---------------------------------------------------------------------------
// M1 · Ondas (osciloscopio con envolvente de pico).
// ---------------------------------------------------------------------------

func (r *Renderer) drawWaveform(grid [][]cell, f Frame, w, h int) {
	center := h / 2
	for c := 0; c < w; c++ {
		grid[center][c] = cell{r: zeroGlyph}
	}

	spc := f.SampleRate / 50
	if spc < 1 {
		spc = 1
	}
	n := len(f.Samples)

	for c := 0; c < w; c++ {
		colEnd := n - (w-1-c)*spc
		colStart := colEnd - spc
		if colEnd <= 0 {
			continue
		}
		if colStart < 0 {
			colStart = 0
		}
		mn, mx := minMax(f.Samples, colStart, colEnd)
		if f.Inverted {
			mn, mx = -mx, -mn
		}
		top := rowFor(mx, h)
		bot := rowFor(mn, h)
		if top > bot {
			top, bot = bot, top
		}
		for row := top; row <= bot; row++ {
			if row >= 0 && row < h {
				grid[row][c] = r.shade(1)
			}
		}
		// Tapa de pico en el máximo.
		if top >= 0 && top < h {
			grid[top][c] = cell{r: peakGlyph}
		}
	}
}

// ---------------------------------------------------------------------------
// M2 · Barras (espectro FFT con tapas de pico).
// ---------------------------------------------------------------------------

func (r *Renderer) drawSpectrum(grid [][]cell, f Frame, mags []float64, w, h int) {
	half := len(mags)
	norm := float64(fftSize) / 2.0

	for c := 0; c < w; c++ {
		b0, b1 := freqBins(c, w, half, f.SampleRate)
		var m float64
		for k := b0; k < b1; k++ {
			if mags[k] > m {
				m = mags[k]
			}
		}
		level := math.Sqrt(m / norm)
		if level > 1 {
			level = 1
		}
		height := int(level * float64(h))
		for row := h - 1; row >= h-height; row-- {
			grid[row][c] = r.bar(level)
		}
		if height > 0 {
			grid[h-height][c] = cell{r: peakGlyph}
		}
	}
}

// ---------------------------------------------------------------------------
// M3 · Cascada / espectrograma (frecuencia × tiempo).
// ---------------------------------------------------------------------------

func (r *Renderer) drawWaterfall(grid [][]cell, f Frame, mags []float64, w, h int) {
	if r.fallW != w {
		r.fall = nil
		r.fallW = w
	}

	half := len(mags)
	norm := float64(fftSize) / 2.0
	row := make([]float32, w)
	for c := 0; c < w; c++ {
		b0, b1 := freqBins(c, w, half, f.SampleRate)
		var m float64
		for k := b0; k < b1; k++ {
			if mags[k] > m {
				m = mags[k]
			}
		}
		v := math.Sqrt(m / norm)
		if v > 1 {
			v = 1
		}
		row[c] = float32(v)
	}

	r.fall = append([][]float32{row}, r.fall...)
	if len(r.fall) > h {
		r.fall = r.fall[:h]
	}

	for y, rr := range r.fall {
		for c := 0; c < w && c < len(rr); c++ {
			grid[y][c] = r.shade(float64(rr[c]))
		}
	}
}

// ---------------------------------------------------------------------------
// M4 · Medidores VU (consola de mezcla, con pico sostenido).
// ---------------------------------------------------------------------------

const vuBands = 12

func (r *Renderer) drawVU(grid [][]cell, f Frame, mags []float64, w, h int) {
	n := vuBands
	if n > w {
		n = w
	}
	if len(r.vuPeaks) != n {
		r.vuPeaks = make([]float32, n)
	}

	half := len(mags)
	norm := float64(fftSize) / 2.0
	bandW := w / n

	for b := 0; b < n; b++ {
		b0, b1 := freqBins(b, n, half, f.SampleRate)
		var m float64
		for k := b0; k < b1; k++ {
			if mags[k] > m {
				m = mags[k]
			}
		}
		level := math.Sqrt(m / norm)
		if level > 1 {
			level = 1
		}
		peak := float64(r.vuPeaks[b]) * 0.95
		if level > peak {
			peak = level
		}
		r.vuPeaks[b] = float32(peak)

		height := int(level * float64(h))
		peakH := int(peak * float64(h))

		for col := b * bandW; col < (b+1)*bandW && col < w; col++ {
			for row := h - 1; row >= h-height; row-- {
				grid[row][col] = r.bar(level)
			}
			capRow := h - 1 - peakH
			if capRow >= 0 && capRow < h {
				grid[capRow][col] = cell{r: peakGlyph}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// M5 · Alcance X-Y (Lissajous): muestra la correlación/fase de la señal.
// ---------------------------------------------------------------------------

func (r *Renderer) drawXY(grid [][]cell, f Frame, w, h int) {
	s := f.Samples
	n := len(s)
	if n < 8 {
		return
	}

	const lag = 8
	const step = 2

	for i := 0; i+lag < n; i += step {
		x := float64(s[i])
		y := float64(s[i+lag])
		if f.Inverted {
			x, y = -x, -y
		}
		col := int(math.Round((x+1)/2*float64(w-1) + 0.5))
		if col < 0 {
			col = 0
		} else if col >= w {
			col = w - 1
		}
		row := h - 1 - int(math.Round((y+1)/2*float64(h-1)))
		if row < 0 {
			row = 0
		} else if row >= h {
			row = h - 1
		}
		if grid[row][col].r == ' ' {
			grid[row][col] = r.shade(1)
		}
	}
}
