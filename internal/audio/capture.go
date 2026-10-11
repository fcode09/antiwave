package audio

import (
	"sync"
	"sync/atomic"
	"time"
)

// captureHandle is returned by each OS-specific implementation.
type captureHandle struct {
	sampleRate int
	name       string
	stop       func()
}

// newSystemCapture (defined per OS in capture_linux.go / capture_windows.go /
// capture_unsupported.go) opens the system capture device and starts calling
// push with mono float32 samples.

// ---------------------------------------------------------------------------
// ringBuffer: a sliding window of the most recent samples.
// ---------------------------------------------------------------------------

type ringBuffer struct {
	mu   sync.Mutex
	data []float32
	max  int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{max: max}
}

func (r *ringBuffer) push(s []float32) {
	r.mu.Lock()
	r.data = append(r.data, s...)
	if len(r.data) > r.max {
		r.data = r.data[len(r.data)-r.max:]
	}
	r.mu.Unlock()
}

func (r *ringBuffer) latest(max int) []float32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := max
	if n > len(r.data) {
		n = len(r.data)
	}
	if n <= 0 {
		return nil
	}
	out := make([]float32, n)
	copy(out, r.data[len(r.data)-n:])
	return out
}

// ---------------------------------------------------------------------------
// CaptureSource: a Source fed by system capture.
// ---------------------------------------------------------------------------

type CaptureSource struct {
	ring       *ringBuffer
	sampleRate int
	name       string
	inverted   atomic.Bool
	paused     atomic.Bool
	pushed     atomic.Int64 // samples captured (for elapsed time)
	stopFn     func()
	closed     atomic.Bool
}

func NewCaptureSource() (*CaptureSource, error) {
	cs := &CaptureSource{ring: newRingBuffer(48000 * 4)}
	h, err := newSystemCapture(cs.push)
	if err != nil {
		return nil, err
	}
	cs.sampleRate = h.sampleRate
	cs.name = h.name
	if cs.sampleRate > 0 {
		cs.ring.max = cs.sampleRate * 4
	}
	cs.stopFn = h.stop
	return cs, nil
}

func (cs *CaptureSource) push(s []float32) {
	if cs.paused.Load() {
		return
	}
	cs.ring.push(s)
	cs.pushed.Add(int64(len(s)))
}

func (cs *CaptureSource) SampleRate() int { return cs.sampleRate }
func (cs *CaptureSource) Latest(max int) []float32 {
	return cs.ring.latest(max)
}
func (cs *CaptureSource) Inverted() bool     { return cs.inverted.Load() }
func (cs *CaptureSource) SetInverted(b bool) { cs.inverted.Store(b) }
func (cs *CaptureSource) Paused() bool       { return cs.paused.Load() }
func (cs *CaptureSource) Pause()             { cs.paused.Store(true) }
func (cs *CaptureSource) Play()              { cs.paused.Store(false) }
func (cs *CaptureSource) Done() bool         { return cs.closed.Load() }
func (cs *CaptureSource) Close() {
	if cs.closed.Swap(true) {
		return
	}
	if cs.stopFn != nil {
		cs.stopFn()
	}
}
func (cs *CaptureSource) Name() string { return cs.name }
func (cs *CaptureSource) Elapsed() time.Duration {
	if cs.sampleRate <= 0 {
		return 0
	}
	return samplesToDuration(int(cs.pushed.Load()), cs.sampleRate)
}
func (cs *CaptureSource) Total() time.Duration { return 0 }
