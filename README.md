# antiwave 🎧

Visualizador de música en la terminal, escrito en Go. Reproduce un archivo
**WAV** o **MP3** y dibuja en vivo sus **ondas** (waveform) o sus **barras** de
espectro (FFT), con un modo **"música opuesta"** (fase invertida + visual
espejado).

```
▶ antiwave  |  ONDAS  |  0:03/0:06  |  fase: OPUESTA
 ▄▄▄▄▄▄▄  ···▄▄▄▄▄▄▄···▄▄▄▄▄▄▄···
 ███████  ···███████···███████···
 ·································
 ███████···███████···███████······
```

## Qué hace

1. **Decodifica** el archivo a mono `float32` (WAV: PCM 8/16/24/32 bits y
   float; MP3: vía `go-mp3`).
2. **Reproduce** el audio por los parlantes (oto).
3. Mientras suena, **dibuja en la terminal** (~30 fps):
   - **`w` — Ondas**: osciloscopio scrolleante (amplitud vs. tiempo).
   - **`b` — Barras**: espectro de frecuencias (FFT + ventana de Hann).
4. Con `o` activas la **"música opuesta"**: invierte la fase (`sample × −1`)
   y espeja el visual.

> ⚠️ **Nota de audio**: la fase invertida **suena idéntica** al oído humano
> (somos insensibles a la fase absoluta). La diferencia se ve claramente en la
> consola (la onda queda "boca abajo"), pero el sonido no cambia.

## Requisitos

- Go 1.21+ (probado con 1.27).
- Un dispositivo de audio para reproducir (opcional: usa `-silent` para
  visualizar sin sonido).

## Instalación y uso

```bash
# Genera un WAV demo (arpegio de notas) si no tienes un archivo de audio
make gen          # → samples/demo.wav

# Reproduce y visualiza
go run ./cmd/antiwave samples/demo.wav

# O compila el binario
make build
./antiwave camino/al/archivo.mp3
```

Opciones:

```bash
./antiwave -silent -view spectrum cancion.wav   # solo visualiza (sin audio), vista de barras
```

## Controles

| Tecla      | Acción                          |
|------------|---------------------------------|
| `espacio`  | pausar / reanudar               |
| `o`        | alternar música opuesta (fase)  |
| `w`        | vista de ondas (waveform)       |
| `b`        | vista de barras (espectro)      |
| `q` / `Ctrl+C` | salir                      |

## Estructura

```
cmd/
  antiwave/        # binario principal (decodifica → reproduce → dibuja)
  genwav/          # genera un WAV demo para probar
internal/
  audio/           # decodificación WAV/MP3 y reproducción (oto + modo silencioso)
  visual/          # render en terminal: waveform y espectro FFT
```

## Stack

- [`ebitengine/oto/v3`](https://github.com/ebitengine/oto) — reproducción (sin cgo).
- [`hajimehoshi/go-mp3`](https://github.com/hajimehoshi/go-mp3) — decodificación MP3.
- [`madelynnblue/go-dsp/fft`](https://github.com/madelynnblue/go-dsp) — FFT para el espectro.
- [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) — modo raw y tamaño de terminal.

## Licencia

Apache License 2.0 — ver [LICENSE](LICENSE).
