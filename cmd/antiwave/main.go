// antiwave: visualizador de audio en la terminal.
//
// Sin argumentos captura el audio del sistema (loopback en Windows, monitor de
// PulseAudio/PipeWire en Linux) y lo dibuja en vivo. Con un archivo WAV/MP3 lo
// reproduce y visualiza. Múltiples modos, temas y caracteres de terminal.
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

const minWindow = 2048 // al menos una FFT completa de muestras

func usage() {
	fmt.Fprintf(os.Stderr, "Uso: antiwave [opciones] [archivo.wav|archivo.mp3]\n\n")
	fmt.Fprintf(os.Stderr, "Sin archivo captura el audio del sistema (Spotify, navegador, etc.).\n\n")
	fmt.Fprintf(os.Stderr, "Opciones:\n")
	flag.PrintDefaults()
}

func main() {
	silent := flag.Bool("silent", false, "modo archivo sin reproducir (avanza en tiempo real, solo visualiza)")
	flag.Usage = usage
	flag.Parse()

	enableVT()

	var src audio.Source

	if flag.NArg() >= 1 {
		path := flag.Arg(0)
		dec, err := audio.Decode(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[x] error decodificando %q: %v\n", path, err)
			os.Exit(1)
		}
		if len(dec.Samples) == 0 {
			fmt.Fprintf(os.Stderr, "[x] %q no contiene muestras de audio\n", path)
			os.Exit(1)
		}
		name := filepath.Base(path)
		if *silent {
			src = audio.NewSilentSource(dec, name)
		} else {
			fsrc, err := audio.NewFileSource(dec, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[x] audio no disponible (%v); continuando en modo silencioso.\n", err)
				src = audio.NewSilentSource(dec, name)
			} else {
				src = fsrc
			}
		}
	} else {
		cs, err := audio.NewCaptureSource()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[x] no se pudo abrir la captura del sistema: %v\n", err)
			os.Exit(1)
		}
		src = cs
	}
	defer src.Close()

	// Modo raw para leer teclas sin esperar Enter.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	fmt.Print("\x1b[?25l\x1b[2J") // ocultar cursor y limpiar pantalla
	defer fmt.Print("\x1b[?25h")  // restaurar cursor

	keys := readKeys()
	renderer := visual.NewRenderer()

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
