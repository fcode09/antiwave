// antiwave: a terminal audio visualizer.
//
// Without arguments it captures the system audio (loopback on Windows, the
// PulseAudio/PipeWire monitor on Linux) and renders it live. With a WAV/MP3
// file it plays and visualizes it. Multiple modes, themes, and terminal
// characters.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"

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

	// Raw mode to read keys without waiting for Enter.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	fmt.Print("\x1b[?25l\x1b[2J") // hide cursor and clear screen
	defer fmt.Print("\x1b[?25h")  // restore cursor

	keys := readKeys()
	renderer := visual.NewRenderer()

	ticker := time.NewTicker(33 * time.Millisecond) // ~30 fps
	defer ticker.Stop()

	for {
		select {
		case k, ok := <-keys:
			if !ok {
				// stdin closed (non-interactive): keep running without key input.
				keys = nil
				continue
			}
			switch k {
			case 'q', 3: // 'q' or Ctrl+C
				return
			case ' ':
				if src.Paused() {
					src.Play()
				} else {
					src.Pause()
				}
			case 'o':
				src.SetInverted(!src.Inverted())
			case '\t', 'n':
				renderer.NextMode()
			case 't':
				renderer.NextTheme()
			case 'm':
				renderer.ToggleMono()
			}

		case <-ticker.C:
			if src.Done() {
				return
			}
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				w, h = 80, 24
			}
			spc := src.SampleRate() / 50
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
			fmt.Print(renderer.Render(frame, w, h))
		}
	}
}

// readKeys reads stdin one byte at a time and sends each key over the channel.
func readKeys() <-chan rune {
	ch := make(chan rune)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				close(ch)
				return
			}
			ch <- rune(buf[0])
		}
	}()
	return ch
}
