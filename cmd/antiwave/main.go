// antiwave: visualizador de música en la terminal.
//
// Reproduce un archivo WAV/MP3 y dibuja en vivo sus ondas (waveform) o barras
// de espectro (FFT), con un modo "música opuesta" (fase invertida + visual
// espejado).
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/fquea/antiwave/internal/audio"
	"github.com/fquea/antiwave/internal/visual"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Uso: antiwave [opciones] <archivo.wav|archivo.mp3>\n\n")
	fmt.Fprintf(os.Stderr, "Opciones:\n")
	flag.PrintDefaults()
}

func main() {
	silent := flag.Bool("silent", false, "no reproducir audio (avanza en tiempo real, solo visualiza)")
	viewFlag := flag.String("view", "waveform", "vista inicial: waveform | spectrum")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}
	path := flag.Arg(0)

	dec, err := audio.Decode(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando %q: %v\n", path, err)
		os.Exit(1)
	}
	if len(dec.Samples) == 0 {
		fmt.Fprintf(os.Stderr, "error: %q no contiene muestras de audio\n", path)
		os.Exit(1)
	}

	// Selección de reproductor (real o silencioso).
	var pos audio.Positioner
	if *silent {
		pos = audio.NewSilentPlayer(dec)
	} else {
		p, err := audio.NewPlayer(dec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  no se pudo abrir el dispositivo de audio (%v); continuando en modo silencioso.\n", err)
			pos = audio.NewSilentPlayer(dec)
		} else {
			pos = p
		}
	}
	defer pos.Close()
	pos.Play()

	// Modo raw para leer teclas sin esperar Enter.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	fmt.Print("\x1b[?25l\x1b[2J") // ocultar cursor y limpiar pantalla
	defer fmt.Print("\x1b[?25h")  // restaurar cursor

	keys := readKeys()

	view := visual.Waveform
	if *viewFlag == "spectrum" {
		view = visual.Spectrum
	}

	ticker := time.NewTicker(33 * time.Millisecond) // ~30 fps
	defer ticker.Stop()

	for {
		select {
		case k, ok := <-keys:
			if !ok {
				// stdin cerrado (no interactivo): seguir sin entrada de teclas.
				keys = nil
				continue
			}
			switch k {
			case 'q', 3: // 'q' o Ctrl+C
				return
			case ' ':
				if pos.IsPlaying() {
					pos.Pause()
				} else {
					pos.Play()
				}
			case 'o':
				pos.SetInverted(!pos.Inverted())
			case 'b':
				view = visual.Spectrum
			case 'w':
				view = visual.Waveform
			}

		case <-ticker.C:
			if pos.Done() {
				return
			}
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				w, h = 80, 24 // fallback si no hay TTY (p. ej. redirigido)
			}
			params := visual.Params{
				Samples:    dec.Samples,
				SampleRate: dec.SampleRate,
				Played:     pos.PlayedSamples(),
				Inverted:   pos.Inverted(),
				Paused:     !pos.IsPlaying(),
				View:       view,
				Total:      pos.TotalSamples(),
			}
			fmt.Print(visual.Render(params, w, h))
		}
	}
}

// readKeys lee stdin byte a byte y envía cada tecla por el canal.
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
