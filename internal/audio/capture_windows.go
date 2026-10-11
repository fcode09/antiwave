//go:build windows

package audio

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

const (
	waveFormatPCM        = 0x0001
	waveFormatIEEEFloat  = 0x0003
	waveFormatExtensible = 0xFFFE
	ksDataFormatIEEE     = 0x00000003 // primer uint32 del SubFormat GUID KSDATAFORMAT_SUBTYPE_IEEE_FLOAT
)

func newSystemCapture(push func([]float32)) (*captureHandle, error) {
	// Inicializa COM en MTA (proceso). Ignora el error: puede estar ya inicializado.
	_ = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return nil, fmt.Errorf("enumerador de dispositivos: %w", err)
	}

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EMultimedia, &mmd); err != nil {
		mmde.Release()
		return nil, fmt.Errorf("endpoint de render: %w", err)
	}

	var ac *wca.IAudioClient
	if err := mmd.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &ac); err != nil {
		mmd.Release()
		mmde.Release()
		return nil, fmt.Errorf("activar IAudioClient: %w", err)
	}

	var mix *wca.WAVEFORMATEX
	if err := ac.GetMixFormat(&mix); err != nil {
		ac.Release()
		mmd.Release()
		mmde.Release()
		return nil, fmt.Errorf("formato de mezcla: %w", err)
	}

	sampleRate, channels, isFloat, bytesPerSample := parseMixFormat(mix)

	if err := ac.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, wca.AUDCLNT_STREAMFLAGS_LOOPBACK, 0, 0, mix, nil); err != nil {
		ole.CoTaskMemFree(uintptr(unsafe.Pointer(mix)))
		ac.Release()
		mmd.Release()
		mmde.Release()
		return nil, fmt.Errorf("inicializar loopback: %w", err)
	}
	ole.CoTaskMemFree(uintptr(unsafe.Pointer(mix)))

	var acc *wca.IAudioCaptureClient
	if err := ac.GetService(wca.IID_IAudioCaptureClient, &acc); err != nil {
		ac.Release()
		mmd.Release()
		mmde.Release()
		return nil, fmt.Errorf("servicio de captura: %w", err)
	}

	if err := ac.Start(); err != nil {
		acc.Release()
		ac.Release()
		mmd.Release()
		mmde.Release()
		return nil, fmt.Errorf("iniciar captura: %w", err)
	}

	var running atomic.Bool
	running.Store(true)

	go func() {
		for running.Load() {
			var packetSize uint32
			if err := acc.GetNextPacketSize(&packetSize); err != nil {
				break
			}
			if packetSize == 0 {
				time.Sleep(time.Millisecond)
				continue
			}
			for running.Load() {
				var data *byte
				var frames, flags uint32
				var devPos, qpcPos uint64
				if err := acc.GetBuffer(&data, &frames, &flags, &devPos, &qpcPos); err != nil {
					break
				}
				if data != nil && frames > 0 {
					push(convertSamples(data, int(frames), channels, isFloat, bytesPerSample))
				}
				if err := acc.ReleaseBuffer(frames); err != nil {
					break
				}
				if frames == 0 {
					break
				}
			}
		}
	}()

	stop := func() {
		running.Store(false)
		ac.Stop()
		acc.Release()
		ac.Release()
		mmd.Release()
		mmde.Release()
		ole.CoUninitialize()
	}

	return &captureHandle{
		sampleRate: sampleRate,
		name:       "loopback (salida del sistema)",
		stop:       stop,
	}, nil
}

// parseMixFormat extrae sampleRate, canales y formato de muestras del
// WAVEFORMATEX (o WAVEFORMATEXTENSIBLE) de mezcla.
func parseMixFormat(mix *wca.WAVEFORMATEX) (sampleRate, channels int, isFloat bool, bytesPerSample int) {
	sampleRate = int(mix.NSamplesPerSec)
	channels = int(mix.NChannels)
	bytesPerSample = int(mix.WBitsPerSample) / 8

	switch mix.WFormatTag {
	case waveFormatIEEEFloat:
		isFloat = true
	case waveFormatExtensible:
		raw := unsafe.Slice((*byte)(unsafe.Pointer(mix)), 40)
		sub := binary.LittleEndian.Uint32(raw[24:28])
		isFloat = sub == ksDataFormatIEEE
	}
	return
}

// convertSamples convierte un bloque crudo de PCM intercalado a mono float32.
func convertSamples(data *byte, frames, channels int, isFloat bool, bytesPerSample int) []float32 {
	total := frames * channels
	raw := unsafe.Slice(data, total*bytesPerSample)
	out := make([]float32, 0, frames)

	for f := 0; f < frames; f++ {
		var sum float64
		for ch := 0; ch < channels; ch++ {
			off := (f*channels + ch) * bytesPerSample
			var v float64
			switch {
			case isFloat && bytesPerSample == 4:
				v = float64(*(*float32)(unsafe.Pointer(&raw[off])))
			case isFloat && bytesPerSample == 8:
				v = *(*float64)(unsafe.Pointer(&raw[off]))
			case bytesPerSample == 2:
				v = float64(int16(binary.LittleEndian.Uint16(raw[off:off+2]))) / 32768.0
			case bytesPerSample == 4:
				v = float64(int32(binary.LittleEndian.Uint32(raw[off:off+4]))) / 2147483648.0
			default:
				v = 0
			}
			sum += v
		}
		out = append(out, float32(sum/float64(channels)))
	}
	return out
}
