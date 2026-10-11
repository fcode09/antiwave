// genwav generates a demo WAV (mono, 16-bit PCM) with an arpeggio of several
// notes, so antiwave can be tested without an external audio file.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
)

func main() {
	out := flag.String("o", "demo.wav", "output file")
	sr := flag.Int("sr", 44100, "sample rate")
	dur := flag.Float64("dur", 6.0, "duration in seconds")
	flag.Parse()

	n := int(float64(*sr) * *dur)

	// Melody: ascending and descending arpeggio (C major).
	notes := []float64{
		261.63, 329.63, 392.00, 523.25, // C4 E4 G4 C5
		659.25, 523.25, 392.00, 329.63, // E5 C5 G4 E4
		261.63, 329.63, 392.00, 523.25,
	}
	noteDur := float64(*dur) / float64(len(notes))

	samples := make([]int16, n)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(*sr)
		idx := int(t / noteDur)
		if idx >= len(notes) {
			idx = len(notes) - 1
		}
		f := notes[idx]
		lt := t - float64(idx)*noteDur // local time within the note

		// Envelope: fast attack + exponential decay.
		env := math.Min(1, lt/0.02) * math.Exp(-3*lt)

		v := env * (0.6*math.Sin(2*math.Pi*f*t) +
			0.3*math.Sin(2*math.Pi*2*f*t) +
			0.2*math.Sin(2*math.Pi*(f/2)*t))
		samples[i] = int16(v * 32767 * 0.7)
	}

	if err := writeWav(*out, *sr, samples); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d samples, %.1fs, %d Hz)\n", *out, n, *dur, *sr)
}

func writeWav(path string, sampleRate int, samples []int16) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	const (
		channels = 1
		bits     = 16
	)
	byteRate := sampleRate * channels * bits / 8
	blockAlign := channels * bits / 8
	dataSize := len(samples) * blockAlign

	write := func(v any) error { return binary.Write(f, binary.LittleEndian, v) }

	// RIFF header.
	f.WriteString("RIFF")
	write(uint32(36 + dataSize))
	f.WriteString("WAVE")

	// fmt chunk.
	f.WriteString("fmt ")
	write(uint32(16))
	write(uint16(1)) // PCM
	write(uint16(channels))
	write(uint32(sampleRate))
	write(uint32(byteRate))
	write(uint16(blockAlign))
	write(uint16(bits))

	// data chunk.
	f.WriteString("data")
	write(uint32(dataSize))
	return write(samples)
}
