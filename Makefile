.PHONY: build run gen demo release tidy vet test clean

# Build the binary (for the current OS) as ./antiwave
build:
	go build -o antiwave ./cmd/antiwave

# Build and run with an audio file
# Usage: make run AUDIO=path/to/file.wav
run: build
	./antiwave $(AUDIO)

# Capture the system audio and visualize it
capture: build
	./antiwave

# Generate a demo WAV (note arpeggio) at samples/demo.wav
gen:
	go run ./cmd/genwav -o samples/demo.wav

# Generate the demo and play it
demo: gen
	go run ./cmd/antiwave samples/demo.wav

# Build static binaries for Windows and Linux under dist/
release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/antiwave.exe ./cmd/antiwave
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/antiwave     ./cmd/antiwave

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf antiwave dist samples/demo.wav
