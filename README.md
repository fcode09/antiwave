# antiwave

Visualizador de audio en la terminal, escrito en Go. Captura el audio del
**sistema** (Spotify, YouTube, navegador…) o reproduce un archivo **WAV/MP3**, y
lo dibuja en vivo con **múltiples modos de visualización** y **temas de color**.
Todo con caracteres de terminal (box-drawing + bloques), sin emojis.

```
┌ antiwave · [>] loopback (salida del sistema) · 48000Hz · CASCADA · fase:+ · 0:12/∞ ──┐
█▓▒░ ░▒▓█ ▓▒░ ░▓█ ▒░█ ▓█                                                               │
▒░ ░▓▒░ ▓█░ ░▒░ ▓░ ▒█                                                               │
▓▒░ ▓█ ░▒░ ░▓ ▒░ █░▓▒                                                                │
░▓█ ░▒▓ ▒█░ ▒░ ▓█ ░░                                                               │
└─ [Tab]modo [t]tema:termal [m]mono [o]fase [espacio]pausa [q]salir · pico:-6dB dom:523Hz ─┘
```

## Uso

```bash
antiwave                 # captura el audio del sistema (lo que esté sonando)
antiwave cancion.mp3     # reproduce y visualiza un archivo
antiwave -silent cancion.wav   # archivo sin reproducir (avanza en tiempo real)
```

Binarios estáticos (sin dependencias) para Windows y Linux:

```bash
make release
# → dist/antiwave.exe  (windows/amd64)
# → dist/antiwave      (linux/amd64)
```

## Modos de visualización

Pulsa `Tab` para ciclar:

| Modo | Descripción |
|---|---|
| **ONDAS** | Osciloscopio scrolleante con envolvente de pico |
| **BARRAS** | Espectro de frecuencias (FFT) con tapas de pico |
| **CASCADA** | Espectrograma (frecuencia × tiempo) en escala de intensidad |
| **VU** | Medidores de nivel por bandas, con pico sostenido |
| **X-Y** | Alcance de fase (Lissajous): correlación de la señal |

## Controles

| Tecla | Acción |
|---|---|
| `Tab` / `n` | cambiar modo |
| `t` | cambiar tema de color |
| `m` | monocromo on/off |
| `o` | fase opuesta (espejo visual) |
| `espacio` | pausar / reanudar |
| `q` / `Ctrl+C` | salir |

## Temas

`mono` (por defecto), `termal`, `aurora`, `matriz`, `oceano`.

## Cómo funciona

- **Linux**: captura el monitor de PulseAudio/PipeWire (`jfreymuth/pulse`, puro Go).
- **Windows**: captura WASAPI en modo loopback (`moutend/go-wca`, puro Go).
- **Sin cgo**: el binario es estático y compila para ambos SO desde una máquina.

> Nota: en captura, la "fase opuesta" (`o`) es solo un **espejo visual**; la fase
> invertida no cambia lo que se oye (el oído es insensible a la fase absoluta).

## Estructura

```
cmd/
  antiwave/        # binario principal
  genwav/          # genera un WAV demo
internal/
  audio/           # decodificación WAV/MP3, reproducción (oto) y captura por SO
  visual/          # render: 5 modos, temas, HUD
```

## Requisitos

- Go 1.21+ (probado con 1.27).
- Linux: PulseAudio o PipeWire (con `pipewire-pulse`).
- Windows: 10/11 (WASAPI; sin "Stereo Mix" necesario, usa loopback).

## Licencia

Apache License 2.0 — ver [LICENSE](LICENSE).
