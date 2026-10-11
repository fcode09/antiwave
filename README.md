# antiwave

A modern terminal audio visualizer written in Go. It captures the **system
audio** (Spotify, browser, whatever is playing) or plays a **WAV/MP3** file, and
renders it live with **eight visualization modes**, **truecolor gradient
themes**, automatic gain, and smoothing — using terminal characters (blocks,
braille, box-drawing), no emojis.

```
 antiwave [LIVE] System output SPECTRUM phase:+ 0:12/∞        aurora · 48000Hz

                          ▁▂▃▄▅▆▇█

 [Tab] mode   [t] theme   [f] focus   [a] auto   [+/-] gain   [q] quit
```

## Usage

```bash
antiwave                 # capture the system audio (whatever is playing)
antiwave song.mp3        # play and visualize a file
antiwave -silent song.wav   # file without playback (advances in real time)

antiwave --immersive              # full-screen visualization (no HUD)
antiwave --mode waterfall --theme ocean
```

Static, dependency-free binaries for Windows and Linux:

```bash
make release
# → dist/antiwave.exe  (windows/amd64)
# → dist/antiwave      (linux/amd64)
```

## Options

| Flag | Description |
|---|---|
| `--silent` | file mode without playback |
| `--immersive` | start full-screen (visualization only) |
| `--mode` | `spectrum` (default) · `mirror` · `waveform` · `oscilloscope` · `waterfall` · `phase` · `radial` · `levels` · `halo` · `orbit` · `nebula` |
| `--theme` | `aurora` (default) · `ocean` · `ember` · `phosphor` · `ion` · `ghost` · `amethyst` · `mono` |
| `--mono` | start in monochrome |
| `--fps` | target frame rate (30 or 60, default 60) |

## Visualization modes

| Mode | Description |
|---|---|
| **SPECTRUM** | Thin gradient bars, log frequency scale, peak caps |
| **MIRROR** | Spectrum growing up and down from the center |
| **WAVEFORM** | Scrolling amplitude envelope |
| **OSCILLOSCOPE** | Thin braille trace, phase-triggered for stability |
| **WATERFALL** | Spectrogram (frequency × time) |
| **PHASE** | X-Y Lissajous figure |
| **RADIAL** | Circular spectrum |
| **LEVELS** | Per-band RMS meters with peak hold |
| **HALO** | Thin reactive energy ring with a fading trail |
| **ORBIT** | Tilted ellipses with bright satellites tracing each path |
| **NEBULA** | Particle swarm pushed outward by bass |

## Controls

| Key | Action |
|---|---|
| `Tab` / `→` | next mode |
| `Shift+Tab` / `←` | previous mode |
| `1`–`8` | jump to a mode |
| `9` | Halo mode |
| `0` | Orbit mode |
| `t` | cycle theme |
| `m` | toggle monochrome |
| `f` | toggle immersive (full-screen) |
| `Esc` | exit immersive |
| `a` | toggle automatic gain |
| `e` | cycle effect level (clean / balanced / cinematic) |
| `+` / `-` | visual gain up / down |
| `o` | inverted phase (visual mirror) |
| `space` | pause / freeze |
| `q` / `Ctrl+C` | quit |

## How it works

- **Linux**: captures the PulseAudio/PipeWire monitor (`jfreymuth/pulse`, pure Go).
- **Windows**: captures WASAPI in loopback mode (`moutend/go-wca`, pure Go).
- **Rendering**: `tcell` with truecolor (with automatic 256-color fallback).
- **DSP**: Hann-windowed FFT, logarithmic bands, dB scaling, attack/release
  smoothing, peak hold, and optional automatic gain.

> Note: in capture mode, "inverted phase" (`o`) is only a **visual mirror**;
> phase inversion does not change what you hear (the ear is insensitive to
> absolute phase).

## Layout

```
cmd/
  antiwave/        # main binary
  genwav/          # generates a demo WAV
internal/
  audio/           # WAV/MP3 decoding, playback (oto), and per-OS capture
  visual/          # rendering: 8 modes, themes, smoothing, HUD
```

## Requirements

- Go 1.21+ (tested with 1.27).
- Linux: PulseAudio or PipeWire (with `pipewire-pulse`).
- Windows: 10/11 (WASAPI; no "Stereo Mix" needed, uses loopback).

## License

Apache License 2.0 — see [LICENSE](LICENSE).
