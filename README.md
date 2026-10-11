# antiwave

A terminal audio visualizer written in Go. It captures the **system audio**
(Spotify, browser, whatever is playing) or plays a **WAV/MP3** file, and renders
it live with **multiple visualization modes** and **color themes** — all using
terminal characters (box-drawing + blocks), no emojis.

```
┌ antiwave · [>] loopback (system output) · 48000Hz · WATERFALL · phase:+ · 0:12/∞ ─┐
█▓▒░ ░▒▓█ ▓▒░ ░▓█ ▒░█ ▓█                                                                
▒░ ░▓▒░ ▓█░ ░▒░ ▓░ ▒█                                                                
└─ [Tab]mode [t]theme:termal [m]mono [o]phase [space]pause [q]quit · peak:-6dB dom:523Hz ─┘
```

## Usage

```bash
antiwave                 # capture the system audio (whatever is playing)
antiwave song.mp3        # play and visualize a file
antiwave -silent song.wav   # file without playback (advances in real time)
```

Static, dependency-free binaries for Windows and Linux:

```bash
make release
# → dist/antiwave.exe  (windows/amd64)
# → dist/antiwave      (linux/amd64)
```

## Visualization modes

Press `Tab` to cycle:

| Mode | Description |
|---|---|
| **WAVE** | Scrolling oscilloscope with peak envelope |
| **BARS** | Frequency spectrum (FFT) with peak caps |
| **WATERFALL** | Spectrogram (frequency × time) in intensity scale |
| **VU** | Per-band level meters with peak hold |
| **X-Y** | Phase scope (Lissajous): signal correlation |

## Controls

| Key | Action |
|---|---|
| `Tab` / `n` | cycle mode |
| `t` | cycle color theme |
| `m` | toggle monochrome |
| `o` | inverted phase (visual mirror) |
| `space` | pause / resume |
| `q` / `Ctrl+C` | quit |

## Themes

`mono` (default), `termal`, `aurora`, `matriz`, `oceano`.

## How it works

- **Linux**: captures the PulseAudio/PipeWire monitor (`jfreymuth/pulse`, pure Go).
- **Windows**: captures WASAPI in loopback mode (`moutend/go-wca`, pure Go).
- **No cgo**: the binary is static and cross-compiles for both OSes from one machine.

> Note: in capture mode, the "inverted phase" (`o`) is only a **visual mirror**;
> phase inversion does not change what you hear (the ear is insensitive to
> absolute phase).

## Layout

```
cmd/
  antiwave/        # main binary
  genwav/          # generates a demo WAV
internal/
  audio/           # WAV/MP3 decoding, playback (oto), and per-OS capture
  visual/          # rendering: 5 modes, themes, HUD
```

## Requirements

- Go 1.21+ (tested with 1.27).
- Linux: PulseAudio or PipeWire (with `pipewire-pulse`).
- Windows: 10/11 (WASAPI; no "Stereo Mix" needed, uses loopback).

## License

Apache License 2.0 — see [LICENSE](LICENSE).
