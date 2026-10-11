package audio

import (
	"encoding/binary"
	"io"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"
)

// Positioner abstrae la reproducción: una implementación real (oto, por
// parlantes) y otra "silenciosa" que avanza por las muestras en tiempo real
// (útil sin dispositivo de audio o para pruebas).
type Positioner interface {
	PlayedSamples() int
	TotalSamples() int
	IsPlaying() bool
	Pause()
	Play()
	Inverted() bool
	SetInverted(bool)
	Done() bool
	Close()
}

// ---------------------------------------------------------------------------
// Player real (oto)
// ---------------------------------------------------------------------------

// sampleReader es el io.Reader que alimenta a oto: sirve las muestras en
// PCM int16 LE mono, aplicando la inversión de fase ("música opuesta") sobre
// la marcha y rastreando cuántos bytes ya fueron entregados.
type sampleReader struct {
	samples    []float32
	totalBytes int64
	served     atomic.Int64 // bytes ya servidos al player
	inverted   atomic.Bool
}

func (r *sampleReader) Read(p []byte) (int, error) {
	start := r.served.Load() / 2
	if start >= int64(len(r.samples)) {
		return 0, io.EOF
	}

	inv := r.inverted.Load()
	remaining := int64(len(r.samples)) - start
	want := int64(len(p)) / 2
	n := want
	if n > remaining {
		n = remaining
	}

	buf := p[:n*2]
	for i := int64(0); i < n; i++ {
		s := r.samples[start+i]
		if inv {
			s = -s
		}
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(int16(s*32767)))
	}
	r.served.Add(n * 2)
	return int(n * 2), nil
}

// Player reproduce un Decoded mediante oto.
type Player struct {
	ctx    *oto.Context
	oto    *oto.Player
	reader *sampleReader
	total  int
}

func NewPlayer(dec *Decoded) (*Player, error) {
	op := &oto.NewContextOptions{
		SampleRate:   dec.SampleRate,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   30 * time.Millisecond,
	}
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}
	<-ready

	r := &sampleReader{samples: dec.Samples, totalBytes: int64(len(dec.Samples)) * 2}
	p := ctx.NewPlayer(r)
	p.SetBufferSize(4096)

	return &Player{ctx: ctx, oto: p, reader: r, total: len(dec.Samples)}, nil
}

func (p *Player) PlayedSamples() int {
	served := p.reader.served.Load()
	buffered := int64(p.oto.BufferedSize())
	played := served - buffered
	if played < 0 {
		played = 0
	}
	return int(played / 2)
}

func (p *Player) TotalSamples() int  { return p.total }
func (p *Player) IsPlaying() bool    { return p.oto.IsPlaying() }
func (p *Player) Pause()             { p.oto.Pause() }
func (p *Player) Play()              { p.oto.Play() }
func (p *Player) Inverted() bool     { return p.reader.inverted.Load() }
func (p *Player) SetInverted(b bool) { p.reader.inverted.Store(b) }
func (p *Player) Done() bool {
	return p.reader.served.Load() >= p.reader.totalBytes && p.oto.BufferedSize() == 0
}
func (p *Player) Close() { p.oto.Pause() }

// ---------------------------------------------------------------------------
// Player silencioso (sin audio)
// ---------------------------------------------------------------------------

// SilentPlayer avanza por las muestras a velocidad real sin reproducir nada.
type SilentPlayer struct {
	samples    []float32
	sampleRate int
	total      int
	paused     atomic.Bool
	inverted   atomic.Bool
	played     atomic.Int64
	done       atomic.Bool
	stop       chan struct{}
}

func NewSilentPlayer(dec *Decoded) *SilentPlayer {
	s := &SilentPlayer{
		samples:    dec.Samples,
		sampleRate: dec.SampleRate,
		total:      len(dec.Samples),
		stop:       make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *SilentPlayer) run() {
	last := time.Now()
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case now := <-t.C:
			if !s.paused.Load() {
				s.played.Add(int64(float64(s.sampleRate) * now.Sub(last).Seconds()))
			}
			last = now
			if s.played.Load() >= int64(s.total) {
				s.played.Store(int64(s.total))
				s.done.Store(true)
				return
			}
		}
	}
}

func (s *SilentPlayer) PlayedSamples() int { return int(s.played.Load()) }
func (s *SilentPlayer) TotalSamples() int  { return s.total }
func (s *SilentPlayer) IsPlaying() bool    { return !s.paused.Load() && !s.done.Load() }
func (s *SilentPlayer) Pause()             { s.paused.Store(true) }
func (s *SilentPlayer) Play()              { s.paused.Store(false) }
func (s *SilentPlayer) Inverted() bool     { return s.inverted.Load() }
func (s *SilentPlayer) SetInverted(b bool) { s.inverted.Store(b) }
func (s *SilentPlayer) Done() bool         { return s.done.Load() }
func (s *SilentPlayer) Close()             { close(s.stop) }
