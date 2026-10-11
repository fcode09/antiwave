package audio

import (
	"encoding/binary"
	"io"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"
)

// Source provides mono float32 audio samples for visualization, either from a
// file being played back or from live system capture.
type Source interface {
	SampleRate() int
	// Latest returns up to max of the most recent samples (oldest first).
	// It may return fewer if not enough are available yet.
	Latest(max int) []float32
	Inverted() bool
	SetInverted(bool)
	Paused() bool
	Pause()
	Play()
	Done() bool
	Close()
	// Name is a human-readable name for the HUD.
	Name() string
	// Elapsed is the played/captured time. Total==0 means a live source.
	Elapsed() time.Duration
	Total() time.Duration
}

// ---------------------------------------------------------------------------
// sampleReader: feeds oto, applies phase inversion, and tracks position.
// ---------------------------------------------------------------------------

type sampleReader struct {
	samples    []float32
	totalBytes int64
	served     atomic.Int64 // bytes already handed to the player
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

// ---------------------------------------------------------------------------
// FileSource: plays a decoded file through oto.
// ---------------------------------------------------------------------------

type FileSource struct {
	samples    []float32
	sampleRate int
	name       string
	total      int
	reader     *sampleReader
	oto        *oto.Player
	ctx        *oto.Context
	paused     atomic.Bool
}

func NewFileSource(dec *Decoded, name string) (*FileSource, error) {
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

	return &FileSource{
		samples:    dec.Samples,
		sampleRate: dec.SampleRate,
		name:       name,
		total:      len(dec.Samples),
		reader:     r,
		oto:        p,
		ctx:        ctx,
	}, nil
}

func (f *FileSource) playedSamples() int {
	served := f.reader.served.Load()
	buffered := int64(f.oto.BufferedSize())
	played := served - buffered
	if played < 0 {
		played = 0
	}
	return int(played / 2)
}

func (f *FileSource) SampleRate() int { return f.sampleRate }
func (f *FileSource) Latest(max int) []float32 {
	pos := f.playedSamples()
	start := pos - max
	if start < 0 {
		start = 0
	}
	return f.samples[start:pos]
}
func (f *FileSource) Inverted() bool     { return f.reader.inverted.Load() }
func (f *FileSource) SetInverted(b bool) { f.reader.inverted.Store(b) }
func (f *FileSource) Paused() bool       { return f.paused.Load() }
func (f *FileSource) Pause() {
	f.paused.Store(true)
	f.oto.Pause()
}
func (f *FileSource) Play() {
	f.paused.Store(false)
	f.oto.Play()
}
func (f *FileSource) Done() bool {
	return f.reader.served.Load() >= f.reader.totalBytes && f.oto.BufferedSize() == 0
}
func (f *FileSource) Close() { f.oto.Pause() }
func (f *FileSource) Name() string {
	return f.name
}
func (f *FileSource) Elapsed() time.Duration {
	return samplesToDuration(f.playedSamples(), f.sampleRate)
}
func (f *FileSource) Total() time.Duration {
	return samplesToDuration(f.total, f.sampleRate)
}

// ---------------------------------------------------------------------------
// SilentSource: advances through the samples in real time without playback.
// ---------------------------------------------------------------------------

type SilentSource struct {
	samples    []float32
	sampleRate int
	name       string
	total      int
	paused     atomic.Bool
	inverted   atomic.Bool
	played     atomic.Int64
	done       atomic.Bool
	stop       chan struct{}
}

func NewSilentSource(dec *Decoded, name string) *SilentSource {
	s := &SilentSource{
		samples:    dec.Samples,
		sampleRate: dec.SampleRate,
		name:       name,
		total:      len(dec.Samples),
		stop:       make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *SilentSource) run() {
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

func (s *SilentSource) SampleRate() int { return s.sampleRate }
func (s *SilentSource) Latest(max int) []float32 {
	pos := int(s.played.Load())
	start := pos - max
	if start < 0 {
		start = 0
	}
	return s.samples[start:pos]
}
func (s *SilentSource) Inverted() bool     { return s.inverted.Load() }
func (s *SilentSource) SetInverted(b bool) { s.inverted.Store(b) }
func (s *SilentSource) Paused() bool       { return s.paused.Load() }
func (s *SilentSource) Pause()             { s.paused.Store(true) }
func (s *SilentSource) Play()              { s.paused.Store(false) }
func (s *SilentSource) Done() bool         { return s.done.Load() }
func (s *SilentSource) Close()             { close(s.stop) }
func (s *SilentSource) Name() string       { return s.name }
func (s *SilentSource) Elapsed() time.Duration {
	return samplesToDuration(int(s.played.Load()), s.sampleRate)
}
func (s *SilentSource) Total() time.Duration {
	return samplesToDuration(s.total, s.sampleRate)
}

func samplesToDuration(samples, sampleRate int) time.Duration {
	if sampleRate <= 0 {
		return 0
	}
	return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}
