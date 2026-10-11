package visual

import "math"

const (
	barW   = 2
	barGap = 1
)

func barCount(w int) int {
	n := w / (barW + barGap)
	if n < 8 {
		n = 8
	}
	return n
}

// columnRaw maps the spectrum to per-column raw levels (log bins, dB scale),
// no smoothing.
func columnRaw(mags []float64, w, sampleRate int) []float64 {
	raw := make([]float64, w)
	half := len(mags)
	norm := float64(fftSize) / 2.0
	for c := 0; c < w; c++ {
		b0, b1 := freqBins(c, w, half, sampleRate)
		var m float64
		for k := b0; k < b1; k++ {
			if mags[k] > m {
				m = mags[k]
			}
		}
		amp := m / norm
		if amp > 1e-6 {
			db := 20 * math.Log10(amp)
			raw[c] = (db + 60) / 60
		}
		if raw[c] < 0 {
			raw[c] = 0
		} else if raw[c] > 1 {
			raw[c] = 1
		}
	}
	return raw
}

// ---------------------------------------------------------------------------
// Spectrum (thin gradient bars with peak caps).
// ---------------------------------------------------------------------------

func (r *Renderer) drawSpectrum(grid [][]Cell, levels, peaks []float64, h, w int) {
	for b, lvl := range levels {
		col0 := b * (barW + barGap)
		height := int(lvl * float64(h))
		peakH := int(peaks[b] * float64(h))
		for x := 0; x < barW; x++ {
			c := col0 + x
			if c >= w {
				break
			}
			for row := h - 1; row >= h-height && row >= 0; row-- {
				rel := float64(h-1-row) / float64(height)
				grid[row][c] = r.barCell(rel)
			}
			if peakH > 0 {
				pr := h - peakH
				if pr >= 0 && pr < h {
					grid[pr][c] = Cell{R: peakGlyph, Fg: r.accent()}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Mirror (spectrum growing up and down from the center).
// ---------------------------------------------------------------------------

func (r *Renderer) drawMirror(grid [][]Cell, levels, peaks []float64, h, w int) {
	center := h / 2
	half := float64(h) / 2
	for b, lvl := range levels {
		col0 := b * (barW + barGap)
		hgt := int(lvl * half)
		for x := 0; x < barW; x++ {
			c := col0 + x
			if c >= w {
				break
			}
			for k := 0; k < hgt; k++ {
				up := center - k
				dn := center + k
				if up >= 0 {
					grid[up][c] = r.barCell(float64(k) / float64(hgt))
				}
				if dn < h {
					grid[dn][c] = r.barCell(float64(k) / float64(hgt))
				}
			}
			// Peak caps at both ends.
			if hgt > 0 {
				if center-hgt >= 0 {
					grid[center-hgt][c] = Cell{R: peakGlyph, Fg: r.accent()}
				}
				if center+hgt < h {
					grid[center+hgt][c] = Cell{R: peakGlyph, Fg: r.accent()}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Waveform (filled scrolling amplitude envelope).
// ---------------------------------------------------------------------------

func (r *Renderer) drawWaveform(grid [][]Cell, f Frame, w, h int) {
	center := h / 2
	for c := 0; c < w; c++ {
		grid[center][c] = Cell{R: zeroGlyph, Fg: cDim}
	}

	spc := f.SampleRate / 60
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
				grid[row][c] = r.barCell(1)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Oscilloscope (thin braille trace, phase-triggered).
// ---------------------------------------------------------------------------

var brailleBits = [4][2]uint8{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

func (r *Renderer) drawOscilloscope(grid [][]Cell, f Frame, w, h int) {
	s := f.Samples
	n := len(s)
	if n < 16 {
		return
	}

	// Phase trigger: start at the first rising zero-crossing in the first quarter.
	start := 0
	for i := 1; i < n/4; i++ {
		if s[i-1] <= 0 && s[i] > 0 {
			start = i
			break
		}
	}
	if n-start < 8 {
		start = 0
	}
	m := n - start

	dw := w * 2
	dh := h * 4
	dots := make([]bool, dw*dh)
	prevY := -1
	for i := 0; i < m; i++ {
		x := i * (dw - 1) / (m - 1)
		v := s[start+i]
		if f.Inverted {
			v = -v
		}
		y := int((1-float64(v))/2*float64(dh-1) + 0.5)
		if y < 0 {
			y = 0
		} else if y >= dh {
			y = dh - 1
		}
		if prevY >= 0 {
			lo, hi := y, prevY
			if lo > hi {
				lo, hi = hi, lo
			}
			for yy := lo; yy <= hi; yy++ {
				dots[yy*dw+x] = true
			}
		} else {
			dots[y*dw+x] = true
		}
		prevY = y
	}

	fg := r.accent()
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			var b uint8
			for dy := 0; dy < 4; dy++ {
				dotY := cy*4 + dy
				for dx := 0; dx < 2; dx++ {
					dotX := cx*2 + dx
					if dotY < dh && dotX < dw && dots[dotY*dw+dotX] {
						b |= brailleBits[dy][dx]
					}
				}
			}
			if b != 0 {
				grid[cy][cx] = Cell{R: rune(0x2800 + int(b)), Fg: fg}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Waterfall / spectrogram (frequency × time).
// ---------------------------------------------------------------------------

func (r *Renderer) drawWaterfall(grid [][]Cell, f Frame, mags []float64, w, h int) {
	if r.fallW != w {
		r.fall = nil
		r.fallW = w
	}
	row := make([]float32, w)
	raw := columnRaw(mags, w, f.SampleRate)
	for c := 0; c < w; c++ {
		row[c] = float32(raw[c])
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
// Phase scope (X-Y Lissajous via a phase-delayed copy of the signal).
// ---------------------------------------------------------------------------

func (r *Renderer) drawPhase(grid [][]Cell, f Frame, w, h int) {
	s := f.Samples
	n := len(s)
	if n < 8 {
		return
	}
	const lag = 8
	const step = 2
	fg := r.accent()
	for i := 0; i+lag < n; i += step {
		x := float64(s[i])
		y := float64(s[i+lag])
		if f.Inverted {
			x, y = -x, -y
		}
		col := int((x+1)/2*float64(w-1) + 0.5)
		if col < 0 {
			col = 0
		} else if col >= w {
			col = w - 1
		}
		row := h - 1 - int((y+1)/2*float64(h-1))
		if row < 0 {
			row = 0
		} else if row >= h {
			row = h - 1
		}
		if grid[row][col].R == 0 {
			grid[row][col] = Cell{R: blockFull, Fg: fg}
		}
	}
}

// ---------------------------------------------------------------------------
// Radial (circular spectrum).
// ---------------------------------------------------------------------------

func (r *Renderer) drawRadial(grid [][]Cell, levels []float64, w, h int) {
	n := len(levels)
	cx := float64(w) / 2
	cy := float64(h) / 2
	// Terminal cells are roughly 2:1 tall, so halve the vertical radius.
	rX := math.Min(float64(w)/2-1, float64(h)-2)
	rY := rX / 2
	if rY < 1 {
		rY = 1
	}

	for b, lvl := range levels {
		a := 2 * math.Pi * float64(b) / float64(n)
		rad := lvl * rX
		x0 := cx
		y0 := cy
		steps := int(rad) + 1
		for k := 0; k <= steps; k++ {
			rr := rad * float64(k) / float64(steps)
			x := int(cx + rr*math.Cos(a) + 0.5)
			y := int(cy + rr*math.Sin(a)*0.5 + 0.5)
			if x >= 0 && x < w && y >= 0 && y < h {
				grid[y][x] = r.barCell(rr / rX)
			}
		}
		_ = x0
		_ = y0
	}
}

// ---------------------------------------------------------------------------
// Level meters (per-band RMS with dB scale and peak hold).
// ---------------------------------------------------------------------------

const levelBands = 12

func (r *Renderer) drawLevels(grid [][]Cell, f Frame, mags []float64, w, h int) {
	n := levelBands
	if n > w/3 {
		n = w / 3
	}
	if n < 4 {
		n = 4
	}
	if len(r.meterPeaks) != n {
		r.meterPeaks = make([]float64, n)
	}

	half := len(mags)
	norm := float64(fftSize) / 2.0
	bandW := w / n

	// Frequency labels along the bottom.
	labels := []string{"60", "250", "1k", "4k", "16k"}

	for b := 0; b < n; b++ {
		b0, b1 := freqBins(b, n, half, f.SampleRate)
		var energy float64
		for k := b0; k < b1; k++ {
			energy += mags[k] * mags[k]
		}
		amp := math.Sqrt(energy) / norm
		lvl := 0.0
		if amp > 1e-6 {
			db := 20 * math.Log10(amp)
			lvl = (db + 60) / 60
		}
		if lvl < 0 {
			lvl = 0
		} else if lvl > 1 {
			lvl = 1
		}
		peak := r.meterPeaks[b] - 0.04
		if lvl > peak {
			peak = lvl
		}
		r.meterPeaks[b] = peak

		height := int(lvl * float64(h-1))
		peakH := int(peak * float64(h-1))

		for col := b * bandW; col < (b+1)*bandW-1 && col < w; col++ {
			for row := h - 2; row >= h-1-height && row >= 0; row-- {
				rel := float64(h-2-row) / float64(height)
				grid[row][col] = r.barCell(rel)
			}
			pr := h - 1 - peakH
			if pr >= 0 && pr < h-1 {
				grid[pr][col] = Cell{R: peakGlyph, Fg: r.accent()}
			}
		}
	}

	// Baseline and labels.
	for c := 0; c < w; c++ {
		grid[h-1][c] = Cell{R: '─', Fg: cDim}
	}
	x := 0
	for _, lb := range labels {
		if x+len(lb) > w {
			break
		}
		for i, ch := range lb {
			grid[h-1][x+i] = Cell{R: ch, Fg: cDim}
		}
		x += w / (len(labels))
	}
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

func rowFor(a float32, h int) int {
	n := float64(a+1) / 2.0
	r := int(math.Round(n * float64(h-1)))
	return h - 1 - r
}
