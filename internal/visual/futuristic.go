package visual

import (
	"math"
	"math/rand"
	"time"
)

// This file implements the futuristic visualization modes (Halo, Orbit,
// Nebula) on top of a shared "glow" buffer: a braille-resolution float grid
// that decays over time, so bright traces leave fading trails.

// ---------------------------------------------------------------------------
// Glow buffer.
// ---------------------------------------------------------------------------

func (r *Renderer) glowInit(w, h int) {
	dw, dh := w*2, h*4
	if r.glowW != dw || r.glowH != dh {
		r.glow = make([]float32, dw*dh)
		r.glowW, r.glowH = dw, dh
	}
}

// glowStep decays the glow and advances the animation clock; it returns the
// elapsed dt (0 when paused or on the first frame).
func (r *Renderer) glowStep(paused bool) float64 {
	now := time.Now()
	dt := now.Sub(r.glowLast).Seconds()
	r.glowLast = now
	if dt <= 0 || dt > 0.1 {
		dt = 0
	}
	if paused || dt == 0 {
		return 0
	}
	r.animT += dt
	f := float32(math.Exp(-r.trailDecay() * dt))
	for i := range r.glow {
		r.glow[i] *= f
	}
	return dt
}

func (r *Renderer) glowSet(x, y int, v float32) {
	if x < 0 || y < 0 || x >= r.glowW || y >= r.glowH {
		return
	}
	i := y*r.glowW + x
	if v > r.glow[i] {
		r.glow[i] = v
	}
}

func (r *Renderer) glowLine(x0, y0, x1, y1 int, v float32) {
	dx := x1 - x0
	dy := y1 - y0
	steps := abs(dx)
	if abs(dy) > steps {
		steps = abs(dy)
	}
	if steps == 0 {
		r.glowSet(x0, y0, v)
		return
	}
	for i := 0; i <= steps; i++ {
		r.glowSet(x0+dx*i/steps, y0+dy*i/steps, v)
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// renderGlow converts the glow buffer to braille cells with intensity-driven
// color.
func (r *Renderer) renderGlow(grid [][]Cell, w, h int) {
	const th = 0.10
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			var b uint8
			var maxv float32
			for dy := 0; dy < 4; dy++ {
				y := cy*4 + dy
				for dx := 0; dx < 2; dx++ {
					x := cx*2 + dx
					if x < r.glowW && y < r.glowH {
						v := r.glow[y*r.glowW+x]
						if v > maxv {
							maxv = v
						}
						if v > th {
							b |= brailleBits[dy][dx]
						}
					}
				}
			}
			if maxv <= 0.02 {
				continue
			}
			rr := rune('·')
			if b != 0 {
				rr = rune(0x2800 + int(b))
			}
			grid[cy][cx] = r.glowCell(maxv, rr)
		}
	}
}

func (r *Renderer) glowCell(v float32, rr rune) Cell {
	if r.mono {
		return Cell{R: rr}
	}
	return Cell{R: rr, Fg: r.themeObj().colorAt(float64(v))}
}

// bandEnergy returns per-band energy levels (log frequency bins, dB scale).
func bandEnergy(mags []float64, n, sampleRate int) []float64 {
	levels := make([]float64, n)
	half := len(mags)
	norm := float64(fftSize) / 2.0
	for c := 0; c < n; c++ {
		b0, b1 := freqBins(c, n, half, sampleRate)
		var e float64
		for k := b0; k < b1; k++ {
			e += mags[k] * mags[k]
		}
		amp := math.Sqrt(e) / norm
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
		levels[c] = lvl
	}
	return levels
}

// ---------------------------------------------------------------------------
// Halo: a thin reactive energy ring with a decaying trail.
// ---------------------------------------------------------------------------

func (r *Renderer) drawHalo(grid [][]Cell, f Frame, mags []float64, w, h int) {
	r.glowInit(w, h)
	dt := r.glowStep(f.Paused)

	const nBands = 96
	const steps = 256
	levels := bandEnergy(mags, nBands, f.SampleRate)

	// Smooth band levels (attack/release) to avoid jitter.
	if len(r.ringSmooth) != nBands {
		r.ringSmooth = make([]float64, nBands)
	}
	attack, release := 0.0, 0.0
	if dt > 0 {
		attack = 1 - math.Exp(-dt/0.05)
		release = 1 - math.Exp(-dt/0.20)
	}
	for b := range levels {
		s := r.ringSmooth[b]
		t := levels[b]
		if t >= s {
			s += (t - s) * attack
		} else {
			s += (t - s) * release
		}
		r.ringSmooth[b] = s
		levels[b] = s
	}

	cx, cy := float64(r.glowW)/2, float64(r.glowH)/2
	minDim := math.Min(float64(r.glowW), float64(r.glowH))
	base := minDim * 0.34
	amp := minDim * 0.10
	maxR := minDim*0.5 - 1
	if base < 6 {
		base = 6
	}

	var bass float64
	for i := 0; i < nBands/5; i++ {
		if levels[i] > bass {
			bass = levels[i]
		}
	}

	// Silence fade: when quiet, do not stamp the reactive ring.
	var total float64
	for _, l := range levels {
		total += l
	}
	silent := total/float64(nBands) < 0.015

	intensity := r.intensity()
	rot := r.animT * 0.25

	if !silent {
		// Symmetric mapping: bass at the bottom, treble at the top, seamless seam.
		half := steps / 2
		prevX, prevY := -1, -1
		for i := 0; i <= steps; i++ {
			var band int
			if i <= half {
				band = i * (nBands - 1) / half
			} else {
				band = (steps - i) * (nBands - 1) / half
			}
			a := 2*math.Pi*float64(i)/float64(steps) + rot
			rad := base + levels[band]*amp + bass*amp*0.5
			if rad > maxR {
				rad = maxR
			}
			x := int(cx + rad*math.Cos(a) + 0.5)
			y := int(cy + rad*math.Sin(a) + 0.5)
			if prevX >= 0 {
				r.glowLine(prevX, prevY, x, y, intensity)
			}
			prevX, prevY = x, y
		}
	}

	// Faint reference ring for depth (also the idle state in silence).
	for i := 0; i <= steps; i += 4 {
		a := 2 * math.Pi * float64(i) / float64(steps)
		rad := base
		if rad > maxR {
			rad = maxR
		}
		r.glowSet(int(cx+rad*math.Cos(a)+0.5), int(cy+rad*math.Sin(a)+0.5), 0.14*intensity)
	}

	r.renderGlow(grid, w, h)
}

// ---------------------------------------------------------------------------
// Orbit: tilted ellipses with a bright satellite tracing each path.
// ---------------------------------------------------------------------------

func (r *Renderer) drawOrbit(grid [][]Cell, f Frame, mags []float64, w, h int) {
	r.glowInit(w, h)
	r.glowStep(f.Paused)

	levels := bandEnergy(mags, 64, f.SampleRate)
	var energy float64
	for _, l := range levels {
		energy += l
	}
	energy /= float64(len(levels))
	if energy < 0.02 {
		energy = 0.02
	}

	cx, cy := float64(r.glowW)/2, float64(r.glowH)/2
	base := math.Min(float64(r.glowW), float64(r.glowH)) * 0.40
	if base < 6 {
		base = 6
	}

	orbits := []struct{ rx, ry, tilt, speed, phase float64 }{
		{1.0, 0.55, 0.0, 0.7, 0.0},
		{0.75, 1.0, 1.1, -0.9, 2.1},
		{0.55, 0.55, 0.6, 1.3, 4.2},
	}

	for _, o := range orbits {
		rx := base * o.rx * (0.9 + energy*0.3)
		ry := base * o.ry * (0.9 + energy*0.3)

		const steps = 180
		for i := 0; i <= steps; i += 2 {
			t := 2 * math.Pi * float64(i) / float64(steps)
			x, y := ellipse(cx, cy, rx, ry, o.tilt, t)
			r.glowSet(int(x+0.5), int(y+0.5), 0.13)
		}

		t := r.animT*o.speed + o.phase
		x, y := ellipse(cx, cy, rx, ry, o.tilt, t)
		r.glowSet(int(x+0.5), int(y+0.5), r.intensity())
	}

	r.renderGlow(grid, w, h)
}

func ellipse(cx, cy, rx, ry, tilt, t float64) (float64, float64) {
	x := rx * math.Cos(t)
	y := ry * math.Sin(t)
	c, s := math.Cos(tilt), math.Sin(tilt)
	return cx + x*c - y*s, cy + x*s + y*c
}

// ---------------------------------------------------------------------------
// Nebula: a swarm of particles orbiting the center, pushed outward by bass.
// ---------------------------------------------------------------------------

type particle struct {
	ang, baseR, angVel, phase float64
}

func (r *Renderer) drawNebula(grid [][]Cell, f Frame, mags []float64, w, h int) {
	r.glowInit(w, h)
	dt := r.glowStep(f.Paused)

	if len(r.parts) == 0 {
		rnd := rand.New(rand.NewSource(42))
		r.parts = make([]particle, 140)
		for i := range r.parts {
			r.parts[i] = particle{
				ang:    rnd.Float64() * 2 * math.Pi,
				baseR:  0.25 + rnd.Float64()*0.75,
				angVel: (rnd.Float64() - 0.5) * 1.2,
				phase:  rnd.Float64() * 2 * math.Pi,
			}
		}
	}

	levels := bandEnergy(mags, 32, f.SampleRate)
	var bass float64
	for i := 0; i < 8; i++ {
		if levels[i] > bass {
			bass = levels[i]
		}
	}

	cx, cy := float64(r.glowW)/2, float64(r.glowH)/2
	maxR := math.Min(float64(r.glowW), float64(r.glowH)) * 0.48

	if dt > 0 {
		for i := range r.parts {
			r.parts[i].ang += r.parts[i].angVel * dt
		}
	}
	for i := range r.parts {
		p := &r.parts[i]
		rr := p.baseR * (0.45 + bass*0.7 + 0.25*math.Sin(r.animT*1.5+p.phase)) * maxR
		x := int(cx + rr*math.Cos(p.ang) + 0.5)
		y := int(cy + rr*math.Sin(p.ang) + 0.5)
		r.glowSet(x, y, r.intensity())
	}

	r.renderGlow(grid, w, h)
}
