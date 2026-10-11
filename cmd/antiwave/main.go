// antiwave: a terminal audio visualizer.
//
// Without arguments it captures the system audio (loopback on Windows, the
// PulseAudio/PipeWire monitor on Linux) and renders it live. With a WAV/MP3
// file it plays and visualizes it. Eight visualization modes, color themes,
// and an immersive full-screen mode.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/fquea/antiwave/internal/audio"
	"github.com/fquea/antiwave/internal/visual"
)

const minWindow = 2048 // at least one full FFT worth of samples

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: antiwave [options] [file.wav|file.mp3]\n\n")
	fmt.Fprintf(os.Stderr, "Without a file it captures the system audio (Spotify, browser, etc.).\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}

func main() {
	silent := flag.Bool("silent", false, "file mode without playback (advances in real time, visualizes only)")
	immersive := flag.Bool("immersive", false, "start in immersive mode (visualization only)")
	modeFlag := flag.String("mode", "spectrum", "initial mode: spectrum|mirror|waveform|oscilloscope|waterfall|phase|radial|levels")
	themeFlag := flag.String("theme", "aurora", "initial theme: mono|aurora|ocean|ember|phosphor")
	mono := flag.Bool("mono", false, "start in monochrome mode")
	fps := flag.Int("fps", 60, "target frames per second (30 or 60)")
	flag.Usage = usage
	flag.Parse()

	enableVT()

	var src audio.Source

	if flag.NArg() >= 1 {
		path := flag.Arg(0)
		dec, err := audio.Decode(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[x] error decoding %q: %v\n", path, err)
			os.Exit(1)
		}
		if len(dec.Samples) == 0 {
			fmt.Fprintf(os.Stderr, "[x] %q contains no audio samples\n", path)
			os.Exit(1)
		}
		name := filepath.Base(path)
		if *silent {
			src = audio.NewSilentSource(dec, name)
		} else {
			fsrc, err := audio.NewFileSource(dec, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[x] audio unavailable (%v); continuing in silent mode.\n", err)
				src = audio.NewSilentSource(dec, name)
			} else {
				src = fsrc
			}
		}
	} else {
		cs, err := audio.NewCaptureSource()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[x] could not open system audio capture: %v\n", err)
			os.Exit(1)
		}
		src = cs
	}
	defer src.Close()

	// Terminal screen.
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[x] could not initialize terminal: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "[x] could not initialize terminal: %v\n", err)
		os.Exit(1)
	}
	defer s.Fini()
	s.HideCursor()
	s.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorReset).Background(tcell.ColorReset))
	s.Clear()

	renderer := visual.NewRenderer()
	renderer.SetImmersive(*immersive)
	renderer.SetMonoForFlag(*mono)
	if m, ok := visual.ModeByName(*modeFlag); ok {
		renderer.SetMode(m)
	}
	if ti, ok := visual.ThemeIndexByName(*themeFlag); ok {
		renderer.SetTheme(ti)
	}

	interval := time.Second / time.Duration(clampFPS(*fps))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	events := make(chan tcell.Event)
	go func() {
		for {
			events <- s.PollEvent()
		}
	}()

	draw := func() {
		w, h := s.Size()
		if w <= 0 || h <= 0 {
			return
		}
		spc := src.SampleRate() / 60
		if spc < 1 {
			spc = 1
		}
		need := w * spc
		if need < minWindow {
			need = minWindow
		}
		frame := visual.Frame{
			Samples:    src.Latest(need),
			SampleRate: src.SampleRate(),
			Inverted:   src.Inverted(),
			Paused:     src.Paused(),
			Name:       src.Name(),
			Elapsed:    src.Elapsed(),
			Total:      src.Total(),
		}
		grid := renderer.Render(frame, w, h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := grid[y][x]
				ch := c.R
				if ch == 0 {
					ch = ' '
				}
				st := tcell.StyleDefault
				if c.Fg != 0 {
					r := int32(c.Fg>>16) & 0xff
					g := int32(c.Fg>>8) & 0xff
					b := int32(c.Fg) & 0xff
					st = st.Foreground(tcell.NewRGBColor(r, g, b))
				}
				s.SetContent(x, y, ch, nil, st)
			}
		}
		s.Show()
	}

	for {
		select {
		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if handleKey(ev, renderer, src) {
					s.Fini()
					return
				}
			case *tcell.EventResize:
				s.Sync()
			}
		case <-ticker.C:
			if src.Done() {
				s.Fini()
				return
			}
			draw()
		}
	}
}

// handleKey processes a key event and returns true when the app should quit.
func handleKey(ev *tcell.EventKey, r *visual.Renderer, src audio.Source) bool {
	switch ev.Key() {
	case tcell.KeyCtrlC:
		return true
	case tcell.KeyEscape:
		if r.Immersive() {
			r.SetImmersive(false)
		}
		return false
	case tcell.KeyTab:
		r.NextMode()
		return false
	case tcell.KeyBacktab:
		r.PrevMode()
		return false
	case tcell.KeyLeft:
		r.PrevMode()
		return false
	case tcell.KeyRight:
		r.NextMode()
		return false
	}

	switch ev.Rune() {
	case 'q':
		return true
	case ' ':
		if src.Paused() {
			src.Play()
		} else {
			src.Pause()
		}
	case 'o':
		src.SetInverted(!src.Inverted())
	case 'f':
		r.ToggleImmersive()
	case 't':
		r.NextTheme()
	case 'm':
		r.ToggleMono()
	case 'a':
		r.ToggleAutoGain()
	case 'e':
		r.NextEffect()
	case '+', '=':
		r.AdjustGain(1.15)
	case '-', '_':
		r.AdjustGain(1 / 1.15)
	case '1', '2', '3', '4', '5', '6', '7', '8':
		if m, ok := visual.ModeByIndex(int(ev.Rune() - '1')); ok {
			r.SetMode(m)
		}
	case '9':
		r.SetMode(visual.ModeHalo)
	case '0':
		r.SetMode(visual.ModeOrbit)
	}
	return false
}

func clampFPS(fps int) int {
	if fps < 10 {
		return 10
	}
	if fps > 60 {
		return 60
	}
	return fps
}
